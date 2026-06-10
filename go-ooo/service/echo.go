package service

import (
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go-ooo/auth"
	"go-ooo/logger"
	go_ooo_types "go-ooo/types"
	"net/http"
)

func (s *Service) initEcho() {
	logger.Info("service", "initEcho", "", "initialise echo")

	s.echoService.Use(middleware.Recover())
	s.echoService.Use(middleware.KeyAuth(func(key string, c echo.Context) (bool, error) {
		// The admin bearer is decoupled from the keystore key: only its bcrypt hash
		// is held (s.adminTokenHash), generated at init/migration from crypto/rand.
		// An empty hash (no admin token configured) or empty key never authenticates.
		return auth.VerifyAdminToken(s.adminTokenHash, key), nil
	}))

	s.echoService.POST("/admin", s.AddAdminTask)
	s.echoService.POST("/analytics", s.AddAnalyticsTask)

	addr := fmt.Sprintf("%s:%s", s.cfg.Serve.Host, s.cfg.Serve.Port)
	// A genuine bind/serve failure is fatal; a graceful Shutdown returns
	// http.ErrServerClosed, which is expected and must not exit the process.
	if err := s.echoService.Start(addr); err != nil && err != http.ErrServerClosed {
		s.echoService.Logger.Fatal(err)
	}
}

func (s *Service) AddAdminTask(c echo.Context) error {
	var request go_ooo_types.AdminTask

	if err := json.NewDecoder(c.Request().Body).Decode(&request); err != nil {
		logger.Error("service", "AddAdminTask", "decode request body", err.Error())
		return c.JSON(http.StatusBadRequest, "invalid request body")
	}

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
	if err := json.NewDecoder(c.Request().Body).Decode(&request); err != nil {
		logger.Error("service", "AddAnalyticsTask", "decode request body", err.Error())
		return c.JSON(http.StatusBadRequest, "invalid request body")
	}

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
