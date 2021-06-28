package vm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/iost-official/go-iost/v3/account"
	"github.com/iost-official/go-iost/v3/common"
	"github.com/iost-official/go-iost/v3/core/block"
	"github.com/iost-official/go-iost/v3/core/contract"
	"github.com/iost-official/go-iost/v3/core/tx"
	"github.com/iost-official/go-iost/v3/ilog"
	"github.com/iost-official/go-iost/v3/metrics"
	"github.com/iost-official/go-iost/v3/vm/database"
	"github.com/iost-official/go-iost/v3/vm/host"
	"github.com/iost-official/go-iost/v3/vm/native"
)

var (
	executionKillCounter = metrics.NewCounter("iost_vm_execution_kill", nil)
)

// Isolator new entrance instead of Engine
type Isolator struct {
	h             *host.Host
	publisherID   string
	t             *tx.Tx
	tr            *tx.TxReceipt
	blockBaseCtx  *host.Context
	genesisMode   bool
	blockBaseMode bool
	limit         time.Duration
}

var staticMonitor = NewMonitor()

// TriggerBlockBaseMode start blockbase mode
func (i *Isolator) TriggerBlockBaseMode() {
	i.blockBaseMode = true
}

// Prepare Isolator
func (i *Isolator) Prepare(bh *block.BlockHead, db *database.Visitor, logger *ilog.Logger) error {
	if db.Contract("system.iost") == nil {
		db.SetContract(native.SystemABI())
	}
	if bh.Number == 0 {
		i.genesisMode = true
	} else {
		i.genesisMode = false
	}

	i.blockBaseCtx = host.NewContext(nil)
	i.blockBaseCtx = loadBlkInfo(i.blockBaseCtx, bh)
	i.h = host.NewHost(i.blockBaseCtx, db, bh.Rules(), staticMonitor, logger)
	i.h.ReadSettings()
	return nil
}

// PrepareTx read tx and ready to run
func (i *Isolator) PrepareTx(t *tx.Tx, limit time.Duration) error {
	i.t = t
	i.limit = limit
	i.h.SetDeadline(time.Now().Add(limit))
	i.publisherID = t.Publisher
	l := len(t.ToBytes(tx.Full))
	i.h.PayCost(contract.NewCost(0, int64(l), 0), t.Publisher)

	if !i.genesisMode && !i.blockBaseMode {
		err := checkTxParams(t)
		if err != nil {
			return err
		}
		if i.h.GasPaid(t.Publisher)*t.GasRatio >= t.GasLimit {
			return fmt.Errorf("gas limit should be larger, paid: %v, gas limit: %v, gas ratio: %v", i.h.GasPaid(t.Publisher), t.GasLimit, t.GasRatio)
		}
		gas := i.h.TotalGas(i.publisherID)
		err = CheckTxGasLimitValid(t, gas, i.h.DB())
		if err != nil {
			return err
		}
	}
	loadTxInfo(i.h, t, i.publisherID)
	if !i.genesisMode && !i.blockBaseMode {
		err := i.checkAuth(t)
		if err != nil {
			return err
		}
	}
	return nil
}

func (i *Isolator) checkAuth(t *tx.Tx) error {
	err := i.h.CheckSigners(t)
	if err != nil {
		return err
	}
	err = i.h.CheckPublisher(t)
	if err != nil {
		return err
	}
	err = i.h.CheckAmountLimit(t.AmountLimit)
	if err != nil {
		return err
	}
	return nil
}

func (i *Isolator) runAction(action tx.Action) (cost contract.Cost, status *tx.Status, ret string, receipts []*tx.Receipt, err error) {
	oLen := len(i.h.Context().GValue("receipts").([]*tx.Receipt))

	i.h.InitStack(i.t.Publisher)
	defer func() {
		i.h.PopStack()
	}()

	var rtn []interface{}

	rtn, cost, err = staticMonitor.Call(i.h, action.Contract, action.ActionName, action.Data)

	if err != nil {
		actionDesc := action.String()
		actionRune := []rune(actionDesc)
		if len(actionRune) > 100 {
			actionDesc = string(actionRune[:100]) + "..."
		}
		if strings.Contains(err.Error(), "execution killed") {
			status = &tx.Status{
				Code:    tx.ErrorTimeout,
				Message: fmt.Sprintf("running action %v error: execution killed", actionDesc),
			}
		} else {
			status = &tx.Status{
				Code:    tx.ErrorRuntime,
				Message: fmt.Sprintf("running action %v error: %v", actionDesc, err.Error()),
			}
		}
		err = nil
		return
	}

	rj, errj := json.Marshal(rtn)
	if errj != nil {
		panic(errj)
	}

	ret = string(rj)

	receipts = i.h.Context().GValue("receipts").([]*tx.Receipt)[oLen:]

	status = &tx.Status{
		Code:    tx.Success,
		Message: "",
	}
	return
}

