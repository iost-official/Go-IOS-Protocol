package integration

import (
	"github.com/iost-official/go-iost/core/global"
	"testing"

	"github.com/iost-official/go-iost/vm/database"
	"github.com/iost-official/go-iost/vm/native"

	"github.com/iost-official/go-iost/core/tx"
	"github.com/iost-official/go-iost/ilog"
	. "github.com/iost-official/go-iost/verifier"
	. "github.com/smartystreets/goconvey/convey"
)

var initialTotal int64 = 128 * 1024 * 1024 * 1024
var increaseInterval int64 = 10 * 60                                        // increase every 10 mins
var increaseAmount int64 = 10 * (64 * 1024 * 1024 * 1024) / (365 * 24 * 60) // 64GB per year
var ramContractName = "ram.iost"

func ramSetup(t *testing.T) (*Simulator, *TestAccount) {
	s := NewSimulator()
	ilog.Stop()

	createAccountsWithResource(s)
	acc := prepareAuth(t, s)
	createToken(t, s, acc)
	s.SetGas(acc.ID, 10000000)

	r, err := setNonNativeContract(s, ramContractName, "ram.js", ContractPath)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status.Code != tx.StatusCode(tx.Success) {
		panic("set contract failed " + r.String())
	}

	s.Head.Number = 0
	admin := acc1
	r, err = s.Call(ramContractName, "initAdmin", array2json([]interface{}{admin.ID}), admin.ID, admin.KeyPair)
	if err != nil {
		panic(err)
	}
	if r.Status.Code != tx.StatusCode(tx.Success) {
		panic("call failed " + r.String())
	}

	r, err = s.Call(ramContractName, "issue", array2json([]interface{}{initialTotal, increaseInterval, increaseAmount, 0}), admin.ID, admin.KeyPair)
	if err != nil {
		panic(err)
	}
	if r.Status.Code != tx.StatusCode(tx.Success) {
		panic("call failed " + r.String())
	}
	dbKey := "token.iost" + database.Separator + native.TokenInfoMapPrefix + "ram"
	if database.MustUnmarshal(s.Visitor.MGet(dbKey, "fullName")) != "IOST system ram" {
		panic("incorrect token full name")
	}
	s.Head.Number = 1
	return s, acc
}

func TestRAM(t *testing.T) {
	s, acc := ramSetup(t)
	defer s.Clear()
	initRAM := s.Visitor.TokenBalance("ram", acc.ID)
	other := acc2.ID
	Convey("test of ram", t, func() {
		Convey("test buy", func() {
			var buyAmount int64 = 1000
			Convey("normal buy", func() {
				//balanceBefore := s.Visitor.TokenBalance(global.Token, acc.ID)
				ramAvailableBefore := s.Visitor.TokenBalance("ram", ramContractName)
				s.Visitor.SetTokenBalance(global.Token, ramContractName, 0)
				r, err := s.Call(ramContractName, "buy", array2json([]interface{}{acc.ID, acc.ID, buyAmount}), acc.ID, acc.KeyPair)
				So(err, ShouldEqual, nil)
				So(r.Status.Message, ShouldEqual, "")
				balanceAfter := s.Visitor.TokenBalance(global.Token, acc.ID)
				ramAvailableAfter := s.Visitor.TokenBalance("ram", ramContractName)
				//var priceEstimated int64 = 30 * 1e8 // TODO when the final function is set, update here
				So(balanceAfter, ShouldEqual, 99490*1e6)
				So(s.Visitor.TokenBalance("ram", acc.ID), ShouldEqual, initRAM+buyAmount)
				So(ramAvailableAfter, ShouldEqual, ramAvailableBefore-buyAmount)
			})
			Convey("when buying triggers increasing total ram", func() {
				head := s.Head
				head.Time = head.Time + 144*increaseInterval*1000*1000*1000
				s.SetBlockHead(head)
				ramAvailableBefore := s.Visitor.TokenBalance("ram", ramContractName)
				r, err := s.Call(ramContractName, "buy", array2json([]interface{}{acc.ID, acc.ID, buyAmount}), acc.ID, acc.KeyPair)
				So(err, ShouldEqual, nil)
				So(r.Status.Message, ShouldEqual, "")
				ramAvailableAfter := s.Visitor.TokenBalance("ram", ramContractName)
				So(ramAvailableAfter, ShouldEqual, ramAvailableBefore+144*increaseAmount-buyAmount)
			})
			Convey("user can buy for others", func() {
				//balanceBefore := s.Visitor.TokenBalance(global.Token, acc.ID)
				otherRAMBefore := s.Visitor.TokenBalance("ram", other)
				myRAMBefore := s.Visitor.TokenBalance("ram", acc.ID)
				r, err := s.Call(ramContractName, "buy", array2json([]interface{}{acc.ID, other, buyAmount}), acc.ID, acc.KeyPair)
				So(err, ShouldEqual, nil)
				So(r.Status.Message, ShouldEqual, "")
				balanceAfter := s.Visitor.TokenBalance(global.Token, acc.ID)
				otherRAMAfter := s.Visitor.TokenBalance("ram", other)
				myRAMAfter := s.Visitor.TokenBalance("ram", acc.ID)
				//var priceEstimated int64 = 30 * 1e8 // TODO when the final function is set, update here
				So(balanceAfter, ShouldEqual, 98472*1e6)
				So(myRAMAfter, ShouldEqual, myRAMBefore)
				So(otherRAMAfter, ShouldEqual, otherRAMBefore+buyAmount)
			})
		})
		Convey("test sell", func() {
			Convey("user cannot sell more than he owns", func() {
				r, err := s.Call(ramContractName, "sell", array2json([]interface{}{acc.ID, acc.ID, 6000}), acc.ID, acc.KeyPair)
				So(err, ShouldEqual, nil)
				So(r.Status.Code, ShouldEqual, tx.StatusCode(tx.ErrorRuntime))
			})
			Convey("normal sell", func() {
				var sellAmount int64 = 300
				//balanceBefore := s.Visitor.TokenBalance(global.Token, acc.ID)
				ramAvailableBefore := s.Visitor.TokenBalance("ram", ramContractName)
				myRAMBefore := s.Visitor.TokenBalance("ram", acc.ID)
				r, err := s.Call(ramContractName, "sell", array2json([]interface{}{acc.ID, acc.ID, sellAmount}), acc.ID, acc.KeyPair)
				So(err, ShouldEqual, nil)
				So(r.Status.Message, ShouldEqual, "")
				balanceAfter := s.Visitor.TokenBalance(global.Token, acc.ID)
				ramAvailableAfter := s.Visitor.TokenBalance("ram", ramContractName)
				myRAMAfter := s.Visitor.TokenBalance("ram", acc.ID)
				//var priceEstimated int64 = 10 * 1e8 // TODO when the final function is set, update here
				So(balanceAfter, ShouldEqual, 98622*1e6)
				So(myRAMAfter, ShouldEqual, myRAMBefore-sellAmount)
				So(ramAvailableAfter, ShouldEqual, ramAvailableBefore+sellAmount)
			})
			Convey("user can sell ram for others", func() {
				var sellAmount int64 = 300
				balanceBefore := s.Visitor.TokenBalance(global.Token, acc.ID)
				//otherBalanceBefore := s.Visitor.TokenBalance(global.Token, other)
				myRAMBefore := s.Visitor.TokenBalance("ram", acc.ID)
				r, err := s.Call(ramContractName, "sell", array2json([]interface{}{acc.ID, other, sellAmount}), acc.ID, acc.KeyPair)
				So(err, ShouldEqual, nil)
				So(r.Status.Message, ShouldEqual, "")
				balanceAfter := s.Visitor.TokenBalance(global.Token, acc.ID)
				otherBalanceAfter := s.Visitor.TokenBalance(global.Token, other)
				myRAMAfter := s.Visitor.TokenBalance("ram", acc.ID)
				//var priceEstimated int64 = 10 * 1e8 // TODO when the final function is set, update here
				So(balanceAfter, ShouldEqual, balanceBefore)
				So(myRAMAfter, ShouldEqual, myRAMBefore-sellAmount)
				So(otherBalanceAfter, ShouldEqual, 150*1e6)
			})
		})
	})
}

