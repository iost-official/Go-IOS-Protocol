package host

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/iost-official/go-iost/v3/common"
	"github.com/iost-official/go-iost/v3/core/contract"
	"github.com/iost-official/go-iost/v3/core/tx"
	"github.com/iost-official/go-iost/v3/core/version"
	"github.com/iost-official/go-iost/v3/ilog"
	"github.com/iost-official/go-iost/v3/vm/database"
)

// Monitor monitor interface
type Monitor interface {
	Call(host *Host, contractName, api string, jarg string) (rtn []interface{}, cost contract.Cost, err error)
	Validate(con *contract.Contract) error
	Compile(con *contract.Contract) (string, error)
}

// Host host struct, used as isolate of vm
type Host struct {
	DBHandler
	Stack
	Info
	Teller
	APIDelegate
	EventPoster
	DNS
	Authority
	GasManager
	*version.Rules

	logger  *ilog.Logger
	ctx     *Context
	db      *database.Visitor
	monitor Monitor

	deadline time.Time
}

// NewHost get a new host
func NewHost(ctx *Context, db *database.Visitor, rules *version.Rules, monitor Monitor, logger *ilog.Logger) *Host {
	h := &Host{
		ctx:     ctx,
		db:      db,
		monitor: monitor,
		logger:  logger,
	}
	h.Rules = rules
	h.DBHandler = NewDBHandler(h)
	h.Stack = NewStack(h)
	h.Info = NewInfo(h)
	h.Teller = NewTeller(h)
	h.APIDelegate = NewAPI(h)
	h.EventPoster = NewEventPoster(h)
	h.DNS = NewDNS(h)
	h.Authority = Authority{h: h}
	h.GasManager = NewGasManager(h)
	return h
}

// Context get context in host
func (h *Host) Context() *Context {
	return h.ctx
}

// SetContext set a new context to host
func (h *Host) SetContext(ctx *Context) {
	h.ctx = ctx
}

func (h *Host) call(cont, api, jarg string, withAuth bool) ([]interface{}, contract.Cost, error) {
	reenter, cost := h.PushStack(cont, api, withAuth)
	defer func() {
		h.PopStack()
	}()

	if reenter {
		return nil, cost, ErrReenter
	}

	if h.StackHeight() > 5 {
		return nil, cost, fmt.Errorf("stack height exceed. actual %v", h.StackHeight())
	}

	rtn, cost0, err := h.monitor.Call(h, cont, api, jarg)
	cost.AddAssign(cost0)

	return rtn, cost, err
}

// Call call a new contract in this context
func (h *Host) Call(contract, api, jarg string) ([]interface{}, contract.Cost, error) {
	return h.call(contract, api, jarg, false)
}

// CallWithAuth call a new contract with permission of current contract
func (h *Host) CallWithAuth(contract, api, jarg string) ([]interface{}, contract.Cost, error) {
	return h.call(contract, api, jarg, true)
}

func (h *Host) checkAbiValid(c *contract.Contract) (contract.Cost, error) {
	cost := contract.Cost0()
	err := h.monitor.Validate(c)
	cost.AddAssign(CodeSavageCost(len(c.Encode())))
	return cost, err
}

func (h *Host) checkAmountLimitValid(c *contract.Contract) (contract.Cost, error) {
	cost := contract.Cost0()
	for _, abi := range c.Info.Abi {
		cost.AddAssign(CommonOpCost(len(abi.AmountLimit)))
		err := h.CheckAmountLimit(abi.AmountLimit)
		if err != nil {
			return cost, err
		}
	}
	return cost, nil
}

// CheckPublisher check publisher of tx
func (h *Host) CheckPublisher(t *tx.Tx) error {
	b, c := h.RequirePublisherAuth(t.Publisher)
	if !b {
		return fmt.Errorf("unauthorized publisher: %v", t.Publisher)
	}
	h.PayCost(c, t.Publisher)
	return nil
}

// CheckSigners check signers of tx
func (h *Host) CheckSigners(t *tx.Tx) error {
	for _, item := range t.Signers {
		ss := strings.Split(item, "@")
		if len(ss) != 2 {
			return fmt.Errorf("illegal signer: %v", item)
		}
		b, c := h.RequireSignerAuth(ss[0], ss[1])
		if !b {
			return fmt.Errorf("unauthorized signer: %v", item)
		}
		h.PayCost(c, t.Publisher)
	}
	return nil
}