func (i *Isolator) delDelaytx(refTxHash, publisher, deferTxHash string) {
	i.h.DB().DelDelaytx(refTxHash)
	cost := host.DelDelayTxCost(len(refTxHash)+len(i.publisherID)+len(deferTxHash), i.publisherID)
	i.h.PayCost(cost, i.publisherID)
}

// Run actions in tx
func (i *Isolator) Run() (*tx.TxReceipt, error) { // nolint
	startTime := time.Now()
	vmGasLimit := i.t.GasLimit/i.t.GasRatio - i.h.GasPaid()
	if vmGasLimit <= 0 {
		ilog.Fatalf("vmGasLimit < 0. It should not happen. %v / %v < %v", i.t.GasLimit, i.t.GasRatio, i.h.GasPaid())
	}
	i.h.Context().GSet("gas_limit", vmGasLimit)
	i.h.Context().GSet("receipts", make([]*tx.Receipt, 0))
	i.h.Context().GSet("amount_total", make(map[string]*common.Decimal))

	i.tr = tx.NewTxReceipt(i.t.Hash())

	if i.t.Delay > 0 {
		txHash := i.t.Hash()
		deferTxHash := i.t.DeferTx().Hash()
		i.h.DB().StoreDelaytx(string(txHash), i.publisherID, string(deferTxHash))
		i.tr.Status = &tx.Status{
			Code:    tx.Success,
			Message: "defertx hash: " + common.Base58Encode(deferTxHash),
		}
		cost := host.DelayTxCost(len(txHash)+len(i.publisherID)+len(deferTxHash), i.publisherID)
		i.h.PayCost(cost, i.publisherID)
		return i.tr, nil
	}

	var refTxHash, deferTxHash string
	if i.t.IsDefer() {
		refTxHash = string(i.t.ReferredTx)
		_, deferTxHash = i.h.DB().GetDelaytx(refTxHash)
		if deferTxHash == "" {
			return nil, fmt.Errorf("delay tx not found, hash=%v", common.Base58Encode(i.t.ReferredTx))
		}

		if !bytes.Equal(i.t.Hash(), []byte(deferTxHash)) {
			return nil, errors.New("defertx hash not match")
		}

		i.h.PayCost(host.Costs["GetCost"], i.publisherID)

		if i.t.IsExpired(i.blockBaseCtx.Value("time").(int64)) {
			i.tr.Status = &tx.Status{
				Code:    tx.ErrorRuntime,
				Message: "transaction expired",
			}
			i.delDelaytx(refTxHash, i.publisherID, deferTxHash)
			return i.tr, nil
		}
	}

	for _, action := range i.t.Actions {
		actionCost, status, ret, receipts, err := i.runAction(*action)
		ilog.Debugf("run action : %v, result is %v\n", action, status.Code)
		ilog.Debugf("used cost %v\n", actionCost)
		ilog.Debugf("status %v\n", status)
		ilog.Debugf("return value: %v\n", ret)
		if err != nil {
			return nil, err
		}

		i.tr.Status = status
		actionCost.AddAssign(contract.NewCost(0, int64(len(ret)), 0))
		if (status.Code == tx.ErrorRuntime && status.Message == "out of gas") ||
			(vmGasLimit < actionCost.ToGas()) ||
			(!i.genesisMode && !i.blockBaseMode && i.h.TotalGas(i.t.Publisher).Mul(common.NewDecimalExp10(2)).FloorInt()/i.t.GasRatio < i.h.GasPaid()+vmGasLimit) {
			ilog.Debugf("out of gas vmGasLimit %v actionCost %v totalGas %v gasPaid %v", vmGasLimit, actionCost.ToGas(), i.h.TotalGas(i.t.Publisher).String(), i.h.GasPaid())
			status.Code = tx.ErrorRuntime
			status.Message = "out of gas"
			actionCost.CPU = vmGasLimit
			actionCost.Net = 0
			ret = ""
		} else if status.Code == tx.ErrorTimeout {
			actionCost.CPU = vmGasLimit
			actionCost.Net = 0
			ret = ""
		}

		i.h.PayCost(actionCost, i.publisherID)

		if status.Code != tx.Success {
			if status.Code == tx.ErrorTimeout && i.limit >= common.MaxTxTimeLimit {
				ilog.Warnf("isolator run action %v failed, status=%+v, will rollback", action, status)
				executionKillCounter.Add(1, nil)
			}
			if i.h.IsFork3_3_0 {
				i.tr.Returns = nil
			}
			i.tr.Receipts = nil
			i.h.DB().Rollback()
			i.h.ClearRAMCosts()
			i.tr.RAMUsage = make(map[string]int64)
			break
		}

		i.tr.Receipts = append(i.tr.Receipts, receipts...)
		i.tr.Returns = append(i.tr.Returns, ret)
		vmGasLimit -= actionCost.ToGas()
		i.h.Context().GSet("gas_limit", vmGasLimit)
	}

	if i.t.IsDefer() {
		i.delDelaytx(refTxHash, i.publisherID, deferTxHash)
	}

	endTime := time.Now()
	ilog.Debugf("tx %v time %v", i.t.Actions, endTime.Sub(startTime))
	return i.tr, nil
}

