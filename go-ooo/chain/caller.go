package chain

import (
	"context"
	"fmt"
	"math/big"

	"go-ooo/logger"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
)

// gasFees carries the price fields for one transaction: either a legacy GasPrice, or the
// EIP-1559 tip + fee caps. Exactly one form is populated; the unused fields stay nil, so
// applying it to a *bind.TransactOpts selects a legacy or a dynamic-fee tx accordingly.
type gasFees struct {
	gasPrice  *big.Int // legacy gas price (nil for dynamic-fee txs)
	gasTipCap *big.Int // EIP-1559 max priority fee / tip (nil for legacy txs)
	gasFeeCap *big.Int // EIP-1559 max fee (nil for legacy txs)
}

func (f gasFees) apply(opts *bind.TransactOpts) {
	opts.GasPrice = f.gasPrice
	opts.GasTipCap = f.gasTipCap
	opts.GasFeeCap = f.gasFeeCap
}

// buildTransactOpts returns a fresh *bind.TransactOpts for a single transaction.
// The nonce is read from the chain (PendingNonceAt) on every call, so it is always
// anchored to reality and self-heals after any divergence (an out-of-band tx, a
// dropped tx, a reorg). The base options (signer, from, gas limit, context, value)
// are cloned, so concurrent callers never share mutable transaction state.
func (o *OoORouterService) buildTransactOpts() (*bind.TransactOpts, error) {
	nonce, err := o.client.PendingNonceAt(o.context, o.oracleAddress)
	if err != nil {
		return nil, fmt.Errorf("get pending nonce: %w", err)
	}

	fees, err := o.suggestGasFees()
	if err != nil {
		return nil, err
	}

	opts := *o.baseTransactOpts
	opts.Nonce = new(big.Int).SetUint64(nonce)
	fees.apply(&opts)
	return &opts, nil
}

// minGasBumpPercent is the network's minimum increase to replace a transaction.
// defaultGasBumpPercent is used when the configured value is unset or below the floor.
const (
	minGasBumpPercent     uint64 = 10
	defaultGasBumpPercent uint64 = 13

	// feeCapBaseFeeMultiplier sets the EIP-1559 max-fee headroom over the current base fee.
	// The base fee can rise at most 12.5% per block, so 2x covers ~6 blocks of increases
	// before the fee cap would bind.
	feeCapBaseFeeMultiplier int64 = 2
)

// determineEip1559 decides whether to send EIP-1559 dynamic-fee txs: the operator must enable
// it in config AND the chain must be post-London (its latest header carries a base fee). If the
// support probe fails, fall back to legacy pricing - legacy txs are valid on every chain, so an
// uncertain probe (or a pre-London chain) never blocks startup and never produces a tx the chain
// will reject.
func determineEip1559(ctx context.Context, client *ethclient.Client, enabled bool, log *logger.Scoped) bool {
	if !enabled {
		log.Info("chain", "determineEip1559", "", "eip1559 disabled in config - using legacy gas pricing")
		return false
	}
	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		log.Error("chain", "determineEip1559", "probe chain for base fee", err.Error())
		return false
	}
	if head.BaseFee == nil {
		log.Warn("chain", "determineEip1559", "", "eip1559 enabled but chain has no base fee (pre-London) - using legacy gas pricing")
		return false
	}
	log.Info("chain", "determineEip1559", "", "using EIP-1559 dynamic-fee gas pricing")
	return true
}

// suggestGasFees returns the price fields for a new transaction: EIP-1559 tip + fee caps when
// dynamic-fee pricing is in use, otherwise a legacy gas price.
func (o *OoORouterService) suggestGasFees() (gasFees, error) {
	if o.useEip1559 {
		return o.suggestDynamicFees()
	}
	gasPrice, err := o.suggestGasPrice()
	if err != nil {
		return gasFees{}, err
	}
	return gasFees{gasPrice: gasPrice}, nil
}

// suggestGasPrice asks the node for a legacy gas price, capped at the chain's max_gas_price.
func (o *OoORouterService) suggestGasPrice() (*big.Int, error) {
	gasPrice, err := o.client.SuggestGasPrice(o.context)
	if err != nil {
		return nil, fmt.Errorf("suggest gas price: %w", err)
	}
	return o.capGasPrice(gasPrice), nil
}

// suggestDynamicFees builds EIP-1559 fees from the node's suggested priority fee and the current
// base fee: feeCap = multiplier*baseFee + tip, capped at MaxGasPrice with the tip clamped so it
// never exceeds the fee cap.
func (o *OoORouterService) suggestDynamicFees() (gasFees, error) {
	tip, err := o.client.SuggestGasTipCap(o.context)
	if err != nil {
		return gasFees{}, fmt.Errorf("suggest gas tip cap: %w", err)
	}
	head, err := o.client.HeaderByNumber(o.context, nil)
	if err != nil {
		return gasFees{}, fmt.Errorf("get head for base fee: %w", err)
	}
	if head.BaseFee == nil {
		return gasFees{}, fmt.Errorf("chain has no base fee for an EIP-1559 transaction")
	}
	feeCap := new(big.Int).Mul(head.BaseFee, big.NewInt(feeCapBaseFeeMultiplier))
	feeCap.Add(feeCap, tip)
	return cappedDynamicFees(tip, feeCap, o.chainCfg.MaxGasPrice), nil
}

