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