// CheckAmountLimit check amountLimit of tx valid
func (h *Host) CheckAmountLimit(amountLimit []*contract.Amount) error {
	tokenMap := make(map[string]bool)
	for _, limit := range amountLimit {
		if h.IsFork3_1_0 {
			if tokenMap[limit.Token] {
				return fmt.Errorf("duplicated token in amountLimit: %s", limit.Token)
			}
			tokenMap[limit.Token] = true
		}
		decimal := h.DB().Decimal(limit.Token)
		if limit.Token == "*" {
			decimal = 0
		}
		if decimal == -1 {
			return fmt.Errorf("token not exists in amountLimit, %v", limit)
		}
		if limit.Val != "unlimited" {
			_, err := common.NewDecimalFromString(limit.Val, decimal)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// SetCode set code to storage
func (h *Host) SetCode(c *contract.Contract, owner string) (contract.Cost, error) {
	if err := c.VerifySelf(); err != nil {
		return CommonErrorCost(1), err
	}
	cost, err := h.checkAbiValid(c)
	if err != nil {
		return cost, err
	}

	cost0, err := h.checkAmountLimitValid(c)
	cost.AddAssign(cost0)
	if err != nil {
		return cost, err
	}

	code, err := h.monitor.Compile(c)
	cost.AddAssign(CodeSavageCost(len(c.Code)))
	if err != nil {
		return cost, err
	}
	c.OrigCode = c.Code
	c.Code = code

	initABI := contract.ABI{
		Name: "init",
		Args: []string{},
	}

	c.Info.Abi = append(c.Info.Abi, &initABI)

	origCode := c.OrigCode
	c.OrigCode = ""
	l := len(c.Encode())
	c.OrigCode = origCode
	cost.AddAssign(contract.Cost{Data: int64(l), DataList: []contract.DataItem{
		{Payer: owner, Val: int64(l)},
	}})

	if h.db.HasContract(c.ID) {
		return cost, ErrContractExists
	}
	h.db.SetContract(c)
	_, cost0, err = h.Call(c.ID, "init", "[]")
	cost.AddAssign(cost0)

	if h.IsFork3_3_0 {
		// remove init
		c.Info.Abi = c.Info.Abi[:len(c.Info.Abi)-1]
		h.db.SetContract(c)
	}

	return cost, err
}

// UpdateCode update code
func (h *Host) UpdateCode(c *contract.Contract, id database.SerializedJSON) (contract.Cost, error) {
	if err := c.VerifySelf(); err != nil {
		return CommonErrorCost(1), err
	}
	oc := h.db.Contract(c.ID)
	if oc == nil {
		return Costs["GetCost"], ErrContractNotFound
	}
	abi := oc.ABI("can_update")
	if abi == nil {
		return Costs["GetCost"], ErrUpdateRefused
	}

	// Make gas cost consistent
	oc.OrigCode = ""
	oldL := len(oc.Encode())

	rtn, cost, err := h.Call(c.ID, "can_update", `["`+string(id)+`"]`)

	if err != nil {
		return cost, fmt.Errorf("call can_update: %v", err)
	}

	if t, ok := rtn[0].(string); !ok || t != "true" {
		return cost, ErrUpdateRefused
	}

	cost0, err := h.checkAbiValid(c)
	cost.AddAssign(cost0)
	if err != nil {
		return cost, err
	}

	cost0, err = h.checkAmountLimitValid(c)
	cost.AddAssign(cost0)
	if err != nil {
		return cost, err
	}

	code, err := h.monitor.Compile(c)
	cost.AddAssign(CodeSavageCost(len(c.Code)))
	if err != nil {
		return cost, err
	}
	c.OrigCode = c.Code
	c.Code = code

	// set code  without invoking init
	h.db.SetContract(c)

	publisher := h.Context().Value("publisher").(string)
	c.OrigCode = ""
	l := len(c.Encode())
	cost.AddAssign(contract.Cost{Data: int64(l - oldL), DataList: []contract.DataItem{
		{Payer: publisher, Val: int64(l - oldL)},
	}})

	return cost, nil
}

// CancelDelaytx deletes delaytx hash.
//
// The given argument txHash is from user's input. So we should Base58Decode it first.
func (h *Host) CancelDelaytx(txHash string) (contract.Cost, error) {
	hashString := string(common.Base58Decode(txHash))
	cost := Costs["GetCost"]
	publisher, deferTxHash := h.db.GetDelaytx(hashString)

	if publisher == "" {
		return cost, ErrDelaytxNotFound
	}
	if publisher != h.Context().Value("publisher").(string) {
		return cost, ErrCannotCancelDelay
	}

	h.db.DelDelaytx(hashString)
	cost.AddAssign(DelDelayTxCost(len(hashString)+len(publisher)+len(deferTxHash), publisher))
	return cost, nil
}

// Logger get a log in host
func (h *Host) Logger() *ilog.Logger {
	return h.logger
}

// DB get current version mvccdb
func (h *Host) DB() *database.Visitor {
	return h.db
}

// PushCtx make a new context based on current one
func (h *Host) PushCtx() {
	ctx := NewContext(h.ctx)
	h.ctx = ctx
}

// PopCtx pop current context
func (h *Host) PopCtx() {
	ctx := h.ctx.Base()
	h.ctx = ctx
}

// Deadline return this host's deadline
func (h *Host) Deadline() time.Time {
	return h.deadline
}

// SetDeadline set this host's deadline
func (h *Host) SetDeadline(t time.Time) {
	h.deadline = t
}

// IsGenesisMode ...
func (h *Host) IsGenesisMode() bool {
	return h.Context().Value("number") == int64(0)
}

// IsValidAccount check whether account exists
func (h *Host) IsValidAccount(name string) bool {
	if h.IsGenesisMode() {
		return true
	}
	return strings.HasPrefix(name, "Contract") || strings.HasSuffix(name, ".iost") || database.Unmarshal(h.DB().MGet("auth.iost"+"-auth", name)) != nil
}

// ReadSettings read settings from db
func (h *Host) ReadSettings() {
	j, _ := h.DBHandler.GlobalMapGet("system.iost", "settings", "host")
	if j == nil {
		return
	}
	var s Setting
	err := json.Unmarshal([]byte(j.(string)), &s)
	if err != nil {
		panic(err)
	}

	for k, v := range s.Costs {
		Costs[k] = v
	}
}

// GetVMFlags return target vm bitwise flags
func (h *Host) GetVMFlags() int64 {
	if h.IsFork3_3_1 {
		return 1
	}
	return 0
}
