package ooo_api

import (
	"context"
	"errors"
	"fmt"
	"go-ooo/ooo_api/dex"
	"go-ooo/ooo_api/dex/modules/bsc_pancakeswapv2"
	"go-ooo/ooo_api/dex/modules/eth_shibaswap"
	"go-ooo/ooo_api/dex/modules/eth_sushiswap"
	"go-ooo/ooo_api/dex/modules/eth_uniswapv2"
	"go-ooo/ooo_api/dex/modules/eth_uniswapv3"
	"go-ooo/ooo_api/dex/modules/polygon_quickswap"
	"go-ooo/ooo_api/dex/modules/xdai_honeyswap"
	"net/http"
	"strings"
	"time"

	"go-ooo/config"
	"go-ooo/database"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type OOOApi struct {
	baseURL          string
	client           *http.Client
	db               *database.DB
	logger           *logrus.Logger
	ctx              context.Context
	dexModuleManager *dex.Manager
}

func NewApi(ctx context.Context, db *database.DB, logger *logrus.Logger) (*OOOApi, error) {

	dexModuleManager := dex.NewDexManager(
		ctx, logger, db,
		eth_shibaswap.NewDexModule(ctx),
		eth_sushiswap.NewDexModule(ctx),
		eth_uniswapv2.NewDexModule(ctx),
		eth_uniswapv3.NewDexModule(ctx),
		polygon_quickswap.NewDexModule(ctx),
		bsc_pancakeswapv2.NewDexModule(ctx),
		xdai_honeyswap.NewDexModule(ctx),
	)

	return &OOOApi{
		baseURL: viper.GetString(config.JobsOooApiUrl),
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		db:               db,
		logger:           logger,
		ctx:              ctx,
		dexModuleManager: dexModuleManager,
	}, nil
}

func (o *OOOApi) UpdateDexPairs() {
	o.dexModuleManager.UpdateAllPairsAndTokens()
}

func (o *OOOApi) RouteQuery(endpoint string, requestId string) (string, error) {
	isAdHoc, err := IsAdhoc(endpoint)

	if err != nil {
		return "", err
	}

	o.logger.WithFields(logrus.Fields{
		"package":    "ooo_api",
		"function":   "RouteQuery",
		"action":     "route",
		"request_id": requestId,
		"is_adhoc":   isAdHoc,
	}).Debug()

	if isAdHoc {
		return o.QueryAdhoc(endpoint, requestId)
	} else {
		return o.QueryFinchainsEndpoint(endpoint, requestId)
	}
}

func IsAdhoc(endpoint string) (bool, error) {
	_, _, qType, _, _, _, _, err := ParseEndpoint(endpoint)

	if err != nil {
		return false, err
	}

	if qType == "AD" {
		return true, nil
	}

	return false, nil
}

func ParseEndpoint(endpoint string) (base string, target string, qType string,
	subtype string, supp1 string, supp2 string, supp3 string, err error) {
	ep := strings.Split(endpoint, ".")

	if len(ep) < 3 {
		err = errors.New(fmt.Sprintf("incorrect endpoint format: %s", endpoint))
		return
	}

	base = ep[0]   // BTC etc
	target = ep[1] // GBP etc
	qType = ep[2]  // PRC, EXC, DCS,

	if len(ep) > 3 {
		subtype = ep[3] // AVG, LAT etc.
	}
	if len(ep) > 4 {
		supp1 = ep[4]
	}
	if len(ep) > 5 {
		supp2 = ep[5]
	}
	if len(ep) > 6 {
		supp3 = ep[6]
	}

	return
}
