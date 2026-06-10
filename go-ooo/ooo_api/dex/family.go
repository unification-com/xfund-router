package dex

import (
	"fmt"

	"go-ooo/ooo_api/dex/types"
)

// SchemaFamily encapsulates the only parts of a DEX integration that vary by subgraph
// schema: the GraphQL query text and the response parsing. Everything else a source needs
// (chain, dex name, subgraph URL, liquidity/tx floors) is plain metadata held by
// FamilyModule.
//
// Adding support for a new schema - Uniswap v4, PancakeSwap Infinity, Solidly, ... - means
// writing one new SchemaFamily implementation under families/<name>/ and mapping its
// manifest subgraphSchemaFamily string to it. No change to FamilyModule, the Manager, or
// the manifest builder is required. This is the pluggability seam the modular-dispatch work
// exists to provide.
type SchemaFamily interface {
	GeneratePairsQuery(contractAddresses string) ([]byte, error)
	ProcessPairsQueryResult(result []byte) ([]types.DexPair, error)
	GenerateDexPricesQuery(pairContractAddress string, minutes, currentBlock, blocksPerMin uint64) ([]byte, uint64, error)
	ProcessDexPricesResult(base, target string, numQueries uint64, result []byte) ([]types.PoolPrices, error)
}

// FamilyModule is a generic Module built from per-source metadata plus a SchemaFamily
// strategy. It replaces the hand-written per-DEX modules: the metadata methods are plain
// field reads and the four query/parse methods delegate to the family.
type FamilyModule struct {
	chain        string
	dex          string
	subgraphUrl  string
	minLiquidity uint64
	minTxCount   uint64
	family       SchemaFamily
}

// Compile-time check that FamilyModule satisfies the Module interface.
var _ Module = FamilyModule{}

// FamilyModuleSpec is the construction input for a FamilyModule. Until the manifest-driven
// builder lands (step 5), specs are produced by the per-DEX seed constructors; afterwards
// they are derived from the dex-pair-verify v3 manifest.
type FamilyModuleSpec struct {
	Chain        string
	Dex          string
	SubgraphUrl  string
	MinLiquidity uint64
	MinTxCount   uint64
	Family       SchemaFamily
}

// NewFamilyModule builds a FamilyModule from a spec.
func NewFamilyModule(spec FamilyModuleSpec) FamilyModule {
	return FamilyModule{
		chain:        spec.Chain,
		dex:          spec.Dex,
		subgraphUrl:  spec.SubgraphUrl,
		minLiquidity: spec.MinLiquidity,
		minTxCount:   spec.MinTxCount,
		family:       spec.Family,
	}
}

func (m FamilyModule) Name() string         { return fmt.Sprintf("%s_%s", m.chain, m.dex) }
func (m FamilyModule) SubgraphUrl() string  { return m.subgraphUrl }
func (m FamilyModule) Chain() string        { return m.chain }
func (m FamilyModule) Dex() string          { return m.dex }
func (m FamilyModule) MinLiquidity() uint64 { return m.minLiquidity }
func (m FamilyModule) MinTxCount() uint64   { return m.minTxCount }

func (m FamilyModule) GeneratePairsQuery(contractAddresses string) ([]byte, error) {
	return m.family.GeneratePairsQuery(contractAddresses)
}

func (m FamilyModule) ProcessPairsQueryResult(result []byte) ([]types.DexPair, error) {
	return m.family.ProcessPairsQueryResult(result)
}

func (m FamilyModule) GenerateDexPricesQuery(pairContractAddress string, minutes, currentBlock, blocksPerMin uint64) ([]byte, uint64, error) {
	return m.family.GenerateDexPricesQuery(pairContractAddress, minutes, currentBlock, blocksPerMin)
}

func (m FamilyModule) ProcessDexPricesResult(base, target string, numQueries uint64, result []byte) ([]types.PoolPrices, error) {
	return m.family.ProcessDexPricesResult(base, target, numQueries, result)
}

// graphGatewayUrl builds a decentralised-gateway subgraph URL, or the hosted fallback when
// no API key is configured. It mirrors the resolution the hand-written modules did, so a
// migrated source queries the identical endpoint.
func graphGatewayUrl(apiKey, subgraphId, hostedUrl string) string {
	if apiKey == "" {
		return hostedUrl
	}
	return fmt.Sprintf("https://gateway-arbitrum.network.thegraph.com/api/%s/subgraphs/id/%s", apiKey, subgraphId)
}
