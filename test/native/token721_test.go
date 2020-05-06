package native

import (
	"strconv"
	"testing"
	"time"

	"fmt"

	"github.com/iost-official/go-iost/core/contract"
	"github.com/iost-official/go-iost/core/tx"
	"github.com/iost-official/go-iost/core/version"
	"github.com/iost-official/go-iost/vm/database"
	"github.com/iost-official/go-iost/vm/host"
	"github.com/iost-official/go-iost/vm/native"
	. "github.com/smartystreets/goconvey/convey"
)

var test721DataPath = "./test_data/"

func initVM(t *testing.T, conName string, optional ...interface{}) (*native.Impl, *host.Host, *contract.Contract) {
	db := database.NewDatabaseFromPath(test721DataPath + conName + ".json")
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
		Info: &contract.Info{Version: "1.0.0"},
	}

	e := &native.Impl{}
	e.Init()

	return e, h, code
}

func TestToken721_Create(t *testing.T) {
	issuer0 := "issuer0"
	e, host, code := initVM(t, "token721.iost")
	code.ID = "token721.iost"
	host.Context().Set("contract_name", "token721.iost")
	host.SetDeadline(time.Now().Add(10 * time.Second))
	authList := host.Context().Value("auth_list").(map[string]int)
	signerList := host.Context().Value("signer_list").(map[string]bool)

	Convey("Test of Token create", t, func() {
		Reset(func() {
			e, host, code = initVM(t, "token721.iost")
			code.ID = "token721.iost"
			host.Context().Set("contract_name", "token721.iost")
			host.SetDeadline(time.Now().Add(10 * time.Second))
			authList = host.Context().Value("auth_list").(map[string]int)
		})

		Convey("token not exists", func() {
			authList[issuer0] = 1
			host.Context().Set("auth_list", authList)
			signerList[issuer0+"@active"] = true
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "issue", global.Token, "user0", "{}")
			So(err.Error(), ShouldEqual, "token not exists")
			_, _, err = e.LoadAndCall(host, code, "transfer", global.Token, "issuer0", "user0", "0")
			So(err.Error(), ShouldEqual, "token not exists")

			_, _, err = e.LoadAndCall(host, code, "balanceOf", global.Token, "issuer0")
			So(err.Error(), ShouldEqual, "token not exists")
		})

		Convey("create token", func() {
			authList[issuer0] = 1
			host.Context().Set("auth_list", authList)
			signerList[issuer0+"@active"] = true
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "create", global.Token, "issuer0", int64(100))
			So(err, ShouldBeNil)
		})

		Convey("create duplicate token", func() {
			authList[issuer0] = 1
			host.Context().Set("auth_list", authList)
			signerList[issuer0+"@active"] = true
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "create", global.Token, "issuer0", int64(100))
			So(err, ShouldBeNil)

			_, _, err = e.LoadAndCall(host, code, "create", global.Token, "issuer0", int64(100))
			So(err.Error(), ShouldEqual, "token exists")
		})

	})
}

