package models

import "gorm.io/gorm"

// AliasConfig persists the asset-class alias payload (S7) from the dex-pair-verify v3 manifest so the
// alias index survives a restart without a live API sync - the same resilience floor SupportedSource
// gives the module catalogue. It is a SINGLETON: only one row is ever kept (UpsertAliasConfig replaces
// it), since the aliases are manifest-level, not per-source. Groups + Pairs are stored as opaque JSON
// strings (never queried by their contents, only ever read or written whole), matching how
// SupportedSource stores its endpoint list.
type AliasConfig struct {
	gorm.Model
	// Groups is the curated fungible asset classes as a JSON array of {symbol, cgIds}.
	Groups string
	// Pairs is the active cross-class alias pairs as a JSON array of {pair, canonicalKeys}.
	Pairs string
}

func (AliasConfig) TableName() string {
	return "alias_configs"
}
