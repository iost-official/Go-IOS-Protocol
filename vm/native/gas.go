package native

import (
	"encoding/json"
	"fmt"

	"github.com/iost-official/go-iost/v3/common"
	"github.com/iost-official/go-iost/v3/core/contract"
	"github.com/iost-official/go-iost/v3/ilog"
	"github.com/iost-official/go-iost/v3/vm/database"
	"github.com/iost-official/go-iost/v3/vm/host"
)

// UnpledgeFreezeSeconds coins will be frozen for 3 days after being unpledged
const UnpledgeFreezeSeconds int64 = 3 * 24 * 3600

var gasABIs *abiSet

// GasContractName the contract name
const GasContractName = "gas.iost"

func init() {
	gasABIs = newAbiSet()
	gasABIs.Register(initFunc, true)
	gasABIs.Register(constructor)
	gasABIs.Register(pledgeGas)
	gasABIs.Register(unpledgeGas)
}

// Pledge Change all gas related storage here. If pledgeAmount > 0. pledge. If pledgeAmount < 0, unpledge.
func pledge(h *host.Host, pledger string, name string, pledgeAmountF *common.Decimal) (contract.Cost, error) { // nolint:gocyclo
	finalCost := contract.Cost0()
	if pledgeAmountF.IsZero() {
		return finalCost, fmt.Errorf("invalid pledge amount %v", pledgeAmountF.String())
	}
	if pledgeAmountF.IsNegative() {
		// do some checking
		unpledgeAmount := pledgeAmountF.Neg()
		// check total pledge is valid
		oldTotalPledge, cost := h.GasPledgeTotal(name)
		finalCost.AddAssign(cost)
		if oldTotalPledge.Sub(unpledgeAmount).LessThan(database.GasMinPledgeOfUser) {
			return finalCost, fmt.Errorf("unpledge too much (%v - %v) less than %v", oldTotalPledge.String(), unpledgeAmount.String(), database.GasMinPledgeOfUser.String())
		}
		// check personal pledge
		pledged, cost := h.GasManager.GasPledge(name, pledger)
		finalCost.AddAssign(cost)
		newPledge := pledged.Sub(unpledgeAmount)
		if newPledge.IsNegative() {
			return finalCost, fmt.Errorf("you cannot unpledge more than your pledge %v > %v", unpledgeAmount.String(), pledged.String())
		}
	}

	limitDelta := pledgeAmountF.Mul(database.GasLimit)
	totalDelta := pledgeAmountF
	gasDelta := pledgeAmountF.Mul(database.GasImmediateReward)
	if gasDelta == nil {
		// this line is compatible, since 'gasDelta' was never nil
		return finalCost, fmt.Errorf("gas overflow")
	}
	if pledgeAmountF.IsNegative() {
		// unpledge should not change current generated gas
		gasDelta = database.EmptyGas()
	}
	//fmt.Printf("limitd rated gasd %v %v %v\n", limitDelta, totalDelta, gasDelta)

	// pledge first time
	t, cost := h.GasManager.GasUpdateTime(name)
	finalCost.AddAssign(cost)
	if t == 0 {
		if pledgeAmountF.IsNegative() {
			return finalCost, fmt.Errorf("cannot unpledge! No pledge before")
		}
		cost = h.GasManager.SetGasPledge(name, pledger, pledgeAmountF)
		finalCost.AddAssign(cost)
		cost = h.GasManager.SetGasUpdateTime(name, h.Context().Value("time").(int64))
		finalCost.AddAssign(cost)
		cost = h.GasManager.SetGasPledgeTotal(name, totalDelta)
		finalCost.AddAssign(cost)
		cost = h.GasManager.SetGasLimit(name, limitDelta)
		finalCost.AddAssign(cost)
		cost = h.GasManager.SetGasStock(name, gasDelta)
		finalCost.AddAssign(cost)
		return finalCost, nil
	}
	cost, _ = h.GasManager.RefreshPGas(name)
	finalCost.AddAssign(cost)

	// change pledge total
	totalOld, cost := h.GasManager.GasPledgeTotal(name)
	finalCost.AddAssign(cost)
	totalNew := totalOld.Add(totalDelta)
	if totalNew.Value <= 0 {
		return finalCost, fmt.Errorf("change gasPledgeTotal failed! current: %v, delta %v", totalOld.String(), totalDelta.String())
	}
	cost = h.GasManager.SetGasPledgeTotal(name, totalNew)
	finalCost.AddAssign(cost)
	// change limit
	limitOld, cost := h.GasManager.GasLimit(name)
	finalCost.AddAssign(cost)
	limitNew := limitOld.Add(limitDelta)
	if limitNew == nil {
		// this line is compatible, since 'limitNew' was never nil
		limitNew = limitOld.Rescale(2).Add(limitDelta.Rescale(2))
	}
	if limitNew.Value <= 0 {
		return finalCost, fmt.Errorf("change gasLimit failed! current: %v, delta %v", limitOld.String(), limitDelta.String())
	}
	// gas limit will never overflow
	cost = h.GasManager.SetGasLimit(name, limitNew)
	finalCost.AddAssign(cost)
	// change stock
	gasOld, cost := h.GasManager.GasStock(name)
	finalCost.AddAssign(cost)
	gasNew := gasOld.Add(gasDelta)
	if gasNew == nil {
		// this line is compatible, since 'gasNew' was never nil
		gasNew = gasOld.Rescale(2).Add(gasDelta.Rescale(2))
	}
	if limitNew.LessThan(gasNew) {
		// clear the gas above the new limit.
		gasNew = limitNew
	}
	cost = h.GasManager.SetGasStock(name, gasNew)
	finalCost.AddAssign(cost)
	// change personal pledge
	pledged, cost := h.GasManager.GasPledge(name, pledger)
	finalCost.AddAssign(cost)
	newPledge := pledged.Add(totalDelta)
	if newPledge.IsZero() {
		cost = h.GasManager.DelGasPledge(name, pledger)
		finalCost.AddAssign(cost)
	} else if newPledge.IsPositive() {
		cost = h.GasManager.SetGasPledge(name, pledger, newPledge)
		finalCost.AddAssign(cost)
	} else {
		ilog.Fatalf("should not reach here pledger %v name %v pledgeAmountF %v pledged %v", pledger, name, pledgeAmountF, pledged)
	}
	return finalCost, nil
}

