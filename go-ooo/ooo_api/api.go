package ooo_api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
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

// Endpoint qualifiers (the 3rd, "qType" field). Both are legacy: AdHoc is the
// going-forward path, and these are slated for removal once live usage drops to zero
// (queries are one-shot, so removal is safe - see QUERY_FORMAT_REVIEW).
const (
	QTypeAdHoc     = "AD" // AdHoc DEX query
	QTypeFinchains = "PR" // Finchains price query (removed with Finchains)
)

// ParsedEndpoint is the structured form of an OoO endpoint string
// (base.target.qType[.subtype.supp1.supp2.supp3]).
type ParsedEndpoint struct {
	Base   string
	Target string
	QType  string
	// AdHoc: the lookback window in minutes (clamped to 0-60), taken from the 4th field.
	Minutes int64
	// Finchains-only positional fields (legacy; removed with Finchains).
	Subtype string
	Supp1   string
	Supp2   string
	Supp3   string
}

// IsAdHoc reports whether the endpoint routes to the AdHoc DEX path.
func (p ParsedEndpoint) IsAdHoc() bool {
	return p.QType == QTypeAdHoc
}

func (o *OOOApi) RouteQuery(endpoint string, requestId string) (string, error) {
	parsed, err := ParseEndpoint(endpoint)
	if err != nil {
		return "", err
	}

	logger.Debug("ooo_api", "RouteQuery", "route", "", logger.Fields{
		"request_id": requestId,
		"is_adhoc":   parsed.IsAdHoc(),
	})

	if parsed.IsAdHoc() {
		return o.QueryAdhoc(parsed, requestId)
	}
	return o.QueryFinchainsEndpoint(endpoint, requestId)
}

// IsAdhoc parses endpoint and reports whether it is an AdHoc DEX query. Used at request
// detection time (chain) to set the job's adhoc flag.
func IsAdhoc(endpoint string) (bool, error) {
	parsed, err := ParseEndpoint(endpoint)
	if err != nil {
		return false, err
	}
	return parsed.IsAdHoc(), nil
}

// ParseEndpoint splits an endpoint string into its fields. The grammar is positional:
// base.target.qType[.subtype.supp1.supp2.supp3]. For AdHoc, the 4th field is the
// lookback window in minutes; for Finchains it is the subtype.
func ParseEndpoint(endpoint string) (ParsedEndpoint, error) {
	ep := strings.Split(endpoint, ".")
	if len(ep) < 3 {
		return ParsedEndpoint{}, fmt.Errorf("incorrect endpoint format %q: expected at least base.target.qType", endpoint)
	}

	p := ParsedEndpoint{Base: ep[0], Target: ep[1], QType: ep[2]}
	if p.Base == "" || p.Target == "" || p.QType == "" {
		return ParsedEndpoint{}, fmt.Errorf("incorrect endpoint format %q: empty base, target or qType", endpoint)
	}

	if len(ep) > 3 {
		p.Subtype = ep[3]
		if p.IsAdHoc() {
			p.Minutes = clampMinutes(ep[3])
		}
	}
	if len(ep) > 4 {
		p.Supp1 = ep[4]
	}
	if len(ep) > 5 {
		p.Supp2 = ep[5]
	}
	if len(ep) > 6 {
		p.Supp3 = ep[6]
	}

	return p, nil
}

// clampMinutes parses the AdHoc lookback window, clamped to 0-60. A non-numeric or
// missing value is treated as 0 (latest).
func clampMinutes(s string) int64 {
	m, _ := strconv.ParseInt(s, 10, 64)
	if m < 0 {
		return 0
	}
	if m > 60 {
		return 60
	}
	return m
}
