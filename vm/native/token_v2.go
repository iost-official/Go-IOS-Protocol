package native

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/iost-official/go-iost/v3/common"
	"github.com/iost-official/go-iost/v3/core/contract"
	"github.com/iost-official/go-iost/v3/vm/host"
)

// fix amount string
var tokenABIsV2 *abiSet

func init() {
	tokenABIsV2 = newAbiSet()
	tokenABIsV2.Register(initTokenABI, true)
	tokenABIsV2.Register(createTokenABI)
	tokenABIsV2.Register(balanceOfTokenABI)
	tokenABIsV2.Register(supplyTokenABI)
	tokenABIsV2.Register(totalSupplyTokenABI)

	// modified methods for V2
	tokenABIsV2.Register(issueTokenABIV2)
	tokenABIsV2.Register(transferTokenABIV2)
	tokenABIsV2.Register(transferFreezeTokenABIV2)
	tokenABIsV2.Register(destroyTokenABIV2)
}

func refineAmount(amountStr string, decimal int64) (string, error) {
	matched, err := regexp.MatchString("^([0-9]+[.])?[0-9]+$", amountStr)
	if err != nil || !matched {
		return "", fmt.Errorf("invalid amount %v %v", amountStr, err)
	}
	amount, err := common.NewDecimalFromString(amountStr, int(decimal))
	if err != nil {
		return "", fmt.Errorf("invalid amount %v %v", amountStr, err)
	}
	return amount.String(), nil
}

