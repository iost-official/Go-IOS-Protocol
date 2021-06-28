package integration

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/iost-official/go-iost/v3/common"
	"github.com/iost-official/go-iost/v3/core/contract"
	"github.com/iost-official/go-iost/v3/core/event"
	"github.com/iost-official/go-iost/v3/core/tx"
	"github.com/iost-official/go-iost/v3/core/version"
	"github.com/iost-official/go-iost/v3/ilog"
	. "github.com/iost-official/go-iost/v3/verifier"
	"github.com/iost-official/go-iost/v3/vm/database"
	"github.com/stretchr/testify/assert"
)

func Test_callWithAuth(t *testing.T) {
	ilog.Stop()
	Convey("test of callWithAuth", t, func() {
		s := NewSimulator()
		defer s.Clear()

		createAccountsWithResource(s)
		createToken(t, s, acc0)

		ca, err := s.Compile("Contracttransfer", "./test_data/transfer", "./test_data/transfer.js")
		if err != nil || ca == nil {
			t.Fatal(err)
		}
		cname, r, err := s.DeployContract(ca, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Code, ShouldEqual, tx.Success)

		Convey("test of callWithoutAuth", func() {
			s.Visitor.SetTokenBalanceFixed("iost", cname, "1000")
			r, err := s.Call(cname, "withdrawWithoutAuth", fmt.Sprintf(`["%v", "%v"]`, acc0.ID, "10"), acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldContainSubstring, "transaction has no permission")
			s.Visitor.Commit()
		})

		Convey("test of callWithAuth", func() {
			s.Visitor.SetTokenBalanceFixed("iost", cname, "1000")
			r, err = s.Call(cname, "withdraw", fmt.Sprintf(`["%v", "%v"]`, acc0.ID, "10"), acc0.ID, acc0.KeyPair)
			s.Visitor.Commit()

			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			balance := common.Decimal{Value: s.Visitor.TokenBalance("iost", cname), Scale: s.Visitor.Decimal("iost")}
			So(balance.String(), ShouldEqual, "990")
		})

		Convey("test of callWithoutAuth after callWithAuth", func() {
			s.Visitor.SetTokenBalanceFixed("iost", cname, "1000")
			r, err = s.Call(cname, "withdrawWithoutAuthAfterWithAuth", fmt.Sprintf(`["%v", "%v"]`, acc0.ID, "10"), acc0.ID, acc0.KeyPair)
			s.Visitor.Commit()
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldContainSubstring, "transaction has no permission")
		})
	})
}

func Test_VMMethod(t *testing.T) {
	ilog.Stop()
	Convey("test of vm method", t, func() {
		s := NewSimulator()
		defer s.Clear()

		createAccountsWithResource(s)
		createToken(t, s, acc0)

		ca, err := s.Compile("", "./test_data/vmmethod", "./test_data/vmmethod")
		if err != nil || ca == nil {
			t.Fatal(err)
		}
		cname, r, err := s.DeployContract(ca, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Code, ShouldEqual, tx.Success)

		Convey("test of contract name", func() {
			r, err := s.Call(cname, "contractName", "[]", acc0.ID, acc0.KeyPair)
			s.Visitor.Commit()

			So(err, ShouldBeNil)
			So(r.Status.Code, ShouldEqual, tx.Success)
			So(len(r.Returns), ShouldEqual, 1)
			res, err := json.Marshal([]interface{}{cname})
			So(err, ShouldBeNil)
			So(r.Returns[0], ShouldEqual, string(res))
		})

		Convey("test of contract owner", func() {
			r, err := s.Call(cname, "contractOwner", "[]", acc0.ID, acc0.KeyPair)
			s.Visitor.Commit()

			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			So(len(r.Returns), ShouldEqual, 1)
			So(r.Returns[0], ShouldEqual, "[\"user_0\"]")
		})

		Convey("test of receipt", func() {
			r, err := s.Call(cname, "receiptf", fmt.Sprintf(`["%v"]`, "receiptdata"), acc0.ID, acc0.KeyPair)
			s.Visitor.Commit()

			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			So(len(r.Receipts), ShouldEqual, 1)
			So(r.Receipts[0].Content, ShouldEqual, "receiptdata")
			So(r.Receipts[0].FuncName, ShouldEqual, cname+"/receiptf")
		})

	})
}

