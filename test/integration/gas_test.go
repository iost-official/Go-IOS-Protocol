package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/iost-official/go-iost/v3/common"
	"github.com/iost-official/go-iost/v3/core/contract"
	"github.com/iost-official/go-iost/v3/core/tx"
	"github.com/iost-official/go-iost/v3/core/version"
	"github.com/iost-official/go-iost/v3/db"
	"github.com/iost-official/go-iost/v3/verifier"
	"github.com/iost-official/go-iost/v3/vm"
	"github.com/iost-official/go-iost/v3/vm/database"
	"github.com/iost-official/go-iost/v3/vm/host"
	"github.com/iost-official/go-iost/v3/vm/native"
)

func toString(n int64) string {
	return strconv.FormatInt(n, 10)
}

func toIOSTAmount(n int64) *common.Decimal {
	return common.NewDecimalFromIntWithScale(int(n), 8)
}

const initCoinAmountInt int64 = 5000

var initCoinAmount = toIOSTAmount(initCoinAmountInt)

var monitor = vm.NewMonitor()

func gasTestInit() (*native.Impl, *host.Host, *contract.Contract, string, db.MVCCDB) {
	var tmpDB db.MVCCDB
	tmpDB, err := db.NewMVCCDB("mvcc")
	visitor := database.NewVisitor(100, tmpDB, version.NewRules(0))
	if err != nil {
		panic(err)
	}
	context := host.NewContext(nil)
	context.Set("gas_ratio", int64(100))
	context.GSet("gas_limit", int64(100000))

	h := host.NewHost(context, visitor, version.NewRules(0), monitor, nil)

	acc0Bytes, err := json.Marshal(acc0.ToAccount())
	if err != nil {
		panic(err)
	}
	h.DB().MPut("auth.iost"+"-auth", acc0.ID, database.MustMarshal(string(acc0Bytes)))

	acc1Bytes, err := json.Marshal(acc1.ToAccount())
	if err != nil {
		panic(err)
	}
	h.DB().MPut("auth.iost"+"-auth", acc1.ID, database.MustMarshal(string(acc1Bytes)))

	h.Context().Set("number", int64(1))
	h.Context().Set("time", int64(1541576370*1e9))
	h.Context().Set("stack_height", 0)
	h.Context().Set("publisher", acc0.ID)

	tokenContract := native.TokenABI()
	_, err = h.SetCode(tokenContract, "")
	if err != nil {
		panic(err)
	}

	authList := make(map[string]int)
	h.Context().Set("auth_contract_list", authList)
	authList[acc0.KeyPair.ReadablePubkey()] = 2
	h.Context().Set("auth_list", authList)
	signerList := make(map[string]bool)
	signerList[acc0.ID+"@active"] = true
	h.Context().Set("signer_list", signerList)

	code := &contract.Contract{
		ID:   native.GasContractName,
		Info: &contract.Info{Version: "1.0.0"},
	}

	e := &native.Impl{}
	e.Init()

	h.Context().Set("contract_name", "token.iost")
	h.Context().Set("abi_name", "abi")
	h.Context().GSet("receipts", []*tx.Receipt{})
	_, _, err = e.LoadAndCall(h, tokenContract, "create", "iost", acc0.ID, int64(initCoinAmountInt), []byte("{}"))
	if err != nil {
		panic("create iost " + err.Error())
	}
	_, _, err = e.LoadAndCall(h, tokenContract, "issue", "iost", acc0.ID, fmt.Sprintf("%d", initCoinAmountInt))
	if err != nil {
		panic("issue iost " + err.Error())
	}
	if initCoinAmountInt*1e8 != visitor.TokenBalance("iost", acc0.ID) {
		panic("set initial coins failed " + strconv.FormatInt(visitor.TokenBalance("iost", acc0.ID), 10))
	}

	h.Context().Set("contract_name", native.GasContractName)
	h.Context().Set("amount_limit", []*contract.Amount{{Token: "*", Val: "unlimited"}})

	return e, h, code, acc0.ID, tmpDB
}

func timePass(h *host.Host, seconds int64) {
	h.Context().Set("time", h.Context().Value("time").(int64)+seconds*1e9)
}

func TestGas_NoPledge(t *testing.T) {
	Convey("test an account who did not pledge has 0 gas", t, func() {
		_, h, _, testAcc, tmpDB := gasTestInit()
		defer func() {
			tmpDB.Close()
			os.RemoveAll("mvcc")
		}()
		gas := h.GasManager.PGas(testAcc)
		So(gas.Value, ShouldEqual, 0)
	})
}