func TestToken721_Issue(t *testing.T) {
	issuer0 := "issuer0"
	e, host, code := initVM(t, "token721.iost")
	code.ID = "token721.iost"
	host.Context().Set("contract_name", "token721.iost")
	host.SetDeadline(time.Now().Add(10 * time.Second))
	authList := host.Context().Value("auth_list").(map[string]int)
	signerList := host.Context().Value("signer_list").(map[string]bool)

	Convey("Test of Token issue", t, func() {

		Reset(func() {
			e, host, code = initVM(t, "token721.iost")
			code.ID = "token721.iost"
			host.SetDeadline(time.Now().Add(10 * time.Second))
			authList = host.Context().Value("auth_list").(map[string]int)
		})

		Convey("correct issue", func() {
			authList[issuer0] = 1
			host.Context().Set("auth_list", authList)
			signerList[issuer0+"@active"] = true
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "create", global.Token, "issuer0", int64(100))
			So(err, ShouldBeNil)
			_, cost, err := e.LoadAndCall(host, code, "issue", global.Token, "issuer0", "{}")
			So(err, ShouldBeNil)
			So(cost.ToGas(), ShouldBeGreaterThan, 0)

			rs, _, err := e.LoadAndCall(host, code, "balanceOf", global.Token, "issuer0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == int64(1))
		})

		Convey("issue token without auth", func() {
			_, _, err := e.LoadAndCall(host, code, "create", global.Token, "user0", int64(100))
			So(err.Error(), ShouldEqual, "transaction has no permission")

			delete(authList, issuer0)
			host.Context().Set("auth_list", authList)
			delete(signerList, issuer0+"@active")
			host.Context().Set("signer_list", signerList)
			_, _, err = e.LoadAndCall(host, code, "create", global.Token, "issuer0", int64(100))
			So(err.Error(), ShouldEqual, "transaction has no permission")
		})

		Convey("issue too much", func() {
			authList[issuer0] = 1
			host.Context().Set("auth_list", authList)
			signerList[issuer0+"@active"] = true
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "create", global.Token, "issuer0", int64(1))
			So(err, ShouldBeNil)
			_, _, err = e.LoadAndCall(host, code, "issue", global.Token, "issuer0", "{}")
			So(err, ShouldBeNil)

			_, _, err = e.LoadAndCall(host, code, "issue", global.Token, "issuer0", "{}")
			So(true, ShouldEqual, err.Error() == "supply too much")

			rs, _, err := e.LoadAndCall(host, code, "balanceOf", global.Token, "issuer0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == int64(1))
		})

	})
}

func TestToken721_Transfer(t *testing.T) {
	issuer0 := "issuer0"
	e, host, code := initVM(t, "token721.iost")
	code.ID = "token721.iost"
	host.Context().Set("contract_name", "token721.iost")
	host.SetDeadline(time.Now().Add(10 * time.Second))
	authList := host.Context().Value("auth_list").(map[string]int)
	signerList := host.Context().Value("signer_list").(map[string]bool)

	Convey("Test of Token transfer", t, func() {
		Reset(func() {
			e, host, code = initVM(t, "token721.iost")
			code.ID = "token721.iost"
			host.SetDeadline(time.Now().Add(10 * time.Second))
			authList = host.Context().Value("auth_list").(map[string]int)
		})

		Convey("correct transfer", func() {
			authList[issuer0] = 1
			host.Context().Set("auth_list", authList)
			signerList[issuer0+"@active"] = true
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "create", global.Token, "issuer0", int64(100))
			So(err, ShouldBeNil)

			for i := 0; i < 10; i++ {
				rs, _, err := e.LoadAndCall(host, code, "issue", global.Token, "issuer0", `{"hp": 100}`)
				So(err, ShouldBeNil)
				So(rs[0].(string), ShouldEqual, fmt.Sprintf(`%v`, i))
			}
			_, _, err = e.LoadAndCall(host, code, "transfer", global.Token, "issuer0", "user0", "3")
			So(err, ShouldBeNil)

			So(host.DB().Token721Balance(global.Token, "user0"), ShouldEqual, 1)
			So(fmt.Sprintf("%v", host.DB().Token721IDList(global.Token, "user0")), ShouldEqual, fmt.Sprintf("%v", []string{"3"}))
			rs, err := host.DB().Token721Owner(global.Token, "3")
			So(err, ShouldBeNil)
			So(rs, ShouldEqual, "user0")
			rs, err = host.DB().Token721Metadata(global.Token, "3")
			So(err, ShouldBeNil)
			So(rs, ShouldEqual, "{\"hp\": 100}")

			tokenID, _, err := e.LoadAndCall(host, code, "tokenOfOwnerByIndex", global.Token, "user0", int64(0))
			So(err, ShouldBeNil)
			So(tokenID[0], ShouldEqual, "3")

			tokenID, _, err = e.LoadAndCall(host, code, "tokenOfOwnerByIndex", global.Token, "issuer0", int64(1))
			So(tokenID[0], ShouldEqual, "1")
			So(err, ShouldBeNil)
		})

		Convey("transfer token without auth", func() {
			authList[issuer0] = 1
			host.Context().Set("auth_list", authList)
			signerList[issuer0+"@active"] = true
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "create", global.Token, "issuer0", int64(100))
			So(err, ShouldBeNil)
			delete(authList, issuer0)
			host.Context().Set("auth_list", authList)
			delete(signerList, issuer0+"@active")
			host.Context().Set("signer_list", signerList)
			_, cost, err := e.LoadAndCall(host, code, "transfer", global.Token, "issuer0", "user0", "3")
			So(true, ShouldEqual, err.Error() == "transaction has no permission")
			So(cost.ToGas(), ShouldBeGreaterThan, 0)
		})

		Convey("transfer with wrong token id", func() {
			authList[issuer0] = 1
			authList["user0"] = 1
			host.Context().Set("auth_list", authList)
			signerList[issuer0+"@active"] = true
			signerList["user0"+"@active"] = true
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "create", global.Token, "issuer0", int64(100))
			So(err, ShouldBeNil)

			for i := 0; i < 10; i++ {
				e.LoadAndCall(host, code, "issue", global.Token, "issuer0", "{}")
			}

			_, cost, err := e.LoadAndCall(host, code, "transfer", global.Token, "user0", "issuer0", "1")
			So(err.Error(), ShouldContainSubstring, "error token owner isn't from")

			_, cost, err = e.LoadAndCall(host, code, "transfer", global.Token, "issuer0", "user0", "10")
			So(err.Error(), ShouldContainSubstring, "error tokenID not exists")
			So(cost.ToGas(), ShouldBeGreaterThan, 0)

			rs, cost, err := e.LoadAndCall(host, code, "balanceOf", global.Token, "issuer0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == int64(10))

			rs, cost, err = e.LoadAndCall(host, code, "balanceOf", global.Token, "user0")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == int64(0))

			rs, cost, err = e.LoadAndCall(host, code, "balanceOf", global.Token, "user1")
			So(err, ShouldBeNil)
			So(true, ShouldEqual, len(rs) > 0 && rs[0] == int64(0))
		})
	})
}