func Test_VMMethod_Event(t *testing.T) {
	ilog.Stop()
	Convey("test of vm method event", t, func() {
		s := NewSimulator()
		defer s.Clear()

		createAccountsWithResource(s)

		ca, err := s.Compile("", "./test_data/vmmethod", "./test_data/vmmethod")
		if err != nil || ca == nil {
			t.Fatal(err)
		}
		cname, r, err := s.DeployContract(ca, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Code, ShouldEqual, tx.Success)

		eve := event.GetCollector()
		// contract event
		ch1 := eve.Subscribe(1, []event.Topic{event.ContractEvent}, nil)
		ch2 := eve.Subscribe(2, []event.Topic{event.ContractReceipt}, nil)

		r, err = s.Call(cname, "event", fmt.Sprintf(`["%v"]`, "eventdata"), acc0.ID, acc0.KeyPair)
		s.Visitor.Commit()

		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")

		e := <-ch1
		So(e.Data, ShouldEqual, "eventdata")
		So(e.Topic, ShouldEqual, event.ContractEvent)

		// receipt event
		r, err = s.Call(cname, "receiptf", fmt.Sprintf(`["%v"]`, "receipteventdata"), acc0.ID, acc0.KeyPair)
		s.Visitor.Commit()

		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")

		e = <-ch2
		So(e.Data, ShouldEqual, "receipteventdata")
		So(e.Topic, ShouldEqual, event.ContractReceipt)
	})
}

