package ooo_api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/montanaflynn/stats"
	"github.com/sirupsen/logrus"
	"go-ooo/utils"
	"io/ioutil"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"
)

// MinLiquidity ToDo - make configurable in config.toml
// MinLiquidity - min liquidity a pair should have for the DEX pair search
const MinLiquidity = 30000

// MinTxCount ToDo - make configurable in config.toml
// MinTxCount - min tx count a pair should have for the DEX pair search
const MinTxCount = 250

// currently supported DEXs for ad-hoc queries
func getQlApis() []map[string]string {
	return []map[string]string{
		{
			"name":              "shibaswap",
			"url":               "https://api.thegraph.com/subgraphs/name/shibaswaparmy/exchange",
			"pairs_endpoint":    "pairs",
			"pair_endpoint":     "pair",
			"tokens_endpoint":   "tokens",
			"token_order_by":    "txCount",
			"pairs_order_by":    "reserveUSD",
			"tx_count":          "txCount",
			"chain":             "eth",
			"blocks_in_one_min": "5",
		},
		{
			"name":              "sushiswap",
			"url":               "https://api.thegraph.com/subgraphs/name/sushiswap/exchange",
			"pairs_endpoint":    "pairs",
			"pair_endpoint":     "pair",
			"tokens_endpoint":   "tokens",
			"token_order_by":    "txCount",
			"pairs_order_by":    "reserveUSD",
			"tx_count":          "txCount",
			"chain":             "eth",
			"blocks_in_one_min": "5",
		},
		{
			"name":              "uniswapv2",
			"url":               "https://api.thegraph.com/subgraphs/name/uniswap/uniswap-v2",
			"pairs_endpoint":    "pairs",
			"pair_endpoint":     "pair",
			"tokens_endpoint":   "tokens",
			"token_order_by":    "txCount",
			"pairs_order_by":    "reserveUSD",
			"tx_count":          "txCount",
			"chain":             "eth",
			"blocks_in_one_min": "5",
		},
		{
			"name":              "uniswapv3",
			"url":               "https://api.thegraph.com/subgraphs/name/uniswap/uniswap-v3",
			"pairs_endpoint":    "pools",
			"pair_endpoint":     "pool",
			"tokens_endpoint":   "tokens",
			"token_order_by":    "txCount",
			"pairs_order_by":    "totalValueLockedUSD",
			"tx_count":          "txCount",
			"chain":             "eth",
			"blocks_in_one_min": "5",
		},
		{
			"name":              "quickswap",
			"url":               "https://api.thegraph.com/subgraphs/name/sameepsi/quickswap06",
			"pairs_endpoint":    "pairs",
			"pair_endpoint":     "pair",
			"tokens_endpoint":   "tokens",
			"token_order_by":    "tradeVolumeUSD",
			"pairs_order_by":    "reserveUSD",
			"tx_count":          "",
			"chain":             "polygon",
			"blocks_in_one_min": "20",
		},
		{
			"name":              "pancakeswap",
			"url":               "https://api.bscgraph.org/subgraphs/name/cakeswap",
			"pairs_endpoint":    "pairs",
			"pair_endpoint":     "pair",
			"tokens_endpoint":   "tokens",
			"token_order_by":    "tradeVolumeUSD",
			"pairs_order_by":    "reserveUSD",
			"tx_count":          "txCount",
			"chain":             "bsc",
			"blocks_in_one_min": "20",
		},
		{
			"name":              "honeyswap",
			"url":               "https://api.thegraph.com/subgraphs/name/1hive/honeyswap-xdai",
			"pairs_endpoint":    "pairs",
			"pair_endpoint":     "pair",
			"tokens_endpoint":   "tokens",
			"token_order_by":    "txCount",
			"pairs_order_by":    "reserveUSD",
			"tx_count":          "txCount",
			"chain":             "xdai",
			"blocks_in_one_min": "12",
		},
	}
}

func getChains() []string {
	qlApiUrls := getQlApis()
	var chains []string

	for _, a := range qlApiUrls {
		contains := false
		for _, c := range chains {
			if a["chain"] == c {
				contains = true
			}
		}
		if !contains {
			chains = append(chains, a["chain"])
		}
	}

	return chains
}

