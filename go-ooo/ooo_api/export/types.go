// Package export holds the Go types + client for the dex-pair-verify provider export (#127).
//
// go-ooo consumes two surfaces from dex-pair-verify, both versioned under /api/ooo/v1:
//   - the discovery manifest (GET /export/manifest, ExportManifestV3) - the source-of-truth
//     registry of priceable (chain, dex) sources that drives modular DEX dispatch; and
//   - one verified-pair feed per source (GET /export/{chain}/{dex}, EXPORT_PAIR_SCHEMA_VERSION 3)
//   - the curated pairs, each with a trust score (confidence) + cross-source canonical key.
//
// These types mirror dex-pair-verify/lib/export.ts. The wire format is additive within a major
// version, so a newer minor may add fields a v3 consumer simply ignores.
package export

// Schema versions go-ooo understands. A feed whose major version exceeds these is rejected; an
// equal-or-lower one is decoded best-effort (additive forward-compat).
const (
	ManifestSchemaVersion = 3
	PairSchemaVersion     = 3
)

// ManifestEndpoint is one subgraph endpoint for a source. URLTemplate carries an `{API_KEY}`
// placeholder - never a literal key; go-ooo substitutes the per-provider key it holds. Tier is
// "paid" (the decentralised Graph network, per-query billed) or "free" (Studio/hosted/self-hosted).
type ManifestEndpoint struct {
	Provider    string `json:"provider"`
	URLTemplate string `json:"urlTemplate"`
	Tier        string `json:"tier"`
}

// ManifestSource is one verified (chain, dex) source: its ordered subgraph endpoint list, schema
// family, factory, rpc + verified pair count. go-ooo builds a priceable module from each source
// whose SubgraphSchemaFamily it recognises, and skips (with a warning) the ones it doesn't.
type ManifestSource struct {
	Chain                string             `json:"chain"`
	Dex                  string             `json:"dex"`
	Endpoints            []ManifestEndpoint `json:"endpoints"` // [0] is the primary; free-tier alternatives follow
	SubgraphSchemaFamily string             `json:"subgraphSchemaFamily"`
	// SourceType is the source's transport (#128): "subgraph" (the EVM/GraphQL default) or "rest-sqs"
	// (a Cosmos Osmosis SQS source - the primary endpoint's urlTemplate is then the SQS base URL).
	SourceType     string `json:"sourceType"`
	FactoryAddress string `json:"factoryAddress"`
	RPCURL         string `json:"rpcUrl"`       // "" when the chain is not mapped (JSON null)
	BlocksPerMin   int    `json:"blocksPerMin"` // 0 when not mapped (JSON null)
	PairCount      int    `json:"pairCount"`
	ExportURL      string `json:"exportUrl"` // /api/ooo/v1/export/{chain}/{dex}
	LastUpdated    int64  `json:"lastUpdated"`
	LastVerifiedAt int64  `json:"lastVerifiedAt"`
	// MinLiquidityUsd is the source's curation floor (XR2). It is NOT part of the manifest wire
	// format - the floor is carried on the per-source pair feed (PairFeed.MinLiquidityUsd) - so it is
	// excluded from (de)serialisation and populated internally from the persisted source on restore.
	MinLiquidityUsd float64 `json:"-"`
}

// ManifestAliasGroup is one curated asset-class alias (S7): an alias symbol (USD, ETH, BTC) and the
// CoinGecko coin ids that are fungible-for-pricing members of it. dex-pair-verify owns this curation
// (the trust boundary); go-ooo consumes it verbatim to classify a pair's two tokens into asset classes
// and expand an alias query (e.g. ETH.USD) to every member pool.
type ManifestAliasGroup struct {
	Symbol string   `json:"symbol"`
	CgIds  []string `json:"cgIds"`
}

// ManifestAliasPair is one active cross-class alias pair (S7): its sorted symbol key (e.g. "ETH.USD")
// and the canonical keys of the verified pairs that back it. go-ooo expands an alias query to these
// canonical keys, selects the matching dex_pairs, and robust-aggregates them as one deep price.
type ManifestAliasPair struct {
	Pair          string   `json:"pair"`
	CanonicalKeys []string `json:"canonicalKeys"`
}

// Manifest is the v3 discovery manifest (GET /api/ooo/v1/export/manifest).
type Manifest struct {
	SchemaVersion    int              `json:"schemaVersion"`
	GeneratedAt      int64            `json:"generatedAt"`
	SupportedSources []ManifestSource `json:"supportedSources"`
	// AliasGroups + AliasPairs carry the asset-class aliases (S7), additive within v3. AliasGroups is
	// the curated fungible-class membership; AliasPairs is the active cross-class pairs derived from the
	// verified set. A manifest reconstructed from the persisted source catalogue (LoadManifestFromDB)
	// carries neither - they live only on the live API manifest - so ApplyManifest preserves the prior
	// in-memory alias index when an incoming manifest has no AliasGroups (see the carry-forward there).
	AliasGroups []ManifestAliasGroup `json:"aliasGroups"`
	AliasPairs  []ManifestAliasPair  `json:"aliasPairs"`
}

// ExportToken is one side of an exported pair.
type ExportToken struct {
	Chain           string `json:"chain"`
	Symbol          string `json:"symbol"`
	Name            string `json:"name"`
	ContractAddress string `json:"contractAddress"`
	// CoinGeckoCoinId is the token's CoinGecko coin id ("" if unidentified). go-ooo needs it to orient
	// an alias query (S7): the token whose cg id is in the base-alias class is the base side. The pair's
	// canonicalKey alone can't say which side is which, as it is cg-id-sorted.
	CoinGeckoCoinId string `json:"coingeckoCoinId"`
}

// ExportPair is one verified pool from a source's pair feed. Confidence ([0,1]) is the per-pool
// trust score go-ooo weights by (XR1); CanonicalKey groups the same logical pair across
// chains/DEXs ("" when unkeyable).
type ExportPair struct {
	ContractAddress string       `json:"contractAddress"`
	Pair            string       `json:"pair"`
	ReserveUsd      float64      `json:"reserveUsd"`
	VolumeUsd       float64      `json:"volumeUsd"`
	TxCount         int64        `json:"txCount"`
	Verdict         string       `json:"verdict"`
	VerdictReason   string       `json:"verdictReason"`
	Confidence      float64      `json:"confidence"`
	CanonicalKey    string       `json:"canonicalKey"` // "" when unkeyable (JSON null)
	Token0          *ExportToken `json:"token0"`
	Token1          *ExportToken `json:"token1"`
}

// PairFeed is one source's verified-pair feed (GET /api/ooo/v1/export/{chain}/{dex}).
// MinLiquidityUsd is the per-source curation floor go-ooo honours as the single source of truth
// for pair eligibility (XR2), rather than re-filtering on its own configured minimum.
type PairFeed struct {
	SchemaVersion   int          `json:"schemaVersion"`
	GeneratedAt     int64        `json:"generatedAt"`
	Chain           string       `json:"chain"`
	Dex             string       `json:"dex"`
	MinLiquidityUsd float64      `json:"minLiquidityUsd"`
	Pairs           []ExportPair `json:"pairs"`
}
