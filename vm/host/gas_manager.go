package host

import (
	"fmt"

	"github.com/iost-official/go-iost/v3/vm/database"

	"github.com/iost-official/go-iost/v3/common"
	"github.com/iost-official/go-iost/v3/core/contract"
	"github.com/iost-official/go-iost/v3/ilog"
)

// GasManager handle the logic of gas. It should be called in a contract
type GasManager struct {
	h *Host
}

// NewGasManager new gas manager
func NewGasManager(h *Host) GasManager {
	return GasManager{
		h: h,
	}
}

func emptyGas() *common.Decimal {
	return &common.Decimal{
		Value: 0,
		Scale: database.GasDecimal,
	}
}

// If no key exists, return 0
func (g *GasManager) getFixed(key string) (*common.Decimal, contract.Cost) {
	result, cost := g.h.Get(key)
	if result == nil {
		// ilog.Errorf("GasManager failed %v", key)
		return nil, cost
	}
	value, ok := result.(*common.Decimal)
	if !ok {
		ilog.Errorf("GasManager failed %v %v", key, result)
		return nil, cost
	}
	return value, cost
}

// putFixed ...
func (g *GasManager) putFixed(key string, value *common.Decimal) contract.Cost {
	if value.Err != nil {
		ilog.Fatalf("GasHandler putFixed %v", value)
	}
	//fmt.Printf("putFixed %v %v\n", key, value)
	cost, err := g.h.Put(key, value)
	if err != nil {
		panic(fmt.Errorf("GasHandler putFixed err %v", err))
	}
	return cost
}

// GasPledgeTotal ...
func (g *GasManager) GasPledgeTotal(name string) (*common.Decimal, contract.Cost) {
	f, cost := g.getFixed(name + database.GasPledgeTotalKey)
	if f == nil {
		return emptyGas(), cost
	}
	return f, cost
}

// SetGasPledgeTotal ...
func (g *GasManager) SetGasPledgeTotal(name string, r *common.Decimal) contract.Cost {
	return g.putFixed(name+database.GasPledgeTotalKey, r)
}

// GasLimit ...
func (g *GasManager) GasLimit(name string) (*common.Decimal, contract.Cost) {
	f, cost := g.getFixed(name + database.GasLimitKey)
	if f == nil {
		return emptyGas(), cost
	}
	return f, cost
}

// SetGasLimit ...
func (g *GasManager) SetGasLimit(name string, l *common.Decimal) contract.Cost {
	return g.putFixed(name+database.GasLimitKey, l)
}

// GasUpdateTime ...
func (g *GasManager) GasUpdateTime(name string) (int64, contract.Cost) {
	value, cost := g.h.Get(name + database.GasUpdateTimeKey)
	if value == nil {
		return 0, cost
	}
	return value.(int64), cost
}

// SetGasUpdateTime ...
func (g *GasManager) SetGasUpdateTime(name string, t int64) contract.Cost {
	//ilog.Debugf("SetGasUpdateTime %v %v", name, t)
	cost, err := g.h.Put(name+database.GasUpdateTimeKey, t)
	if err != nil {
		panic(fmt.Errorf("gas manager set gas update time err, %v", err))
	}
	return cost
}

// GasStock `gasStock` means the gas amount at last update time.
func (g *GasManager) GasStock(name string) (*common.Decimal, contract.Cost) {
	f, cost := g.getFixed(name + database.GasStockKey)
	if f == nil {
		return emptyGas(), cost
	}
	return f, cost
}

// SetGasStock ...
func (g *GasManager) SetGasStock(name string, gas *common.Decimal) contract.Cost {
	//ilog.Debugf("SetGasStock %v %v", name, g)
	return g.putFixed(name+database.GasStockKey, gas)
}

// GasPledge ...
func (g *GasManager) GasPledge(name string, pledger string) (*common.Decimal, contract.Cost) {
	finalCost := contract.Cost0()
	ok, cost := g.h.MapHas(pledger+database.GasPledgeKey, name)
	finalCost.AddAssign(cost)
	if !ok {
		return &common.Decimal{
			Value: 0,
			Scale: 8,
		}, finalCost
	}
	result, cost := g.h.MapGet(pledger+database.GasPledgeKey, name)
	finalCost.AddAssign(cost)
	value, ok := result.(*common.Decimal)
	if !ok {
		return nil, finalCost
	}
	return value, finalCost
}

// SetGasPledge ...
func (g *GasManager) SetGasPledge(name string, pledger string, p *common.Decimal) contract.Cost {
	cost, err := g.h.MapPut(pledger+database.GasPledgeKey, name, p)
	if err != nil {
		panic(fmt.Errorf("gas manager set gas pledge err %v", err))
	}
	return cost
}

// DelGasPledge ...
func (g *GasManager) DelGasPledge(name string, pledger string) contract.Cost {
	cost, err := g.h.MapDel(pledger+database.GasPledgeKey, name)
	if err != nil {
		panic(fmt.Errorf("gas manager del gas pledge err %v", err))
	}
	return cost
}

func (g *GasManager) refreshPGasWithValue(name string, value *common.Decimal) (contract.Cost, error) {
	finalCost := contract.Cost0()
	cost := g.SetGasStock(name, value)
	finalCost.AddAssign(cost)
	cost = g.SetGasUpdateTime(name, g.h.ctx.Value("time").(int64))
	finalCost.AddAssign(cost)
	return finalCost, nil
}

// PGas returns the current total gas of a user. It is dynamically calculated
func (g *GasManager) PGas(name string) *common.Decimal {
	return g.h.DB().PGasAtTime(name, g.h.ctx.Value("time").(int64))
}

// TotalGas ...
func (g *GasManager) TotalGas(name string) *common.Decimal {
	return g.h.DB().TotalGasAtTime(name, g.h.ctx.Value("time").(int64))
}

// RefreshPGas update the gas status
func (g *GasManager) RefreshPGas(name string) (contract.Cost, error) {
	finalCost := contract.Cost0()
	value := g.PGas(name)
	cost, err := g.refreshPGasWithValue(name, value)
	finalCost.AddAssign(cost)
	return cost, err
}

// CostGas subtract gas of a user. It is not called in a contract. Need a better design here
func (g *GasManager) CostGas(name string, gasCost *common.Decimal) error {
	// todo modify CostGas
	oldVal := g.h.ctx.Value("contract_name")
	g.h.ctx.Set("contract_name", "gas.iost")
	_, err := g.RefreshPGas(name)
	if err != nil {
		return err
	}
	currentGas, _ := g.GasStock(name)
	if currentGas.LessThan(gasCost) {
		return fmt.Errorf("gas not enough! Now: %v(tgas:0,pgas:%v), Need %v", currentGas.String(), currentGas.String(), gasCost.String())
	}
	newGas := currentGas.Sub(gasCost)
	g.SetGasStock(name, newGas)

	g.h.ctx.Set("contract_name", oldVal)
	return nil
}
