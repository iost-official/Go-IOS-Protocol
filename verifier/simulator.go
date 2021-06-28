package verifier

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/iost-official/go-iost/v3/account"
	"github.com/iost-official/go-iost/v3/common"
	"github.com/iost-official/go-iost/v3/core/block"
	"github.com/iost-official/go-iost/v3/core/contract"
	"github.com/iost-official/go-iost/v3/core/tx"
	"github.com/iost-official/go-iost/v3/core/version"
	"github.com/iost-official/go-iost/v3/db"
	"github.com/iost-official/go-iost/v3/ilog"
	"github.com/iost-official/go-iost/v3/vm"
	"github.com/iost-official/go-iost/v3/vm/database"
)

var txTime = 2 * common.MaxTxTimeLimit

// Simulator of txs and contract
type Simulator struct {
	Visitor  *database.Visitor
	Verifier *Verifier
	Head     *block.BlockHead
	Logger   *ilog.Logger
	Mvcc     db.MVCCDB
	GasLimit int64
}

// NewSimulator get a simulator with default settings
func NewSimulator() *Simulator {
	mvccdb, err := db.NewMVCCDB("mvcc")
	if err != nil {
		panic(err)
	}
	v := Verifier{}

	s := &Simulator{
		Visitor:  database.NewVisitor(0, mvccdb, version.NewRules(0)),
		Verifier: &v,
		Mvcc:     mvccdb,
		Head: &block.BlockHead{
			ParentHash: []byte("abc"),
			Number:     1,
			Witness:    "witness",
			Time:       int64(1541541540 * 1000 * 1000 * 1000),
		},
		Logger:   ilog.DefaultLogger(),
		GasLimit: 100000000,
	}
	return s
}

// SetBlockHead ...
func (s *Simulator) SetBlockHead(head *block.BlockHead) {
	s.Head = head
}

// SetAccount ...
func (s *Simulator) SetAccount(acc *account.Account) {
	buf, err := json.Marshal(acc)
	if err != nil {
		panic(err)
	}
	s.Visitor.MPut("auth.iost"+"-auth", acc.ID, database.MustMarshal(string(buf)))
}

// SetGas to id
func (s *Simulator) SetGas(id string, i int64) {
	prefix := database.GasContractName + database.Separator
	value := &common.Decimal{
		Value: i * 100,
		Scale: 2,
	}
	valueStr := database.MustMarshal(value)
	s.Visitor.Put(prefix+id+database.GasStockKey, valueStr)
	s.Visitor.Put(prefix+id+database.GasLimitKey, valueStr)
	s.Visitor.Commit()
}

// GetGas ...
func (s *Simulator) GetGas(id string) int64 {
	return s.Visitor.TotalGasAtTime(id, s.Head.Time).Value / 100
}

// SetRAM to id
func (s *Simulator) SetRAM(id string, r int64) {
	s.Visitor.SetTokenBalance("ram", id, r)
	s.Visitor.Commit()
}

// GetRAM of id
func (s *Simulator) GetRAM(id string) int64 {
	return s.Visitor.TokenBalance("ram", id)
}

// SetContract without run init
func (s *Simulator) SetContract(c *contract.Contract) {
	s.Visitor.SetContract(c)
}

// DeploySystemContract via system.iost/initSetCode
func (s *Simulator) DeploySystemContract(c *contract.Contract, publisher string, kp *account.KeyPair) (*tx.TxReceipt, error) {
	sc := c.B64Encode()

	jargs, err := json.Marshal([]string{c.ID, sc})
	if err != nil {
		panic(err)
	}

	trx := tx.NewTx([]*tx.Action{{
		Contract:   "system.iost",
		ActionName: "initSetCode",
		Data:       string(jargs),
	}}, nil, 400000000, 100, s.Head.Time+int64(txTime), 0, 0)

	trx.Time = s.Head.Time

	bn := s.Head.Number
	s.Head.Number = 0
	r, err := s.CallTx(trx, publisher, kp)
	s.Head.Number = bn
	if err != nil {
		return r, err
	}
	if r.Status.Code != 0 {
		return r, errors.New(r.Status.Message)
	}
	return r, nil
}

