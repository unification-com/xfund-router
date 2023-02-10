package bsc_pancakeswapv2

import (
	"encoding/json"
	"errors"
	"fmt"
)

func (d DexModule) generatePairsListQuery(skip uint64) ([]byte, error) {

	// first check API key set
	if d.nodeRealApiKey == "" {
		return nil, errors.New("nodreal API key not set in config")
	}

	if skip > 2000 {
		return nil, errors.New("nodereal.io skip limited to 2000")
	}

	jsonData := map[string]string{
		"query": fmt.Sprintf(`
            {
	            pairs(
	                first: 1000,
                    skip: %d,
                    orderBy: trackedReserveBNB,
                    orderDirection: desc,
                    where :
	                {
	                     totalTransactions_gt: %d
	                }
	            ) 
                {
                    id
	                reserveUSD
                    totalTransactions
                    trackedReserveBNB
                    volumeUSD
	                untrackedVolumeUSD
	                token0Price
	                token1Price
	                token0 {
	                    id
	                    symbol
	                    name
	                    decimals
	                }
	                token1 {
	                    id
	                    symbol
	                    name
	                    decimals
	                }
	            }
	        }`, skip, MinTxCount),
	}
	return json.Marshal(jsonData)
}

func (d DexModule) generatePricesQuery(pairContractAddress string, minutes, currentBlock, blocksPerMin uint64) ([]byte, error) {

	// first check API key set
	if d.nodeRealApiKey == "" {
		return nil, errors.New("nodreal API key not set in config")
	}

	baseQuery := fmt.Sprintf(`
                    id
	                token0 {
                         id
                         name
                         symbol
                    }
                    token1 {
                         id
                         name
                         symbol
                    }
                    token0Price
                    token1Price`)

	// API only allows real-time data, so only a single result returned.
	jsonData := map[string]string{
		"query": fmt.Sprintf(`{pair(id: "%s") {
                     %s
                }}`, pairContractAddress, baseQuery),
	}

	return json.Marshal(jsonData)
}
