package ooo_api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go-ooo/config"
	"go-ooo/database"
	"go-ooo/logger"
	"go-ooo/ooo_api/dex"
	"go-ooo/ooo_api/dex/modules/bsc_pancakeswap_v3"
	"go-ooo/ooo_api/dex/modules/eth_shibaswap"
	"go-ooo/ooo_api/dex/modules/eth_sushiswap"
	"go-ooo/ooo_api/dex/modules/eth_uniswap_v2"
	"go-ooo/ooo_api/dex/modules/eth_uniswap_v3"
	"go-ooo/ooo_api/dex/modules/polygon_pos_quickswap_v3"
	"go-ooo/ooo_api/dex/modules/xdai_honeyswap"
)

type OOOApi struct {
	baseURL          string
	client           *http.Client
	db               *database.DB
	ctx              context.Context
	dexModuleManager *dex.Manager
}

func NewApi(ctx context.Context, cfg *config.Config, db *database.DB) (*OOOApi, error) {

	dexModuleManager := dex.NewDexManager(
		ctx, cfg, db,
		eth_shibaswap.NewDexModule(ctx, cfg),
		eth_sushiswap.NewDexModule(ctx, cfg),
		eth_uniswap_v2.NewDexModule(ctx, cfg),
		eth_uniswap_v3.NewDexModule(ctx, cfg),
		polygon_pos_quickswap_v3.NewDexModule(ctx, cfg),
		bsc_pancakeswap_v3.NewDexModule(ctx, cfg),
		xdai_honeyswap.NewDexModule(ctx, cfg),
	)

	return &OOOApi{
		baseURL: cfg.Jobs.OooApiUrl,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		db:               db,
		ctx:              ctx,
		dexModuleManager: dexModuleManager,
	}, nil
}

func (o *OOOApi) UpdateDexPairs() {
	o.dexModuleManager.GetSupportedPairs()
	o.dexModuleManager.UpdateAllPairsMetaDataFromDexs()
}

func (o *OOOApi) RouteQuery(endpoint string, requestId string) (string, error) {
	isAdHoc, err := IsAdhoc(endpoint)

	if err != nil {
		return "", err
	}

	logger.Debug("ooo_api", "RouteQuery", "route", "", logger.Fields{
		"request_id": requestId,
		"is_adhoc":   isAdHoc,
	})

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
