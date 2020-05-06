package v8vm

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	. "github.com/golang/mock/gomock"
	"github.com/iost-official/go-iost/common"
	"github.com/iost-official/go-iost/core/contract"
	"github.com/iost-official/go-iost/core/version"
	"github.com/iost-official/go-iost/crypto"
	"github.com/iost-official/go-iost/ilog"
	"github.com/iost-official/go-iost/vm/database"
	"github.com/iost-official/go-iost/vm/host"
	v8 "github.com/iost-official/go-iost/vm/v8vm"
	. "github.com/smartystreets/goconvey/convey"
)

var vmPool *v8.VMPool

func init() {
	vmPool = v8.NewVMPool(3, 3)
	//vmPool.SetJSPath("./v8/libjs/")
	vmPool.Init()
}

var testDataPath = "./test_data/"

func Init(t *testing.T) *database.Visitor {
	mc := NewController(t)
	defer mc.Finish()
	db := database.NewMockIMultiValue(mc)
	vi := database.NewVisitor(100, db, version.NewRules(0))
	return vi
}

func MyInit(t *testing.T, conName string, optional ...interface{}) (*host.Host, *contract.Contract) {
	db := database.NewDatabaseFromPath(testDataPath + conName + ".json")
	vi := database.NewVisitor(100, db, version.NewRules(0))

	ctx := host.NewContext(nil)
	ctx.Set("gas_ratio", int64(100))
	var gasLimit = int64(10000000)
	if len(optional) > 0 {
		gasLimit = optional[0].(int64)
	}
	ctx.GSet("gas_limit", gasLimit)
	ctx.Set("contract_name", conName)
	h := host.NewHost(ctx, vi, version.NewRules(0), nil, ilog.DefaultLogger())

	fd, err := ioutil.ReadFile(testDataPath + conName + ".js")
	if err != nil {
		t.Fatal("Read file failed: ", err.Error())
		return nil, nil
	}
	rawCode := string(fd)

	code := &contract.Contract{
		ID:   conName,
		Code: rawCode,
	}

	expTime := time.Now().Add(time.Second * 10)
	h.SetDeadline(expTime)
	code.Code, err = vmPool.Compile(code)
	if err != nil {
		t.Fatal(err)
	}

	return h, code
}

