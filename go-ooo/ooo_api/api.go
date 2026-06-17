package ooo_api

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"go-ooo/config"
	"go-ooo/database"
	"go-ooo/logger"
	"go-ooo/ooo_api/dex"
	"go-ooo/ooo_api/export"
)

type OOOApi struct {
	db               *database.DB
	ctx              context.Context
	dexModuleManager *dex.Manager
	quality          config.PriceQualityConfig
}

func NewApi(ctx context.Context, cfg *config.Config, db *database.DB) (*OOOApi, error) {

	dexModuleManager := dex.NewDexManager(ctx, cfg, db)

	// Restore the last-synced source catalogue from the database so the oracle can price from it
	// immediately, before (or even without) a fresh API sync. A fresh database simply restores an
	// empty set, and the first SyncDexSources populates it.
	if m, err := dexModuleManager.LoadManifestFromDB(); err != nil {
		logger.ErrorWithFields("ooo_api", "NewApi", "restore source catalogue", err.Error(), logger.Fields{})
	} else {
		dexModuleManager.ApplyManifest(m)
	}

	return &OOOApi{
		db:               db,
		ctx:              ctx,
		dexModuleManager: dexModuleManager,
		quality:          cfg.PriceQuality,
	}, nil
}

// ErrFinchainsRemoved is returned when a legacy Finchains-shape (.PR) request is redirected to the
// DEX path but the pair is not available on any supported DEX. Finchains itself is gone.
var ErrFinchainsRemoved = errors.New("Finchains endpoints removed; pair is not available on any supported DEX")

// UpdateDexPairs refreshes the DEX source catalogue + pair set from the dex-pair-verify manifest and
// per-source feeds, then refreshes live pair metadata. Driven by the startup + periodic pair refresh.
func (o *OOOApi) UpdateDexPairs() {
	o.dexModuleManager.SyncDexSources()
}

// SetExportAuthenticator installs the authenticator used for dex-pair-verify export pulls (T8). The
// service calls this at startup to switch from the static-token default to provider wallet-auth.
func (o *OOOApi) SetExportAuthenticator(a export.Authenticator) {
	o.dexModuleManager.SetExportAuth(a)
}

// Endpoint qualifiers - the explicit 3rd field in the legacy grammar
// (base.target.qType...). Both are deprecated: the going-forward form is the suffix-less
// base.target[.minutes] (always AdHoc), and these are slated for removal once live usage
// drops to zero (queries are one-shot, so removal is safe - see QUERY_FORMAT_REVIEW).
const (
	QTypeAdHoc     = "AD" // AdHoc DEX query
	QTypeFinchains = "PR" // legacy Finchains price query - still recognised so it routes to the redirect
)

// ParsedEndpoint is the structured form of an OoO endpoint string. Two positional grammars
// are accepted:
//
//   - canonical (going-forward): base.target[.minutes] - always an AdHoc DEX query.
//   - legacy (deprecated):       base.target.qType[.subtype.supp1.supp2.supp3] - where
//     qType is AD (AdHoc) or PR (Finchains). Slated for removal once usage drops to zero.
type ParsedEndpoint struct {
	Base   string
	Target string
	QType  string
	// Minutes is the AdHoc lookback window (clamped to 0-60), taken from the minutes field
	// (the 3rd field in the canonical form, the 4th in the legacy .AD form).
	Minutes int64
	// Legacy is true when the endpoint used the deprecated explicit-qualifier form
	// (base.target.AD / base.target.PR...) rather than the canonical suffix-less form.
	Legacy bool
	// IgnoredFields holds any trailing fields an AdHoc query cannot honour (e.g. leftover
	// Finchains subtype/exchange/window params). Per the silent-drop policy the request is
	// still served; these are surfaced in logs + a metric.
	IgnoredFields []string
	// Finchains-only positional fields (legacy grammar; parsed but discarded by the redirect).
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

	// RouteQuery is the single per-request routing choke point, so deprecation + silent-drop
	// telemetry is recorded here (not in ParseEndpoint, which is also called at detection time).
	observeEndpoint(parsed)

	logger.Debug("ooo_api", "RouteQuery", "route", "", logger.Fields{
		"request_id": requestId,
		"is_adhoc":   parsed.IsAdHoc(),
		"legacy":     parsed.Legacy,
	})

	if parsed.IsAdHoc() {
		return o.QueryDexPrice(parsed, requestId)
	}
	// Finchains is gone: lossily redirect a legacy .PR request to the DEX path on base/target.
	return o.redirectFinchainsToAdHoc(parsed, requestId)
}

