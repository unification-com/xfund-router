package models

import (
	"gorm.io/gorm"
)

type DexPairs struct {
	gorm.Model
	Chain           string `gorm:"index:idx_dex_pair;index:idx_dex_pair_chain_name"`
	Dex             string `gorm:"index:idx_dex_pair;index:idx_dex_pair_dex_name"`
	Pair            string `gorm:"index:idx_dex_pair;index:idx_dex_pair_pair"`
	T0TokenId       uint   `gorm:"index:idx_dex_pair_t0;index:idx_dex_pair_t0_t1"`
	T1TokenId       uint   `gorm:"index:idx_dex_pair_t1;index:idx_dex_pair_t0_t1"`
	T0Symbol        string `gorm:"index"`
	T1Symbol        string `gorm:"index"`
	ContractAddress string `gorm:"index"`
	ReserveUsd      float64
	TxCount         uint64
	Verified        bool
	// Confidence is the per-pool trust score [0,1] from the dex-pair-verify export (XR1), used to
	// weight the pool in the price aggregator. 0 for legacy/unscored rows.
	Confidence float64
	// CanonicalKey groups the same logical pair across chains/DEXs (from the export); "" when the
	// pair is unkeyable.
	CanonicalKey string `gorm:"index"`
	// T0Cg / T1Cg are the CoinGecko coin ids of token0 / token1 from the export (S7). They orient an
	// alias query (ETH.USD): the side whose cg id is in the base-alias class is the base. "" when the
	// token is unidentified or for legacy rows predating the alias export.
	T0Cg string `gorm:"index"`
	T1Cg string `gorm:"index"`
}

func (DexPairs) TableName() string {
	return "dex_pairs"
}

func (d *DexPairs) GetChain() string {
	return d.Chain
}

func (d *DexPairs) GetDexName() string {
	return d.Dex
}

func (d *DexPairs) GetPair() string {
	return d.Pair
}

func (d *DexPairs) GetT0DexTokenId() uint {
	return d.T0TokenId
}

func (d *DexPairs) GetT1DexTokenId() uint {
	return d.T1TokenId
}

func (d *DexPairs) GetContractAddress() string {
	return d.ContractAddress
}
