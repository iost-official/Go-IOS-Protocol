package native

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/iost-official/go-iost/v3/common"
	"github.com/iost-official/go-iost/v3/core/contract"
	"github.com/iost-official/go-iost/v3/vm/database"
	"github.com/iost-official/go-iost/v3/vm/host"
)

var tokenABIs *abiSet

// const prefix
const (
	TokenInfoMapPrefix            = "TI"
	TokenBalanceMapPrefix         = "TB"
	TokenFreezeMapPrefix          = "TF"
	IssuerMapField                = "issuer"
	SupplyMapField                = "supply"
	TotalSupplyMapField           = "totalSupply"
	CanTransferMapField           = "canTransfer"
	OnlyIssuerCanTransferMapField = "onlyIssuerCanTransfer"
	DefaultRateMapField           = "defaultRate"
	DecimalMapField               = "decimal"
	FullNameMapField              = "fullName"

	NamespacePrefix = "NS"
)

func init() {
	tokenABIs = newAbiSet()
	tokenABIs.Register(initTokenABI, true)
	tokenABIs.Register(createTokenABI)
	tokenABIs.Register(issueTokenABI)
	tokenABIs.Register(transferTokenABI)
	tokenABIs.Register(transferFreezeTokenABI)
	tokenABIs.Register(balanceOfTokenABI)
	tokenABIs.Register(supplyTokenABI)
	tokenABIs.Register(totalSupplyTokenABI)
	tokenABIs.Register(destroyTokenABI)
}

func checkTokenExists(h *host.Host, tokenSym string) (ok bool, cost contract.Cost) {
	exists, cost0 := h.MapHas(TokenInfoMapPrefix+tokenSym, IssuerMapField)
	return exists, cost0
}

func setBalance(h *host.Host, tokenSym string, from string, balance int64, ramPayer string) (cost contract.Cost) {
	ok, cost := h.MapHas(TokenBalanceMapPrefix+from, tokenSym)
	if ok {
		cost0, _ := h.MapPut(TokenBalanceMapPrefix+from, tokenSym, balance)
		cost.AddAssign(cost0)
	} else if (tokenSym == "iost" || tokenSym == "ram") && !strings.HasPrefix(from, "Contract") {
		cost0, _ := h.MapPut(TokenBalanceMapPrefix+from, tokenSym, balance)
		cost.AddAssign(cost0)
	} else {
		cost0, _ := h.MapPut(TokenBalanceMapPrefix+from, tokenSym, balance, ramPayer)
		cost.AddAssign(cost0)
	}
	return cost
}

func getBalance(h *host.Host, tokenSym string, from string, ramPayer string) (balance int64, cost contract.Cost, err error) {
	balance = int64(0)
	cost = contract.Cost0()
	ok, cost0 := h.MapHas(TokenBalanceMapPrefix+from, tokenSym)
	cost.AddAssign(cost0)
	if ok {
		tmp, cost0 := h.MapGet(TokenBalanceMapPrefix+from, tokenSym)
		cost.AddAssign(cost0)
		balance = tmp.(int64)
	}

	ok, cost0 = h.MapHas(TokenFreezeMapPrefix+from, tokenSym)
	cost.AddAssign(cost0)
	if !ok {
		return balance, cost, nil
	}

	ntime, cost0 := h.BlockTime()
	cost.AddAssign(cost0)

	freezeJSON, cost0 := h.MapGet(TokenFreezeMapPrefix+from, tokenSym)
	cost.AddAssign(cost0)
	freezeList := make([]database.FreezeItem, 0)

	err = json.Unmarshal([]byte(freezeJSON.(database.SerializedJSON)), &freezeList)
	cost.AddAssign(host.CommonOpCost(1))
	if err != nil {
		return balance, cost, err
	}

	addBalance := int64(0)
	i := 0
	for i < len(freezeList) {
		if freezeList[i].Ftime > ntime {
			break
		}
		addBalance += freezeList[i].Amount
		i++
	}
	cost.AddAssign(host.CommonOpCost(i))

	if addBalance > 0 {
		balance += addBalance
		cost0 = setBalance(h, tokenSym, from, balance, ramPayer)
		cost.AddAssign(cost0)
	}

	if i > 0 {
		freezeList = freezeList[i:]
		freezeJSON, err = json.Marshal(freezeList)
		cost.AddAssign(host.CommonOpCost(1))
		if err != nil {
			return balance, cost, err
		}
		cost0, err = h.MapPut(TokenFreezeMapPrefix+from, tokenSym, database.SerializedJSON(freezeJSON.([]byte)))
		cost.AddAssign(cost0)
		if err != nil {
			return balance, cost, err
		}
	}

	return balance, cost, nil
}

func freezeBalance(h *host.Host, tokenSym string, from string, balance int64, ftime int64, ramPayer string) (cost contract.Cost, err error) {
	ok, cost := h.MapHas(TokenFreezeMapPrefix+from, tokenSym)
	freezeList := make([]database.FreezeItem, 0)
	if ok {
		freezeJSON, cost0 := h.MapGet(TokenFreezeMapPrefix+from, tokenSym)
		cost.AddAssign(cost0)
		err = json.Unmarshal([]byte(freezeJSON.(database.SerializedJSON)), &freezeList)
		cost.AddAssign(host.CommonOpCost(1))
		if err != nil {
			return cost, err
		}
	}

	freezeList = append(freezeList, database.FreezeItem{Amount: balance, Ftime: ftime})
	sort.Slice(freezeList, func(i, j int) bool {
		return freezeList[i].Ftime < freezeList[j].Ftime ||
			freezeList[i].Ftime == freezeList[j].Ftime && freezeList[i].Amount < freezeList[j].Amount
	})
	cost.AddAssign(host.CommonOpCost(len(freezeList)))

	freezeJSON, err := json.Marshal(freezeList)
	cost.AddAssign(host.CommonOpCost(1))
	if err != nil {
		return cost, nil
	}
	cost0, err := h.MapPut(TokenFreezeMapPrefix+from, tokenSym, database.SerializedJSON(freezeJSON), ramPayer)
	cost.AddAssign(cost0)
	if err != nil {
		return cost, err
	}

	return cost, nil
}

