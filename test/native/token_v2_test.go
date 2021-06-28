package native

import (
	"strings"
	"testing"
	"time"

	"github.com/iost-official/go-iost/v3/core/contract"
	"github.com/iost-official/go-iost/v3/core/tx"
	"github.com/iost-official/go-iost/v3/core/version"
	"github.com/iost-official/go-iost/v3/vm/database"
	"github.com/iost-official/go-iost/v3/vm/host"
	"github.com/iost-official/go-iost/v3/vm/native"
	. "github.com/smartystreets/goconvey/convey"
)

func InitVMV2(t *testing.T, conName string, optional ...interface{}) (*native.Impl, *host.Host, *contract.Contract) {
	db := database.NewDatabaseFromPath(testDataPath + conName + ".json")
	vi := database.NewVisitor(100, db, version.NewRules(0))
	vi.MPut("auth.iost-auth", "issuer0", database.MustMarshal(`{"id":"issuer0","permissions":{"active":{"name":"active","groups":[],"items":[{"id":"issuer0","is_key_pair":true,"weight":1}],"threshold":1},"owner":{"name":"owner","groups":[],"items":[{"id":"issuer0","is_key_pair":true,"weight":1}],"threshold":1}}}`))
	vi.MPut("auth.iost-auth", "user0", database.MustMarshal(`{"id":"user0","permissions":{"active":{"name":"active","groups":[],"items":[{"id":"user0","is_key_pair":true,"weight":1}],"threshold":1},"owner":{"name":"owner","groups":[],"items":[{"id":"user0","is_key_pair":true,"weight":1}],"threshold":1}}}`))
	vi.MPut("auth.iost-auth", "user1", database.MustMarshal(`{"id":"user1","permissions":{"active":{"name":"active","groups":[],"items":[{"id":"user1","is_key_pair":true,"weight":1}],"threshold":1},"owner":{"name":"owner","groups":[],"items":[{"id":"user1","is_key_pair":true,"weight":1}],"threshold":1}}}`))

	ctx := host.NewContext(nil)
	ctx.Set("gas_ratio", int64(100))
	var gasLimit = int64(1000000)
	if len(optional) > 0 {
		gasLimit = optional[0].(int64)
	}
	ctx.GSet("gas_limit", gasLimit)
	ctx.Set("contract_name", conName)
	ctx.Set("tx_hash", []byte("iamhash"))
	ctx.Set("auth_list", make(map[string]int))
	ctx.Set("signer_list", make(map[string]bool))
	ctx.Set("time", int64(0))
	ctx.Set("abi_name", "abi")
	ctx.GSet("receipts", []*tx.Receipt{})
	ctx.Set("publisher", "")

	// pm := NewMonitor()
	h := host.NewHost(ctx, vi, version.NewRules(0), nil, nil)
	h.Context().Set("stack_height", 0)

	code := &contract.Contract{
		ID:   "system.iost",
		Info: &contract.Info{Version: "1.0.2"},
	}

	e := &native.Impl{}
	e.Init()

	return e, h, code
}

