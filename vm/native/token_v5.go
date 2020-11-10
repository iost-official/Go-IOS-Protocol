package native

import (
	"encoding/json"
	"errors"
	"math"

	"github.com/iost-official/go-iost/core/contract"
	"github.com/iost-official/go-iost/vm/host"
)

var tokenABIsV5 *abiSet

func init() {
	tokenABIsV5 = newAbiSet()
	tokenABIsV5.Register(initTokenABI, true)
	tokenABIsV5.Register(balanceOfTokenABI)
	tokenABIsV5.Register(supplyTokenABI)
	tokenABIsV5.Register(totalSupplyTokenABI)
	tokenABIsV5.Register(issueTokenABIV2)
	tokenABIsV5.Register(transferFreezeTokenABIV2) // this function need not be modified
	tokenABIsV5.Register(destroyTokenABIV2)
	tokenABIsV5.Register(transferTokenABIV3)

	tokenABIsV5.Register(createTokenABIV5)
}

var (
	createTokenABIV5 = &abi{
		name: "create",
		args: []string{"string", "string", "number", "json"},
		do: func(h *host.Host, args ...interface{}) (rtn []interface{}, cost contract.Cost, err error) {
			cost = contract.Cost0()
			cost.AddAssign(host.CommonOpCost(1))
			tokenSym := args[0].(string)
			issuer := args[1].(string)
			totalSupply := args[2].(int64)
			configJSON := args[3].([]byte)

			cost.AddAssign(host.CommonOpCost(1))
			err = checkTokenSymValid(tokenSym)
			if err != nil {
				return nil, cost, err
			}

			// config
			config := make(map[string]interface{})
			err = json.Unmarshal(configJSON, &config)
			cost.AddAssign(host.CommonOpCost(2))
			if err != nil {
				return nil, cost, err
			}
			decimal := 8
			canTransfer := true
			defaultRate := "1.0"
			fullName := tokenSym
			cost.AddAssign(host.CommonOpCost(3))
			onlyIssuerCanTransfer := false
			if tmp, ok := config[DecimalMapField]; ok {
				if _, ok = tmp.(float64); !ok {
					return nil, cost, errors.New("decimal in config should be number")
				}
				decimal = int(tmp.(float64))
			}
			if tmp, ok := config[CanTransferMapField]; ok {
				canTransfer, ok = tmp.(bool)
				if !ok {
					return nil, cost, errors.New("canTransfer in config should be bool")
				}
			}
			if tmp, ok := config[OnlyIssuerCanTransferMapField]; ok {
				onlyIssuerCanTransfer, ok = tmp.(bool)
				if !ok {
					return nil, cost, errors.New("onlyIssuerCanTransfer in config should be bool")
				}
			}
			if tmp, ok := config[DefaultRateMapField]; ok {
				defaultRate, ok = tmp.(string)
				if !ok {
					return nil, cost, errors.New("defaultRate in config should be string")
				}
			}
			if tmp, ok := config[FullNameMapField]; ok {
				fullName, ok = tmp.(string)
				if !ok {
					return nil, cost, errors.New("fullName in config should be string")
				}
				if len(fullName) > 50 {
					return nil, cost, errors.New("fullName is too long")
				}
			}
			if !CheckCost(h, cost) {
				return nil, cost, host.ErrOutOfGas
			}

			// check auth
			ok, cost0 := h.RequireAuth(issuer, TokenPermission)
			cost.AddAssign(cost0)
			if !ok {
				return nil, cost, host.ErrPermissionLost
			}
			if !CheckCost(h, cost) {
				return nil, cost, host.ErrOutOfGas
			}

			// check exists
			ok, cost0 = checkTokenExists(h, tokenSym)
			cost.AddAssign(cost0)
			if ok {
				return nil, cost, host.ErrTokenExists
			}

			// check valid
			if decimal < 0 || decimal >= 19 {
				return nil, cost, errors.New("invalid decimal")
			}
			if totalSupply > math.MaxInt64/int64(math.Pow10(decimal)) {
				return nil, cost, errors.New("invalid totalSupply")
			}
			totalSupply *= int64(math.Pow10(decimal))

			publisher := h.Context().Value("publisher").(string)
			// put info
			cost0, _ = h.MapPut(TokenInfoMapPrefix+tokenSym, IssuerMapField, issuer, publisher)
			cost.AddAssign(cost0)
			cost0, _ = h.MapPut(TokenInfoMapPrefix+tokenSym, TotalSupplyMapField, totalSupply, publisher)
			cost.AddAssign(cost0)
			cost0, _ = h.MapPut(TokenInfoMapPrefix+tokenSym, SupplyMapField, int64(0), publisher)
			cost.AddAssign(cost0)
			cost0, _ = h.MapPut(TokenInfoMapPrefix+tokenSym, CanTransferMapField, canTransfer, publisher)
			cost.AddAssign(cost0)
			cost0, _ = h.MapPut(TokenInfoMapPrefix+tokenSym, OnlyIssuerCanTransferMapField, onlyIssuerCanTransfer, publisher)
			cost.AddAssign(cost0)
			cost0, _ = h.MapPut(TokenInfoMapPrefix+tokenSym, DefaultRateMapField, defaultRate, publisher)
			cost.AddAssign(cost0)
			cost0, _ = h.MapPut(TokenInfoMapPrefix+tokenSym, DecimalMapField, int64(decimal), publisher)
			cost.AddAssign(cost0)
			cost0, _ = h.MapPut(TokenInfoMapPrefix+tokenSym, FullNameMapField, fullName, publisher)
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