func parseAmount(h *host.Host, tokenSym string, amountStr string) (amount int64, cost contract.Cost, err error) {
	decimal, cost := h.MapGet(TokenInfoMapPrefix+tokenSym, DecimalMapField)
	amountNumber, err := common.NewDecimalFromString(amountStr, int(decimal.(int64)))

	cost.AddAssign(host.CommonOpCost(3))
	if err != nil {
		return 0, cost, fmt.Errorf("invalid amount %v %v", amountStr, err)
	}
	return amountNumber.Value, cost, err
}

func genAmount(h *host.Host, tokenSym string, amount int64) (amountStr string, cost contract.Cost) {
	decimal, cost := h.MapGet(TokenInfoMapPrefix+tokenSym, DecimalMapField)
	amountNumber := common.Decimal{Value: amount, Scale: int(decimal.(int64))}
	cost.AddAssign(host.CommonOpCost(1))
	return amountNumber.String(), cost
}

func checkTokenSymValid(symbol string) error {
	if len(symbol) < 2 || len(symbol) > 16 {
		return fmt.Errorf("token symbol invalid. token symbol length should be between 2,16 got %v", symbol)
	}
	for _, ch := range symbol {
		if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '_') {
			return fmt.Errorf("token symbol invalid. token symbol contains invalid character %v", ch)
		}
	}
	return nil
}

var (
	initTokenABI = &abi{
		name: "init",
		args: []string{},
		do: func(h *host.Host, args ...interface{}) (rtn []interface{}, cost contract.Cost, err error) {
			return []interface{}{}, host.CommonErrorCost(1), nil
		},
	}
	createTokenABI = &abi{
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

	issueTokenABI = &abi{
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

	transferTokenABI = &abi{
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

	transferFreezeTokenABI = &abi{
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

	destroyTokenABI = &abi{
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

	balanceOfTokenABI = &abi{
		name: "balanceOf",
		args: []string{"string", "string"},
		do: func(h *host.Host, args ...interface{}) (rtn []interface{}, cost contract.Cost, err error) {
			cost = contract.Cost0()
			cost.AddAssign(host.CommonOpCost(1))
			tokenSym := args[0].(string)
			to := args[1].(string)

			// check token info
			ok, cost0 := checkTokenExists(h, tokenSym)
			cost.AddAssign(cost0)
			if !ok {
				return nil, cost, host.ErrTokenNotExists
			}

			publisher := h.Context().Value("publisher").(string)
			balance, cost0, err := getBalance(h, tokenSym, to, publisher)
			cost.AddAssign(cost0)
			if err != nil {
				return nil, cost, err
			}
			if !CheckCost(h, cost) {
				return nil, cost, host.ErrOutOfGas
			}

			balanceStr, cost0 := genAmount(h, tokenSym, balance)

			cost.AddAssign(cost0)

			return []interface{}{balanceStr}, cost, nil
		},
	}

	supplyTokenABI = &abi{
		name: "supply",
		args: []string{"string"},
		do: func(h *host.Host, args ...interface{}) (rtn []interface{}, cost contract.Cost, err error) {
			cost = contract.Cost0()
			cost.AddAssign(host.CommonOpCost(1))
			tokenSym := args[0].(string)

			// check token info
			ok, cost0 := checkTokenExists(h, tokenSym)
			cost.AddAssign(cost0)
			if !ok {
				return nil, cost, host.ErrTokenNotExists
			}
			if !CheckCost(h, cost) {
				return nil, cost, host.ErrOutOfGas
			}

			supply, cost0 := h.MapGet(TokenInfoMapPrefix+tokenSym, SupplyMapField)
			cost.AddAssign(cost0)
			supplyStr, cost0 := genAmount(h, tokenSym, supply.(int64))
			cost.AddAssign(cost0)

			return []interface{}{supplyStr}, cost, nil
		},
	}

	totalSupplyTokenABI = &abi{
		name: "totalSupply",
		args: []string{"string"},
		do: func(h *host.Host, args ...interface{}) (rtn []interface{}, cost contract.Cost, err error) {
			cost = contract.Cost0()
			cost.AddAssign(host.CommonOpCost(1))
			tokenSym := args[0].(string)

			// check token info
			ok, cost0 := checkTokenExists(h, tokenSym)
			cost.AddAssign(cost0)
			if !ok {
				return nil, cost, host.ErrTokenNotExists
			}

			totalSupply, cost0 := h.MapGet(TokenInfoMapPrefix+tokenSym, TotalSupplyMapField)
			cost.AddAssign(cost0)
			totalSupplyStr, cost0 := genAmount(h, tokenSym, totalSupply.(int64))
			cost.AddAssign(cost0)

			return []interface{}{totalSupplyStr}, cost, nil
		},
	}
)