func TestTokenV2_Create(t *testing.T) {
	issuer0 := "issuer0"
	e, host, code := InitVMV2(t, "token")
	code.ID = "token.iost"
	host.Context().Set("contract_name", "token.iost")
	host.SetDeadline(time.Now().Add(10 * time.Second))
	authList := host.Context().Value("auth_list").(map[string]int)
	signerList := host.Context().Value("signer_list").(map[string]bool)

	Convey("Test of Token create", t, func() {
		Reset(func() {
			e, host, code = InitVMV2(t, "token")
			code.ID = "token.iost"
			host.Context().Set("contract_name", "token.iost")
			host.SetDeadline(time.Now().Add(10 * time.Second))
			authList = host.Context().Value("auth_list").(map[string]int)
		})

		Convey("token not exists", func() {
			authList[issuer0] = 1
			host.Context().Set("auth_list", authList)
			signerList[issuer0+"@active"] = true
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "issue", "iost", "user0", "100")
			So(err.Error(), ShouldEqual, "token not exists")

			_, _, err = e.LoadAndCall(host, code, "transfer", "iost", "issuer0", "user0", "100", "")
			So(err.Error(), ShouldEqual, "token not exists")

			_, _, err = e.LoadAndCall(host, code, "balanceOf", "iost", "issuer0")
			So(err.Error(), ShouldEqual, "token not exists")

			_, _, err = e.LoadAndCall(host, code, "supply", "iost")
			So(err.Error(), ShouldEqual, "token not exists")

			_, _, err = e.LoadAndCall(host, code, "totalSupply", "iost")
			So(err.Error(), ShouldEqual, "token not exists")
		})

		Convey("create token without auth", func() {
			_, _, err := e.LoadAndCall(host, code, "create", "iost", "issuer0", int64(100), []byte("{}"))
			So(err.Error(), ShouldEqual, "transaction has no permission")
		})

		Convey("create token", func() {
			authList[issuer0] = 1
			host.Context().Set("auth_list", authList)
			signerList[issuer0+"@active"] = true
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "create", "iost", "issuer0", int64(100), []byte("{}"))
			So(err, ShouldBeNil)
		})

		Convey("create duplicate token", func() {
			authList[issuer0] = 1
			host.Context().Set("auth_list", authList)
			signerList[issuer0+"@active"] = true
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "create", "iost", "issuer0", int64(100), []byte("{}"))
			So(err, ShouldBeNil)

			_, _, err = e.LoadAndCall(host, code, "create", "iost", "issuer0", int64(100), []byte("{}"))
			So(err.Error(), ShouldEqual, "token exists")
		})

		Convey("create invalid token symbol", func() {
			authList[issuer0] = 1
			host.Context().Set("auth_list", authList)
			signerList[issuer0+"@active"] = true
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "create", "iostaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "issuer0", int64(100), []byte("{}"))
			So(err.Error(), ShouldContainSubstring, "token symbol invalid.")

			_, _, err = e.LoadAndCall(host, code, "create", "IOST", "issuer0", int64(100), []byte("{}"))
			So(err.Error(), ShouldContainSubstring, "token symbol invalid.")
		})

		Convey("create token config", func() {
			authList[issuer0] = 1
			host.Context().Set("auth_list", authList)
			signerList[issuer0+"@active"] = true
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "create", "iost", "issuer0", int64(100), []byte(`{"canTransfer": false, "decimal": 1, "defaultRate": "1.1"}`))
			So(err, ShouldBeNil)

			_, _, err = e.LoadAndCall(host, code, "issue", "iost", "issuer0", "10.222")
			So(err, ShouldBeNil)

			rs, _, err := e.LoadAndCall(host, code, "balanceOf", "iost", "issuer0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "10.2")

			_, _, err = e.LoadAndCall(host, code, "transfer", "iost", "issuer0", "user0", "22.3", "")
			So(err.Error(), ShouldEqual, "token can't transfer")

			dr, _ := host.MapGet("TIiost", "defaultRate")
			So(dr.(string), ShouldEqual, "1.1")

			// transfer truncate
			_, _, err = e.LoadAndCall(host, code, "create", "iost1", "issuer0", int64(100), []byte(`{"decimal": 1}`))
			So(err, ShouldBeNil)
			_, _, err = e.LoadAndCall(host, code, "issue", "iost1", "issuer0", "100")
			So(err, ShouldBeNil)
			_, _, err = e.LoadAndCall(host, code, "transfer", "iost1", "issuer0", "user0", "22.33", "")
			So(err, ShouldBeNil)
			rs, _, err = e.LoadAndCall(host, code, "balanceOf", "iost1", "issuer0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "77.7")
			rs, _, err = e.LoadAndCall(host, code, "balanceOf", "iost1", "user0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "22.3")
		})
	})
}