func (o *OOOApi) getCurrentBlockNumForChain(chain string) (uint64, error) {
	switch chain {
	case "eth":
		return o.subchainEthClient.BlockNumber(o.ctx)
	case "polygon":
		return o.subchainPolygonClient.BlockNumber(o.ctx)
	case "bsc":
		return o.subchainBscClient.BlockNumber(o.ctx)
	case "xdai":
		return o.subchainXdaiClient.BlockNumber(o.ctx)
	}

	return 0, nil
}

func (o *OOOApi) UpdateDexTokensAndPairs() {
	o.logger.WithFields(logrus.Fields{
		"package":  "ooo_api",
		"function": "UpdateDexTokensAndPairs",
	}).Info("begin update popular tokens")

	qlApiUrls := getQlApis()

	for _, api := range qlApiUrls {
		o.updateAllTokensAndPairs(api)
	}
}

func (o *OOOApi) updateAllTokensAndPairs(api map[string]string) {

	pairs := o.getPairsFromGraphQl(api, 0)

	o.logger.WithFields(logrus.Fields{
		"package":   "ooo_api",
		"function":  "updateAllTokensAndPairs",
		"dex":       api["name"],
		"num_pairs": len(pairs),
	}).Info("found pairs")

	o.updatePairsInDb(pairs, api["name"], api["chain"])

	skip := uint64(1000)

	for len(pairs) == 1000 {
		o.logger.WithFields(logrus.Fields{
			"package":  "ooo_api",
			"function": "updateAllTokensAndPairs",
			"dex":      api["name"],
		}).Info("pairs > 1000. Get next pairs")

		pairs = o.getPairsFromGraphQl(api, skip)
		skip += 1000

		o.logger.WithFields(logrus.Fields{
			"package":   "ooo_api",
			"function":  "updateAllTokensAndPairs",
			"dex":       api["name"],
			"num_pairs": len(pairs),
		}).Info("found more pairs")

		o.updatePairsInDb(pairs, api["name"], api["chain"])
	}
}

func (o *OOOApi) getPairsFromGraphQl(api map[string]string, skip uint64) []GraphQlPairContent {
	query := generatePairsListQuery(api["pairs_endpoint"], api["pairs_order_by"], api["tx_count"], skip)

	var decodedResponse GraphQlPairsResponse

	o.runQuery(query, api["url"], &decodedResponse)

	pairs := decodedResponse.Data.Pairs
	if api["name"] == "uniswapv3" {
		pairs = decodedResponse.Data.Pools
	}

	return pairs
}

func (o *OOOApi) updatePairsInDb(pairs []GraphQlPairContent, dex, chain string) {
	for _, pair := range pairs {
		// pair.Token0.Id is the token's contract address
		t0Db, _ := o.db.FindOrInsertNewTokenContract(pair.Token0.Symbol, pair.Token0.Id, chain)
		t1Db, _ := o.db.FindOrInsertNewTokenContract(pair.Token1.Symbol, pair.Token1.Id, chain)

		t0DtDb, _ := o.db.FindOrInsertNewDexToken(pair.Token0.Symbol, t0Db.ID, dex, chain)
		t1DtDb, _ := o.db.FindOrInsertNewDexToken(pair.Token1.Symbol, t1Db.ID, dex, chain)

		dexReserveUSD := pair.ReserveUSD
		// todo - betterize
		if dex == "uniswapv3" {
			dexReserveUSD = pair.TotalValueLockedUSD
		}

		// store liquidity
		reserve, _ := utils.ParseBigFloat(dexReserveUSD)
		reserveUsd, _ := reserve.Float64()

		// todo - update latest liquidity
		_, _ = o.db.FindOrInsertNewDexPair(pair.Token0.Symbol, pair.Token1.Symbol, pair.Id, dex, t0DtDb.ID, t1DtDb.ID, reserveUsd)
	}
}