func TestGas_PledgeAuth(t *testing.T) {
	Convey("test pledging requires auth", t, func() {
		e, h, code, testAcc, tmpDB := gasTestInit()
		defer func() {
			tmpDB.Close()
			os.RemoveAll("mvcc")
		}()
		pledgeAmount := toIOSTAmount(200)
		authList := make(map[string]int)
		h.Context().Set("auth_list", authList)
		signerList := make(map[string]bool)
		h.Context().Set("signer_list", signerList)
		_, _, err := e.LoadAndCall(h, code, "pledge", testAcc, testAcc, pledgeAmount.String())
		So(err, ShouldNotBeNil)
	})
}

func TestGas_NotEnoughMoney(t *testing.T) {
	Convey("test pledging with not enough money", t, func() {
		e, h, code, testAcc, tmpDB := gasTestInit()
		defer func() {
			tmpDB.Close()
			os.RemoveAll("mvcc")
		}()
		pledgeAmount := toIOSTAmount(20000)
		_, _, err := e.LoadAndCall(h, code, "pledge", testAcc, testAcc, pledgeAmount.String())
		So(err, ShouldNotBeNil)
	})
}

func TestGas_Pledge(t *testing.T) {
	Convey("test pledge", t, func() {
		e, h, code, testAcc, tmpDB := gasTestInit()
		defer func() {
			tmpDB.Close()
			os.RemoveAll("mvcc")
		}()
		pledgeAmount := toIOSTAmount(200)
		_, _, err := e.LoadAndCall(h, code, "pledge", testAcc, testAcc, pledgeAmount.String())
		So(err, ShouldBeNil)
		So(h.DB().TokenBalance("iost", testAcc), ShouldEqual, initCoinAmount.Value-pledgeAmount.Value)
		So(h.DB().TokenBalance("iost", native.GasContractName), ShouldEqual, pledgeAmount.Value)
		Convey("After pledge, you will get some gas immediately", func() {
			gas := h.GasManager.PGas(testAcc)
			gasEstimated := pledgeAmount.Mul(database.GasImmediateReward)
			So(gas.Equals(gasEstimated), ShouldBeTrue)
		})
		Convey("Then gas increases at a predefined rate", func() {
			delta := int64(5)
			timePass(h, delta)
			gas := h.GasManager.PGas(testAcc)
			gasEstimated := pledgeAmount.Mul(database.GasImmediateReward).Add(pledgeAmount.Mul(database.GasIncreaseRate).MulInt(delta))
			So(gas.Equals(gasEstimated), ShouldBeTrue)
		})
		Convey("Then gas will reach limit and not increase any longer", func() {
			delta := int64(2 * database.GasFulfillSeconds)
			timePass(h, delta)
			gas := h.GasManager.PGas(testAcc)
			gasEstimated := pledgeAmount.Mul(database.GasLimit)
			fmt.Printf("gas %v es %v\n", gas, gasEstimated)
			So(gas.Equals(gasEstimated), ShouldBeTrue)
		})
	})
}

func TestGas_PledgeMore(t *testing.T) {
	Convey("test you can pledge more after first time pledge", t, func() {
		e, h, code, testAcc, tmpDB := gasTestInit()
		defer func() {
			tmpDB.Close()
			os.RemoveAll("mvcc")
		}()
		firstTimePledgeAmount := toIOSTAmount(200)
		_, _, err := e.LoadAndCall(h, code, "pledge", testAcc, testAcc, firstTimePledgeAmount.String())
		So(err, ShouldBeNil)
		delta1 := int64(5)
		timePass(h, delta1)
		gasBeforeSecondPledge := h.GasManager.PGas(testAcc)
		secondTimePledgeAmount := toIOSTAmount(300)
		_, _, err = e.LoadAndCall(h, code, "pledge", testAcc, testAcc, secondTimePledgeAmount.String())
		So(err, ShouldBeNil)
		delta2 := int64(10)
		timePass(h, delta2)
		gasAfterSecondPledge := h.GasManager.PGas(testAcc)
		gasEstimated := gasBeforeSecondPledge.Add(secondTimePledgeAmount.Mul(database.GasImmediateReward).Add(
			secondTimePledgeAmount.Add(firstTimePledgeAmount).Mul(database.GasIncreaseRate).MulInt(delta2)))
		So(gasAfterSecondPledge.Equals(gasEstimated), ShouldBeTrue)
		So(h.DB().TokenBalance("iost", testAcc), ShouldEqual, initCoinAmount.Sub(firstTimePledgeAmount).Sub(secondTimePledgeAmount).Value)
		So(h.DB().TokenBalance("iost", native.GasContractName), ShouldEqual, firstTimePledgeAmount.Add(secondTimePledgeAmount).Value)
	})
}

