package univ4

import (
	"encoding/json"
	"fmt"
	"strings"
)

// entity is the Uniswap v4 pool collection. As with Uniswap v3 the liquidity figure is
// totalValueLockedUSD; the v4-specific additions are the hooks field and the native-currency
// token id, both of which the queries below select.
const entity = "pools"

// GeneratePairsQuery builds the metadata query for a batch of pool ids. It selects hooks (so the
// parser can skip hooked pools) and the token ids (so the parser can normalise the native
// currency to the wrapped symbol).
func (f Family) GeneratePairsQuery(contractAddresses string) ([]byte, error) {
	c := strings.ToLower(contractAddresses)

	jsonData := map[string]string{
		"query": fmt.Sprintf(`
            {
                %s(where: { id_in: [%s] }) {
                    id
                    hooks
                    totalValueLockedUSD
                    volumeUSD
                    txCount
                    untrackedVolumeUSD
                    token0Price
                    token1Price
                    token0 { id symbol name decimals }
                    token1 { id symbol name decimals }
                }
            }`, entity, c),
	}

	return json.Marshal(jsonData)
}

// GenerateDexPricesQuery builds an aliased multi-query: p0 is the latest price and p1..pN each
// read one historical block (one per minute requested). It mirrors the uniswap family's
// pagination and underflow guard, adding hooks + token ids to the selection so the parser can
// gate hooked pools and normalise the native currency.
func (f Family) GenerateDexPricesQuery(pairContractAddress string, minutes, currentBlock, blocksPerMin uint64) ([]byte, uint64, error) {
	c := strings.ToLower(pairContractAddress)

	baseQuery := `
                id
                hooks
                token0 { id symbol }
                token1 { id symbol }
                token0Price
                token1Price`

	queries := make(map[string]string)

	// p0 - latest price
	queries["p0"] = fmt.Sprintf(`%s(where: {id_in: [%s]}) { %s }`, entity, c, baseQuery)

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
			entity, currentBlock-sub, c, baseQuery)
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
