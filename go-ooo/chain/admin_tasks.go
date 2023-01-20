package chain

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"go-ooo/logger"
	go_ooo_types "go-ooo/types"
	"math/big"
)

func (o *OoORouterService) ProcessAdminTask(task go_ooo_types.AdminTask) go_ooo_types.AdminTaskResponse {

	err := o.RenewTransactOpts()
	if err != nil {
		logger.Error("chain", "ProcessAdminTask", "RenewTransactOpts", err.Error())

		return go_ooo_types.AdminTaskResponse{
			AdminTask: task,
			Success:   false,
			Error:     err.Error(),
		}
	}

	switch task.Task {
	case "register":
		return o.registerAsProvider(task)
	case "set_fee":
		return o.setGlobalFee(task)
	case "set_granular_fee":
		return o.setGranularFee(task)
	case "withdraw":
		return o.withdraw(task)
	case "query_withdrawable":
		return o.queryWithdrawable(task)
	case "query_fees":
		return o.queryFees(task)
	case "query_granular_fees":
		return o.queryGranularFees(task)
	default:
		return go_ooo_types.AdminTaskResponse{
			AdminTask: task,
			Error:     "unknown task",
		}
	}
}

func (o *OoORouterService) registerAsProvider(task go_ooo_types.AdminTask) go_ooo_types.AdminTaskResponse {

	var resp go_ooo_types.AdminTaskResponse
	resp.AdminTask = task

	fee := task.FeeOrAmount
	logger.Debug("chain", "registerAsProvider", "", "begin", logger.Fields{
		"address": o.oracleAddress.Hex(),
		"fee":     fee,
	})

	tx, err := o.contractInstance.RegisterAsProvider(o.transactOpts, big.NewInt(int64(fee)))
	if err != nil {
		logger.Error("chain", "registerAsProvider", "register", err.Error())
		resp.Error = err.Error()
		resp.Success = false
	} else {
		logger.InfoWithFields("chain", "registerAsProvider", "", "tx sent", logger.Fields{
			"address": o.oracleAddress.Hex(),
			"tx":      tx.Hash(),
		})

		o.setNextTxNonce(tx.Nonce(), false)

		resp.Result = fmt.Sprintf("Sent! Tx Hash: %s", tx.Hash().String())
		resp.Success = true
	}

	return resp
}

func (o *OoORouterService) setGlobalFee(task go_ooo_types.AdminTask) go_ooo_types.AdminTaskResponse {

	var resp go_ooo_types.AdminTaskResponse
	resp.AdminTask = task

	fee := task.FeeOrAmount

	logger.Debug("chain", "setGlobalFee", "", "begin", logger.Fields{
		"address": o.oracleAddress.Hex(),
		"fee":     fee,
	})

	tx, err := o.contractInstance.SetProviderMinFee(o.transactOpts, big.NewInt(int64(fee)))
	if err != nil {
		logger.ErrorWithFields("chain", "setGlobalFee", "register", err.Error(), logger.Fields{
			"address": o.oracleAddress.Hex(),
			"fee":     fee,
		})

		resp.Success = false
		resp.Error = err.Error()
	} else {
		logger.InfoWithFields("chain", "setGlobalFee", "", "tx sent", logger.Fields{
			"address": o.oracleAddress.Hex(),
			"fee":     fee,
			"tx":      tx.Hash(),
		})

		resp.Result = fmt.Sprintf("Sent! Tx Hash: %s", tx.Hash().String())
		resp.Success = true
		o.setNextTxNonce(tx.Nonce(), false)
	}

	return resp
}

func (o *OoORouterService) setGranularFee(task go_ooo_types.AdminTask) go_ooo_types.AdminTaskResponse {

	var resp go_ooo_types.AdminTaskResponse
	resp.AdminTask = task

	fee := task.FeeOrAmount
	consumer := task.ToOrConsumer
	logger.Debug("chain", "setGranularFee", "", "begin", logger.Fields{
		"address":  o.oracleAddress.Hex(),
		"fee":      fee,
		"consumer": consumer,
	})

	tx, err := o.contractInstance.SetProviderGranularFee(o.transactOpts, common.HexToAddress(consumer), big.NewInt(int64(fee)))
	if err != nil {
		logger.ErrorWithFields("chain", "setGranularFee", "set in contract", err.Error(), logger.Fields{
			"address":  o.oracleAddress.Hex(),
			"fee":      fee,
			"consumer": consumer,
		})
		resp.Error = err.Error()
		resp.Success = false
	} else {
		logger.InfoWithFields("chain", "setGranularFee", "", "tx sent", logger.Fields{
			"address":  o.oracleAddress.Hex(),
			"fee":      fee,
			"consumer": consumer,
			"tx":       tx.Hash(),
		})

		resp.Result = fmt.Sprintf("Sent! Tx Hash: %s", tx.Hash().String())
		resp.Success = true
		o.setNextTxNonce(tx.Nonce(), false)
	}

	return resp
}