func (o *OOOApi) QueryAdhoc(endpoint string, requestId string) (string, error) {
	qlApiUrls := getQlApis()

	currentBlocks := make(map[string]uint64)
	for _, api := range qlApiUrls {
		if _, ok := currentBlocks[api["chain"]]; !ok {
			currentBlock, _ := o.getCurrentBlockNumForChain(api["chain"])
			currentBlocks[api["chain"]] = currentBlock
		}
	}

	base, target, _, mins, _, _, _, err := ParseEndpoint(endpoint)

	minutes, _ := strconv.ParseInt(mins, 10, 64)

	// default to 10 minutes' of data
	if minutes == 0 {
		minutes = 10
	}

	// no more than 1 hour's data
	if minutes > 60 {
		minutes = 60
	}

	if err != nil {
		return "", err
	}

	o.logger.WithFields(logrus.Fields{
		"package":   "ooo_api",
		"function":  "QueryAdhoc",
		"action":    "ParseEndpoint",
		"requestId": requestId,
		"endpoint":  endpoint,
		"base":      base,
		"target":    target,
		"minutes":   minutes,
	}).Debug("AdHoc endpoint parsed")

	var rawPrices []float64
	var outliersRemoved []float64
	priceCount := 0
	total := big.NewInt(0)

	for _, a := range qlApiUrls {
		dexPrices := o.getPairPricesFromDex(base, target, a, currentBlocks[a["chain"]], uint64(minutes))
		for _, price := range dexPrices {
			if price != 0 {
				rawPrices = append(rawPrices, price)
			}
		}
	}

	if len(rawPrices) == 0 {
		o.logger.WithFields(logrus.Fields{
			"package":  "ooo_api",
			"function": "QueryAdhoc",
			"base":     base,
			"target":   target,
		}).Error("no prices found on DEXs for pair")

		return "0", errors.New("no prices found on DEXs for pair")
	}

	mean, err := stats.Mean(rawPrices)

	if err != nil {
		return "", err
	}

	stdDev, err := stats.StandardDeviation(rawPrices)

	if err != nil {
		return "", err
	}

	dMax := float64(3)
	chauvenetUsed := false

	// remove outliers with Chauvenet Criterion, but only if stdDev > 0
	// as some pair prices are too small to calculate stdDev
	for _, p := range rawPrices {
		if stdDev > 0 {
			chauvenetUsed = true
			d := math.Abs(p-mean) / stdDev
			if dMax > d {
				outliersRemoved = append(outliersRemoved, p)
			}
		} else {
			// prices are too small to use Chauvenet Criterion
			outliersRemoved = append(outliersRemoved, p)
		}
	}

	// calculate mean from data set with outliers removed
	for _, o := range outliersRemoved {
		p := big.NewFloat(o)
		wei := utils.EtherToWei(p)
		if wei.Cmp(big.NewInt(0)) > 0 {
			total = new(big.Int).Add(total, wei)
			priceCount++
		}
	}

	if total.Cmp(big.NewInt(0)) <= 0 {
		return "", errors.New("cannot calculate mean, price is zero")
	}

	meanPrice := new(big.Int).Div(total, big.NewInt(int64(priceCount)))

	o.logger.WithFields(logrus.Fields{
		"package":            "ooo_api",
		"function":           "QueryAdhoc",
		"base":               base,
		"target":             target,
		"num_prices_raw":     len(rawPrices),
		"num_prices_chauv":   len(outliersRemoved),
		"num_prices_removed": len(rawPrices) - len(outliersRemoved),
		"raw_prices_mean":    mean,
		"raw_std_dev":        stdDev,
		"final_wei_mean":     meanPrice.String(),
		"chauvenet_used":     chauvenetUsed,
	}).Debug("price stats")

	return meanPrice.String(), nil
}