// PayCost as name
func (i *Isolator) PayCost() (*tx.TxReceipt, error) {
	if i.t.GasLimit < i.h.GasPaid()*i.t.GasRatio {
		ilog.Fatalf("total gas cost is above limit %v < %v * %v", i.t.GasLimit, i.h.GasPaid(), i.t.GasRatio)
	}
	paidGas, err := i.h.DoPay(i.t.GasRatio)
	if err != nil {
		ilog.Errorf("DoPay failed, rollback %v", err)

		if i.h.IsFork3_3_0 {
			i.tr.Returns = nil
			i.tr.Receipts = nil
		}
		i.h.DB().Rollback()
		i.h.ClearRAMCosts()
		i.tr.RAMUsage = make(map[string]int64)
		i.tr.Status.Code = tx.ErrorBalanceNotEnough
		i.tr.Status.Message = "balance not enough after executing actions: " + err.Error()
		paidGas, err = i.h.DoPay(i.t.GasRatio)
		if err != nil {
			return nil, err
		}
	}
	i.tr.GasUsage = paidGas.Value
	for k, v := range i.h.Costs() {
		if v.Data != 0 {
			i.tr.RAMUsage[k] = v.Data
		}
	}

	return i.tr, nil
}

// Commit flush changes to db
func (i *Isolator) Commit() {
	i.h.DB().Commit()
}

// ClearAll clear this isolator
func (i *Isolator) ClearAll() {
	i.h = nil
}

// ClearTx clear this tx
func (i *Isolator) ClearTx() {
	i.h.SetContext(i.blockBaseCtx)
	i.h.Context().GClear()
	i.blockBaseMode = false
	i.h.ClearCosts()
	i.h.DB().Rollback()
}
func checkTxParams(t *tx.Tx) error {
	return t.CheckGas()
}

func loadBlkInfo(ctx *host.Context, bh *block.BlockHead) *host.Context {
	c := host.NewContext(ctx)
	c.Set("parent_hash", common.Base58Encode(bh.ParentHash))
	c.Set("number", bh.Number)
	c.Set("witness", bh.Witness)
	c.Set("time", bh.Time)
	if bh.Time <= 1 {
		panic(fmt.Sprintf("invalid blockhead time %v", bh.Time))
	}
	return c
}

func loadTxInfo(h *host.Host, t *tx.Tx, publisherID string) {
	h.PushCtx()
	h.Context().Set("tx_time", t.Time)
	h.Context().Set("expiration", t.Expiration)
	h.Context().Set("gas_ratio", t.GasRatio)
	h.Context().Set("tx_hash", common.Base58Encode(t.Hash()))
	h.Context().Set("publisher", publisherID)
	h.Context().Set("amount_limit", t.AmountLimit)
	h.Context().Set("actions", t.Actions)

	authList := make(map[string]int)
	for _, v := range t.Signs {
		authList[account.EncodePubkey(v.Pubkey)] = 1
	}
	for _, v := range t.PublishSigns {
		authList[account.EncodePubkey(v.Pubkey)] = 2
	}

	signers := make(map[string]bool)
	for _, v := range t.Signers {
		x := strings.Split(v, "@")
		if len(x) != 2 {
			ilog.Error("signer format error. " + v)
			continue
		}
		signers[v] = true
	}
	signers[t.Publisher+"@active"] = true

	h.Context().Set("auth_list", authList)
	h.Context().Set("signer_list", signers)
	h.Context().Set("auth_contract_list", make(map[string]int))
}