func TestToken721_Metadata(t *testing.T) {
	issuer0 := "issuer0"
	e, host, code := initVM(t, "token721.iost")
	code.ID = "token721.iost"
	host.SetDeadline(time.Now().Add(10 * time.Second))
	authList := host.Context().Value("auth_list").(map[string]int)
	signerList := host.Context().Value("signer_list").(map[string]bool)

	Convey("Test of Token transfer", t, func() {
		Reset(func() {
			e, host, code = initVM(t, "token721.iost")
			code.ID = "token721.iost"
			host.SetDeadline(time.Now().Add(10 * time.Second))
			authList = host.Context().Value("auth_list").(map[string]int)
		})

		Convey("correct metadata", func() {
			authList[issuer0] = 1
			host.Context().Set("auth_list", authList)
			signerList[issuer0+"@active"] = true
			host.Context().Set("signer_list", signerList)
			_, _, err := e.LoadAndCall(host, code, "create", global.Token, "issuer0", int64(100))
			So(err, ShouldBeNil)

			for i := 0; i < 10; i++ {
				e.LoadAndCall(host, code, "issue", global.Token, "issuer0", "{\"id\":"+strconv.FormatInt(int64(i), 10)+"}")
			}

			md, _, err := e.LoadAndCall(host, code, "tokenMetadata", global.Token, "3")
			So(err, ShouldBeNil)
			So(md[0], ShouldEqual, "{\"id\":3}")
			_, _, err = e.LoadAndCall(host, code, "transfer", global.Token, "issuer0", "user0", "3")
			So(err, ShouldBeNil)

			md, _, err = e.LoadAndCall(host, code, "tokenMetadata", global.Token, "3")
			So(err, ShouldBeNil)
			So(md[0], ShouldEqual, "{\"id\":3}")
		})

	})
}
