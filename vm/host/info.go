package host

import (
	"encoding/json"

	"github.com/iost-official/Go-IOS-Protocol/core/contract"
	"github.com/iost-official/Go-IOS-Protocol/vm/database"
)

// Info current info handler of this isolate
type Info struct {
	h *Host
}

// NewInfo new info
func NewInfo(h *Host) Info {
	return Info{h: h}
}

// BlockInfo get block info, in json
func (h *Info) BlockInfo() (info database.SerializedJSON, cost *contract.Cost) {

	blkInfo := make(map[string]interface{})

	blkInfo["parent_hash"] = h.h.ctx.Value("parent_hash")
	blkInfo["number"] = h.h.ctx.Value("number")
	blkInfo["witness"] = h.h.ctx.Value("witness")
	blkInfo["time"] = h.h.ctx.Value("time")

	bij, err := json.Marshal(blkInfo)
	if err != nil {
		panic(err)
	}

	return database.SerializedJSON(bij), BlockInfoCost
}

// TxInfo get tx info
func (h *Info) TxInfo() (info database.SerializedJSON, cost *contract.Cost) {

	txInfo := make(map[string]interface{})
	txInfo["time"] = h.h.ctx.Value("time")
	txInfo["hash"] = h.h.ctx.Value("tx_hash")
	txInfo["expiration"] = h.h.ctx.Value("expiration")
	txInfo["gas_limit"] = h.h.ctx.GValue("gas_limit")
	txInfo["gas_price"] = h.h.ctx.Value("gas_price")
	txInfo["auth_list"] = h.h.ctx.Value("auth_list")
	txInfo["publisher"] = h.h.ctx.Value("publisher")

	tij, err := json.Marshal(txInfo)
	if err != nil {
		panic(err)
	}

	return database.SerializedJSON(tij), TxInfoCost
}

// ABIConfig set this abi config
func (h *Info) ABIConfig(key, value string) {
	switch key {
	case "payment":
		if value == "contract_pay" {
			h.h.ctx.GSet("abi_payment", 1)
		}
	}
}

// GasLimit get gas limit
func (h *Info) GasLimit() int64 {
	return h.h.ctx.GValue("gas_limit").(int64)
}