func TestTokenV2_Issue(t *testing.T) {
	issuer0 := "issuer0"
	e, host, code := InitVMV2(t, "token")
	code.ID = "token.iost"
	host.Context().Set("contract_name", "token.iost")
	host.SetDeadline(time.Now().Add(10 * time.Second))
	authList := host.Context().Value("auth_list").(map[string]int)
	signerList := host.Context().Value("signer_list").(map[string]bool)

	Convey("Test of Token issue", t, func() {

		Reset(func() {
			e, host, code = InitVMV2(t, "token")
			code.ID = "token.iost"
			host.Context().Set("contract_name", "token.iost")
			host.SetDeadline(time.Now().Add(10 * time.Second))
			authList = host.Context().Value("auth_list").(map[string]int)

			authList[issuer0] = 1
			host.Context().Set("auth_list", authList)
			signerList[issuer0+"@active"] = true
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "create", "iost", "issuer0", int64(100), []byte("{}"))
			So(err, ShouldBeNil)
		})

		Convey("issue prepare", func() {
			authList[issuer0] = 1
			host.Context().Set("auth_list", authList)
			signerList[issuer0+"@active"] = true
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "create", "iost", "issuer0", int64(100), []byte("{}"))
			So(err, ShouldBeNil)
		})

		Convey("correct issue", func() {
			_, cost, err := e.LoadAndCall(host, code, "issue", "iost", "user0", "1.1")
			So(err, ShouldBeNil)
			So(cost.ToGas(), ShouldBeGreaterThan, 0)

			rs, cost, err := e.LoadAndCall(host, code, "balanceOf", "iost", "user0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "1.1")

			rs, cost, err = e.LoadAndCall(host, code, "supply", "iost")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "1.1")

			rs, cost, err = e.LoadAndCall(host, code, "totalSupply", "iost")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "100")
		})

		Convey("issue token without auth", func() {
			delete(authList, issuer0)
			host.Context().Set("auth_list", authList)
			delete(signerList, issuer0+"@active")
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "issue", "iost", "user0", "1.1")
			So(true, ShouldEqual, err.Error() == "transaction has no permission")
		})

		Convey("issue too much", func() {
			_, cost, err := e.LoadAndCall(host, code, "issue", "iost", "user0", "1.1")
			So(err, ShouldBeNil)
			So(cost.ToGas(), ShouldBeGreaterThan, 0)

			_, cost, err = e.LoadAndCall(host, code, "issue", "iost", "user0", "100")
			So(true, ShouldEqual, err.Error() == "supply too much")

			rs, cost, err := e.LoadAndCall(host, code, "balanceOf", "iost", "user0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "1.1")

			rs, cost, err = e.LoadAndCall(host, code, "supply", "iost")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "1.1")
		})

		Convey("issue invalid amount", func() {
			_, cost, err := e.LoadAndCall(host, code, "issue", "iost", "issuer0", "-1.1")
			So(true, ShouldEqual, strings.Contains(err.Error(), "invalid amount"))
			So(cost.ToGas(), ShouldBeGreaterThan, 0)

			_, _, err = e.LoadAndCall(host, code, "issue", "iost", "issuer0", "1.1")
			So(err, ShouldBeNil)

			_, _, err = e.LoadAndCall(host, code, "issue", "iost", "issuer0", "")
			So(err.Error(), ShouldContainSubstring, "invalid")

			_, _, err = e.LoadAndCall(host, code, "issue", "iost", "issuer0", "1abc")
			So(err.Error(), ShouldContainSubstring, "invalid")

			_, _, err = e.LoadAndCall(host, code, "issue", "iost", "issuer0", "11111111111111111111111111111111")
			So(err.Error(), ShouldContainSubstring, "invalid")

			_, _, err = e.LoadAndCall(host, code, "issue", "iost", "issuer0", "1.012345600000007e1")
			So(err.Error(), ShouldContainSubstring, "invalid")

			_, _, err = e.LoadAndCall(host, code, "issue", "iost", "issuer0", ".012345600000007e1")
			So(err.Error(), ShouldContainSubstring, "invalid")

			_, _, err = e.LoadAndCall(host, code, "issue", "iost", "issuer0", "1.")
			So(err.Error(), ShouldContainSubstring, "invalid")
		})

	})
}

