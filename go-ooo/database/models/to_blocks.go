package models

import "gorm.io/gorm"

type ToBlocks struct {
	gorm.Model
	// ChainId scopes the resume cursor per chain, so each worker tracks its own last-scanned block.
	ChainId  int64 `gorm:"index"`
	BlockNum uint64
}

func (ToBlocks) TableName() string {
	return "to_blocks"
}

func (d ToBlocks) GetId() uint {
	return d.ID
}

func (d ToBlocks) GetBlockNum() uint64 {
	return d.BlockNum
}