var (
	issueTokenABIV2 = &abi{
		name: "issue",
		args: []string{"string", "string", "string"},
		do: func(h *host.Host, args ...interface{}) (rtn []interface{}, cost contract.Cost, err error) {
			cost = contract.Cost0()
			cost.AddAssign(host.CommonOpCost(1))
			tokenSym := args[0].(string)
			to := args[1].(string)
			amountStr := args[2].(string)

			// get token info
			ok, cost0 := checkTokenExists(h, tokenSym)
			cost.AddAssign(cost0)
			if !ok {
				return nil, cost, host.ErrTokenNotExists
			}
			//refine amount
			decimal, cost0 := h.MapGet(TokenInfoMapPrefix+tokenSym, DecimalMapField)
			cost.AddAssign(cost0)
			amountStr, err = refineAmount(amountStr, decimal.(int64))
			if err != nil {
				return nil, cost, err
			}
			args[2] = amountStr

			issuer, cost0 := h.MapGet(TokenInfoMapPrefix+tokenSym, IssuerMapField)
			cost.AddAssign(cost0)
			supply, cost0 := h.MapGet(TokenInfoMapPrefix+tokenSym, SupplyMapField)
			cost.AddAssign(cost0)
			totalSupply, cost0 := h.MapGet(TokenInfoMapPrefix+tokenSym, TotalSupplyMapField)
			cost.AddAssign(cost0)
			if !CheckCost(h, cost) {
				return nil, cost, host.ErrOutOfGas
			}

			// check auth
			ok, cost0 = h.RequireAuth(issuer.(string), TokenPermission)
			cost.AddAssign(cost0)
			if !ok {
				return nil, cost, host.ErrPermissionLost
			}
			if !CheckCost(h, cost) {
				return nil, cost, host.ErrOutOfGas
			}

			// get amount by fixed point number
			amount, cost0, err := parseAmount(h, tokenSym, amountStr)
			cost.AddAssign(cost0)
			if err != nil {
				return nil, cost, err
			}
			if amount <= 0 {
				return nil, cost, host.ErrInvalidAmount
			}

			// check supply
			if totalSupply.(int64)-supply.(int64) < amount {
				return nil, cost, errors.New("supply too much")
			}
			if !CheckCost(h, cost) {
				return nil, cost, host.ErrOutOfGas
			}

			publisher := h.Context().Value("publisher").(string)
			// set supply, set balance
			cost0, err = h.MapPut(TokenInfoMapPrefix+tokenSym, SupplyMapField, supply.(int64)+amount)
			cost.AddAssign(cost0)
			if err != nil {
				return nil, cost, err
			}

			balance, cost0, err := getBalance(h, tokenSym, to, publisher)
			cost.AddAssign(cost0)
			if err != nil {
				return nil, cost, err
			}
			cost.AddAssign(cost0)

			balance += amount
			cost0 = setBalance(h, tokenSym, to, balance, publisher)
			cost.AddAssign(cost0)

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

	transferTokenABIV2 = &abi{
		name: "transfer",
		args: []string{"string", "string", "string", "string", "string"},
		do: func(h *host.Host, args ...interface{}) (rtn []interface{}, cost contract.Cost, err error) {
			cost = contract.Cost0()
			cost.AddAssign(host.CommonOpCost(1))
			tokenSym := args[0].(string)
			from := args[1].(string)
			to := args[2].(string)
			amountStr := args[3].(string)
			memo := args[4].(string) // memo
			if len(memo) > 512 {
				return nil, cost, host.ErrMemoTooLarge
			}
			if !h.IsValidAccount(from) {
				return nil, cost, fmt.Errorf("invalid account %v", from)
			}
			if !h.IsValidAccount(to) {
				return nil, cost, fmt.Errorf("invalid account %v", to)
			}

			//fmt.Printf("token transfer %v %v %v %v\n", tokenSym, from, to, amountStr)

			if from == to {
				return []interface{}{}, cost, nil
			}

			// get token info
			ok, cost0 := checkTokenExists(h, tokenSym)
			cost.AddAssign(cost0)
			if !ok {
				return nil, cost, host.ErrTokenNotExists
			}
			//refine amount
			decimal, cost0 := h.MapGet(TokenInfoMapPrefix+tokenSym, DecimalMapField)
			cost.AddAssign(cost0)
			amountStr, err = refineAmount(amountStr, decimal.(int64))
			if err != nil {
				return nil, cost, err
			}
			args[3] = amountStr

			canTransfer, cost0 := h.MapGet(TokenInfoMapPrefix+tokenSym, CanTransferMapField)
			cost.AddAssign(cost0)
			if !(canTransfer.(bool)) {
				return nil, cost, host.ErrTokenNoTransfer
			}
			onlyIssuerCanTransfer, cost0 := h.MapGet(TokenInfoMapPrefix+tokenSym, OnlyIssuerCanTransferMapField)
			cost.AddAssign(cost0)
			if onlyIssuerCanTransfer.(bool) {
				issuer, cost0 := h.MapGet(TokenInfoMapPrefix+tokenSym, IssuerMapField)
				cost.AddAssign(cost0)
				ok, cost0 = h.RequireAuth(issuer.(string), TransferPermission)
				cost.AddAssign(cost0)
				if !ok {
					return nil, cost, fmt.Errorf("transfer need issuer permission")
				}
			}
			if !CheckCost(h, cost) {
				return nil, cost, host.ErrOutOfGas
			}

			// check auth
			ok, cost0 = h.RequireAuth(from, TransferPermission)
			cost.AddAssign(cost0)
			if !ok {
				return nil, cost, host.ErrPermissionLost
			}
			if !CheckCost(h, cost) {
				return nil, cost, host.ErrOutOfGas
			}

			// get amount by fixed point number
			amount, cost0, err := parseAmount(h, tokenSym, amountStr)
			cost.AddAssign(cost0)
			if err != nil {
				return nil, cost, err
			}
			if amount <= 0 {
				return nil, cost, host.ErrInvalidAmount
			}
			if !CheckCost(h, cost) {
				return nil, cost, host.ErrOutOfGas
			}

			publisher := h.Context().Value("publisher").(string)
			// set balance
			fbalance, cost0, err := getBalance(h, tokenSym, from, publisher)
			cost.AddAssign(cost0)
			if err != nil {
				return nil, cost, err
			}
			tbalance, cost0, err := getBalance(h, tokenSym, to, publisher)
			cost.AddAssign(cost0)
			if err != nil {
				return nil, cost, err
			}
			if fbalance < amount {
				d, cost0 := h.MapGet(TokenInfoMapPrefix+tokenSym, DecimalMapField)
				decimal := int(d.(int64))
				cost.AddAssign(cost0)
				fBalanceFixed := &common.Decimal{Value: fbalance, Scale: decimal}
				amountFixed := &common.Decimal{Value: amount, Scale: decimal}
				return nil, cost, fmt.Errorf("balance not enough %v < %v", fBalanceFixed.String(), amountFixed.String())
			}
			if !CheckCost(h, cost) {
				return nil, cost, host.ErrOutOfGas
			}

			fbalance -= amount
			tbalance += amount

			cost0 = setBalance(h, tokenSym, to, tbalance, publisher)
			//fmt.Printf("transfer set %v %v %v\n", tokenSym, to, tbalance)
			cost.AddAssign(cost0)
			cost0 = setBalance(h, tokenSym, from, fbalance, publisher)
			cost.AddAssign(cost0)

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

	transferFreezeTokenABIV2 = &abi{
		name: "transferFreeze",
		args: []string{"string", "string", "string", "string", "number", "string"},
		do: func(h *host.Host, args ...interface{}) (rtn []interface{}, cost contract.Cost, err error) {
			cost = contract.Cost0()
			cost.AddAssign(host.CommonOpCost(1))
			tokenSym := args[0].(string)
			from := args[1].(string)
			to := args[2].(string)
			amountStr := args[3].(string)
			ftime := args[4].(int64) // time.Now().UnixNano()
			memo := args[5].(string) // memo
			if len(memo) > 512 {
				return nil, cost, host.ErrMemoTooLarge
			}

			// get token info
			ok, cost0 := checkTokenExists(h, tokenSym)
			cost.AddAssign(cost0)
			if !ok {
				return nil, cost, host.ErrTokenNotExists
			}

			//refine amount
			decimal, cost0 := h.MapGet(TokenInfoMapPrefix+tokenSym, DecimalMapField)
			cost.AddAssign(cost0)
			amountStr, err = refineAmount(amountStr, decimal.(int64))
			if err != nil {
				return nil, cost, err
			}
			args[3] = amountStr

			canTransfer, cost0 := h.MapGet(TokenInfoMapPrefix+tokenSym, CanTransferMapField)
			cost.AddAssign(cost0)
			if !(canTransfer.(bool)) {
				return nil, cost, host.ErrTokenNoTransfer
			}
			onlyIssuerCanTransfer, cost0 := h.MapGet(TokenInfoMapPrefix+tokenSym, OnlyIssuerCanTransferMapField)
			cost.AddAssign(cost0)
			if onlyIssuerCanTransfer.(bool) {
				issuer, cost0 := h.MapGet(TokenInfoMapPrefix+tokenSym, IssuerMapField)
				cost.AddAssign(cost0)
				ok, cost0 = h.RequireAuth(issuer.(string), TransferPermission)
				cost.AddAssign(cost0)
				if !ok {
					return nil, cost, fmt.Errorf("transfer need issuer permission")
				}
			}
			if !CheckCost(h, cost) {
				return nil, cost, host.ErrOutOfGas
			}

			// check auth
			ok, cost0 = h.RequireAuth(from, TransferPermission)
			cost.AddAssign(cost0)
			if !ok {
				return nil, cost, host.ErrPermissionLost
			}
			if !CheckCost(h, cost) {
				return nil, cost, host.ErrOutOfGas
			}

			// get amount by fixed point number
			amount, cost0, err := parseAmount(h, tokenSym, amountStr)
			cost.AddAssign(cost0)
			if err != nil {
				return nil, cost, err
			}
			if amount <= 0 {
				return nil, cost, host.ErrInvalidAmount
			}
			if !CheckCost(h, cost) {
				return nil, cost, host.ErrOutOfGas
			}

			publisher := h.Context().Value("publisher").(string)
			// sub balance of from
			fbalance, cost0, err := getBalance(h, tokenSym, from, publisher)
			cost.AddAssign(cost0)
			if err != nil {
				return nil, cost, err
			}
			if fbalance < amount {
				d, cost0 := h.MapGet(TokenInfoMapPrefix+tokenSym, DecimalMapField)
				decimal := int(d.(int64))
				cost.AddAssign(cost0)
				fBalanceFixed := &common.Decimal{Value: fbalance, Scale: decimal}
				amountFixed := &common.Decimal{Value: amount, Scale: decimal}
				return nil, cost, fmt.Errorf("balance not enough %v < %v", fBalanceFixed.String(), amountFixed.String())
			}

			fbalance -= amount
			cost0 = setBalance(h, tokenSym, from, fbalance, publisher)
			cost.AddAssign(cost0)
			if !CheckCost(h, cost) {
				return nil, cost, host.ErrOutOfGas
			}

			// freeze token of to
			cost0, err = freezeBalance(h, tokenSym, to, amount, ftime, publisher)
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

	destroyTokenABIV2 = &abi{
		name: "destroy",
		args: []string{"string", "string", "string"},
		do: func(h *host.Host, args ...interface{}) (rtn []interface{}, cost contract.Cost, err error) {
			cost = contract.Cost0()
			cost.AddAssign(host.CommonOpCost(1))
			tokenSym := args[0].(string)
			from := args[1].(string)
			amountStr := args[2].(string)

			// get token info
			ok, cost0 := checkTokenExists(h, tokenSym)
			cost.AddAssign(cost0)
			if !ok {
				return nil, cost, host.ErrTokenNotExists
			}

			//refine amount
			decimal, cost0 := h.MapGet(TokenInfoMapPrefix+tokenSym, DecimalMapField)
			cost.AddAssign(cost0)
			amountStr, err = refineAmount(amountStr, decimal.(int64))
			if err != nil {
				return nil, cost, err
			}
			args[2] = amountStr

			// check auth
			ok, cost0 = h.RequireAuth(from, TransferPermission)
			cost.AddAssign(cost0)
			if !ok {
				if !checkReserveToken(h, tokenSym) {
					return nil, cost, host.ErrPermissionLost
				}
			}
			if !CheckCost(h, cost) {
				return nil, cost, host.ErrOutOfGas
			}

			// get amount by fixed point number
			amount, cost0, err := parseAmount(h, tokenSym, amountStr)
			cost.AddAssign(cost0)
			if err != nil {
				return nil, cost, err
			}
			if amount <= 0 {
				return nil, cost, host.ErrInvalidAmount
			}
			if !CheckCost(h, cost) {
				return nil, cost, host.ErrOutOfGas
			}

			publisher := h.Context().Value("publisher").(string)
			// set balance
			fbalance, cost0, err := getBalance(h, tokenSym, from, publisher)
			cost.AddAssign(cost0)
			if err != nil {
				return nil, cost, err
			}
			if fbalance < amount {
				d, cost0 := h.MapGet(TokenInfoMapPrefix+tokenSym, DecimalMapField)
				decimal := int(d.(int64))
				cost.AddAssign(cost0)
				fBalanceFixed := &common.Decimal{Value: fbalance, Scale: decimal}
				amountFixed := &common.Decimal{Value: amount, Scale: decimal}
				return nil, cost, fmt.Errorf("balance not enough %v < %v", fBalanceFixed.String(), amountFixed.String())
			}
			fbalance -= amount
			cost0 = setBalance(h, tokenSym, from, fbalance, publisher)
			cost.AddAssign(cost0)
			if !CheckCost(h, cost) {
				return nil, cost, host.ErrOutOfGas
			}

			// set supply
			tmp, cost0 := h.MapGet(TokenInfoMapPrefix+tokenSym, SupplyMapField)
			supply := tmp.(int64)
			cost.AddAssign(cost0)

			supply -= amount
			cost0, err = h.MapPut(TokenInfoMapPrefix+tokenSym, SupplyMapField, supply)
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
