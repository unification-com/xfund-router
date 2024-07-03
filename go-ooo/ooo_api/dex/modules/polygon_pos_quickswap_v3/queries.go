package polygon_pos_quickswap_v3

import (
	"encoding/json"
	"fmt"
	"strings"
)

func (d DexModule) generatePairsListQuery(contractAddresses string) ([]byte, error) {

	c := strings.ToLower(contractAddresses)

	jsonData := map[string]string{
		"query": fmt.Sprintf(`
            {
	            pools(
                    where :
                     {
                          id_in: [%s]
                     }
	            ) 
                {
                     id
	                 totalValueLockedUSD
	                 volumeUSD
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
	        }`, c),
	}

	return json.Marshal(jsonData)
}

func (d DexModule) generatePricesQuery(pairContractAddress string, minutes, currentBlock, blocksPerMin uint64) ([]byte, uint64, error) {

	c := strings.ToLower(pairContractAddress)

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

	// latest price
	queries["p0"] = fmt.Sprintf(`pools(where: {id_in: [%s]}) {
                     %s
                }`, c, baseQuery)

	if minutes > 0 {
		//lastBlock := currentBlock - uint64(1)
		//subBlocks := minutes * blocksPerMin
		//for p := uint64(0); p < subBlocks; p++ {
		//	q := fmt.Sprintf(`pools(block: { number: %d }, where: {id_in: [%s]}) {
		//             %s
		//        }`, lastBlock-p, c, baseQuery)
		//	queries[fmt.Sprintf(`p%d`, p+1)] = q
		//}
		for i := 1; i <= int(minutes); i++ {
			q := fmt.Sprintf(`pools(block: { number: %d }, where: {id_in: [%s]}) {
		                %s
		           }`, currentBlock-(blocksPerMin*uint64(i)), c, baseQuery)
			queries[fmt.Sprintf(`p%d`, i)] = q
		}
	}

	qs := []string{}
	for i, s := range queries {
		q := fmt.Sprintf("%s: %s", i, s)
		qs = append(qs, q)
	}

	jsonData := map[string]string{
		"query": fmt.Sprintf(`{%s}`, strings.Join(qs, ",")),
	}

	ret, err := json.Marshal(jsonData)

	return ret, uint64(len(queries)), err
}