func TestEngine_LoadAndCall(t *testing.T) {
	vi := Init(t)
	ctx := host.NewContext(nil)
	ctx.Set("gas_ratio", int64(100))
	ctx.GSet("gas_limit", int64(1000000000))
	ctx.Set("contract_name", "contractName")
	tHost := host.NewHost(ctx, vi, version.NewRules(0), nil, nil)
	expTime := time.Now().Add(time.Second * 10)
	tHost.SetDeadline(expTime)

	code := &contract.Contract{
		ID: "test.js",
		Code: `
var Contract = function() {
}

	Contract.prototype = {
	fibonacci: function(cycles) {
			if (cycles == 0) return 0;
			if (cycles == 1) return 1;
			return this.fibonacci(cycles - 1) + this.fibonacci(cycles - 2);
		}
	}

	module.exports = Contract
`,
	}

	rs, _, err := vmPool.LoadAndCall(tHost, code, "fibonacci", "12")

	if err != nil {
		t.Fatalf("LoadAndCall run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0] != "144" {
		t.Fatalf("LoadAndCall except 144, got %s\n", rs[0])
	}
}

func TestEngine_bigNumber(t *testing.T) {
	host, code := MyInit(t, "bignumber1")
	vmPool.LoadAndCall(host, code, "constructor")
	rs, _, err := vmPool.LoadAndCall(host, code, "getVal")

	if err != nil {
		t.Fatalf("LoadAndCall getVal error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "\"8.00029e-11\"" {
		t.Errorf("LoadAndCall except 8.00029e-11, got %s\n", rs[0])
	}
}

// nolint
func TestEngine_Storage(t *testing.T) {
	host, code := MyInit(t, "storage1")

	// vmPool.LoadAndCall(host, code, "constructor")
	rs, _, err := vmPool.LoadAndCall(host, code, "get", "a")
	if err != nil {
		t.Fatalf("LoadAndCall get run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "1000" {
		t.Fatalf("LoadAndCall except mySetVal, got %s\n", rs[0])
	}

	rtn, _, err := vmPool.LoadAndCall(host, code, "put", "mySetKey", "mySetVal")
	if err != nil {
		t.Fatalf("LoadAndCall put run error: %v\n", err)
	}
	if len(rtn) != 1 || rtn[0] != "" {
		t.Fatalf("return of put should be \"\"")
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "has", "mySetKey", "string")
	if err != nil {
		t.Fatalf("LoadAndCall has run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "true" {
		t.Fatalf("LoadAndCall except false, got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "get", "mySetKey")
	if err != nil {
		t.Fatalf("LoadAndCall get run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "mySetVal" {
		t.Fatalf("LoadAndCall except mySetVal, got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "getThisNum")
	if err != nil {
		t.Fatalf("LoadAndCall getThisNum run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "99" {
		t.Fatalf("LoadAndCall except 99, got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "getThisStr")
	if err != nil {
		t.Fatalf("LoadAndCall getThisStr run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "yeah" {
		t.Fatalf("LoadAndCall except yeah, got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "has", "mySetKeynotfound", "string")
	if err != nil {
		t.Fatalf("LoadAndCall has run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "false" {
		t.Fatalf("LoadAndCall except false, got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "get", "mySetKeynotfound", "string")
	if err != nil {
		t.Fatalf("LoadAndCall get run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "null" {
		t.Fatalf("LoadAndCall except null, got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "mhas", "ptable", "a")
	if err != nil {
		t.Fatalf("LoadAndCall mhas run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "false" {
		t.Fatalf("LoadAndCall except false, got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "mset", "ptable", "a", "aa")
	if err != nil {
		t.Fatalf("LoadAndCall mset run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "" {
		t.Fatalf("LoadAndCall except , got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "mhas", "ptable", "a")
	if err != nil {
		t.Fatalf("LoadAndCall mhas run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "true" {
		t.Fatalf("LoadAndCall except true, got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "mget", "ptable", "a")
	if err != nil {
		t.Fatalf("LoadAndCall mget run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "aa" {
		t.Fatalf("LoadAndCall except aa, got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "mset", "ptable", "b", "aa")
	if err != nil {
		t.Fatalf("LoadAndCall mset run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "" {
		t.Fatalf("LoadAndCall except , got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "mkeys", "ptable")
	if err != nil {
		t.Fatalf("LoadAndCall mhas run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "[\"a\",\"b\"]" {
		t.Fatalf("LoadAndCall except [\"a\",\"b\"], got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "mlen", "ptable")
	if err != nil {
		t.Fatalf("LoadAndCall mhas run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "2" {
		t.Fatalf("LoadAndCall except 2, got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "mdelete", "ptable", "a")
	if err != nil {
		t.Fatalf("LoadAndCall mdelete run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "" {
		t.Fatalf("LoadAndCall except , got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "mhas", "ptable", "a")
	if err != nil {
		t.Fatalf("LoadAndCall mhas run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "false" {
		t.Fatalf("LoadAndCall except false, got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "mkeys", "ptable")
	if err != nil {
		t.Fatalf("LoadAndCall mhas run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "[\"b\"]" {
		t.Fatalf("LoadAndCall except [\"b\"], got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "mlen", "ptable")
	if err != nil {
		t.Fatalf("LoadAndCall mhas run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "1" {
		t.Fatalf("LoadAndCall except 2, got %s\n", rs[0])
	}
}

// nolint
func TestEngine_Storage2(t *testing.T) {
	host, code := MyInit(t, "storage2")

	rs, _, err := vmPool.LoadAndCall(host, code, "put", "a", "aa")
	if err != nil {
		t.Fatalf("LoadAndCall put run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "" {
		t.Fatalf("LoadAndCall except , got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "mset", "a", "b", "bb")
	if err != nil {
		t.Fatalf("LoadAndCall mset run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "" {
		t.Fatalf("LoadAndCall except , got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "ghas", "storage2", "a")
	if err != nil {
		t.Fatalf("LoadAndCall ghas run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "true" {
		t.Fatalf("LoadAndCall except true, got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "gget", "storage2", "a")
	if err != nil {
		t.Fatalf("LoadAndCall get run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "aa" {
		t.Fatalf("LoadAndCall except aa, got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "gmhas", "storage2", "a", "b")
	if err != nil {
		t.Fatalf("LoadAndCall get run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "true" {
		t.Fatalf("LoadAndCall except null, got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "gmget", "storage2", "a", "b")
	if err != nil {
		t.Fatalf("LoadAndCall get run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "bb" {
		t.Fatalf("LoadAndCall except bb, got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "gmkeys", "storage2", "a")
	if err != nil {
		t.Fatalf("LoadAndCall mhas run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "[\"b\"]" {
		t.Fatalf("LoadAndCall except [\"b\"], got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "gmlen", "storage2", "a")
	if err != nil {
		t.Fatalf("LoadAndCall mhas run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "1" {
		t.Fatalf("LoadAndCall except 1, got %s\n", rs[0])
	}

	// test owner
	rs, _, err = vmPool.LoadAndCall(host, code, "puto", "a", "aa")
	if err != nil {
		t.Fatalf("LoadAndCall puto run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "" {
		t.Fatalf("LoadAndCall except , got %s\n", rs[0])
	}
	rs, _, err = vmPool.LoadAndCall(host, code, "ggeto", "storage2", "a")
	if err != nil {
		t.Fatalf("LoadAndCall ggeto run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "aa" {
		t.Fatalf("LoadAndCall except aa, got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "mseto", "a", "b", "bb")
	if err != nil {
		t.Fatalf("LoadAndCall mseto run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "" {
		t.Fatalf("LoadAndCall except , got %s\n", rs[0])
	}
	rs, _, err = vmPool.LoadAndCall(host, code, "gmgeto", "storage2", "a", "b")
	if err != nil {
		t.Fatalf("LoadAndCall gmgeto run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "bb" {
		t.Fatalf("LoadAndCall except bb, got %s\n", rs[0])
	}
}

// nolint
func TestEngine_DataType(t *testing.T) {
	host, code := MyInit(t, "datatype")

	rs, _, err := vmPool.LoadAndCall(host, code, "number", 1)
	if err != nil {
		t.Fatalf("LoadAndCall number run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0] != "0.5555555555" {
		t.Fatalf("LoadAndCall except 0.5555555555, got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "number_big", 1)
	if err != nil {
		t.Fatalf("LoadAndCall number_big run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0] != "0.5555555555" {
		t.Fatalf("LoadAndCall except 0.5555555555, got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "number_op")
	if err != nil {
		t.Fatalf("LoadAndCall number_op run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0] != "3" {
		t.Fatalf("LoadAndCall except 3, got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "number_op2")
	if err != nil {
		t.Fatalf("LoadAndCall number_op2 run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0] != "2" {
		t.Fatalf("LoadAndCall except 2, got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "number_strange")
	if err != nil {
		t.Fatalf("LoadAndCall number_strange run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "-Infinity" {
		t.Fatalf("LoadAndCall except Infinity, got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "param", 2)
	if err != nil {
		t.Fatalf("LoadAndCall param run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "4" {
		t.Fatalf("LoadAndCall except 4, got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "param2")
	if err != nil {
		t.Fatalf("LoadAndCall param2 run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0] != "" {
		t.Fatalf("LoadAndCall except undefined, got %s\n", rs[0])
	}

	b, _ := json.Marshal([]int64{1, 2})
	rs, _, err = vmPool.LoadAndCall(host, code, "param3", b)
	if err != nil {
		t.Fatalf("LoadAndCall param3 run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0] != "[1,2,{\"a\":3}]" {
		t.Fatalf("LoadAndCall except [1,2,{\"a\":3}], got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "bool")
	if err != nil {
		t.Fatalf("LoadAndCall bool run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "false" {
		t.Fatalf("LoadAndCall except undefined, got %s\n", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "string")
	if err != nil {
		t.Fatalf("LoadAndCall string run error: %v\n", err)
	}
	if len(rs) != 1 || len(rs[0].(string)) != 4096 {
		t.Fatalf("LoadAndCall except len 4096, got %d\n", len(rs[0].(string)))
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "array")
	if err != nil {
		t.Fatalf("LoadAndCall array run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "[0,1,2,3]" {
		t.Fatalf("LoadAndCall except [0,1,2,3], got %s\n", rs[0].(string))
	}

	// todo get return string [object Object]
	/*
		rs, _,err = e.LoadAndCall(host, code, "object")
		if err != nil {
			t.Fatalf("LoadAndCall array run error: %v\n", err)
		}
		if len(rs) != 1 || rs[0].(string) != "0,1,2,3"{
			t.Errorf("LoadAndCall except 0,1,2,3, got %s\n", rs[0].(string))
		}
	*/
}

// nolint
func TestEngine_Loop(t *testing.T) {
	t.Skip()
	host, code := MyInit(t, "loop")

	_, _, err := vmPool.LoadAndCall(host, code, "for")
	if err == nil || err.Error() != "out of gas" {
		t.Fatalf("LoadAndCall for should return error: out of gas, but got %v\n", err)
	}

	host, code = MyInit(t, "loop")
	_, _, err = vmPool.LoadAndCall(host, code, "for2")
	if err == nil || err.Error() != "out of gas" {
		t.Fatalf("LoadAndCall for should return error: out of gas, but got %v\n", err)
	}

	host, code = MyInit(t, "loop")
	rs, _, err := vmPool.LoadAndCall(host, code, "for3")
	if err != nil {
		t.Fatalf("LoadAndCall for3 run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "10" {
		t.Fatalf("LoadAndCall except 10, got %s\n", rs[0].(string))
	}

	host, code = MyInit(t, "loop")
	rs, _, err = vmPool.LoadAndCall(host, code, "forin")
	if err != nil {
		t.Fatalf("LoadAndCall forin run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "12" {
		t.Fatalf("LoadAndCall except 12, got %s\n", rs[0].(string))
	}

	host, code = MyInit(t, "loop")
	rs, _, err = vmPool.LoadAndCall(host, code, "forof")
	if err != nil {
		t.Fatalf("LoadAndCall forof run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "6" {
		t.Fatalf("LoadAndCall except 6, got %s\n", rs[0].(string))
	}

	host, code = MyInit(t, "loop")
	_, _, err = vmPool.LoadAndCall(host, code, "while")
	if err == nil || err.Error() != "out of gas" {
		t.Fatalf("LoadAndCall for should return error: out of gas, but got %v\n", err)
	}

	host, code = MyInit(t, "loop")
	_, _, err = vmPool.LoadAndCall(host, code, "dowhile")
	if err == nil || err.Error() != "out of gas" {
		t.Fatalf("LoadAndCall for should return error: out of gas, but got %v\n", err)
	}
}

func TestEngine_Func(t *testing.T) {
	// Please @shiqi fix it
	t.SkipNow()
	host, code := MyInit(t, "func")
	_, _, err := vmPool.LoadAndCall(host, code, "func1")
	if err == nil || err.Error() != "out of gas" {
		t.Fatalf("LoadAndCall for should return error: out of gas, but got %v\n", err)
	}

	// illegal instruction on mac
	/*
		host, code = MyInit(t, "func", int64(100000000000))
		_, _, err = vmPool.LoadAndCall(host, code, "func1")
		if err == nil || !strings.Contains(err.Error(), "Uncaught exception: RangeError: Maximum call stack size exceeded") {
			t.Fatalf("LoadAndCall for should return error: Uncaught exception: RangeError: Maximum call stack size exceeded, but got %v\n", err)
		}
	*/

	host, code = MyInit(t, "func")
	rs, _, err := vmPool.LoadAndCall(host, code, "func3", 4)
	if err != nil {
		t.Fatalf("LoadAndCall func3 run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "9" {
		t.Fatalf("LoadAndCall except 9, got %s\n", rs[0].(string))
	}

	host, code = MyInit(t, "func")
	rs, _, err = vmPool.LoadAndCall(host, code, "func4")
	if err != nil {
		t.Fatalf("LoadAndCall func4 run error: %v\n", err)
	}
	if len(rs) != 1 || rs[0].(string) != "[2,5,5]" {
		t.Fatalf("LoadAndCall except [2,5,5], got %s\n", rs[0].(string))
	}
}

func TestEngine_Danger(t *testing.T) {
	host, code := MyInit(t, "danger", int64(1e12))
	_, _, err := vmPool.LoadAndCall(host, code, "jsonparse")
	if err == nil || !strings.Contains(err.Error(), "SyntaxError: Unexpected token o in JSON at position 1") {
		t.Fatalf("LoadAndCall jsonparse should return error: SyntaxError: Unexpected token o in JSON at position 1, got err = %v\n", err)
	}

	rtn, cost, err := vmPool.LoadAndCall(host, code, "objadd")
	if err != nil || cost.ToGas() != int64(5052845) {
		t.Fatalf("LoadAndCall objadd should cost 5052842, got err = %v, cost = %v, rtn = %v\n", err, cost.ToGas(), rtn)
	}

	_, _, err = vmPool.LoadAndCall(host, code, "tooBigArray")
	if err == nil || !strings.Contains(err.Error(), "IOSTContractInstruction_Incr gas overflow max int") {
		t.Fatalf("LoadAndCall tooBigArray should return error: Uncaught exception: IOSTContractInstruction_Incr gas overflow max int, got %v\n", err)
	}

	_, _, err = vmPool.LoadAndCall(host, code, "bigArray")
	if err == nil || !strings.Contains(err.Error(), "result too long") {
		t.Fatalf("LoadAndCall bigArray should return error: result too long, got %v\n", err)
	}

	_, _, err = vmPool.LoadAndCall(host, code, "visitUndefined")
	if err == nil || !strings.Contains(err.Error(), "Uncaught exception: TypeError: Cannot set property 'c' of undefined") {
		t.Fatalf("LoadAndCall visitUndefined should return error: Uncaught exception: TypeError: Cannot set property 'c' of undefined, but got %v\n", err)
	}

	_, _, err = vmPool.LoadAndCall(host, code, "throw")
	if err == nil || !strings.Contains(err.Error(), "Uncaught exception: test throw") {
		t.Fatalf("LoadAndCall throw should return error: Uncaught exception: test throw, but got %v\n", err)
	}

	_, _, err = vmPool.LoadAndCall(host, code, "putlong")
	if err == nil || !strings.Contains(err.Error(), "input string too long") {
		t.Fatalf("LoadAndCall putlong should return error: input string too long, but got %v\n", err)
	}

	// will fail in compile
	/*
		_, _, err = vmPool.LoadAndCall(host, code, "asyncfunc")
		if err == nil || !strings.Contains(err.Error(), "use of async or await is not supported") {
			t.Fatalf("LoadAndCall throw should return error: use of async or await is not supported, but got %v\n", err)
		}
	*/
}

// nolint
func TestEngine_Int64(t *testing.T) {
	host, code := MyInit(t, "int64Test")
	rs, _, err := vmPool.LoadAndCall(host, code, "getPlus")
	if err != nil {
		t.Fatalf("LoadAndCall getPlus error: %v", err)
	}
	if len(rs) > 0 && rs[0] != "1234501234" {
		t.Fatalf("LoadAndCall getPlus except: , got: %v", rs[0])
	}
	rs, _, err = vmPool.LoadAndCall(host, code, "getMinus")
	if err != nil {
		t.Fatalf("LoadAndCall getMinus error: %v", err)
	}
	if len(rs) > 0 && rs[0] != "123400109" {
		t.Fatalf("LoadAndCall getMinus except: , got: %v", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "getMulti")
	if err != nil {
		t.Fatalf("LoadAndCall getMulti error: %v", err)
	}
	if len(rs) > 0 && rs[0] != "148148004" {
		t.Fatalf("LoadAndCall getMulti except: , got: %v", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "getDiv")
	if err != nil {
		t.Fatalf("LoadAndCall getDiv error: %v", err)
	}
	if len(rs) > 0 && rs[0] != "1028805" {
		t.Fatalf("LoadAndCall getDiv except: , got: %v", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "getMod")
	if err != nil {
		t.Fatalf("LoadAndCall getMod error: %v", err)
	}
	if len(rs) > 0 && rs[0] != "7" {
		t.Fatalf("LoadAndCall getMod except: , got: %v", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "getPow", 3)
	if err != nil {
		t.Fatalf("LoadAndCall getPow error: %v", err)
	}
	if len(rs) > 0 && rs[0] != "1879080904" {
		t.Fatalf("LoadAndCall getPow except: 1879080904, got: %v", rs[0])
	}
}

func TestEngine_Console(t *testing.T) {
	host, code := MyInit(t, "console1")
	_, _, err := vmPool.LoadAndCall(host, code, "log")
	if err != nil && !strings.Contains(err.Error(), "console.fatal is not a function") {
		t.Fatalf("LoadAndCall console error: %v", err)
	}
}

func TestEngine_Float64(t *testing.T) {
	host, code := MyInit(t, "float64Test")
	rs, _, err := vmPool.LoadAndCall(host, code, "number")
	if err != nil {
		t.Fatalf("LoadAndCall console error: %v", err)
	}
	if len(rs) > 0 && rs[0] != "11.11" {
		t.Fatalf("LoadAndCall number except: 11.11, got: %v", rs[0])
	}
	rs, _, err = vmPool.LoadAndCall(host, code, "getMinus")
	if err != nil {
		t.Fatalf("LoadAndCall getMinus error: %v", err)
	}
	if len(rs) > 0 && rs[0] != "11665.6789" {
		t.Fatalf("LoadAndCall getMinus except: 11665.6789, got: %v", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "getMulti", 3)
	if err != nil {
		t.Fatalf("LoadAndCall getMulti error: %v", err)
	}
	if len(rs) > 0 && rs[0] != "148148.1468" {
		t.Fatalf("LoadAndCall getMulti except: 148148.1468, got: %v", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "getDiv", 3)
	if err != nil {
		t.Fatalf("LoadAndCall getDiv error: %v", err)
	}
	if len(rs) > 0 && rs[0] != "1028.806575" {
		t.Fatalf("LoadAndCall getDiv except: 1028.806575, got: %v", rs[0])
	}

	rs, _, err = vmPool.LoadAndCall(host, code, "getPow", 3)
	if err != nil {
		t.Fatalf("LoadAndCall getPow error: %v", err)
	}
	if len(rs) > 0 && rs[0] != "1881676371789.154860897069" {
		t.Fatalf("LoadAndCall getPow except: 1881676371789.154860897069, got: %v", rs[0])
	}
}

func TestEngine_Crypto(t *testing.T) {
	host, code := MyInit(t, "crypto1")

	// test sha3
	testStr := "hello world"
	rs, _, err := vmPool.LoadAndCall(host, code, "sha3", testStr)
	if err != nil {
		t.Fatalf("LoadAndCall console error: %v", err)
	}
	if rs[0] != common.Base58Encode(common.Sha3([]byte(testStr))) {
		t.Fatalf("LoadAndCall sha3 invalid result")
	}

	// test normal case
	algo := crypto.Ed25519
	secKey := algo.GenSeckey()
	pubKey := algo.GetPubkey(secKey)
	msg := []byte(testStr)
	rs, c, err := vmPool.LoadAndCall(host, code, "verify", algo.String(), common.Base58Encode(msg),
		common.Base58Encode(algo.Sign(msg, secKey)), common.Base58Encode(pubKey))
	if err != nil {
		t.Fatalf("LoadAndCall error: %v", err)
	}
	if rs[0] != "1" {
		t.Fatalf("LoadAndCall verify invalid result %v", rs[0])
	}
	if c.ToGas() != 285 {
		t.Fatalf("wrong gas %v", c.ToGas())
	}
	rs, c, err = vmPool.LoadAndCall(host, code, "verify", algo.String(), common.Base58Encode(msg[1:]),
		common.Base58Encode(algo.Sign(msg, secKey)), common.Base58Encode(pubKey))
	if err != nil {
		t.Fatalf("LoadAndCall error: %v", err)
	}
	if rs[0] != "0" {
		t.Fatalf("LoadAndCall verify invalid result %v", rs[0])
	}
	if c.ToGas() != 284 {
		t.Fatalf("wrong gas %v", c.ToGas())
	}

	// test wrong algo
	rs, c, err = vmPool.LoadAndCall(host, code, "verify", "wrong_algo", common.Base58Encode(msg),
		common.Base58Encode(algo.Sign(msg, secKey)), common.Base58Encode(pubKey))
	if err != nil {
		t.Fatalf("LoadAndCall error: %v", err)
	}
	if rs[0] != "0" {
		t.Fatalf("LoadAndCall verify invalid result %v", rs[0])
	}
	if c.ToGas() != 285 {
		t.Fatalf("wrong gas %v", c.ToGas())
	}
}


func TestEngine_Crypto2(t *testing.T) {
	host, code := MyInit(t, "crypto2", int64(1e8))
	helloWorld := "hello world"
	input := "823b54d3aabaf8e3122800ca5238afb2ccef071ce83b8d5654a597a5dd06347e"

	// invalid function before fork
	host.IsFork3_3_1 = false
	rs, c, err := vmPool.LoadAndCall(host, code, "ripemd160Hex", input)
	if err == nil || !strings.Contains(err.Error(), "IOSTCrypto.ripemd160Hex is not a function") {
		t.Fatalf("LoadAndCall console error: %v", err)
	}

	if c.ToGas() != 180 {
		t.Fatalf("invalid gas %v", c.ToGas())
	}

	host.IsFork3_3_1 = true
	// test sha3Hex
	rs, c, err = vmPool.LoadAndCall(host, code, "sha3Hex", common.ToHex([]byte(helloWorld)))
	if err != nil {
		t.Fatalf("LoadAndCall console error: %v", err)
	}

	if c.ToGas() != 291 {
		t.Fatalf("invalid gas %v", c.ToGas())
	}
	if rs[0] != common.ToHex(common.Sha3([]byte(helloWorld))) {
		t.Fatalf("LoadAndCall sha3Hex invalid result")
	}
	rs, c, err = vmPool.LoadAndCall(host, code, "sha3Hex", helloWorld)
	if err != nil {
		t.Fatalf("LoadAndCall console error: %v", err)
	}

	if c.ToGas() != 280 {
		t.Fatalf("invalid gas %v", c.ToGas())
	}
	if rs[0] != "" {
		t.Fatalf("LoadAndCall sha3Hex invalid result")
	}

	// test ripemd160Hex
	rs, c, err = vmPool.LoadAndCall(host, code, "ripemd160Hex", input)
	if err != nil {
		t.Fatalf("LoadAndCall console error: %v", err)
	}

	if c.ToGas() != 312 {
		t.Fatalf("invalid gas %v", c.ToGas())
	}
	expected := "3dbb2167cbfc2186343356125fff4163e6ebcce7"
	if rs[0] != expected {
		t.Fatalf("LoadAndCall ripemd160Hex invalid result")
	}
	rs, c, err = vmPool.LoadAndCall(host, code, "ripemd160Hex", helloWorld)
	if err != nil {
		t.Fatalf("LoadAndCall console error: %v", err)
	}
	if rs[0] != "" {
		t.Fatalf("LoadAndCall ripemd160Hex invalid result")
	}
	if c.ToGas() != 280 {
		t.Fatalf("invalid gas %v", c.ToGas())
	}
}

func TestEngine_ArrayOfFrom(t *testing.T) {
	host, code := MyInit(t, "arrayfunc")
	_, _, err := vmPool.LoadAndCall(host, code, "from")
	if err != nil && !strings.Contains(err.Error(), "is not a function") {
		t.Fatalf("LoadAndCall array from error: %v", err)
	}
	_, _, err = vmPool.LoadAndCall(host, code, "to")
	if err != nil && !strings.Contains(err.Error(), "is not a function") {
		t.Fatalf("LoadAndCall array from error: %v", err)
	}
}

func TestNativeRun(t *testing.T) {
	host, code := MyInit(t, "danger")
	Convey("test nativerun0", t, func() {
		_, _, err := vmPool.LoadAndCall(host, code, "nativerun")
		So(err.Error(), ShouldContainSubstring, "TypeError: _native_run is not a function")
	})
}

func TestV8Safe(t *testing.T) {
	host, code := MyInit(t, "v8Safe")
	_, _, err := vmPool.LoadAndCall(host, code, "CVE_2018_6149")
	if err != nil && !strings.Contains(err.Error(), "out of gas") {
		t.Fatalf("LoadAndCall V8Safe CVE_2018_6149 error: %v", err)
	}

	_, _, err = vmPool.LoadAndCall(host, code, "CVE_2018_6143")
	if err == nil {
		t.Fatalf("LoadAndCall V8Safe CVE_2018_6143 should return error")
	}

	_, _, err = vmPool.LoadAndCall(host, code, "CVE_2018_6136")
	if err == nil {
		t.Fatalf("LoadAndCall V8Safe CVE_2018_6136 should return error")
	}

	_, _, err = vmPool.LoadAndCall(host, code, "CVE_2018_6092")
	if err == nil {
		t.Fatalf("LoadAndCall V8Safe CVE_2018_6092 should return error")
	}

	_, _, err = vmPool.LoadAndCall(host, code, "CVE_2018_6065")
	if err == nil {
		t.Fatalf("LoadAndCall V8Safe CVE_2018_6065 should return error")
	}

	_, _, err = vmPool.LoadAndCall(host, code, "CVE_2018_6056")
	if err == nil {
		t.Fatalf("LoadAndCall V8Safe CVE_2018_6056 should return error")
	}
	_, _, err = vmPool.LoadAndCall(host, code, "Test_Intl")
	fmt.Println(err)
	if err == nil {
		t.Fatalf("LoadAndCall V8Safe Test_Intl should return error")
	}
}

func TestEngine_JSON(t *testing.T) {
	Convey("test stringify1", t, func() {
		host, code := MyInit(t, "json", int64(1e8))
		_, cost, err := vmPool.LoadAndCall(host, code, "stringify10")
		So(err, ShouldBeNil)
		So(cost.ToGas(), ShouldEqual, int64(5019))

		_, cost, err = vmPool.LoadAndCall(host, code, "stringify11")
		So(err, ShouldBeNil)
		So(cost.ToGas(), ShouldEqual, int64(4362))
	})

	Convey("test stringify2", t, func() {
		host, code := MyInit(t, "json", int64(1e8))
		_, cost, err := vmPool.LoadAndCall(host, code, "stringify20")
		So(err, ShouldBeNil)
		So(cost.ToGas(), ShouldEqual, int64(605657))

		_, cost, err = vmPool.LoadAndCall(host, code, "stringify21")
		So(err, ShouldBeNil)
		So(cost.ToGas(), ShouldEqual, int64(628475))
	})

	Convey("test stringify3", t, func() {
		host, code := MyInit(t, "json", int64(1e8))
		_, cost, err := vmPool.LoadAndCall(host, code, "stringify30")
		So(err, ShouldBeNil)
		So(cost.ToGas(), ShouldEqual, int64(3149322))

		_, cost, err = vmPool.LoadAndCall(host, code, "stringify31")
		So(err, ShouldBeNil)
		So(cost.ToGas(), ShouldEqual, int64(6087268))
	})

	Convey("test stringify4", t, func() {
		host, code := MyInit(t, "json", int64(1e8))
		_, cost, err := vmPool.LoadAndCall(host, code, "stringify40")
		So(err.Error(), ShouldContainSubstring, "Converting circular structure to JSON")
		So(cost.ToGas(), ShouldEqual, int64(380))
	})

	Convey("test stringify5", t, func() {
		host, code := MyInit(t, "json", int64(1e8))
		rtn, cost, err := vmPool.LoadAndCall(host, code, "stringify50")
		So(err, ShouldBeNil)
		So(cost.ToGas(), ShouldEqual, int64(999))
		So(len(rtn), ShouldEqual, int64(1))
		So(rtn[0], ShouldEqual, `{"week":45,"month":7}`)

		rtn, cost, err = vmPool.LoadAndCall(host, code, "stringify51")
		So(err, ShouldBeNil)
		So(cost.ToGas(), ShouldEqual, int64(586))
		So(len(rtn), ShouldEqual, int64(1))
		So(rtn[0], ShouldEqual, `{"week":45,"month":7}`)
	})

	Convey("test stringify6", t, func() {
		host, code := MyInit(t, "json", int64(1e8))
		rtn, cost, err := vmPool.LoadAndCall(host, code, "stringify60")
		So(err, ShouldBeNil)
		So(cost.ToGas(), ShouldEqual, int64(1335))
		So(len(rtn), ShouldEqual, int64(1))
		So(rtn[0], ShouldEqual, `{"a":{"b":{"c":""}}} {"a":{"b":{"c":""}}}`)
	})
}

func TestEngine_Libstring(t *testing.T) {
	host, code := MyInit(t, "libstring")
	_, cost, err := vmPool.LoadAndCall(host, code, "ops")

	if err != nil {
		t.Fatalf("LoadAndCall ops error: %v\n", err)
	}
	if cost.ToGas() != 1082 {
		t.Errorf("cost except 1082, got %d\n", cost.ToGas())
	}
}

func TestEngine_Libbignumber(t *testing.T) {
	host, code := MyInit(t, "libbignumber")
	_, cost, err := vmPool.LoadAndCall(host, code, "ops")

	if err != nil {
		t.Fatalf("LoadAndCall ops error: %v\n", err)
	}
	if cost.ToGas() != 15976 {
		t.Errorf("cost except 15976, got %d\n", cost.ToGas())
	}
}
