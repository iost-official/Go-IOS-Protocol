package database

import (
	"github.com/iost-official/go-iost/v3/common"
	"github.com/iost-official/go-iost/v3/ilog"
)

const (
	// GasPledgeTotalKey : how many IOST is pledged
	GasPledgeTotalKey = "gt"
	// GasLimitKey : how much gas can be generated max per IOST
	GasLimitKey = "gl"
	// GasUpdateTimeKey : when the gas state is refreshed last time, for internal use
	GasUpdateTimeKey = "gu"
	// GasStockKey : how much gas is there when last time refreshed
	GasStockKey = "gs"
	// GasPledgeKey : i pledge how much coins for others
	GasPledgeKey = "gp"
)

// decimals of gas
const (
	GasDecimal = 2
)
const gasMaxIncreaseSeconds = 3 * 24 * 3600

// Each IOST you pledge, you will get `GasImmediateReward` gas immediately.
// Then gas will be generated at a rate of `GasIncreaseRate` gas per second.
// Then it takes `GasFulfillSeconds` time to reach the limit.
// Your gas production will stop when it reaches the limit.
// When you use some gas later, the total amount will be less than the limit,
// so gas production will resume again util the limit.

// GasMinPledgeOfUser Each user must pledge a minimum amount of IOST
var GasMinPledgeOfUser = &common.Fixed{Value: 10 * IOSTRatio, Decimal: 8}

// GasMinPledgePerAction One must (un)pledge more than 1 IOST
var GasMinPledgePerAction = &common.Fixed{Value: 1 * IOSTRatio, Decimal: 8}

// GasImmediateReward immediate reward per IOST
var GasImmediateReward = &common.Fixed{Value: 100000 * 100, Decimal: 2}

// GasLimit gas limit per IOST
var GasLimit = &common.Fixed{Value: 300000 * 100, Decimal: 2}

// GasFulfillSeconds it takes 2 days to fulfill the gas buffer.
const GasFulfillSeconds int64 = 2 * 24 * 3600

// GasIncreaseRate gas increase per IOST per second
var GasIncreaseRate = GasLimit.Sub(GasImmediateReward).Div(GasFulfillSeconds)

// IOSTRatio ...
const IOSTRatio int64 = 100000000

// PledgerInfo ...
type PledgerInfo struct {
	Pledger string
	Amount  *common.Fixed
}

// GasContractName name of basic token contract
const GasContractName = "gas.iost"

// GasHandler easy to get balance of token.iost
type GasHandler struct {
	BasicHandler
	MapHandler
}

// EmptyGas ...
func EmptyGas() *common.Fixed {
	return &common.Fixed{
		Value:   0,
		Decimal: GasDecimal,
	}
}

// If no key exists, return 0
func (g *GasHandler) getFixed(key string) *common.Fixed {
	result := MustUnmarshal(g.BasicHandler.Get(GasContractName + Separator + key))
	if result == nil {
		//ilog.Errorf("GasHandler failed %v", key)
		return nil
	}
	value, ok := result.(*common.Fixed)
	if !ok {
		ilog.Errorf("GasHandler failed %v %v", key, result)
		return nil
	}
	return value
}

// GasPledgeTotal ...
func (g *GasHandler) GasPledgeTotal(name string) *common.Fixed {
	f := g.getFixed(name + GasPledgeTotalKey)
	if f == nil {
		return EmptyGas()
	}
	return f
}

// GasLimit ...
func (g *GasHandler) GasLimit(name string) *common.Fixed {
	f := g.getFixed(name + GasLimitKey)
	if f == nil {
		return EmptyGas()
	}
	return f
}

// GasUpdateTime ...
func (g *GasHandler) GasUpdateTime(name string) int64 {
	value := MustUnmarshal(g.BasicHandler.Get(GasContractName + Separator + name + GasUpdateTimeKey))
	if value == nil {
		return 0
	}
	return value.(int64)
}

// GasStock `gasStock` means the gas amount at last update time.
func (g *GasHandler) GasStock(name string) *common.Fixed {
	f := g.getFixed(name + GasStockKey)
	if f == nil {
		return EmptyGas()
	}
	return f
}

// GasPledge ...
func (g *GasHandler) GasPledge(name string, pledger string) *common.Fixed {
	ok := g.MapHandler.MHas(GasContractName+Separator+pledger+GasPledgeKey, name)
	if !ok {
		return &common.Fixed{
			Value:   0,
			Decimal: 8,
		}
	}
	result := MustUnmarshal(g.MapHandler.MGet(GasContractName+Separator+pledger+GasPledgeKey, name))
	value, ok := result.(*common.Fixed)
	if !ok {
		return nil
	}
	return value
}

// PledgerInfo get I pledged how much coins for others
func (g *GasHandler) PledgerInfo(name string) []PledgerInfo {
	pledgees := g.MapHandler.MKeys(GasContractName + Separator + name + GasPledgeKey)
	result := make([]PledgerInfo, 0)
	usedPledgees := make(map[string]bool)
	for _, pledgee := range pledgees {
		if _, ok := usedPledgees[pledgee]; ok {
			continue
		}
		usedPledgees[pledgee] = true
		s := g.MapHandler.MGet(GasContractName+Separator+name+GasPledgeKey, pledgee)
		v := MustUnmarshal(s)
		pledge, ok := v.(*common.Fixed)
		if !ok || pledge.IsZero() {
			continue
		}
		result = append(result, PledgerInfo{pledgee, pledge})
	}
	return result
}

// PGasAtTime return pledge gas at given time. It is min(limit, last_updated_gas + time_since_last_updated * increase_speed)
func (g *GasHandler) PGasAtTime(name string, t int64) (result *common.Fixed) {
	if t <= 0 {
		ilog.Fatalf("PGasAtTime failed. invalid t time %v", t)
	}
	result = g.GasStock(name)
	gasUpdateTime := g.GasUpdateTime(name)
	var durationSeconds float64
	if gasUpdateTime > 0 {
		durationSeconds = float64(t-gasUpdateTime) / float64(1e9)
		if durationSeconds > gasMaxIncreaseSeconds {
			durationSeconds = gasMaxIncreaseSeconds
		}
	}
	if durationSeconds < 0 {
		ilog.Errorf("PGasAtTime durationSeconds invalid %v = %v - %v", durationSeconds, t, gasUpdateTime)
	}
	rate := g.GasPledgeTotal(name).Multiply(GasIncreaseRate)
	if rate == nil {
		// this line is compatible, since 'rate' was never nil
		rate = g.GasPledgeTotal(name).ChangeDecimal(0).Multiply(GasIncreaseRate)
	}
	limit := g.GasLimit(name)
	//fmt.Printf("PGasAtTime user %v stock %v rate %v limit %v durationSeconds %v\n", name, result, rate, limit, durationSeconds)
	delta := rate.TimesF(durationSeconds)
	if delta == nil {
		ilog.Errorf("PGasAtTime may overflow rate %v durationSeconds %v", rate, durationSeconds)
		return
	}
	finalResult := delta.Add(result)
	if finalResult == nil {
		// this line is compatible, since 'finalResult' was never nil
		ilog.Errorf("PGasAtTime may overflow result %v delta %v", result, delta)
		return
	}
	if limit.LessThan(finalResult) {
		result = limit
	} else {
		result = finalResult
	}
	result = result.ChangeDecimal(GasDecimal)
	return
}

// TotalGasAtTime return total gas at given time..
func (g *GasHandler) TotalGasAtTime(name string, t int64) (result *common.Fixed) {
	return g.PGasAtTime(name, t).ChangeDecimal(GasDecimal)
}
