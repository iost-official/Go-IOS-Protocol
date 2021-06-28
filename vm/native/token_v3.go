package native

import (
	"encoding/json"
	"fmt"

	"github.com/iost-official/go-iost/v3/common"
	"github.com/iost-official/go-iost/v3/core/contract"
	"github.com/iost-official/go-iost/v3/vm/host"
)

var tokenABIsV3 *abiSet

func init() {
	tokenABIsV3 = newAbiSet()
	tokenABIsV3.Register(initTokenABI, true)
	tokenABIsV3.Register(createTokenABI)
	tokenABIsV3.Register(balanceOfTokenABI)
	tokenABIsV3.Register(supplyTokenABI)
	tokenABIsV3.Register(totalSupplyTokenABI)

	// modified methods for V2
	tokenABIsV3.Register(issueTokenABIV2)
	tokenABIsV3.Register(transferFreezeTokenABIV2) // this function need not be modified
	tokenABIsV3.Register(destroyTokenABIV2)

	// modified methods for V3
	tokenABIsV3.Register(transferTokenABIV3)
}

var (
	transferTokenABIV3 = &abi{
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

			// change 'from' balance
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

			// change 'to' balance
			tbalance, cost0, err := getBalance(h, tokenSym, to, publisher)
			cost.AddAssign(cost0)
			if err != nil {
				return nil, cost, err
			}
			tbalance += amount
			cost0 = setBalance(h, tokenSym, to, tbalance, publisher)
			//fmt.Printf("transfer set %v %v %v\n", tokenSym, to, tbalance)
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
)