func TestRAM2(t *testing.T) {
	s, _ := ramSetup(t)
	defer s.Clear()
	Convey("borrowed ram cannot be selled", t, func() {
		s.Visitor.SetTokenBalance(global.Token, acc2.ID, 100*100000000)
		s.Visitor.SetTokenBalance(global.Token, acc3.ID, 100*100000000)
		s.SetRAM(acc2.ID, 0)
		s.SetRAM(acc3.ID, 0)
		r, err := s.Call(ramContractName, "buy", array2json([]interface{}{acc2.ID, acc2.ID, 1000}), acc2.ID, acc2.KeyPair)
		So(err, ShouldEqual, nil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call(ramContractName, "buy", array2json([]interface{}{acc3.ID, acc3.ID, 300}), acc3.ID, acc3.KeyPair)
		So(err, ShouldEqual, nil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("token.iost", "transfer", array2json([]interface{}{"ram", acc3.ID, acc2.ID, "200", ""}), acc3.ID, acc3.KeyPair)
		So(err, ShouldEqual, nil)
		So(r.Status.Message, ShouldContainSubstring, "transfer need issuer permission")
		r, err = s.Call(ramContractName, "lend", array2json([]interface{}{acc2.ID, acc3.ID, 500}), acc2.ID, acc2.KeyPair)
		So(err, ShouldEqual, nil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call(ramContractName, "sell", array2json([]interface{}{acc3.ID, acc3.ID, 200}), acc3.ID, acc3.KeyPair)
		So(err, ShouldEqual, nil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call(ramContractName, "sell", array2json([]interface{}{acc3.ID, acc3.ID, 200}), acc3.ID, acc3.KeyPair)
		So(err, ShouldEqual, nil)
		So(r.Status.Message, ShouldContainSubstring, "self ram amount 100, not enough for sell")
		r, err = s.Call(ramContractName, "lend", array2json([]interface{}{acc3.ID, acc2.ID, 200}), acc3.ID, acc3.KeyPair)
		So(err, ShouldEqual, nil)
		So(r.Status.Message, ShouldContainSubstring, "self ram amount 100, not enough for lend")
		r, err = s.Call("token.iost", "transfer", array2json([]interface{}{"ram", acc3.ID, acc2.ID, "200", ""}), acc3.ID, acc3.KeyPair)
		So(err, ShouldEqual, nil)
		So(r.Status.Message, ShouldContainSubstring, "transfer need issuer permission")
	})
}
