package host

import (
	"fmt"
	"strings"

	"github.com/iost-official/go-iost/v3/common"
	"github.com/iost-official/go-iost/v3/core/contract"
	"github.com/iost-official/go-iost/v3/ilog"
	"github.com/iost-official/go-iost/v3/vm/database"
)

// Teller handler of iost
type Teller struct {
	h         *Host
	cost      map[string]contract.Cost
	cacheCost contract.Cost
}

// NewTeller new teller
func NewTeller(h *Host) Teller {
	return Teller{
		h:    h,
		cost: make(map[string]contract.Cost),
	}
}

// Costs ...
func (t *Teller) Costs() map[string]contract.Cost {
	return t.cost
}

// GasPaid ...
func (t *Teller) GasPaid(publishers ...string) int64 {
	var publisher string
	if len(publishers) > 0 {
		publisher = publishers[0]
	} else {
		publisher = t.h.Context().Value("publisher").(string)
	}
	v, ok := t.cost[publisher]
	if !ok {
		return 0
	}
	return v.ToGas()
}

// ClearCosts ...
func (t *Teller) ClearCosts() {
	t.cost = make(map[string]contract.Cost)
}

// ClearRAMCosts ...
func (t *Teller) ClearRAMCosts() {
	newCost := make(map[string]contract.Cost)
	for k, c := range t.cost {
		if c.Net != 0 || c.CPU != 0 {
			newCost[k] = contract.NewCost(0, c.Net, c.CPU)
		}
	}
	t.cost = newCost
}

// AddCacheCost ...
func (t *Teller) AddCacheCost(c contract.Cost) {
	t.cacheCost.AddAssign(c)
}

// CacheCost ...
func (t *Teller) CacheCost() contract.Cost {
	return t.cacheCost
}

// FlushCacheCost ...
func (t *Teller) FlushCacheCost() {
	t.PayCost(t.cacheCost, "")
	t.cacheCost = contract.Cost0()
}

// ClearCacheCost ...
func (t *Teller) ClearCacheCost() {
	t.cacheCost = contract.Cost0()
}

// PayCost ...
func (t *Teller) PayCost(c contract.Cost, who string) {
	//fmt.Printf("paycost [%v] %v(%v)\n", who, c, c.ToGas())
	costMap := make(map[string]contract.Cost)
	if c.CPU > 0 || c.Net > 0 {
		costMap[who] = contract.Cost{CPU: c.CPU, Net: c.Net}
	}
	for _, item := range c.DataList {
		if oc, ok := costMap[item.Payer]; ok {
			oc.AddAssign(contract.Cost{Data: item.Val, DataList: []contract.DataItem{item}})
			costMap[item.Payer] = oc
		} else {
			costMap[item.Payer] = contract.Cost{Data: item.Val, DataList: []contract.DataItem{item}}
		}
	}
	for who, c := range costMap {
		if oc, ok := t.cost[who]; ok {
			oc.AddAssign(c)
			t.cost[who] = oc
		} else {
			t.cost[who] = c
		}
	}
}

// DoPay ...
func (t *Teller) DoPay(gasRatio int64) (paidGas *common.Fixed, err error) {
	for payer, costOfPayer := range t.cost {
		gas := &common.Fixed{
			Value:   gasRatio * costOfPayer.ToGas(),
			Decimal: database.GasDecimal,
		}
		if !gas.IsZero() {
			err := t.h.CostGas(payer, gas)
			if err != nil {
				return nil, fmt.Errorf("pay gas cost failed: %v %v %v", err, payer, gas)
			}
		}

		if payer == t.h.Context().Value("publisher").(string) {
			paidGas = gas
		}
		// contracts in "iost" domain will not pay for ram
		if !strings.HasSuffix(payer, ".iost") {
			var ramPayer string
			if t.h.IsContract(payer) {
				p, _ := t.h.GlobalMapGet("system.iost", "contract_owner", payer)
				var ok bool
				ramPayer, ok = p.(string)
				if !ok {
					ilog.Fatalf("DoPay failed: contract %v has no owner", payer)
				}
			} else {
				ramPayer = payer
			}

			ram := costOfPayer.Data
			currentRAM := t.h.db.TokenBalance("ram", ramPayer)
			if currentRAM < ram {
				err = fmt.Errorf("pay ram failed. id: %v need %v, actual %v", ramPayer, ram, currentRAM)
				return
			}
			t.h.db.SetTokenBalance("ram", ramPayer, currentRAM-ram)
			t.h.db.ChangeUsedRAMInfo(ramPayer, ram)
		}
	}
	return
}

// Privilege ...
func (t *Teller) Privilege(id string) int {
	am, ok := t.h.ctx.Value("auth_list").(map[string]int)
	if !ok {
		return 0
	}
	i, ok := am[id]
	if !ok {
		i = 0
	}
	return i
}