var ( // nolint: deadcode
	constructor = &abi{
		name: "constructor",
		args: []string{},
		do: func(h *host.Host, args ...interface{}) (rtn []interface{}, cost contract.Cost, err error) {
			return []interface{}{}, host.CommonErrorCost(1), nil
		},
	}
	initFunc = &abi{
		name: "init",
		args: []string{},
		do: func(h *host.Host, args ...interface{}) (rtn []interface{}, cost contract.Cost, err error) {
			return []interface{}{}, host.CommonErrorCost(1), nil
		},
	}
	pledgeGas = &abi{
		name: "pledge",
		args: []string{"string", "string", "string"},
		do: func(h *host.Host, args ...interface{}) (rtn []interface{}, cost contract.Cost, err error) {
			cost = contract.Cost0()
			pledger, ok := args[0].(string)
			cost.AddAssign(host.CommonErrorCost(1))
			if !ok || !h.IsValidAccount(pledger) {
				return nil, cost, fmt.Errorf("invalid user name %s", args[0])
			}
			gasUser, ok := args[1].(string)
			cost.AddAssign(host.CommonErrorCost(1))
			if !ok || !h.IsValidAccount(gasUser) {
				return nil, cost, fmt.Errorf("invalid user name %s", args[1])
			}
			auth, cost0 := h.RequireAuth(pledger, TransferPermission)
			cost.AddAssign(cost0)
			if !auth {
				return nil, cost, host.ErrPermissionLost
			}
			pledgeAmountStr, ok := args[2].(string)
			if !ok {
				return nil, cost, fmt.Errorf("invalid amount %s", args[2])
			}
			pledgeAmount, err := common.NewDecimalFromString(pledgeAmountStr, 8)
			cost.AddAssign(host.CommonErrorCost(1))
			if err != nil || pledgeAmount.Value <= 0 {
				return nil, cost, fmt.Errorf("invalid amount %s", args[2])
			}
			if pledgeAmount.LessThan(database.GasMinPledgePerAction) {
				return nil, cost, fmt.Errorf("min pledge num is %d", database.GasMinPledgePerAction)
			}
			cost0, err = pledge(h, pledger, gasUser, pledgeAmount)
			cost.AddAssign(cost0)
			if err != nil {
				return nil, cost, err
			}
			contractName, cost0 := h.ContractName()
			cost.AddAssign(cost0)
			_, cost0, err = h.Call("token.iost", "transfer", fmt.Sprintf(`["iost", "%v", "%v", "%v", ""]`, pledger, contractName, pledgeAmountStr))
			cost.AddAssign(cost0)
			if err != nil {
				return nil, cost, err
			}

			// generate receipt
			message, err := json.Marshal(args)
			cost.AddAssign(host.CommonOpCost(1))
			if err != nil {
				return nil, cost, err
			}
			cost0 = h.Receipt(string(message))
			cost.AddAssign(cost0)
			return []interface{}{}, cost, nil
		},
	}
	unpledgeGas = &abi{
		name: "unpledge",
		args: []string{"string", "string", "string"},
		do: func(h *host.Host, args ...interface{}) (rtn []interface{}, cost contract.Cost, err error) {
			cost = contract.Cost0()
			pledger, ok := args[0].(string)
			cost.AddAssign(host.CommonErrorCost(1))
			if !ok || !h.IsValidAccount(pledger) {
				return nil, cost, fmt.Errorf("invalid user name %s", args[1])
			}
			gasUser, ok := args[1].(string)
			cost.AddAssign(host.CommonErrorCost(1))
			if !ok || !h.IsValidAccount(gasUser) {
				return nil, cost, fmt.Errorf("invalid user name %s", args[0])
			}
			auth, cost0 := h.RequireAuth(pledger, TransferPermission)
			cost.AddAssign(cost0)
			if !auth {
				return nil, cost, host.ErrPermissionLost
			}
			unpledgeAmountStr, ok := args[2].(string)
			if !ok {
				return nil, cost, fmt.Errorf("invalid amount %s", args[2])
			}
			unpledgeAmount, err := common.NewDecimalFromString(unpledgeAmountStr, 8)
			cost.AddAssign(host.CommonErrorCost(1))
			if err != nil || unpledgeAmount.Value <= 0 {
				return nil, cost, fmt.Errorf("invalid amount %s", args[2])
			}
			if unpledgeAmount.LessThan(database.GasMinPledgePerAction) {
				return nil, cost, fmt.Errorf("min unpledge num is %d", database.GasMinPledgePerAction)
			}
			pledged, cost0 := h.GasManager.GasPledge(gasUser, pledger)
			cost.AddAssign(cost0)
			if pledged.IsZero() {
				return nil, cost, fmt.Errorf("%v did not pledge for %v", pledger, gasUser)
			}

			cost0, err = pledge(h, pledger, gasUser, unpledgeAmount.Neg())
			cost.AddAssign(cost0)
			if err != nil {
				return nil, cost, err
			}

			contractName, cost0 := h.ContractName()
			cost.AddAssign(cost0)
			freezeTime := h.Context().Value("time").(int64) + UnpledgeFreezeSeconds*1e9
			_, cost0, err = h.CallWithAuth("token.iost", "transferFreeze",
				fmt.Sprintf(`["iost", "%v", "%v", "%v", %v, ""]`, contractName, pledger, unpledgeAmount.String(), freezeTime))
			cost.AddAssign(cost0)

			if err != nil {
				return nil, cost, err
			}

			// generate receipt
			message, err := json.Marshal(args)
			cost.AddAssign(host.CommonOpCost(1))
			if err != nil {
				return nil, cost, err
			}
			cost0 = h.Receipt(string(message))
			cost.AddAssign(cost0)
			return []interface{}{}, cost, nil
		},
	}
)
