package messari

import (
	"encoding/json"
	"fmt"
	"strings"
)

// GeneratePairsQuery builds the metadata query for a batch of pool addresses. inputTokens
// carries the per-token identity + lastPriceUSD used by the prices path.
func (f Family) GeneratePairsQuery(contractAddresses string) ([]byte, error) {
	c := strings.ToLower(contractAddresses)

	jsonData := map[string]string{
		"query": fmt.Sprintf(`
            {
                %s(where: { id_in: [%s] }) {
                    id
                    name
                    symbol
                    totalValueLockedUSD
                    inputTokens { id symbol name decimals lastPriceUSD }
                }
            }`, entity, c),
	}

	return json.Marshal(jsonData)
}

// GenerateDexPricesQuery builds an aliased multi-query: p0 is the latest price and p1..pN
// each read one historical block. Each alias only needs the per-token symbol + lastPriceUSD.
func (f Family) GenerateDexPricesQuery(pairContractAddress string, minutes, currentBlock, blocksPerMin uint64) ([]byte, uint64, error) {
	c := strings.ToLower(pairContractAddress)

	baseQuery := `
                id
                inputTokens { symbol lastPriceUSD }`

	queries := make(map[string]string)

	// p0 - latest price
	queries["p0"] = fmt.Sprintf(`%s(where: {id_in: [%s]}) { %s }`, entity, c, baseQuery)

	// p1..pN - one historical block per minute requested
	for i := 1; i <= int(minutes); i++ {
		sub := blocksPerMin * uint64(i)
		if sub >= currentBlock {
			// Look-back predates the chain; stop before currentBlock-sub underflows uint64.
			// i only grows, so breaking keeps the alias keys contiguous (p0..p{i-1}).
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