func Test_RamPayer(t *testing.T) {
	ilog.Stop()
	Convey("test of ram payer", t, func() {
		s := NewSimulator()
		defer s.Clear()

		createAccountsWithResource(s)
		createToken(t, s, acc0)

		ca, err := s.Compile("", "./test_data/vmmethod", "./test_data/vmmethod")
		if err != nil || ca == nil {
			t.Fatal(err)
		}
		cname, r, err := s.DeployContract(ca, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Code, ShouldEqual, tx.Success)
		ram0 := s.GetRAM(acc0.ID)

		Convey("test of put and get", func() {
			//ram := s.GetRAM(acc0.ID)
			r, err := s.Call(cname, "putwithpayer", fmt.Sprintf(`["k", "v", "%v"]`, acc0.ID), acc0.ID, acc0.KeyPair)
			s.Visitor.Commit()
			So(s.GetRAM(acc0.ID), ShouldEqual, ram0-63)
			So(err, ShouldBeNil)
			So(r.Status.Code, ShouldEqual, tx.Success)

			r, err = s.Call(cname, "get", fmt.Sprintf(`["k"]`), acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Code, ShouldEqual, tx.Success)
			So(len(r.Returns), ShouldEqual, 1)
			So(r.Returns[0], ShouldEqual, "[\"v\"]")
		})

		Convey("test of map put and get", func() {
			ram0 := s.GetRAM(acc0.ID)
			r, err := s.Call(cname, "mapputwithpayer", fmt.Sprintf(`["k", "f", "v", "%v"]`, acc0.ID), acc0.ID, acc0.KeyPair)
			s.Visitor.Commit()
			So(err, ShouldBeNil)
			So(r.Status.Code, ShouldEqual, tx.Success)
			So(s.GetRAM(acc0.ID), ShouldEqual, ram0-65)

			r, err = s.Call(cname, "mapget", fmt.Sprintf(`["k", "f"]`), acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Code, ShouldEqual, tx.Success)
			So(len(r.Returns), ShouldEqual, 1)
			So(r.Returns[0], ShouldEqual, "[\"v\"]")
		})

		Convey("test of map put and get change payer", func() {
			ram0 := s.GetRAM(acc0.ID)
			r, err := s.Call(cname, "mapputwithpayer", fmt.Sprintf(`["k", "f", "vv", "%v"]`, acc0.ID), acc0.ID, acc0.KeyPair)
			s.Visitor.Commit()
			So(err, ShouldBeNil)
			So(r.Status.Code, ShouldEqual, tx.Success)
			So(s.GetRAM(acc0.ID), ShouldEqual, ram0-66)

			ram1 := s.GetRAM(acc1.ID)
			r, err = s.Call(cname, "mapputwithpayer", fmt.Sprintf(`["k", "f", "vvv", "%v"]`, acc1.ID), acc1.ID, acc1.KeyPair)
			s.Visitor.Commit()
			So(err, ShouldBeNil)
			So(r.Status.Code, ShouldEqual, tx.Success)
			So(s.GetRAM(acc0.ID), ShouldEqual, ram0)
			So(s.GetRAM(acc1.ID), ShouldEqual, ram1-67)

			ram1 = s.GetRAM(acc1.ID)
			r, err = s.Call(cname, "mapputwithpayer", fmt.Sprintf(`["k", "f", "v", "%v"]`, acc1.ID), acc1.ID, acc1.KeyPair)
			s.Visitor.Commit()
			So(err, ShouldBeNil)
			So(r.Status.Code, ShouldEqual, tx.Success)
			So(s.GetRAM(acc1.ID), ShouldEqual, ram1+2)

			ram1 = s.GetRAM(acc1.ID)
			r, err = s.Call(cname, "mapputwithpayer", fmt.Sprintf(`["k", "f", "vvvvv", "%v"]`, acc1.ID), acc1.ID, acc1.KeyPair)
			s.Visitor.Commit()
			So(err, ShouldBeNil)
			So(r.Status.Code, ShouldEqual, tx.Success)
			So(s.GetRAM(acc1.ID), ShouldEqual, ram1-4)
		})

		Convey("test nested call check payer", func() {
			ram0 := s.GetRAM(acc0.ID)
			if err != nil {
				t.Fatal(err)
			}
			ca, err := s.Compile("", "./test_data/nest0", "./test_data/nest0")
			if err != nil || ca == nil {
				t.Fatal(err)
			}
			cname0, r, err := s.DeployContract(ca, acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Code, ShouldEqual, tx.Success)

			ca, err = s.Compile("", "./test_data/nest1", "./test_data/nest1")
			if err != nil || ca == nil {
				t.Fatal(err)
			}
			cname1, r, err := s.DeployContract(ca, acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Code, ShouldEqual, tx.Success)

			So(s.GetRAM(acc0.ID), ShouldEqual, ram0-2994)

			ram0 = s.GetRAM(acc0.ID)
			ram4 := s.GetRAM(acc2.ID)
			ram6 := s.GetRAM(acc3.ID)
			s.Visitor.SetTokenBalanceFixed("iost", acc2.ID, "100")
			r, err = s.Call(cname0, "call", fmt.Sprintf(`["%v", "test", "%v"]`, cname1,
				fmt.Sprintf(`[\"%v\", \"%v\"]`, acc2.ID, acc3.ID)), acc2.ID, acc2.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			So(r.Status.Code, ShouldEqual, tx.Success)

			So(s.GetRAM(acc3.ID), ShouldEqual, ram6)
			So(s.GetRAM(acc2.ID), ShouldEqual, ram4)
			So(s.GetRAM(acc0.ID), ShouldEqual, ram0-6)
		})
	})
}

func Test_StackHeight(t *testing.T) {
	ilog.Stop()
	Convey("test of stack height", t, func() {
		s := NewSimulator()
		defer s.Clear()

		createAccountsWithResource(s)
		createToken(t, s, acc0)

		ca, err := s.Compile("", "./test_data/nest0", "./test_data/nest0")
		if err != nil || ca == nil {
			t.Fatal(err)
		}
		cname0, r, err := s.DeployContract(ca, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Code, ShouldEqual, tx.Success)

		ca, err = s.Compile("", "./test_data/nest1", "./test_data/nest1")
		if err != nil || ca == nil {
			t.Fatal(err)
		}
		cname1, r, err := s.DeployContract(ca, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Code, ShouldEqual, tx.Success)

		Convey("test of out of stack height", func() {
			r, err := s.Call(cname0, "sh0", fmt.Sprintf(`["%v"]`, cname1), acc0.ID, acc0.KeyPair)
			s.Visitor.Commit()
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldContainSubstring, "stack height exceed.")
		})
	})
}

