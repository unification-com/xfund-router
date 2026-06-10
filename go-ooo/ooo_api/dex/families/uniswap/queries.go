package uniswap

import (
	"encoding/json"
	"fmt"
	"strings"
)

// GeneratePairsQuery builds the metadata query for a batch of pair/pool contract
// addresses. The entity name and liquidity field are taken from the family's Params so the
// one query template serves both the v2 (pairs/reserveUSD) and v3 (pools/totalValueLockedUSD)
// schemas.
func (f Family) GeneratePairsQuery(contractAddresses string) ([]byte, error) {
	c := strings.ToLower(contractAddresses)

	jsonData := map[string]string{
		"query": fmt.Sprintf(`
            {
                %s(where: { id_in: [%s] }) {
                    id
                    %s
                    volumeUSD
                    txCount
                    untrackedVolumeUSD
                    token0Price
                    token1Price
                    __typename
                    token0 { id symbol name decimals __typename }
                    token1 { id symbol name decimals __typename }
                }
            }`, f.params.Entity, c, f.params.LiquidityField),
	}

	return json.Marshal(jsonData)
}

// GenerateDexPricesQuery builds an aliased multi-query: p0 is the latest price and p1..pN
// each read one historical block (one per minute requested). It returns the marshalled
// query plus the number of aliases, which ProcessDexPricesResult uses to walk the response.
func (f Family) GenerateDexPricesQuery(pairContractAddress string, minutes, currentBlock, blocksPerMin uint64) ([]byte, uint64, error) {
	c := strings.ToLower(pairContractAddress)

	baseQuery := `
                id
                token0 { id name symbol }
                token1 { id name symbol }
                token0Price
                token1Price`

	queries := make(map[string]string)

	// p0 - latest price
	queries["p0"] = fmt.Sprintf(`%s(where: {id_in: [%s]}) { %s }`, f.params.Entity, c, baseQuery)

	// p1..pN - one historical block per minute requested
	for i := 1; i <= int(minutes); i++ {
		sub := blocksPerMin * uint64(i)
		if sub >= currentBlock {
			// The requested look-back predates the chain. Stop here rather than let
			// currentBlock-sub underflow uint64 into a bogus future block number. i only
			// grows, so breaking keeps the alias keys contiguous (p0..p{i-1}), which
			// ProcessDexPricesResult relies on.
			break
		}
		queries[fmt.Sprintf("p%d", i)] = fmt.Sprintf(
			`%s(block: { number: %d }, where: {id_in: [%s]}) { %s }`,
			f.params.Entity, currentBlock-sub, c, baseQuery)
	}

	qs := make([]string, 0, len(queries))
	for alias, body := range queries {
		qs = append(qs, fmt.Sprintf("%s: %s", alias, body))
	}

	jsonData := map[string]string{
		"query": fmt.Sprintf(`{%s}`, strings.Join(qs, ",")),
	}

	ret, err := json.Marshal(jsonData)
	return ret, uint64(len(queries)), err
}