// capGasPrice limits gasPrice to the chain's max_gas_price (in gwei) when that is set.
func (o *OoORouterService) capGasPrice(gasPrice *big.Int) *big.Int {
	return capToMaxGwei(gasPrice, o.chainCfg.MaxGasPrice)
}

// capToMaxGwei limits gasPrice (in wei) to maxGwei. maxGwei <= 0 means uncapped.
func capToMaxGwei(gasPrice *big.Int, maxGwei int64) *big.Int {
	if maxGwei > 0 {
		maxWei := new(big.Int).Mul(big.NewInt(maxGwei), big.NewInt(params.GWei))
		if gasPrice.Cmp(maxWei) > 0 {
			return maxWei
		}
	}
	return gasPrice
}

// cappedDynamicFees caps feeCap at maxGwei and clamps tip so it never exceeds the fee cap (a tip
// above the total max fee is meaningless and can make a tx invalid).
func cappedDynamicFees(tip, feeCap *big.Int, maxGwei int64) gasFees {
	feeCap = capToMaxGwei(feeCap, maxGwei)
	if tip.Cmp(feeCap) > 0 {
		tip = new(big.Int).Set(feeCap)
	}
	return gasFees{gasTipCap: tip, gasFeeCap: feeCap}
}

// gasBumpPercent returns the configured replacement bump, enforcing the network floor.
func (o *OoORouterService) gasBumpPercent() uint64 {
	if p := o.chainCfg.GasBumpPercent; p >= minGasBumpPercent {
		return p
	}
	return defaultGasBumpPercent
}

// buildReplacementTransactOpts builds opts that REPLACE a stuck transaction at the SAME nonce.
// Legacy: the gas price is bumped above the stuck price. EIP-1559: BOTH the tip and fee caps are
// bumped above the stuck tx's values - the network requires both to rise to evict a pending
// dynamic-fee tx. In each case the bumped value is floored at the current market suggestion (in
// case the market rose) and capped at MaxGasPrice.
func (o *OoORouterService) buildReplacementTransactOpts(nonce, stuckGasPrice, stuckTip uint64) (*bind.TransactOpts, error) {
	opts := *o.baseTransactOpts
	opts.Nonce = new(big.Int).SetUint64(nonce)
	bump := o.gasBumpPercent()

	if o.useEip1559 {
		suggested, err := o.suggestDynamicFees()
		if err != nil {
			return nil, err
		}
		computeReplacementDynamicFees(stuckGasPrice, stuckTip, suggested, bump, o.chainCfg.MaxGasPrice).apply(&opts)
		return &opts, nil
	}

	suggested, err := o.suggestGasPrice()
	if err != nil {
		return nil, err
	}
	opts.GasPrice = computeReplacementGasPrice(stuckGasPrice, suggested, bump, o.chainCfg.MaxGasPrice)
	return &opts, nil
}

// bumpAtLeast bumps stuck (wei) by bumpPercent and raises the result to at least floor when the
// market has moved up past the bumped value.
func bumpAtLeast(stuck uint64, floor *big.Int, bumpPercent uint64) *big.Int {
	bumped := new(big.Int).SetUint64(stuck)
	bumped.Mul(bumped, new(big.Int).SetUint64(100+bumpPercent))
	bumped.Div(bumped, big.NewInt(100))
	if floor != nil && bumped.Cmp(floor) < 0 {
		bumped = new(big.Int).Set(floor)
	}
	return bumped
}

// computeReplacementGasPrice bumps a stuck legacy gas price (wei) by bumpPercent, raises it to at
// least the node suggestion if the market has moved up, and caps it at maxGwei.
func computeReplacementGasPrice(stuckGasPrice uint64, suggested *big.Int, bumpPercent uint64, maxGwei int64) *big.Int {
	return capToMaxGwei(bumpAtLeast(stuckGasPrice, suggested, bumpPercent), maxGwei)
}

// computeReplacementDynamicFees bumps BOTH the stuck fee and tip caps by bumpPercent, floors each
// at the current market suggestion, caps the fee cap at maxGwei and clamps the tip to it.
func computeReplacementDynamicFees(stuckFeeCap, stuckTip uint64, suggested gasFees, bumpPercent uint64, maxGwei int64) gasFees {
	feeCap := bumpAtLeast(stuckFeeCap, suggested.gasFeeCap, bumpPercent)
	tip := bumpAtLeast(stuckTip, suggested.gasTipCap, bumpPercent)
	return cappedDynamicFees(tip, feeCap, maxGwei)
}
