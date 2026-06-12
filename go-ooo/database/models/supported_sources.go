package models

import "gorm.io/gorm"

// SupportedSource is one verified (chain, dex) DEX source, persisted from the dex-pair-verify v3
// manifest. It is the go-ooo-side source catalogue that drives modular DEX dispatch: the stored
// rows rebuild the priceable module set + per-chain block/RPC config on startup, so the oracle can
// price from the last-synced manifest even before (or without) a fresh fetch - the resilience floor
// that replaces the old hard-coded seed modules. Refreshed every time the manifest is polled; a
// source dropped from the manifest is soft-deleted (mirrors the dex_pairs feed handling).
type SupportedSource struct {
	gorm.Model
	Chain                string `gorm:"index:idx_supported_source"`
	Dex                  string `gorm:"index:idx_supported_source"`
	SubgraphSchemaFamily string
	// Endpoints is the ordered subgraph endpoint list as a JSON array of {provider,urlTemplate,tier}.
	// Stored as an opaque JSON string (not a child table): it is small metadata only ever read or
	// written whole, and avoids pulling in a gorm datatypes dependency.
	Endpoints      string
	FactoryAddress string
	RpcUrl         string
	BlocksPerMin   int
	PairCount      int
	ExportUrl      string
	// MinLiquidityUsd is the source's curation floor (XR2): the per-(chain,dex) minLiquidityUsd from
	// its pair feed, the authoritative liquidity floor go-ooo applies in the price path instead of
	// its own hard-coded default. 0 means not yet known (no feed processed) - the default applies.
	MinLiquidityUsd float64
	// SourceUpdatedAt / SourceVerifiedAt mirror the manifest source's lastUpdated / lastVerifiedAt
	// (unix seconds) - the dex-pair-verify-side freshness, distinct from this row's own GORM
	// CreatedAt / UpdatedAt.
	SourceUpdatedAt  int64
	SourceVerifiedAt int64
}

func (SupportedSource) TableName() string {
	return "supported_sources"
}

func (s *SupportedSource) GetChain() string { return s.Chain }
func (s *SupportedSource) GetDex() string   { return s.Dex }
