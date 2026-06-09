package service

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go-ooo/logger"
	go_ooo_types "go-ooo/types"
	"net/http"
)

func (s *Service) initEcho() {
	logger.Info("service", "initEcho", "", "initialise echo")

	s.echoService.Use(middleware.Recover())
	s.echoService.Use(middleware.KeyAuth(func(key string, c echo.Context) (bool, error) {
		// Reject empty credentials outright (a blank authToken must never accept a
		// blank key), and compare in constant time to avoid leaking the token via
		// response timing. NB: the admin bearer is decoupled from the keystore key
		// and moved to a bcrypt-hashed secret as part of the keystore migration (#106);
		// this is the interim hardening of the existing == compare.
		if s.authToken == "" || key == "" {
			return false, nil
		}
		return subtle.ConstantTimeCompare([]byte(key), []byte(s.authToken)) == 1, nil
	}))

	s.echoService.POST("/admin", s.AddAdminTask)
	s.echoService.POST("/analytics", s.AddAnalyticsTask)

	s.echoService.Logger.Fatal(s.echoService.Start(fmt.Sprintf("%s:%s", s.cfg.Serve.Host, s.cfg.Serve.Port)))
}

func (s *Service) AddAdminTask(c echo.Context) error {
	var request go_ooo_types.AdminTask

	json.NewDecoder(c.Request().Body).Decode(&request)

	logger.InfoWithFields("service", "AddAdminTask", "", "admin task received", logger.Fields{
		"task":           request.Task,
		"fee_or_amount":  request.FeeOrAmount,
		"to_or_consumer": request.ToOrConsumer,
	})

	// send received task to chanel for processing
	s.adminTasks <- request

	// listen for result and send HTTP response back
	for {
		select {
		case tr := <-s.adminTasksResp:
			if tr.Success {
				return c.JSON(http.StatusOK, tr)
			}
			return c.JSON(http.StatusInternalServerError, tr.Error)
		}
	}

}

func (s *Service) AddAnalyticsTask(c echo.Context) error {
	var request go_ooo_types.AnalyticsTask
	json.NewDecoder(c.Request().Body).Decode(&request)

	logger.Info("service", "AddAnalyticsTask", "", "analytics task received")

	// send received task to chanel for processing
	s.analyticsTasks <- request

	// listen for result and send HTTP response back
	for {
		select {
		case tr := <-s.analyticsTasksResp:
			if tr.Success {
				return c.JSON(http.StatusOK, tr)
			}
			return c.JSON(http.StatusInternalServerError, tr.Error)
		}
	}
}
