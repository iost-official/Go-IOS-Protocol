package native

import (
	"sort"

	"github.com/iost-official/Go-IOS-Protocol/core/contract"
)

// ABI generate iost.system abi and contract
func ABI() *contract.Contract {
	return genNativeAbi("iost.system", systemABIs)
}

// BonusABI generate iost.bonus abi and contract
func BonusABI() *contract.Contract {
	return genNativeAbi("iost.bonus", bonusABIs)
}

func DomainABI() *contract.Contract {
	return genNativeAbi("iost.domain", domainABIs)
}

func genNativeAbi(id string, abi map[string]*abi) *contract.Contract {
	c := &contract.Contract{
		ID:   id,
		Code: "codes",
		Info: &contract.Info{
			Lang:        "native",
			VersionCode: "1.0.0",
			Abis:        make([]*contract.ABI, 0),
		},
	}

	for _, v := range abi {
		c.Info.Abis = append(c.Info.Abis, &contract.ABI{
			Name:     v.name,
			Args:     v.args,
			Payment:  0,
			GasPrice: int64(1000),
			Limit:    contract.NewCost(100, 100, 100),
		})
	}

	sort.Sort(abiSlice(c.Info.Abis))

	return c
}

type abiSlice []*contract.ABI

func (s abiSlice) Len() int {
	return len(s)
}
func (s abiSlice) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}
func (s abiSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