// redirectFinchainsToAdHoc serves a legacy Finchains-shape request from the DEX mean, discarding
// the Finchains-only qualifiers (time window, aggregation strategy, per-exchange filter). If
// base/target is on no supported DEX it returns ErrFinchainsRemoved. Every redirect is logged so
// post-deploy analytics can show how much Finchains-shape traffic is still arriving.
func (o *OOOApi) redirectFinchainsToAdHoc(parsed ParsedEndpoint, requestId string) (string, error) {
	logger.InfoWithFields("ooo_api", "redirectFinchainsToAdHoc", "redirect",
		"Finchains endpoint redirected to the AdHoc DEX path (Finchains qualifiers discarded)",
		logger.Fields{
			"request_id": requestId,
			"base":       parsed.Base,
			"target":     parsed.Target,
			"redirected": true,
		})

	// Synthesise the canonical AdHoc form (base.target, default window); drop subtype/supp/window.
	adhoc := ParsedEndpoint{Base: parsed.Base, Target: parsed.Target, QType: QTypeAdHoc}
	price, err := o.QueryDexPrice(adhoc, requestId)
	if err != nil {
		logger.WarnWithFields("ooo_api", "redirectFinchainsToAdHoc", "",
			"redirect failed - pair not available on any supported DEX", logger.Fields{
				"request_id": requestId,
				"base":       parsed.Base,
				"target":     parsed.Target,
				"error":      err.Error(),
			})
		return "", ErrFinchainsRemoved
	}
	return price, nil
}

// ParseEndpoint splits an endpoint string into its fields. Two positional grammars are
// accepted, disambiguated by the third field:
//
//   - canonical: base.target[.minutes]           (suffix-less; always AdHoc)
//   - legacy:    base.target.qType[.subtype...]   (qType AD or PR; deprecated)
//
// A third field equal to a known qualifier (AD/PR) selects the legacy grammar; anything
// else (or no third field) is the canonical AdHoc form with an optional minutes window.
func ParseEndpoint(endpoint string) (ParsedEndpoint, error) {
	ep := strings.Split(endpoint, ".")
	if len(ep) < 2 {
		return ParsedEndpoint{}, fmt.Errorf("incorrect endpoint format %q: expected at least base.target", endpoint)
	}

	p := ParsedEndpoint{Base: ep[0], Target: ep[1]}
	if p.Base == "" || p.Target == "" {
		return ParsedEndpoint{}, fmt.Errorf("incorrect endpoint format %q: empty base or target", endpoint)
	}

	// Legacy grammar: an explicit qualifier in the third field.
	if len(ep) >= 3 && isQualifier(ep[2]) {
		p.Legacy = true
		p.QType = ep[2]
		if p.IsAdHoc() {
			// base.target.AD[.minutes]
			if len(ep) > 3 {
				p.Minutes, p.IgnoredFields = parseAdhocMinutes(ep[3], ep[4:])
			}
		} else {
			// base.target.PR.subtype[.supp1.supp2.supp3] - the legacy Finchains grammar. Still
			// parsed (so the request routes + logs as a redirect) but the qualifiers are then
			// discarded by the lossy redirect (Finchains is gone).
			if len(ep) > 3 {
				p.Subtype = ep[3]
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
		}
		return p, nil
	}

	// Canonical grammar: base.target[.minutes] - always AdHoc.
	p.QType = QTypeAdHoc
	if len(ep) >= 3 {
		p.Minutes, p.IgnoredFields = parseAdhocMinutes(ep[2], ep[3:])
	}
	return p, nil
}

// isQualifier reports whether s is one of the legacy explicit endpoint qualifiers.
func isQualifier(s string) bool {
	return s == QTypeAdHoc || s == QTypeFinchains
}

// parseAdhocMinutes interprets the AdHoc lookback-window field (clamped to 0-60) and collects
// any fields the AdHoc path cannot honour. A non-numeric window value is itself treated as an
// ignored field (e.g. a leftover Finchains subtype) and the window defaults to 0.
func parseAdhocMinutes(window string, rest []string) (int64, []string) {
	var ignored []string
	minutes, ok := clampMinutes(window)
	if !ok && window != "" {
		ignored = append(ignored, window)
	}
	for _, f := range rest {
		if f != "" {
			ignored = append(ignored, f)
		}
	}
	return minutes, ignored
}

// clampMinutes parses the AdHoc lookback window, clamped to 0-60. The bool reports whether the
// input was a valid integer; false for non-numeric input, which callers treat as an ignored
// field with the window defaulting to 0.
func clampMinutes(s string) (int64, bool) {
	m, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	if m < 0 {
		return 0, true
	}
	if m > 60 {
		return 60, true
	}
	return m, true
}
