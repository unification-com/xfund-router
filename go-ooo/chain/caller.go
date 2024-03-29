package chain

import (
	"github.com/ethereum/go-ethereum/params"
	"go-ooo/logger"
	"math/big"
)

func (o *OoORouterService) setNextTxNonce(nonce uint64, isFromPending bool) {
	nextNonce := nonce
	if !isFromPending {
		nextNonce += 1
	}

	// only set if it's + 1
	if nextNonce-o.prevTxNonce == 1 {

		logger.Debug("chain", "setNextTxNonce", "", "", logger.Fields{
			"prev_nonce": o.prevTxNonce,
			"nonce_in":   nonce,
			"next_nonce": nextNonce,
			"is_pending": isFromPending,
		})

		o.prevTxNonce = nextNonce
		o.transactOpts.Nonce = big.NewInt(int64(nextNonce))
	}
}

func (o *OoORouterService) GetOnChainPendingNonce() (uint64, error) {
	return o.client.PendingNonceAt(o.context, o.oracleAddress)
}

func (o *OoORouterService) RenewTransactOpts() error {

	nonce, err := o.client.PendingNonceAt(o.context, o.oracleAddress)
	if err != nil {
		return err
	}

	o.setNextTxNonce(nonce, true)

	gasPrice, err := o.client.SuggestGasPrice(o.context)
	if err != nil {
		return err
	}
	o.transactOpts.GasPrice = gasPrice

	maxGasPriceConf := o.cfg.Chain.MaxGasPrice

	if maxGasPriceConf > 0 {
		maxGasPrice := big.NewInt(0).Mul(big.NewInt(maxGasPriceConf), big.NewInt(params.GWei))
		if gasPrice.Cmp(maxGasPrice) > 0 {
			o.transactOpts.GasPrice = maxGasPrice
		}
	}

	return nil
}