func TestGas_UseGas(t *testing.T) {
	Convey("test using gas", t, func() {
		e, h, code, testAcc, tmpDB := gasTestInit()
		defer func() {
			tmpDB.Close()
			os.RemoveAll("mvcc")
		}()
		pledgeAmount := int64(200)
		_, _, err := e.LoadAndCall(h, code, "pledge", testAcc, testAcc, toString(pledgeAmount))
		So(err, ShouldBeNil)
		delta1 := int64(5)
		timePass(h, delta1)
		gasBeforeUse := h.GasManager.PGas(testAcc)
		gasCost := toIOSTAmount(100)
		err = h.GasManager.CostGas(testAcc, gasCost)
		So(err, ShouldBeNil)
		gasAfterUse := h.GasManager.PGas(testAcc)
		gasEstimated := gasBeforeUse.Sub(gasCost)
		So(gasAfterUse.Equals(gasEstimated), ShouldBeTrue)
	})
}

func TestGas_unpledge(t *testing.T) {
	Convey("test unpledge", t, func() {
		e, h, code, testAcc, tmpDB := gasTestInit()
		defer func() {
			tmpDB.Close()
			os.RemoveAll("mvcc")
		}()
		pledgeAmount := toIOSTAmount(200)
		_, _, err := e.LoadAndCall(h, code, "pledge", testAcc, testAcc, pledgeAmount.String())
		So(err, ShouldBeNil)
		delta1 := int64(10)
		timePass(h, delta1)
		unpledgeAmount := toIOSTAmount(190)
		balanceBeforeunpledge := h.DB().TokenBalance("iost", testAcc)
		_, _, err = e.LoadAndCall(h, code, "unpledge", testAcc, testAcc, unpledgeAmount.String())
		So(err, ShouldBeNil)
		So(h.DB().TokenBalance("iost", testAcc), ShouldEqual, balanceBeforeunpledge)
		So(h.DB().TokenBalance("iost", native.GasContractName), ShouldEqual, pledgeAmount.Sub(unpledgeAmount).Value)
		gas := h.GasManager.PGas(testAcc)
		Convey("After unpledging, the gas limit will decrease. If current gas is more than the new limit, it will be decrease.", func() {
			gasEstimated := pledgeAmount.Sub(unpledgeAmount).Mul(database.GasLimit)
			So(gas.Equals(gasEstimated), ShouldBeTrue)
		})
		Convey("after 3 days, the frozen money is available", func() {
			timePass(h, native.UnpledgeFreezeSeconds)
			h.Context().Set("contract_name", "token.iost")
			rs, _, err := e.LoadAndCall(h, native.TokenABI(), "balanceOf", "iost", testAcc)
			So(err, ShouldBeNil)
			expected := initCoinAmount.Sub(pledgeAmount).Add(unpledgeAmount)
			So(rs[0], ShouldEqual, expected.String())
			So(h.DB().TokenBalance("iost", testAcc), ShouldEqual, expected.Value)
		})
	})
}

func TestGas_unpledgeTooMuch(t *testing.T) {
	Convey("test unpledge too much: each account has a minimum pledge", t, func() {
		e, h, code, testAcc, tmpDB := gasTestInit()
		defer func() {
			tmpDB.Close()
			os.RemoveAll("mvcc")
		}()
		pledgeAmount := int64(200)
		_, _, err := e.LoadAndCall(h, code, "pledge", testAcc, testAcc, toString(pledgeAmount))
		So(err, ShouldBeNil)
		delta1 := int64(1)
		timePass(h, delta1)
		unpledgeAmount := (pledgeAmount - int64(database.GasMinPledgeOfUser.Float64())) + int64(1)
		_, _, err = e.LoadAndCall(h, code, "unpledge", testAcc, testAcc, toString(unpledgeAmount))
		So(err, ShouldNotBeNil)
	})
}

