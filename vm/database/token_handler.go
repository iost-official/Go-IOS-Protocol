package database

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/iost-official/go-iost/v3/common"
	"github.com/iost-official/go-iost/v3/ilog"
)

// TokenContractName name of basic token contract
const TokenContractName = "token.iost"

// TokenHandler easy to get balance of token.iost
type TokenHandler struct {
	db database
}

// FreezeItem represents freezed balance, will unfreeze after Ftime
type FreezeItem struct {
	Amount int64
	Ftime  int64
}

// FreezeItemFixed ...
type FreezeItemFixed struct {
	Amount common.Decimal
	Ftime  int64
}

func (m *TokenHandler) balanceKey(tokenName, acc string) string {
	return "m-" + TokenContractName + "-" + "TB" + acc + "-" + tokenName
}

func (m *TokenHandler) freezedBalanceKey(tokenName, acc string) string {
	return "m-" + TokenContractName + "-" + "TF" + acc + "-" + tokenName
}

func (m *TokenHandler) decimalKey(tokenName string) string {
	key := "m-" + TokenContractName + "-" + "TI" + tokenName + "-" + "decimal"
	return key
}

// TokenBalance get token balance of acc
func (m *TokenHandler) TokenBalance(tokenName, acc string) int64 {
	currentRaw := m.db.Get(m.balanceKey(tokenName, acc))
	balance := Unmarshal(currentRaw)
	ib, ok := balance.(int64)
	if !ok {
		ib = 0
	}
	return ib
}

// TokenBalanceFixed get token balance of acc
func (m *TokenHandler) TokenBalanceFixed(tokenName, acc string) *common.Decimal {
	ib := m.TokenBalance(tokenName, acc)
	return &common.Decimal{Value: ib, Scale: m.Decimal(tokenName)}
}

// FreezedTokenBalance get freezed token balance of acc
func (m *TokenHandler) FreezedTokenBalance(tokenName, acc string) int64 {
	freezeJSON := Unmarshal(m.db.Get(m.freezedBalanceKey(tokenName, acc)))
	if freezeJSON == nil {
		return 0
	}
	freezeList := []FreezeItem{}
	//fmt.Println(string(freezeJSON.(SerializedJSON)))
	err := json.Unmarshal([]byte(freezeJSON.(SerializedJSON)), &freezeList)
	if err != nil {
		return 0
	}
	ilog.Debugf("FreezedTokenBalance is %v %v %v", tokenName, acc, string([]byte(freezeJSON.(SerializedJSON))))

	ib := int64(0)
	for _, item := range freezeList {
		ib += item.Amount
	}
	return ib
}

// AllFreezedTokenBalance get freezed token balance of acc
func (m *TokenHandler) AllFreezedTokenBalance(tokenName, acc string) []FreezeItem {
	freezeList := make([]FreezeItem, 0)
	freezeJSON := Unmarshal(m.db.Get(m.freezedBalanceKey(tokenName, acc)))
	if freezeJSON == nil {
		return freezeList
	}
	//fmt.Println(string(freezeJSON.(SerializedJSON)))
	err := json.Unmarshal([]byte(freezeJSON.(SerializedJSON)), &freezeList)
	if err != nil {
		ilog.Errorf("frozen token balance is invalid json %v %v", string(freezeJSON.(SerializedJSON)), err)
		return freezeList
	}
	ilog.Debugf("FreezedTokenBalance is %v %v %v", tokenName, acc, string(freezeJSON.(SerializedJSON)))
	return freezeList
}

// AllFreezedTokenBalanceFixed get freezed token balance of acc
func (m *TokenHandler) AllFreezedTokenBalanceFixed(tokenName, acc string) []FreezeItemFixed {
	freezeList := m.AllFreezedTokenBalance(tokenName, acc)
	result := make([]FreezeItemFixed, 0)
	for _, item := range freezeList {
		result = append(result, FreezeItemFixed{Amount: common.Decimal{Value: item.Amount, Scale: m.Decimal(tokenName)}, Ftime: item.Ftime})
	}
	return result
}

// FreezedTokenBalanceFixed get token balance of acc
func (m *TokenHandler) FreezedTokenBalanceFixed(tokenName, acc string) *common.Decimal {
	ib := m.FreezedTokenBalance(tokenName, acc)
	return &common.Decimal{Value: ib, Scale: m.Decimal(tokenName)}
}

// SetTokenBalance set token balance of acc, used for test
func (m *TokenHandler) SetTokenBalance(tokenName, acc string, amount int64) {
	m.db.Put(m.balanceKey(tokenName, acc), MustMarshal(amount))
}

// SetTokenBalanceFixed set token balance of acc, used for test
func (m *TokenHandler) SetTokenBalanceFixed(tokenName, acc string, amountStr string) {
	amountNumber, err := common.NewDecimalFromString(amountStr, m.Decimal(tokenName))
	if err != nil {
		panic(errors.New("construct Fixed number failed. str = " + amountStr + ", decimal = " + fmt.Sprintf("%d", m.Decimal(tokenName))))
	}
	m.db.Put(m.balanceKey(tokenName, acc), MustMarshal(amountNumber.Value))
}

// Decimal get decimal in token info
func (m *TokenHandler) Decimal(tokenName string) int {
	decimalRaw := m.db.Get(m.decimalKey(tokenName))
	decimal, ok := Unmarshal(decimalRaw).(int64)
	if !ok {
		decimal = -1
	}
	return int(decimal)
}
