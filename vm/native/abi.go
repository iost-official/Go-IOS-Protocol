package native

import (
	"sort"

	"fmt"
	"strconv"

	"github.com/iost-official/go-iost/core/contract"
	"github.com/iost-official/go-iost/ilog"
	"github.com/iost-official/go-iost/vm/host"
)

var (
	onlyAdminCanUpdateABI = &abi{
		name: "can_update",
		args: []string{"string"},
		do: func(h *host.Host, args ...interface{}) (rtn []interface{}, cost contract.Cost, err error) {
			ok, cost := h.RequireAuth(AdminAccount, SystemPermission)
			return []interface{}{strconv.FormatBool(ok)}, cost, nil
		},
	}
)

// SystemABI generate system.iost abi and contract
func SystemABI() *contract.Contract {
	return SystemContractABI("system.iost", "1.0.0")
}

// GasABI generate gas.iost abi and contract
func GasABI() *contract.Contract {
	return SystemContractABI("gas.iost", "1.0.0")
}

// TokenABI generate token.iost abi and contract
func TokenABI() *contract.Contract {
	return SystemContractABI("token.iost", "1.0.0")
}

// Token721ABI generate token.iost abi and contract
func Token721ABI() *contract.Contract {
	return SystemContractABI("token721.iost", "1.0.0")
}

// DomainABI generate domain.iost abi and contract
func DomainABI() *contract.Contract {
	return SystemContractABI("domain.iost", "1.0.0")
}

// SystemContractABI return system contract abi
func SystemContractABI(conID, version string) *contract.Contract {
	aset, err := getABISetByVersion(conID, version)
	if err != nil {
		return nil
	}
	return ABI(conID, aset, version)
}

// GetABISetByVersion return the corrent abi set according to contract ID and version
func getABISetByVersion(conID string, version string) (aset *abiSet, err error) {
	abiMap := make(map[string]map[string]*abiSet)
	abiMap["system.iost"] = make(map[string]*abiSet)
	abiMap["system.iost"]["1.0.0"] = systemABIs
	abiMap["domain.iost"] = make(map[string]*abiSet)
	abiMap["domain.iost"]["0.0.0"] = domain0ABIs
	abiMap["domain.iost"]["1.0.0"] = domainABIs
	abiMap["gas.iost"] = make(map[string]*abiSet)
	abiMap["gas.iost"]["1.0.0"] = gasABIs
	abiMap["token.iost"] = make(map[string]*abiSet)
	abiMap["token.iost"]["1.0.0"] = tokenABIs
	abiMap["token.iost"]["1.0.2"] = tokenABIsV2
	abiMap["token.iost"]["1.0.3"] = tokenABIsV3
	tokenABIsV4 := tokenABIsV3.Clone()
	tokenABIsV4.Register(updateTokenTotalSupplyABI)
	abiMap["token.iost"]["1.0.4"] = tokenABIsV4
	abiMap["token.iost"]["1.0.5"] = tokenABIsV5
	abiMap["token721.iost"] = make(map[string]*abiSet)
	abiMap["token721.iost"]["1.0.0"] = token721ABIs

	var amap map[string]*abiSet
	var ok bool
	if amap, ok = abiMap[conID]; !ok {
		ilog.Errorf("invalid contract name: %v %v, please check `Monitor.prepareContract`", conID, version)
		return nil, fmt.Errorf("invalid contract name: %v %v", conID, version)
	}
	if aset, ok = amap[version]; !ok {
		ilog.Errorf("invalid contract version: %v %v, please check `Monitor.prepareContract`", conID, version)
		return nil, fmt.Errorf("invalid contract version: %v %v", conID, version)
	}

	if _, ok = aset.Get("can_update"); !ok {
		aset.Register(onlyAdminCanUpdateABI)
	}
	return aset, nil
}

// ABI generate native abis
func ABI(id string, abiSet *abiSet, version string) *contract.Contract {
	c := &contract.Contract{
		ID:   id,
		Code: "codes",
		Info: &contract.Info{
			Lang:    "native",
			Version: version,
			Abi:     make([]*contract.ABI, 0),
		},
	}

	for _, v := range abiSet.PublicAbis() {
		c.Info.Abi = append(c.Info.Abi, &contract.ABI{
			Name: v.name,
			Args: v.args,
			AmountLimit: []*contract.Amount{
				{Token: "*", Val: "unlimited"},
			},
		})
	}

	sort.Sort(abiSlice(c.Info.Abi))

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