func Test_Validate(t *testing.T) {
	ilog.Stop()
	s := NewSimulator()
	defer s.Clear()
	acc := prepareAuth(t, s)
	s.SetAccount(acc.ToAccount())
	s.SetGas(acc.ID, 10000000)
	s.SetRAM(acc.ID, 300)

	c, err := s.Compile("validate", "test_data/validate", "test_data/validate")
	assert.Nil(t, err)
	assert.Equal(t, len(c.Encode()), 133)

	_, r, err := s.DeployContract(c, acc.ID, acc.KeyPair)
	s.Visitor.Commit()
	assert.Contains(t, err.Error(), "abi not defined in source code: c")
	assert.Contains(t, r.Status.Message, "validate code error: , result: Error: abi not defined in source code: c")

	c, err = s.Compile("validate1", "test_data/validate1", "test_data/validate1")
	assert.Nil(t, err)
	_, r, err = s.DeployContract(c, acc.ID, acc.KeyPair)
	s.Visitor.Commit()
	assert.Contains(t, err.Error(), "Error: args should be one of ")
	assert.Contains(t, r.Status.Message, "validate code error: , result: Error: args should be one of ")

	c, err = s.Compile("validate2", "test_data/validate2", "test_data/validate2")
	assert.Nil(t, err)
	_, r, err = s.DeployContract(c, acc.ID, acc.KeyPair)
	s.Visitor.Commit()
	assert.Contains(t, err.Error(), "Error: abi shouldn't contain internal function: init")
	assert.Contains(t, r.Status.Message, "validate code error: , result: Error: abi shouldn't contain internal function: init")
}