func TestTokenV2_Transfer(t *testing.T) {
	issuer0 := "issuer0"
	e, host, code := InitVMV2(t, "token")
	code.ID = "token.iost"
	host.Context().Set("contract_name", "token.iost")
	host.SetDeadline(time.Now().Add(10 * time.Second))
	authList := host.Context().Value("auth_list").(map[string]int)
	signerList := host.Context().Value("signer_list").(map[string]bool)

	Convey("Test of Token transfer", t, func() {

		Reset(func() {
			e, host, code = InitVMV2(t, "token")
			code.ID = "token.iost"
			host.Context().Set("contract_name", "token.iost")
			host.SetDeadline(time.Now().Add(10 * time.Second))
			authList = host.Context().Value("auth_list").(map[string]int)

			authList[issuer0] = 1
			host.Context().Set("auth_list", authList)
			signerList[issuer0+"@active"] = true
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "create", "iost", "issuer0", int64(100), []byte("{}"))
			So(err, ShouldBeNil)

			_, _, err = e.LoadAndCall(host, code, "issue", "iost", "issuer0", "100.0")
			So(err, ShouldBeNil)
		})

		Convey("transfer prepare", func() {
			authList[issuer0] = 1
			host.Context().Set("auth_list", authList)
			signerList[issuer0+"@active"] = true
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "create", "iost", "issuer0", int64(100), []byte("{}"))
			So(err, ShouldBeNil)

			_, _, err = e.LoadAndCall(host, code, "issue", "iost", "issuer0", "100.0")
			So(err, ShouldBeNil)
		})

		Convey("correct transfer", func() {
			_, cost, err := e.LoadAndCall(host, code, "transfer", "iost", "issuer0", "user0", "22.3", "")
			So(err, ShouldBeNil)
			So(cost.ToGas(), ShouldBeGreaterThan, 0)

			rs, cost, err := e.LoadAndCall(host, code, "balanceOf", "iost", "user0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "22.3")

			rs, cost, err = e.LoadAndCall(host, code, "balanceOf", "iost", "issuer0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "77.7")

			authList["user0"] = 1
			signerList["user0"+"@active"] = true
			_, cost, err = e.LoadAndCall(host, code, "transfer", "iost", "user0", "user1", "22.3", "")
			So(err, ShouldBeNil)

			rs, cost, err = e.LoadAndCall(host, code, "balanceOf", "iost", "user0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "0")

			rs, cost, err = e.LoadAndCall(host, code, "balanceOf", "iost", "user1")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "22.3")

			// transfer to self
			authList["user1"] = 1
			signerList["user1"+"@active"] = true
			_, cost, err = e.LoadAndCall(host, code, "transfer", "iost", "user1", "user1", "22.3", "")
			So(err, ShouldBeNil)

			rs, cost, err = e.LoadAndCall(host, code, "balanceOf", "iost", "user1")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "22.3")
		})

		Convey("transfer token without auth", func() {
			delete(authList, issuer0)
			host.Context().Set("auth_list", authList)
			delete(signerList, issuer0+"@active")
			host.Context().Set("signer_list", signerList)
			_, cost, err := e.LoadAndCall(host, code, "transfer", "iost", "issuer0", "user0", "1.1", "")
			So(true, ShouldEqual, err.Error() == "transaction has no permission")
			So(cost.ToGas(), ShouldBeGreaterThan, 0)
		})

		Convey("transfer too much", func() {
			_, cost, err := e.LoadAndCall(host, code, "transfer", "iost", "issuer0", "user0", "100.1", "")
			So(true, ShouldEqual, strings.HasPrefix(err.Error(), "balance not enough"))
			So(cost.ToGas(), ShouldBeGreaterThan, 0)

			rs, cost, err := e.LoadAndCall(host, code, "balanceOf", "iost", "issuer0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "100")

			rs, cost, err = e.LoadAndCall(host, code, "balanceOf", "iost", "user0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "0")

			rs, cost, err = e.LoadAndCall(host, code, "balanceOf", "iost", "user1")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "0")
		})

		Convey("transfer invalid amount", func() {
			_, cost, err := e.LoadAndCall(host, code, "transfer", "iost", "issuer0", "user0", "-1.1", "")
			So(true, ShouldEqual, strings.Contains(err.Error(), "invalid amount"))
			So(cost.ToGas(), ShouldBeGreaterThan, 0)

			_, cost, err = e.LoadAndCall(host, code, "transfer", "iost", "issuer0", "user0", "1.1", "")
			So(err, ShouldBeNil)

			_, cost, err = e.LoadAndCall(host, code, "transfer", "iost", "issuer0", "user0", "", "")
			So(err.Error(), ShouldContainSubstring, "invalid")

			_, cost, err = e.LoadAndCall(host, code, "transfer", "iost", "issuer0", "user0", "1abc", "")
			So(err.Error(), ShouldContainSubstring, "invalid")

			_, cost, err = e.LoadAndCall(host, code, "transfer", "iost", "issuer0", "user0", "11111111111111111111111111111111", "")
			So(err.Error(), ShouldContainSubstring, "invalid")

			_, _, err = e.LoadAndCall(host, code, "transfer", "iost", "issuer0", "user0", "1.012345600000007e1", "")
			So(err.Error(), ShouldContainSubstring, "invalid")

			_, _, err = e.LoadAndCall(host, code, "transfer", "iost", "issuer0", "user0", ".012345600000007e1", "")
			So(err.Error(), ShouldContainSubstring, "invalid")

			_, _, err = e.LoadAndCall(host, code, "transfer", "iost", "issuer0", "user0", "1.", "")
			So(err.Error(), ShouldContainSubstring, "invalid")
		})
	})
}