// DeployContract via system.iost/setCode
func (s *Simulator) DeployContract(c *contract.Contract, publisher string, kp *account.KeyPair) (string, *tx.TxReceipt, error) {
	sc, err := json.Marshal(c)
	if err != nil {
		return "", nil, nil
	}

	jargs, err := json.Marshal([]string{string(sc)})
	if err != nil {
		panic(err)
	}

	trx := tx.NewTx([]*tx.Action{{
		Contract:   "system.iost",
		ActionName: "setCode",
		Data:       string(jargs),
	}}, nil, 400000000, 100, s.Head.Time+int64(txTime), 0, 0)

	trx.Time = s.Head.Time

	r, err := s.CallTx(trx, publisher, kp)
	if err != nil {
		return "", r, err
	}
	if r.Status.Code != 0 {
		return "", r, errors.New(r.Status.Message)
	}
	return "Contract" + common.Base58Encode(trx.Hash()), r, nil
}

// Compile files
func (s *Simulator) Compile(id, src, abi string) (*contract.Contract, error) {
	bs, err := os.ReadFile(src + ".js")
	if err != nil {
		return nil, err
	}
	code := string(bs)

	as, err := os.ReadFile(abi + ".abi")
	if err != nil {
		return nil, err
	}

	var info contract.Info
	err = json.Unmarshal(as, &info)
	if err != nil {
		return nil, err
	}
	c := contract.Contract{
		ID:   id,
		Info: &info,
		Code: code,
	}

	return &c, nil
}

// Call abi with basic settings
func (s *Simulator) Call(contractName, abi, args string, publisher string, auth *account.KeyPair, otherArgs ...interface{}) (*tx.TxReceipt, error) {
	var signers []string
	if len(otherArgs) >= 1 {
		if s, ok := otherArgs[0].([]string); ok {
			signers = s
		}
	}
	trx := tx.NewTx([]*tx.Action{{
		Contract:   contractName,
		ActionName: abi,
		Data:       args,
	}}, signers, s.GasLimit, 100, s.Head.Time+int64(txTime), 0, 0)

	trx.Time = s.Head.Time
	trx.AmountLimit = append(trx.AmountLimit, &contract.Amount{Token: "*", Val: "unlimited"})

	return s.CallTx(trx, publisher, auth)
}

// CallTx with user defiened tx
func (s *Simulator) CallTx(trx *tx.Tx, publisher string, auth *account.KeyPair) (*tx.TxReceipt, error) {
	//if len(auths) > 1 {
	//	for _, auth := range auths[1:] {
	//		sig, err := tx.SignTxContent(trx, auth)
	//		if err != nil {
	//			return nil, err
	//		}
	//		sigs = append(sigs, sig)
	//	}
	//}

	stx, err := tx.SignTx(trx, publisher, []*account.KeyPair{auth})
	if err != nil {
		return nil, err
	}

	return s.RunTx(stx)
}

// RunTx run tx with signed tx
func (s *Simulator) RunTx(stx *tx.Tx) (*tx.TxReceipt, error) {
	var isolator vm.Isolator
	var err error

	err = isolator.Prepare(s.Head, s.Visitor, s.Logger)
	if err != nil {
		return &tx.TxReceipt{}, err
	}
	err = isolator.PrepareTx(stx, 3*time.Second)

	if err != nil {
		return &tx.TxReceipt{}, fmt.Errorf("prepare tx error: %v", err)
	}
	_, err = isolator.Run()
	if err != nil {
		return &tx.TxReceipt{}, err
	}
	r, err := isolator.PayCost()
	if err != nil {
		return nil, err
	}
	isolator.Commit()

	return r, nil
}

// Clear mvccdb
func (s *Simulator) Clear() {
	s.Mvcc.Close()
	os.RemoveAll("mvcc")
}