func Test_SpecialChar(t *testing.T) {
	ilog.Stop()
	spStr := ""
	for i := 0x00; i <= 0x1F; i++ {
		spStr += fmt.Sprintf("const char%d = `%s`;\n", i, string(rune(i)))
	}
	spStr += fmt.Sprintf("const char%d = `%s`;\n", 0x7F, string(rune(0x7F)))
	for i := 0x80; i <= 0x9F; i++ {
		spStr += fmt.Sprintf("const char%d = `%s`;\n", i, string(rune(i)))
	}
	spStr += fmt.Sprintf("const char%d = `%s`;\n", 0x2028, string(rune(0x2028)))
	spStr += fmt.Sprintf("const char%d = `%s`;\n", 0x2029, string(rune(0x2029)))
	spStr += fmt.Sprintf("const char%d = `%s`;\n", 0xE0001, string(rune(0xE0001)))
	for i := 0xE0020; i <= 0xE007F; i++ {
		spStr += fmt.Sprintf("const char%d = `%s`;\n", i, string(rune(i)))
	}
	lst := []int64{0x061C, 0x200E, 0x200F, 0x202A, 0x202B, 0x202C, 0x202D, 0x202E, 0x2066, 0x2067, 0x2068, 0x2069}
	for _, i := range lst {
		spStr += fmt.Sprintf("const char%d = `%s`;\n", i, string(rune(i)))
	}
	for i := 0xE0100; i <= 0xE01EF; i++ {
		spStr += fmt.Sprintf("const char%d = `%s`;\n", i, string(rune(i)))
	}
	for i := 0x180B; i <= 0x180E; i++ {
		spStr += fmt.Sprintf("const char%d = `%s`;\n", i, string(rune(i)))
	}
	for i := 0x200C; i <= 0x200D; i++ {
		spStr += fmt.Sprintf("const char%d = `%s`;\n", i, string(rune(i)))
	}
	for i := 0xFFF0; i <= 0xFFFF; i++ {
		spStr += fmt.Sprintf("const char%d = `%s`;\n", i, string(rune(i)))
	}
	code := spStr +
		"class Test {" +
		"	init() {}" +
		"	transfer(from, to, amountJson) {" +
		"		blockchain.transfer(from, to, amountJson.amount, '');" +
		"	}" +
		"};" +
		"module.exports = Test;"

	abi := `
	{
		"lang": "javascript",
		"version": "1.0.0",
		"abi": [
			{
				"name": "transfer",
				"args": [
					"string",
					"string",
					"json"
				],
      			"amountLimit": [{
      			  "token": "iost",
      			  "val": "unlimited"
      			}]
			}
		]
	}
	`
	Convey("test validate", t, func() {
		s := NewSimulator()
		defer s.Clear()
		acc := prepareAuth(t, s)
		createAccountsWithResource(s)
		createToken(t, s, acc)
		s.SetGas(acc.ID, 10000000)
		s.SetRAM(acc.ID, 100000)

		c, err := (&contract.Compiler{}).Parse("", code, abi)
		So(err, ShouldBeNil)

		cname, _, err := s.DeployContract(c, acc.ID, acc.KeyPair)
		s.Visitor.Commit()
		So(err, ShouldBeNil)

		s.Visitor.SetTokenBalanceFixed("iost", acc.ID, "1000")
		s.Visitor.SetTokenBalanceFixed("iost", acc1.ID, "1000")
		params := []interface{}{
			acc.ID,
			acc1.ID,
			map[string]string{
				"amount": "1000",
				"hack":   "\u2028\u2029\u0000",
			},
		}
		paramsByte, err := json.Marshal(params)
		So(err, ShouldBeNil)
		r, err := s.Call(cname, "transfer", string(paramsByte), acc.ID, acc.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		So(s.Visitor.TokenBalanceFixed("iost", acc1.ID).String(), ShouldEqual, "2000")
	})
}

func Test_LargeContract(t *testing.T) {
	ilog.Stop()
	longStr := strings.Repeat("x", 1024*64)
	code := "class Test {" +
		"	init() {" +
		"		let longStr = '" + longStr + "';" +
		"	}" +
		"	transfer(from, to, amountJson) {" +
		"		blockchain.transfer(from, to, amountJson.amount, '');" +
		"	}" +
		"};" +
		"module.exports = Test;"

	abi := `
	{
		"lang": "javascript",
		"version": "1.0.0",
		"abi": [
			{
				"name": "transfer",
				"args": [
					"string",
					"string",
					"json"
				]
			}
		]
	}
	`
	Convey("test large contract", t, func() {
		s := NewSimulator()
		defer s.Clear()
		acc := prepareAuth(t, s)
		createAccountsWithResource(s)
		createToken(t, s, acc)
		s.SetGas(acc.ID, 1e12)
		s.SetRAM(acc.ID, 1e12)
		s.GasLimit = int64(1e12)

		c, err := (&contract.Compiler{}).Parse("", code, abi)
		So(err, ShouldBeNil)

		sc, err := json.Marshal(c)
		So(err, ShouldBeNil)

		jargs, err := json.Marshal([]string{string(sc)})
		So(err, ShouldBeNil)

		trx := tx.NewTx([]*tx.Action{{
			Contract:   "system.iost",
			ActionName: "setCode",
			Data:       string(jargs),
		}}, nil, int64(400000000), 100, s.Head.Time+100000000, 0, 0)

		trx.Time = s.Head.Time

		r, err := s.CallTx(trx, acc.ID, acc.KeyPair)
		s.Visitor.Commit()
		So(err, ShouldBeNil)
		So(r.Status.Code, ShouldEqual, tx.ErrorRuntime)
		So(r.Status.Message, ShouldContainSubstring, "out of gas")
	})
}

func Test_CallResult(t *testing.T) {
	ilog.Stop()
	Convey("test call result", t, func() {
		s := NewSimulator()
		defer s.Clear()
		acc := prepareAuth(t, s)
		s.SetAccount(acc.ToAccount())
		s.SetGas(acc.ID, 4000000)
		s.SetRAM(acc.ID, 10000)

		c, err := s.Compile("", "test_data/callresult", "test_data/callresult")
		So(err, ShouldBeNil)
		cname, r, err := s.DeployContract(c, acc.ID, acc.KeyPair)
		//s.Visitor.Put(cname+"-ret", database.MustMarshal("ab\x00c"))
		s.Visitor.Commit()
		So(err, ShouldBeNil)
		r, err = s.Call(cname, "ret_eof", `[]`, acc.ID, acc.KeyPair)
		So(err, ShouldBeNil)
		a := s.Visitor.Get(cname + "-ret")
		b := strings.NewReplacer("\x00", "\\x00").Replace(a)
		So(b, ShouldEqual, "sab\\x00c@"+cname)
		So(len(r.Returns), ShouldEqual, 1)
		So(r.Returns[0], ShouldEqual, `["ab\u0000cd"]`)
	})
}

func Test_ReturnObjectToJsonError(t *testing.T) {
	ilog.Stop()
	Convey("test return object toJSON error", t, func() {
		s := NewSimulator()
		defer s.Clear()
		acc := prepareAuth(t, s)
		s.SetAccount(acc.ToAccount())
		s.SetGas(acc.ID, 4000000)
		s.SetRAM(acc.ID, 10000)

		c, err := s.Compile("", "test_data/callresult", "test_data/callresult")
		So(err, ShouldBeNil)
		cname, r, err := s.DeployContract(c, acc.ID, acc.KeyPair)
		s.Visitor.Commit()
		So(err, ShouldBeNil)

		r, err = s.Call(cname, "ret_obj", `[]`, acc.ID, acc.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Code, ShouldEqual, tx.ErrorRuntime)
		So(r.Status.Message, ShouldContainSubstring, "error in JSON.stringfy")
	})
}

func Test_Exception(t *testing.T) {
	ilog.Stop()
	Convey("test throw exception", t, func() {
		s := NewSimulator()
		defer s.Clear()
		acc := prepareAuth(t, s)
		s.SetAccount(acc.ToAccount())
		s.SetGas(acc.ID, 4000000)
		s.SetRAM(acc.ID, 10000)

		c, err := s.Compile("", "test_data/vmmethod", "test_data/vmmethod")
		So(err, ShouldBeNil)
		cname, r, err := s.DeployContract(c, acc.ID, acc.KeyPair)
		s.Visitor.Commit()
		So(err, ShouldBeNil)

		r, err = s.Call(cname, "testException0", `[]`, acc.ID, acc.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Code, ShouldEqual, tx.Success)
		//So(r.Status.Message, ShouldContainSubstring, "test exception")
	})
}

func Test_Arguments(t *testing.T) {
	ilog.Stop()
	Convey("test of arguments", t, func() {
		s := NewSimulator()
		defer s.Clear()

		createAccountsWithResource(s)
		createToken(t, s, acc0)

		ca, err := s.Compile("Contractarguments", "./test_data/arguments", "./test_data/arguments.js")
		if err != nil || ca == nil {
			t.Fatal(err)
		}
		cname, r, err := s.DeployContract(ca, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Code, ShouldEqual, tx.Success)

		arguments := `["abc",1,true,{}]`
		r, err = s.Call(cname, "test", arguments, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")

		arguments = `[null,1,true,{}]`
		r, err = s.Call(cname, "test", arguments, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldContainSubstring, `error parse string arg &{<nil>}`)

		arguments = `[{"c":1,"d":2,"a":3,"b":4},1,true,{}]`
		r, err = s.Call(cname, "test", arguments, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldContainSubstring, `error parse string arg &{"a":3,"b":4,"c":1,"d":2}`)

		arguments = `["abc",[{"c":1,"d":2,"a":3,"b":4}],true,{}]`
		r, err = s.Call(cname, "test", arguments, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldContainSubstring, `error parse number arg &[{"a":3,"b":4,"c":1,"d":2}]`)

		arguments = `["abc",1,{"c":1,"d":2,"a":3,"b":4},{}]`
		r, err = s.Call(cname, "test", arguments, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldContainSubstring, `error parse bool arg &{"a":3,"b":4,"c":1,"d":2}`)

		arguments = `[10,1,true,{}]`
		r, err = s.Call(cname, "test", arguments, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldContainSubstring, `error parse string arg &{10}`)

		arguments = `["abc",true,true,{}]`
		r, err = s.Call(cname, "test", arguments, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldContainSubstring, `error parse number arg &{true}`)

		arguments = `["abc",1,"abc",{}]`
		r, err = s.Call(cname, "test", arguments, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldContainSubstring, `error parse bool arg &{abc}`)

		arguments = `["abc",1,true,1]`
		r, err = s.Call(cname, "test", arguments, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
	})
}

func Test_MapDel(t *testing.T) {
	ilog.Stop()
	s := NewSimulator()
	defer s.Clear()

	acc := prepareAuth(t, s)
	s.SetAccount(acc.ToAccount())
	s.SetGas(acc.ID, 4000000)
	s.SetRAM(acc.ID, 10000)

	c, err := s.Compile("", "test_data/mapdel", "test_data/mapdel.js")
	assert.Nil(t, err)
	cname, _, err := s.DeployContract(c, acc.ID, acc.KeyPair)
	s.Visitor.Commit()
	assert.Nil(t, err)

	r, err := s.Call(cname, "keys", `[]`, acc.ID, acc.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Len(t, r.Returns, 1)
	assert.Equal(t, `["[\"s\",\"abcd\",\"abc\",\"ab\"]"]`, r.Returns[0])

	r, err = s.Call(cname, "del", `[]`, acc.ID, acc.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Len(t, r.Returns, 1)
	assert.Equal(t, `["[\"s\",\"abcd\",\"abc\"]"]`, r.Returns[0])
}

func Test_MapDel2(t *testing.T) {
	ilog.Stop()
	s := NewSimulator()
	defer s.Clear()

	acc := prepareAuth(t, s)
	s.SetAccount(acc.ToAccount())
	s.SetGas(acc.ID, 4000000)
	s.SetRAM(acc.ID, 10000)

	c, err := s.Compile("", "test_data/mapdel", "test_data/mapdel.js")
	assert.Nil(t, err)
	cname, _, err := s.DeployContract(c, acc.ID, acc.KeyPair)
	s.Visitor.Commit()
	assert.Nil(t, err)

	conf := &common.Config{
		P2P: &common.P2PConfig{
			ChainID: 1024,
		},
	}
	version.InitChainConf(conf)
	rules := version.NewRules(0)
	assert.False(t, rules.IsFork3_1_0)
	s.Visitor = database.NewVisitor(0, s.Mvcc, rules)

	r, err := s.Call(cname, "keys", `[]`, acc.ID, acc.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Len(t, r.Returns, 1)
	assert.Equal(t, `["[\"scd\",\"abc\",\"ab\",\"ab\"]"]`, r.Returns[0])

	conf.P2P.ChainID = 1000
	version.InitChainConf(conf)
	rules = version.NewRules(0)
	assert.True(t, rules.IsFork3_1_0)
	s.Visitor = database.NewVisitor(0, s.Mvcc, rules)

	r, err = s.Call(cname, "del", `[]`, acc.ID, acc.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Len(t, r.Returns, 1)
	assert.Equal(t, `["[\"abc\"]"]`, r.Returns[0])

	conf.P2P.ChainID = 0
	version.InitChainConf(conf)
}

func Test_SelfAuth(t *testing.T) {
	ilog.Stop()
	s := NewSimulator()
	defer s.Clear()

	acc := prepareAuth(t, s)
	s.SetAccount(acc.ToAccount())
	s.SetGas(acc.ID, 4000000)
	s.SetRAM(acc.ID, 10000)

	c, err := s.Compile("", "test_data/auth", "test_data/auth.js")
	assert.Nil(t, err)
	cname, _, err := s.DeployContract(c, acc.ID, acc.KeyPair)
	s.Visitor.Commit()
	assert.Nil(t, err)

	r, err := s.Call(cname, "inner", `[]`, acc.ID, acc.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Len(t, r.Returns, 1)
	assert.Equal(t, `["false"]`, r.Returns[0])

	r, err = s.Call(cname, "outer", `[]`, acc.ID, acc.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Len(t, r.Returns, 1)
	assert.Equal(t, `["[\"true\"]"]`, r.Returns[0])
}