func TestGas_PledgeunpledgeForOther(t *testing.T) {
	Convey("test pledge for others", t, func() {
		e, h, code, testAcc, tmpDB := gasTestInit()
		defer func() {
			tmpDB.Close()
			os.RemoveAll("mvcc")
		}()
		otherAcc := acc1.ID
		pledgeAmount := toIOSTAmount(200)
		_, _, err := e.LoadAndCall(h, code, "pledge", testAcc, otherAcc, pledgeAmount.String())
		h.FlushCacheCost()
		h.ClearCosts()
		So(err, ShouldBeNil)
		So(h.DB().TokenBalance("iost", testAcc), ShouldEqual, initCoinAmount.Value-pledgeAmount.Value)
		So(h.DB().TokenBalance("iost", native.GasContractName), ShouldEqual, pledgeAmount.Value)
		Convey("After pledge, you will get some gas immediately", func() {
			gas := h.GasManager.PGas(otherAcc)
			gasEstimated := pledgeAmount.Mul(database.GasImmediateReward)
			So(gas.Equals(gasEstimated), ShouldBeTrue)
		})
		Convey("If one pledge for others, he will get no gas himself", func() {
			gas := h.GasManager.PGas(testAcc)
			So(gas.Value, ShouldBeZeroValue)
		})
		Convey("Test unpledge for others", func() {
			unpledgeAmount := toIOSTAmount(190)
			_, _, err = e.LoadAndCall(h, code, "unpledge", testAcc, otherAcc, unpledgeAmount.String())
			So(err, ShouldBeNil)
			timePass(h, native.UnpledgeFreezeSeconds)
			h.Context().Set("contract_name", "token.iost")
			rs, _, err := e.LoadAndCall(h, native.TokenABI(), "balanceOf", "iost", testAcc)
			So(err, ShouldBeNil)
			expected := initCoinAmount.Sub(pledgeAmount).Add(unpledgeAmount)
			So(rs[0], ShouldEqual, expected.String())
		})
		Convey("Test unpledge amount", func() {
			authList := make(map[string]int)
			h.Context().Set("auth_contract_list", authList)
			authList[acc1.KeyPair.ReadablePubkey()] = 2
			h.Context().Set("auth_list", authList)
			h.Context().Set("publisher", otherAcc)
			signerList := make(map[string]bool)
			signerList[otherAcc+"@active"] = true
			h.Context().Set("signer_list", signerList)
			h.DB().SetTokenBalanceFixed("iost", otherAcc, "20")
			_, _, err = e.LoadAndCall(h, code, "pledge", otherAcc, otherAcc, "20")
			So(err, ShouldBeNil)
			_, _, err = e.LoadAndCall(h, code, "unpledge", otherAcc, otherAcc, "20")
			So(err, ShouldBeNil)
		})
	})
}

func TestGas_Increase(t *testing.T) {
	Convey("check gas increase rate", t, func() {
		s := verifier.NewSimulator()
		defer s.Clear()
		createAccountsWithResource(s)
		createToken(t, s, acc0)
		s.SetContract(native.GasABI())
		r, err := s.Call("gas.iost", "pledge", array2json([]interface{}{acc0.ID, acc0.ID, "10"}), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		oldGas := s.Visitor.PGasAtTime(acc0.ID, s.Head.Time)
		var usage int64 = 0
		for i := 0; i < 10; i += 1 {
			s.Head.Time += 3 * 1e8
			r, err = s.Call("token.iost", "transfer", array2json([]interface{}{"iost", acc0.ID, acc1.ID, "1", ""}), acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			usage += r.GasUsage
		}
		newGas := s.Visitor.PGasAtTime(acc0.ID, s.Head.Time)
		expectedIncrease := database.GasIncreaseRate.MulInt(10).Value * 3
		actualIncrease := newGas.Value - oldGas.Value + usage
		So(actualIncrease, ShouldEqual, expectedIncrease)
	})
}

func TestGas_Overflow(t *testing.T) {
	Convey("check gas should not be negative", t, func() {
		s := verifier.NewSimulator()
		defer s.Clear()
		createAccountsWithResource(s)
		createToken(t, s, acc0)
		r, err := s.Call("token.iost", "issue", fmt.Sprintf(`["%v", "%v", "%v"]`, "iost", acc0.ID, "100000"), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		s.SetContract(native.GasABI())
		r, err = s.Call("gas.iost", "pledge", array2json([]interface{}{acc0.ID, acc0.ID, "20000.00000000"}), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		So(s.Visitor.PGasAtTime(acc0.ID, s.Head.Time).Value, ShouldBeGreaterThan, 0)
		s.Head.Time += 3 * 24 * 3600 * 1e9
		r, err = s.Call("gas.iost", "pledge", array2json([]interface{}{acc0.ID, acc0.ID, "1.00000010"}), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		So(s.Visitor.PGasAtTime(acc0.ID, s.Head.Time).Value, ShouldBeGreaterThan, 0)
		s.Head.Time += 3 * 24 * 3600 * 1e9
		So(s.Visitor.PGasAtTime(acc0.ID, s.Head.Time).Value, ShouldBeGreaterThan, 0)
	})
}
