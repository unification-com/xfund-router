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

// suggestGasPrice asks the node for a gas price, capped at cfg.Chain.MaxGasPrice
// (in gwei) when that is configured.
func (o *OoORouterService) suggestGasPrice() (*big.Int, error) {
	gasPrice, err := o.client.SuggestGasPrice(o.context)
	if err != nil {
		return nil, fmt.Errorf("suggest gas price: %w", err)
	}

	if maxConf := o.cfg.Chain.MaxGasPrice; maxConf > 0 {
		maxGasPrice := new(big.Int).Mul(big.NewInt(maxConf), big.NewInt(params.GWei))
		if gasPrice.Cmp(maxGasPrice) > 0 {
			gasPrice = maxGasPrice
		}
	}
	return gasPrice, nil
}