func (o *OOOApi) processPriceData(base string, target string, dexName string, pair map[string]any) float64 {
	price := float64(0)

	// check reserve USD and reject if < MIN_LIQUIDITY
	dexRes := pair["reserveUSD"]
	// todo - betterize
	if dexName == "uniswapv3" {
		dexRes = pair["totalValueLockedUSD"]
	}

	limit := big.NewFloat(MinLiquidity)

	reserve, err := utils.ParseBigFloat(dexRes.(string))

	if err != nil {
		o.logger.WithFields(logrus.Fields{
			"package":  "ooo_api",
			"function": "processPriceData",
			"dex":      dexName,
			"base":     base,
			"target":   target,
			"dexRes":   dexRes,
		}).Error(err.Error())
		return price
	}

	if reserve.Cmp(limit) == -1 {
		o.logger.WithFields(logrus.Fields{
			"package":  "ooo_api",
			"function": "processPriceData",
			"dex":      dexName,
			"base":     base,
			"target":   target,
			"reserve":  reserve.String(),
			"price":    price,
		}).Warn("low liquidity")
		return price
	}

	priceBf := big.NewFloat(MinLiquidity)

	t0 := pair["token0"].(map[string]any)
	t1 := pair["token1"].(map[string]any)

	if base == t0["symbol"] && target == t1["symbol"] {
		priceBf, err = utils.ParseBigFloat(pair["token1Price"].(string))
	} else {
		priceBf, err = utils.ParseBigFloat(pair["token0Price"].(string))
	}

	if err != nil {
		o.logger.WithFields(logrus.Fields{
			"package":  "ooo_api",
			"function": "processPriceData",
			"action":   "utils.ParseBigFloat",
			"dex":      dexName,
			"base":     base,
			"target":   target,
			"price":    price,
		}).Error(err)
		return price
	}

	price, _ = priceBf.Float64()

	return price
}

func (o *OOOApi) getPairPricesFromDex(base string, target string, api map[string]string, currentBlock, minutes uint64) []float64 {

	var prices []float64
	// check DB for pair contract address
	dbPairRes, _ := o.db.FindByDexPairName(base, target, api["name"])

	if dbPairRes.ID != 0 {
		pairPricesRes, err := o.getRecentPairPrices(dbPairRes.ContractAddress, api, currentBlock, minutes)

		if err != nil {
			o.logger.WithFields(logrus.Fields{
				"package":  "ooo_api",
				"function": "getRecentPairPrices",
				"dex":      api["name"],
				"base":     base,
				"target":   target,
			}).Error(err)
		}

		if pairPricesRes != nil {
			for i := 0; i < int(minutes); i++ {
				p := o.processPriceData(base, target, api["name"], pairPricesRes[fmt.Sprintf(`p%d`, i)].(map[string]any))
				prices = append(prices, p)
			}
		}
	} else {
		o.logger.WithFields(logrus.Fields{
			"package":  "ooo_api",
			"function": "getPairPricesFromDex",
			"dex":      api["name"],
			"base":     base,
			"target":   target,
		}).Error("pair not found in database for this dex")
	}

	return prices
}

func (o *OOOApi) getRecentPairPrices(pairAddress string, api map[string]string, currentBlock, minutes uint64) (map[string]any, error) {
	o.logger.WithFields(logrus.Fields{
		"package":       "ooo_api",
		"function":      "getKnownPairPrice",
		"dex":           api["name"],
		"url":           api["url"],
		"pair_address":  pairAddress,
		"current_block": currentBlock,
	}).Debug()

	blocksPerMin, err := strconv.Atoi(api["blocks_in_one_min"])
	if err != nil {
		blocksPerMin = 10
	}

	query := generatePairPricesQuery(pairAddress, api["pair_endpoint"], api["pairs_order_by"], uint64(blocksPerMin), currentBlock, minutes)

	var decodedResponse map[string]any

	o.runQuery(query, api["url"], &decodedResponse)

	if decodedResponse["errors"] != nil {
		retErrors := decodedResponse["errors"].([]interface{})
		retErr := retErrors[0].(map[string]any)
		return nil, errors.New(retErr["message"].(string))
	}

	return decodedResponse["data"].(map[string]any), nil

}

