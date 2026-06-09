package chain

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/params"
)

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

	gasPrice, err := o.suggestGasPrice()
	if err != nil {
		return nil, err
	}

	opts := *o.baseTransactOpts
	opts.Nonce = new(big.Int).SetUint64(nonce)
	opts.GasPrice = gasPrice
	return &opts, nil
}

// minGasBumpPercent is the network's minimum increase to replace a transaction.
// defaultGasBumpPercent is used when the configured value is unset or below the floor.
const (
	minGasBumpPercent     uint64 = 10
	defaultGasBumpPercent uint64 = 13
)

// suggestGasPrice asks the node for a gas price, capped at cfg.Chain.MaxGasPrice.
func (o *OoORouterService) suggestGasPrice() (*big.Int, error) {
	gasPrice, err := o.client.SuggestGasPrice(o.context)
	if err != nil {
		return nil, fmt.Errorf("suggest gas price: %w", err)
	}
	return o.capGasPrice(gasPrice), nil
}

// capGasPrice limits gasPrice to cfg.Chain.MaxGasPrice (in gwei) when that is set.
func (o *OoORouterService) capGasPrice(gasPrice *big.Int) *big.Int {
	return capToMaxGwei(gasPrice, o.cfg.Chain.MaxGasPrice)
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

// gasBumpPercent returns the configured replacement bump, enforcing the network floor.
func (o *OoORouterService) gasBumpPercent() uint64 {
	if p := o.cfg.Chain.GasBumpPercent; p >= minGasBumpPercent {
		return p
	}
	return defaultGasBumpPercent
}

// buildReplacementTransactOpts builds opts that REPLACE a stuck transaction: the SAME
// nonce, and a gas price bumped above the stuck tx's price (by gasBumpPercent), but
// never below the node's current suggestion if the market has risen, and capped at
// MaxGasPrice.
func (o *OoORouterService) buildReplacementTransactOpts(nonce uint64, stuckGasPrice uint64) (*bind.TransactOpts, error) {
	suggested, err := o.suggestGasPrice()
	if err != nil {
		return nil, err
	}

	opts := *o.baseTransactOpts
	opts.Nonce = new(big.Int).SetUint64(nonce)
	opts.GasPrice = computeReplacementGasPrice(stuckGasPrice, suggested, o.gasBumpPercent(), o.cfg.Chain.MaxGasPrice)
	return &opts, nil
}

// computeReplacementGasPrice bumps stuckGasPrice (wei) by bumpPercent, raises the
// result to at least suggested if the market has moved up, and caps it at maxGwei.
func computeReplacementGasPrice(stuckGasPrice uint64, suggested *big.Int, bumpPercent uint64, maxGwei int64) *big.Int {
	bumped := new(big.Int).SetUint64(stuckGasPrice)
	bumped.Mul(bumped, new(big.Int).SetUint64(100+bumpPercent))
	bumped.Div(bumped, big.NewInt(100))
	if bumped.Cmp(suggested) < 0 {
		bumped = new(big.Int).Set(suggested)
	}
	return capToMaxGwei(bumped, maxGwei)
}
