package eth_uniswapv3

import (
	"encoding/json"
	"fmt"
	"strings"
)

func (d DexModule) generatePairsListQuery(skip uint64) ([]byte, error) {
	txCountFilter := fmt.Sprintf(`txCount_gt: "%d"`, MinTxCount)
	skipFilter := ""
	if skip > 0 {
		skipFilter = fmt.Sprintf(`skip: %d,`, skip)
	}

	jsonData := map[string]string{
		"query": fmt.Sprintf(`
            {
	            pools(
	                first: 1000,
                    %s
	                orderBy: totalValueLockedUSD,
	                orderDirection: desc,
                    where :
                     {
                          totalValueLockedUSD_gt: "%d",
                          %s
                     }
	            ) 
                {
                     id
	                 totalValueLockedUSD
	                 volumeUSD
	                 txCount
	                 untrackedVolumeUSD
	                 token0Price
	                 token1Price
	                 __typename
                     token0 {
	                     id
	                     symbol
	                     name
	                     decimals
	                     __typename
	                 }
	                 token1 {
	                     id
                         symbol 
                         name 
                         decimals
                         __typename
	                 }
	            }
	        }`, skipFilter, MinLiquidity, txCountFilter),
	}

	return json.Marshal(jsonData)
}

func (d DexModule) generatePricesQuery(pairContractAddress string, minutes, currentBlock, blocksPerMin uint64) ([]byte, error) {
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

	var queries = make(map[string]string)

	// Subgraphs are sometimes 2 or 3 blocks behind with indexing
	qBlock := currentBlock - 3

	for i := 0; i < int(minutes); i++ {
		q := fmt.Sprintf(`pool(id: "%s", block: { number: %d }) {
                     %s
                }`, pairContractAddress, qBlock-(blocksPerMin*uint64(i)), baseQuery)
		queries[fmt.Sprintf(`p%d`, i)] = q
	}

	qs := []string{}
	for i, s := range queries {
		q := fmt.Sprintf("%s: %s", i, s)
		qs = append(qs, q)
	}

	jsonData := map[string]string{
		"query": fmt.Sprintf(`{%s}`, strings.Join(qs, ",")),
	}

	return json.Marshal(jsonData)
}