func TestTokenV2_Destroy(t *testing.T) {
	issuer0 := "issuer0"
	e, host, code := InitVMV2(t, "token")
	code.ID = "token.iost"
	host.Context().Set("contract_name", "token.iost")
	host.SetDeadline(time.Now().Add(10 * time.Second))
	authList := host.Context().Value("auth_list").(map[string]int)
	signerList := host.Context().Value("signer_list").(map[string]bool)

	Convey("Test of Token destroy", t, func() {

		Reset(func() {
			e, host, code = InitVMV2(t, "token")
			code.ID = "token.iost"
			host.Context().Set("contract_name", "token.iost")
			host.SetDeadline(time.Now().Add(10 * time.Second))
			authList = host.Context().Value("auth_list").(map[string]int)

			authList[issuer0] = 1
			host.Context().Set("auth_list", authList)
			signerList[issuer0+"@active"] = true
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "create", "iost", "issuer0", int64(100), []byte("{}"))
			So(err, ShouldBeNil)

			_, _, err = e.LoadAndCall(host, code, "issue", "iost", "issuer0", "100.0")
			So(err, ShouldBeNil)
		})

		Convey("destroy prepare", func() {
			authList[issuer0] = 1
			host.Context().Set("auth_list", authList)
			signerList[issuer0+"@active"] = true
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "create", "iost", "issuer0", int64(100), []byte("{}"))
			So(err, ShouldBeNil)

			_, _, err = e.LoadAndCall(host, code, "issue", "iost", "issuer0", "100.0")
			So(err, ShouldBeNil)
		})

		Convey("correct destroy", func() {
			rs, cost, err := e.LoadAndCall(host, code, "balanceOf", "iost", "issuer0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "100")
			So(cost.ToGas(), ShouldBeGreaterThan, 0)

			_, cost, err = e.LoadAndCall(host, code, "destroy", "iost", "issuer0", "22.3")
			So(err, ShouldBeNil)

			rs, cost, err = e.LoadAndCall(host, code, "balanceOf", "iost", "issuer0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "77.7")

			rs, cost, err = e.LoadAndCall(host, code, "supply", "iost")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "77.7")

			rs, cost, err = e.LoadAndCall(host, code, "totalSupply", "iost")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "100")

		})

		Convey("issuer after destroy", func() {
			_, cost, err := e.LoadAndCall(host, code, "destroy", "iost", "issuer0", "22.3")
			So(err, ShouldBeNil)

			rs, cost, err := e.LoadAndCall(host, code, "balanceOf", "iost", "issuer0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "77.7")

			_, cost, err = e.LoadAndCall(host, code, "issue", "iost", "user0", "11")
			So(err, ShouldBeNil)

			rs, cost, err = e.LoadAndCall(host, code, "balanceOf", "iost", "user0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "11")

			rs, cost, err = e.LoadAndCall(host, code, "supply", "iost")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "88.7")
			So(cost.ToGas(), ShouldBeGreaterThan, 0)

			_, cost, err = e.LoadAndCall(host, code, "issue", "iost", "user0", "21")
			So(true, ShouldEqual, err.Error() == "supply too much")
		})

		Convey("destroy token without auth", func() {
			delete(authList, issuer0)
			host.Context().Set("auth_list", authList)
			delete(signerList, issuer0+"@active")
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "destroy", "iost", "issuer0", "1.1")
			So(true, ShouldEqual, err.Error() == "transaction has no permission")
		})

		Convey("destroy too much", func() {
			_, cost, err := e.LoadAndCall(host, code, "destroy", "iost", "issuer0", "100.1")
			So(true, ShouldEqual, strings.HasPrefix(err.Error(), "balance not enough"))

			rs, cost, err := e.LoadAndCall(host, code, "balanceOf", "iost", "issuer0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "100")

			rs, cost, err = e.LoadAndCall(host, code, "supply", "iost")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "100")
			So(cost.ToGas(), ShouldBeGreaterThan, 0)
		})

		Convey("destroy invalid amount", func() {
			_, cost, err := e.LoadAndCall(host, code, "destroy", "iost", "issuer0", "-1.1")
			So(true, ShouldEqual, strings.Contains(err.Error(), "invalid amount"))
			So(cost.ToGas(), ShouldBeGreaterThan, 0)

			_, cost, err = e.LoadAndCall(host, code, "destroy", "iost", "issuer0", "1.1")
			So(err, ShouldBeNil)

			_, cost, err = e.LoadAndCall(host, code, "destroy", "iost", "issuer0", "")
			So(err.Error(), ShouldContainSubstring, "invalid")

			_, cost, err = e.LoadAndCall(host, code, "destroy", "iost", "issuer0", "1abc")
			So(err.Error(), ShouldContainSubstring, "invalid")

			_, cost, err = e.LoadAndCall(host, code, "destroy", "iost", "issuer0", "11111111111111111111111111111111")
			So(err.Error(), ShouldContainSubstring, "invalid")

			_, _, err = e.LoadAndCall(host, code, "destroy", "iost", "issuer0", "1.012345600000007e1")
			So(err.Error(), ShouldContainSubstring, "invalid")

			_, _, err = e.LoadAndCall(host, code, "destroy", "iost", "issuer0", ".012345600000007e1")
			So(err.Error(), ShouldContainSubstring, "invalid")

			_, _, err = e.LoadAndCall(host, code, "destroy", "iost", "issuer0", "1.")
			So(err.Error(), ShouldContainSubstring, "invalid")
		})
	})
}