func (o *OoORouterService) withdraw(task go_ooo_types.AdminTask) go_ooo_types.AdminTaskResponse {
	var resp go_ooo_types.AdminTaskResponse
	resp.AdminTask = task

	amountBig := big.NewInt(0)
	amountBig = amountBig.SetUint64(task.FeeOrAmount)
	recipient := task.ToOrConsumer

	logger.Debug("chain", "withdraw", "", "begin", logger.Fields{
		"recipient": recipient,
		"amount":    task.FeeOrAmount,
	})

	available, err := o.contractInstance.GetWithdrawableTokens(o.callOpts, o.oracleAddress)

	if err != nil {
		logger.ErrorWithFields("chain", "withdraw", "get withdrawable tokens", err.Error(), logger.Fields{
			"oracle_Wallet": o.oracleAddress,
		})

		resp.Error = err.Error()
		resp.Success = false
		return resp
	}

	if available.Cmp(amountBig) < 0 {
		resp.Error = fmt.Sprintf("cannot withdraw %s. Only %s available", amountBig.String(), available.String())
		resp.Success = false
		return resp
	}

	tx, err := o.contractInstance.Withdraw(o.transactOpts, common.HexToAddress(recipient), amountBig)
	if err != nil {
		logger.ErrorWithFields("chain", "withdraw", "send tx to contract", err.Error(), logger.Fields{
			"recipient": recipient,
			"amount":    task.FeeOrAmount,
		})

		resp.Error = err.Error()
		resp.Success = false
	} else {
		logger.InfoWithFields("chain", "withdraw", "", "tx sent", logger.Fields{
			"address":   o.oracleAddress.Hex(),
			"recipient": recipient,
			"amount":    task.FeeOrAmount,
			"tx":        tx.Hash(),
		})

		resp.Result = fmt.Sprintf("Sent! Tx Hash: %s", tx.Hash().String())
		resp.Success = true
		o.setNextTxNonce(tx.Nonce(), false)
	}

	return resp
}

func (o *OoORouterService) queryWithdrawable(task go_ooo_types.AdminTask) go_ooo_types.AdminTaskResponse {
	var resp go_ooo_types.AdminTaskResponse
	resp.AdminTask = task

	available, err := o.contractInstance.GetWithdrawableTokens(o.callOpts, o.oracleAddress)

	if err != nil {
		logger.Error("chain", "queryWithdrawable", "send query", err.Error())

		resp.Error = err.Error()
		resp.Success = false
	} else {
		resp.Result = fmt.Sprintf("amount available: %s", available.String())
		resp.Success = true
	}

	return resp
}

func (o *OoORouterService) queryFees(task go_ooo_types.AdminTask) go_ooo_types.AdminTaskResponse {
	var resp go_ooo_types.AdminTaskResponse
	resp.AdminTask = task

	fee, err := o.contractInstance.GetProviderMinFee(o.callOpts, o.oracleAddress)

	if err != nil {
		logger.Error("chain", "queryFees", "send query", err.Error())

		resp.Error = err.Error()
		resp.Success = false
	} else {
		feeAsBigFloat, _ := new(big.Float).SetString(fee.String())
		humanFee := new(big.Float).Quo(feeAsBigFloat, big.NewFloat(params.GWei))
		resp.Result = fmt.Sprintf("global fee: %s (%s)", fee.String(), humanFee.String())
		resp.Success = true
	}

	return resp
}

func (o *OoORouterService) queryGranularFees(task go_ooo_types.AdminTask) go_ooo_types.AdminTaskResponse {
	var resp go_ooo_types.AdminTaskResponse
	resp.AdminTask = task

	fee, err := o.contractInstance.GetProviderGranularFee(o.callOpts, o.oracleAddress, common.HexToAddress(task.ToOrConsumer))

	if err != nil {
		logger.Error("chain", "queryGranularFees", "send query", err.Error())
		resp.Error = err.Error()
		resp.Success = false
	} else {
		feeAsBigFloat, _ := new(big.Float).SetString(fee.String())
		humanFee := new(big.Float).Quo(feeAsBigFloat, big.NewFloat(params.GWei))
		resp.Result = fmt.Sprintf("granular fee for %s: %s (%s)", task.ToOrConsumer, fee.String(), humanFee.String())
		resp.Success = true
	}

	return resp
}