// runQuery will run the subgraph query
func (o *OOOApi) runQuery(query interface{}, url string, decodedResponse interface{}) {
	jsonValue, _ := json.Marshal(query)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonValue))

	if err != nil {
		o.logger.WithFields(logrus.Fields{
			"package":  "ooo_api",
			"function": "runQuery",
		}).Error(err.Error())
		return
	}

	resp, err := o.client.Do(req)

	if err != nil {
		o.logger.WithFields(logrus.Fields{
			"package":  "ooo_api",
			"function": "runQuery",
		}).Error(err.Error())
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		body, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			o.logger.WithFields(logrus.Fields{
				"package":  "ooo_api",
				"function": "runQuery",
			}).Error(err.Error())
			return
		}

		err = json.Unmarshal(body, &decodedResponse)

		if err != nil {
			o.logger.WithFields(logrus.Fields{
				"package":  "ooo_api",
				"function": "runQuery",
			}).Error(err.Error())
			return
		}
	} else {
		o.logger.WithFields(logrus.Fields{
			"package":  "ooo_api",
			"function": "runQuery",
		}).Error(fmt.Errorf("non-200 OK status code: %v", resp.Status))
		return
	}
	return
}

// generateTokenQuery uses a fuzzy token query to get token ids from symbol names
func generateTokenQuery(symbol string, tokensEndpoint string, tokenOrderBy string) map[string]string {

	// since there may be multiple potentially fake tokens using the same
	// symbol we'll try and grab the one with the most Txs, using
	// orderBy: txCount desc
	jsonData := map[string]string{
		"query": fmt.Sprintf(`
            { 
                %s (
                    first:1,
                    orderBy: %s,
                    orderDirection: desc,
                    where: { symbol: "%s"}){
                    id
                    symbol
                    name
                }
            }
        `, tokensEndpoint, tokenOrderBy, symbol),
	}

	return jsonData
}

func generatePairsListQuery(pairEndpoint, pairOrderBy, txCount string, skip uint64) map[string]string {

	txCountFilter := ""
	if txCount != "" {
		txCountFilter = fmt.Sprintf(`txCount_gt: "%d"`, MinTxCount)
	}
	skipFilter := ""
	if skip > 0 {
		skipFilter = fmt.Sprintf(`skip: %d,`, skip)
	}

	jsonData := map[string]string{
		"query": fmt.Sprintf(`
            {
	            %s(
	                first: 1000,
                    %s
	                orderBy: %s,
	                orderDirection: desc,
                    where :
                     {
                          %s_gt: "%d",
                          %s
                     }
	            ) 
                {
                     id
	                 %s
	                 volumeUSD
	                 %s
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
	        }`, pairEndpoint, skipFilter, pairOrderBy, pairOrderBy, MinLiquidity, txCountFilter, pairOrderBy, txCount),
	}

	return jsonData
}

// generateNewPairQuery uses a fuzzy token query to get the top pair by txCount
func generateNewPairQuery(t0 string, t1 string, pairEndpoint string, pairOrderBy string) map[string]string {
	jsonData := map[string]string{
		"query": fmt.Sprintf(`
            {
                 %s (
                     first:1
                     orderBy:%s
                     orderDirection: desc
                     where :
                     {
                          token0_in : ["%s", "%s"],
                          token1_in : ["%s", "%s"],
                          %s_gt: "%d"
                     }
                 ) {
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
                     token1Price
                     %s
                 }
            }
        `, pairEndpoint, pairOrderBy, t0, t1, t0, t1, pairOrderBy, MinLiquidity, pairOrderBy),
	}

	return jsonData
}

func generateKnownPairQuery(pairAddress string, pairEndpoint string, pairOrderBy string) map[string]string {
	jsonData := map[string]string{
		"query": fmt.Sprintf(`
            {
	            %s(id: "%s") {
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
                     token1Price
                     %s
                }
	        }
        `, pairEndpoint, pairAddress, pairOrderBy),
	}

	return jsonData
}

func generatePairPricesQuery(pairAddress string, pairEndpoint string, pairOrderBy string, blocksPerMin, currentBlock, minutes uint64) map[string]string {

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
                    token1Price
                    %s`, pairOrderBy)

	var queries = make(map[string]string)

	// Subgraphs are sometimes 2 or 3 blocks behind with indexing
	qBlock := currentBlock - 3

	for i := 0; i < int(minutes); i++ {
		q := fmt.Sprintf(`%s(id: "%s", block: { number: %d }) {
                     %s
                }`, pairEndpoint, pairAddress, qBlock-(blocksPerMin*uint64(i)), baseQuery)
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

	return jsonData
}