func TestTokenV2_TransferFreeze(t *testing.T) {
	issuer0 := "issuer0"
	e, host, code := InitVMV2(t, "token")
	code.ID = "token.iost"
	host.Context().Set("contract_name", "token.iost")
	host.SetDeadline(time.Now().Add(10 * time.Second))
	authList := host.Context().Value("auth_list").(map[string]int)
	signerList := host.Context().Value("signer_list").(map[string]bool)
	now := int64(time.Now().Unix()) * 1e9

	Convey("Test of Token transferFreeze", t, func() {

		Reset(func() {
			e, host, code = InitVMV2(t, "token")
			code.ID = "token.iost"
			host.Context().Set("contract_name", "token.iost")
			host.SetDeadline(time.Now().Add(10 * time.Second))
			authList = host.Context().Value("auth_list").(map[string]int)

			authList[issuer0] = 1
			host.Context().Set("auth_list", authList)
			signerList[issuer0+"@active"] = true
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "create", "iost", "issuer0", int64(100), []byte("{}"))
			So(err, ShouldBeNil)

			_, _, err = e.LoadAndCall(host, code, "issue", "iost", "issuer0", "100.0")
			So(err, ShouldBeNil)
		})

		Convey("transferFreeze prepare", func() {
			authList[issuer0] = 1
			host.Context().Set("auth_list", authList)
			signerList[issuer0+"@active"] = true
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "create", "iost", "issuer0", int64(100), []byte("{}"))
			So(err, ShouldBeNil)

			_, _, err = e.LoadAndCall(host, code, "issue", "iost", "issuer0", "100.0")
			So(err, ShouldBeNil)
		})

		Convey("correct transferFreeze", func() {
			_, cost, err := e.LoadAndCall(host, code, "transferFreeze", "iost", "issuer0", "user0", "22.3", now, "")
			So(err, ShouldBeNil)
			So(cost.ToGas(), ShouldBeGreaterThan, 0)

			rs, cost, err := e.LoadAndCall(host, code, "balanceOf", "iost", "user0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "0")

			freezedBalance := host.DB().FreezedTokenBalanceFixed("iost", "user0")
			So(freezedBalance.String(), ShouldEqual, "22.3")

			rs, cost, err = e.LoadAndCall(host, code, "balanceOf", "iost", "issuer0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "77.7")

			rs, cost, err = e.LoadAndCall(host, code, "balanceOf", "iost", "user0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "0")

			host.Context().Set("time", now+1)

			rs, cost, err = e.LoadAndCall(host, code, "balanceOf", "iost", "user0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "22.3")

			freezedBalance = host.DB().FreezedTokenBalanceFixed("iost", "user0")
			So(freezedBalance.String(), ShouldEqual, "0")

			// transferFreeze to self
			authList["user0"] = 1
			signerList["user0"+"@active"] = true
			_, cost, err = e.LoadAndCall(host, code, "transferFreeze", "iost", "user0", "user0", "10", now+10, "")
			So(err, ShouldBeNil)
			So(cost.ToGas(), ShouldBeGreaterThan, 0)

			rs, cost, err = e.LoadAndCall(host, code, "balanceOf", "iost", "user0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "12.3")

			_, cost, err = e.LoadAndCall(host, code, "transferFreeze", "iost", "user0", "user0", "1", now+20, "")
			So(err, ShouldBeNil)

			rs, cost, err = e.LoadAndCall(host, code, "balanceOf", "iost", "user0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "11.3")

			freezedBalance = host.DB().FreezedTokenBalanceFixed("iost", "user0")
			So(freezedBalance.String(), ShouldEqual, "11")

			host.Context().Set("time", now+11)
			rs, cost, err = e.LoadAndCall(host, code, "balanceOf", "iost", "user0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "21.3")

			freezedBalance = host.DB().FreezedTokenBalanceFixed("iost", "user0")
			So(freezedBalance.String(), ShouldEqual, "1")

			host.Context().Set("time", now+21)
			rs, cost, err = e.LoadAndCall(host, code, "balanceOf", "iost", "user0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "22.3")

			freezedBalance = host.DB().FreezedTokenBalanceFixed("iost", "user0")
			So(freezedBalance.String(), ShouldEqual, "0")
		})

		Convey("transferFreeze token without auth", func() {
			delete(authList, issuer0)
			host.Context().Set("auth_list", authList)
			delete(signerList, issuer0+"@active")
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "transferFreeze", "iost", "issuer0", "user0", "1.1", now, "")
			So(true, ShouldEqual, err.Error() == "transaction has no permission")
		})

		Convey("transferFreeze too much", func() {
			_, _, err := e.LoadAndCall(host, code, "transferFreeze", "iost", "issuer0", "user0", "100.1", now-1, "")
			So(true, ShouldEqual, strings.HasPrefix(err.Error(), "balance not enough"))

			rs, _, err := e.LoadAndCall(host, code, "balanceOf", "iost", "issuer0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "100")

			rs, _, err = e.LoadAndCall(host, code, "balanceOf", "iost", "user0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == "0")

			_, _, err = e.LoadAndCall(host, code, "transferFreeze", "iost", "issuer0", "user0", "100", now+100, "")
			So(err, ShouldBeNil)

			authList["user0"] = 1
			signerList["user0"+"@active"] = true
			host.Context().Set("auth_list", authList)
			_, _, err = e.LoadAndCall(host, code, "transferFreeze", "iost", "user0", "user1", "10", now+100, "")
			So(true, ShouldEqual, strings.HasPrefix(err.Error(), "balance not enough"))

			_, _, err = e.LoadAndCall(host, code, "transfer", "iost", "user0", "user1", "10", "")
			So(true, ShouldEqual, strings.HasPrefix(err.Error(), "balance not enough"))
		})

		Convey("transferFreeze invalid amount", func() {
			_, _, err := e.LoadAndCall(host, code, "transferFreeze", "iost", "issuer0", "user0", "1.012345600000007e1", now, "")
			So(err.Error(), ShouldContainSubstring, "invalid")

			_, _, err = e.LoadAndCall(host, code, "transferFreeze", "iost", "issuer0", "user0", ".012345600000007e1", now, "")
			So(err.Error(), ShouldContainSubstring, "invalid")

			_, _, err = e.LoadAndCall(host, code, "transferFreeze", "iost", "issuer0", "user0", "1.", now, "")
			So(err.Error(), ShouldContainSubstring, "invalid")
		})

	})
}
