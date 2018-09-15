package vm

import (
	"fmt"
	"os"
	"testing"
	"time"

	"strconv"

	"github.com/iost-official/Go-IOS-Protocol/account"
	"github.com/iost-official/Go-IOS-Protocol/common"
	"github.com/iost-official/Go-IOS-Protocol/core/block"
	"github.com/iost-official/Go-IOS-Protocol/core/contract"
	"github.com/iost-official/Go-IOS-Protocol/core/tx"
	"github.com/iost-official/Go-IOS-Protocol/crypto"
	"github.com/iost-official/Go-IOS-Protocol/db"
	"github.com/iost-official/Go-IOS-Protocol/ilog"
	"github.com/iost-official/Go-IOS-Protocol/vm/database"
	"github.com/iost-official/Go-IOS-Protocol/vm/host"
	"github.com/iost-official/Go-IOS-Protocol/vm/native"
	. "github.com/smartystreets/goconvey/convey"
)

func watchTime(f func()) time.Duration {
	ta := time.Now()
	f()
	return time.Now().Sub(ta)
}

func TestJS1_Vote1(t *testing.T) {
	ilog.Stop()

	js := NewJSTester(t)
	defer js.Clear()
	lc, err := ReadFile("../config/vote.js")
	if err != nil {
		t.Fatal(err)
	}
	js.SetJS(string(lc))
	js.SetAPI("RegisterProducer", "string", "string", "string", "string")
	js.SetAPI("UpdateProducer", "string", "string", "string", "string")
	js.SetAPI("LogInProducer", "string")
	js.SetAPI("LogOutProducer", "string")
	js.SetAPI("UnregisterProducer", "string")
	js.SetAPI("Vote", "string", "string", "number")
	js.SetAPI("Unvote", "string", "string", "number")
	js.SetAPI("Stat")
	js.SetAPI("Init")
	for i := 0; i <= 18; i += 2 {
		js.vi.SetBalance(testID[i], 5e+7)
	}
	js.vi.Commit()
	r := js.DoSet()
	if r.Status.Code != 0 {
		t.Fatal(r.Status.Message)
	}
	//r = js.TestJS("init", `[]`)
	//if r.Status.Code != 0 {
	//	t.Fatal(r.Status.Message)
	//}
	for i := 6; i <= 18; i += 2 {
		if int64(50000000) != js.vi.Balance(testID[i]) {
			t.Fatal("error in balance :", i, js.vi.Balance(testID[i]))
		}
	}

	//keys := []string{
	//	"producerRegisterFee", "producerNumber", "preProducerThreshold", "preProducerMap",
	//	"voteLockTime", "currentProducerList", "pendingProducerList", "pendingBlockNumber",
	//	"producerTable",
	//	"voteTable",
	//}
	////js.FlushDB(t, keys)

	// test register, should success
	r = js.TestJS("RegisterProducer", fmt.Sprintf(`["%v","loc","url","netid"]`, testID[0]))
	if r.Status.Code != 0 {
		t.Fatal(r.Status.Message)
	}

	// test require auth
	r = js.TestJS("RegisterProducer", fmt.Sprintf(`["%v","loc","url","netid"]`, testID[2]))
	if r.Status.Code != 4 {
		t.Fatal(r.Status.Message)
	}

	// get pending producer info
	rtn := database.MustUnmarshal(js.vi.Get(js.cname + "-" + "pendingBlockNumber"))
	if rtn != "0" {
		t.Fatal(rtn)
	}
	srtn := js.ReadMap("producerTable", testID[0])
	if srtn != `{"loc":"loc","url":"url","netId":"netid","online":false,"score":0,"votes":0}` {
		t.Fatal(srtn)
	}
	// test re register
	r = js.TestJS("RegisterProducer", fmt.Sprintf(`["%v","loc","url","netid"]`, testID[0]))
	if r.Status.Code != 4 {
		t.Fatal(r.Status.Message)
	}
}

func TestJS_VoteServi(t *testing.T) {
	ilog.Stop()

	js := NewJSTester(t)
	defer js.Clear()
	lc, err := ReadFile("../config/vote.js")
	if err != nil {
		t.Fatal(err)
	}
	js.SetJS(string(lc))
	js.SetAPI("RegisterProducer", "string", "string", "string", "string")
	js.SetAPI("UpdateProducer", "string", "string", "string", "string")
	js.SetAPI("LogInProducer", "string")
	js.SetAPI("LogOutProducer", "string")
	js.SetAPI("UnregisterProducer", "string")
	js.SetAPI("Vote", "string", "string", "number")
	js.SetAPI("Unvote", "string", "string", "number")
	js.SetAPI("Stat")
	js.SetAPI("Init")
	for i := 0; i <= 18; i += 2 {
		js.vi.SetBalance(testID[i], 5e+7)
	}
	js.vi.Commit()
	r := js.DoSet()
	if r.Status.Code != 0 {
		t.Fatal(r.Status.Message)
	}
	//keys := []string{
	//	"producerRegisterFee", "producerNumber", "preProducerThreshold", "preProducerMap",
	//	"voteLockTime", "currentProducerList", "pendingProducerList", "pendingBlockNumber",
	//	"producerTable",
	//	"voteTable",
	//}
	////js.FlushDB(t, keys)
}

func TestJS_Vote(t *testing.T) {
	//t.Skip()
	Convey("test of vote", t, func() {
		ilog.Stop()

		js := NewJSTester(t)
		bh := &block.BlockHead{
			ParentHash: []byte("abc"),
			Number:     0,
			Witness:    "witness",
			Time:       123456,
		}
		js.NewBlock(bh)

		defer js.Clear()
		lc, err := ReadFile("../config/vote.js")
		if err != nil {
			t.Fatal(err)
		}
		js.SetJS(string(lc))
		js.SetAPI("RegisterProducer", "string", "string", "string", "string")
		js.SetAPI("UpdateProducer", "string", "string", "string", "string")
		js.SetAPI("LogInProducer", "string")
		js.SetAPI("LogOutProducer", "string")
		js.SetAPI("UnregisterProducer", "string")
		js.SetAPI("Vote", "string", "string", "number")
		js.SetAPI("Unvote", "string", "string", "number")
		js.SetAPI("Stat")
		js.SetAPI("init")
		js.SetAPI("InitProducer", "string")
		for i := 0; i <= 18; i += 2 {
			js.vi.SetBalance(testID[i], 5e+7)
		}
		js.vi.Commit()
		r := js.DoSet()
		if r.Status.Code != 0 {
			t.Fatal(r.Status.Message)
		}

		for i := 0; i < 14; i += 2 {
			tt := watchTime(func() {
				r = js.TestJS("InitProducer", fmt.Sprintf(`["%v"]`, testID[i]))
			})
			if r.Status.Code != 0 {
				t.Log(tt)
				t.Fatal(r.Status.Message)
			}
			t.Log(r.GasUsage)
			t.Log(tt)
		}

		keys := []string{
			"producerRegisterFee", "producerNumber", "preProducerThreshold", "preProducerMap",
			"voteLockTime", "currentProducerList", "pendingProducerList", "pendingBlockNumber",
			"producerTable",
			"voteTable",
		}
		_ = keys
		//js.FlushDB(t, keys)

		bh = &block.BlockHead{
			ParentHash: []byte("abc"),
			Number:     10,
			Witness:    "witness",
			Time:       123456,
		}
		js.NewBlock(bh)

		// test register, login, logout
		r = js.TestJS("LogOutProducer", `["a"]`)
		So(r.Status.Message, ShouldContainSubstring, "require auth failed")
		t.Log("time of log in", watchTime(func() {
			r = js.TestJS("LogInProducer", fmt.Sprintf(`["%v"]`, testID[0]))
		}))

		So(r.Status.Message, ShouldEqual, "")

		So(js.ReadMap("producerTable", testID[0]).(string), ShouldEqual, `{"loc":"","url":"","netId":"","online":true,"score":0,"votes":0}`)

		t.Log("time of register", watchTime(func() {
			r = js.TestJS("RegisterProducer", fmt.Sprintf(`["%v","loc","url","netid"]`, testID[0]))
		}))
		So(r.Status.Message, ShouldContainSubstring, "producer exists")

		r = js.TestJS("LogInProducer", fmt.Sprintf(`["%v"]`, testID[0]))
		So(r.Status.Message, ShouldEqual, "")

		r = js.TestJS("UpdateProducer", fmt.Sprintf(`["%v", "%v", "%v", "%v"]`, testID[0], "nloc", "nurl", "nnetid"))
		So(r.Status.Message, ShouldEqual, "")

		So(js.ReadMap("producerTable", testID[0]).(string), ShouldEqual, `{"loc":"nloc","url":"nurl","netId":"nnetid","online":true,"score":0,"votes":0}`)

		// stat, no changes
		r = js.TestJS("Stat", `[]`)
		So(r.Status.Message, ShouldContainSubstring, "block number mismatch")

		So(js.ReadDB(`pendingProducerList`), ShouldEqual, `["IOST4wQ6HPkSrtDRYi2TGkyMJZAB3em26fx79qR3UJC7fcxpL87wTn",`+
			`"IOST558jUpQvBD7F3WTKpnDAWg6HwKrfFiZ7AqhPFf4QSrmjdmBGeY","IOST7ZNDWeh8pHytAZdpgvp7vMpjZSSe5mUUKxDm6AXPsbdgDMAYhs",`+
			`"IOST54ETA3q5eC8jAoEpfRAToiuc6Fjs5oqEahzghWkmEYs9S9CMKd","IOST7GmPn8xC1RESMRS6a62RmBcCdwKbKvk2ZpxZpcXdUPoJdapnnh",`+
			`"IOST7ZGQL4k85v4wAxWngmow7JcX4QFQ4mtLNjgvRrEnEuCkGSBEHN","IOST59uMX3Y4ab5dcq8p1wMXodANccJcj2efbcDThtkw6egvcni5L9"]`)

		// vote and unvote
		r = js.TestJS("Vote", fmt.Sprintf(`["%v", "%v", %d]`, testID[0], testID[0], 10000000))
		So(r.Status.Message, ShouldEqual, "")

		r = js.TestJS("Vote", fmt.Sprintf(`["%v", "%v", %d]`, testID[0], testID[0], 10000000))
		So(r.Status.Message, ShouldEqual, "")

		r = js.TestJS("Vote", fmt.Sprintf(`["%v", "%v", %d]`, testID[0], testID[2], 10000000))
		So(r.Status.Message, ShouldContainSubstring, "require auth failed")

		r = js.TestJS("Unvote", fmt.Sprintf(`["%v", "%v", %d]`, testID[0], testID[0], 10000000))
		So(r.Status.Message, ShouldContainSubstring, "vote still locked")

		//js.FlushDB(t, keys)

		// stat testID[0] become pending producer
		r = js.TestJS("Stat", `[]`)
		So(r.Status.Message, ShouldContainSubstring, "block number mismatch")

		bh = &block.BlockHead{
			ParentHash: []byte("abc"),
			Number:     200,
			Witness:    "witness",
			Time:       123456,
		}
		js.NewBlock(bh)
		t.Log("time of stat", watchTime(func() {
			r = js.TestJS("Stat", `[]`)
		}))
		if r.Status.Code != 0 {
			t.Fatal(r.Status.Message)
		}

		So(js.ReadDB(`pendingProducerList`), ShouldEqual, `["IOST4wQ6HPkSrtDRYi2TGkyMJZAB3em26fx79qR3UJC7fcxpL87wTn",`+
			`"IOST558jUpQvBD7F3WTKpnDAWg6HwKrfFiZ7AqhPFf4QSrmjdmBGeY","IOST7ZNDWeh8pHytAZdpgvp7vMpjZSSe5mUUKxDm6AXPsbdgDMAYhs",`+
			`"IOST54ETA3q5eC8jAoEpfRAToiuc6Fjs5oqEahzghWkmEYs9S9CMKd","IOST7GmPn8xC1RESMRS6a62RmBcCdwKbKvk2ZpxZpcXdUPoJdapnnh",`+
			`"IOST7ZGQL4k85v4wAxWngmow7JcX4QFQ4mtLNjgvRrEnEuCkGSBEHN","IOST59uMX3Y4ab5dcq8p1wMXodANccJcj2efbcDThtkw6egvcni5L9"]`)

		bh = &block.BlockHead{
			ParentHash: []byte("abc"),
			Number:     211,
			Witness:    "witness",
			Time:       123456,
		}
		js.NewBlock(bh)

		// test unvote
		r = js.TestJS("Unvote", fmt.Sprintf(`["%v", "%v", %d]`, testID[0], testID[0], 20000001))
		So(r.Status.Message, ShouldContainSubstring, "vote amount less than expected")

		r = js.TestJS("Unvote", fmt.Sprintf(`["%v", "%v", %d]`, testID[0], testID[0], 1000000))
		So(r.Status.Message, ShouldEqual, "")

		So(js.vi.Servi(testID[0]), ShouldEqual, int64(1055000))
		So(js.vi.TotalServi(), ShouldEqual, int64(1055000))
		// stat pending producers don't get score

		// seven
		for i := 2; i <= 14; i += 2 {
			js.vi.SetBalance(testID[i], 5e+7)
		}
		r = js.TestJSWithAuth("RegisterProducer", fmt.Sprintf(`["%v","loc","url","netid"]`, testID[14]), testID[15])
		So(r.Status.Message, ShouldEqual, "")
		for i := 2; i <= 14; i += 2 {
			r = js.TestJSWithAuth("Vote", fmt.Sprintf(`["%v", "%v", %d]`, testID[i], testID[i], 30000000+i), testID[i+1])
			So(r.Status.Message, ShouldEqual, "")
			So(js.ReadMap("producerTable", testID[i]), ShouldContainSubstring, strconv.Itoa(30000000+i))
		}
		So(js.ReadMap("preProducerMap", testID[14]), ShouldEqual, "true")

		bh = &block.BlockHead{
			ParentHash: []byte("abc"),
			Number:     400,
			Witness:    "witness",
			Time:       123456,
		}
		js.NewBlock(bh)

		// stat, offline producers don't get score
		r = js.TestJS("Stat", `[]`)
		So(r.Status.Message, ShouldEqual, "")

		for i := 2; i <= 14; i += 2 {
			r = js.TestJSWithAuth("LogInProducer", fmt.Sprintf(`["%v"]`, testID[i]), testID[i+1])
			So(r.Status.Message, ShouldEqual, "")
		}

		bh = &block.BlockHead{
			ParentHash: []byte("abc"),
			Number:     600,
			Witness:    "witness",
			Time:       123456,
		}
		js.NewBlock(bh)

		// stat, 1 producer become pending
		t.Log("time of stat", watchTime(func() {
			r = js.TestJS("Stat", `[]`)
		}))
		So(r.Status.Message, ShouldEqual, "")

		So(js.ReadMap("producerTable", testID[14]), ShouldContainSubstring, `"score":9000014`)
		So(js.ReadDB("pendingProducerList"), ShouldContainSubstring, "IOST8mFxe4kq9XciDtURFZJ8E76B8UssBgRVFA5gZN9HF5kLUVZ1BB")
		return

		t.Log(js.TestJS("LogOutProducer", fmt.Sprintf(`["%v"]`, testID[12])))

		// stat, offline producer doesn't become pending. offline and pending producer don't get score, other pre producers get score
		t.Log(js.TestJS("Stat", `[]`))
		//js.FlushDB(t, keys)

		t.Log(js.TestJS("LogInProducer", fmt.Sprintf(`["%v"]`, testID[12])))

		// stat, offline producer doesn't become pending. offline and pending producer don't get score, other pre producers get score
		t.Log(js.TestJS("Stat", `[]`))
		//js.FlushDB(t, keys)

		t.Log(js.TestJS("Stat", `[]`))
		//js.FlushDB(t, keys)

		t.Log(js.TestJS("Stat", `[]`))
		//js.FlushDB(t, keys)

		t.Log(js.TestJS("Stat", `[]`))
		//js.FlushDB(t, keys)

		// testID[0] become pre producer from pending producer, score = 0
		t.Log(js.TestJS("Stat", `[]`))
		//js.FlushDB(t, keys)

		t.Log(js.TestJS("Stat", `[]`))
		//js.FlushDB(t, keys)

		t.Log(js.TestJS("Unvote", fmt.Sprintf(`["%v", "%v", %d]`, testID[0], testID[0], 10000000)))
		//js.FlushDB(t, keys)
		t.Log(js.vi.Servi(testID[0]))
		t.Log(js.vi.TotalServi())

		// unregister
		t.Log(js.TestJS("UnregisterProducer", fmt.Sprintf(`["%v"]`, testID[0])))
		//js.FlushDB(t, keys)

		// unvote after unregister
		t.Log(js.TestJS("Unvote", fmt.Sprintf(`["%v", "%v", %d]`, testID[0], testID[0], 9000000)))
		//js.FlushDB(t, keys)
		t.Log(js.vi.Servi(testID[0]))
		t.Log(js.vi.TotalServi())

		// re register, score = 0, vote = 0
		t.Log(js.TestJS("RegisterProducer", fmt.Sprintf(`["%v","loc","url","netid"]`, testID[0])))
		t.Log(js.TestJS("LogInProducer", fmt.Sprintf(`["%v"]`, testID[0])))
		//js.FlushDB(t, keys)

		t.Log(js.TestJS("Vote", fmt.Sprintf(`["%v", "%v", %d]`, testID[0], testID[2], 21000001)))
		//js.FlushDB(t, keys)

		t.Log(js.TestJS("Stat", `[]`))
		//js.FlushDB(t, keys)

		// unregister pre producer
		t.Log(js.TestJS("UnregisterProducer", fmt.Sprintf(`["%v"]`, testID[0])))
		//js.FlushDB(t, keys)

		// test bonus
		t.Log(js.vi.Servi(testID[0]))
		t.Log(js.vi.Balance(host.ContractAccountPrefix + "iost.bonus"))
		act2 := tx.NewAction("iost.bonus", "ClaimBonus", fmt.Sprintf(`["%v", %d]`, testID[0], 1))

		trx2, err := MakeTx(act2)
		if err != nil {
			t.Fatal(err)
		}

		r, err = js.e.Exec(trx2)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(r)

		t.Log(js.vi.Servi(testID[0]))
		t.Log(js.vi.Balance(host.ContractAccountPrefix + "iost.bonus"))
		t.Log(js.vi.Balance(testID[0]))
		act2 = tx.NewAction("iost.bonus", "ClaimBonus", fmt.Sprintf(`["%v", %d]`, testID[0], 21099999))

		trx2, err = MakeTx(act2)
		if err != nil {
			t.Fatal(err)
		}

		r, err = js.e.Exec(trx2)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(r)

		t.Log(js.vi.Servi(testID[0]))
		t.Log(js.vi.Balance(host.ContractAccountPrefix + "iost.bonus"))
		t.Log(js.vi.Balance(testID[0]))
	})

}

//nolint
func TestJS_Genesis(t *testing.T) {
	t.Skip("skip genesis")

	witnessInfo := testID
	var acts []*tx.Action
	for i := 0; i < len(witnessInfo)/2; i++ {
		act := tx.NewAction("iost.system", "IssueIOST", fmt.Sprintf(`["%v", %v]`, witnessInfo[2*i], 50000000))
		acts = append(acts, &act)
	}
	VoteContractPath := os.Getenv("GOPATH") + "/src/github.com/iost-official/Go-IOS-Protocol/config/"
	// deploy iost.vote
	voteFilePath := VoteContractPath + "vote.js"
	voteAbiPath := VoteContractPath + "vote.js.abi"
	fd, err := common.ReadFile(voteFilePath)
	if err != nil {
		t.Fatal(err)
	}
	rawCode := string(fd)
	fd, err = common.ReadFile(voteAbiPath)
	if err != nil {
		t.Fatal(err)
	}
	rawAbi := string(fd)
	c := contract.Compiler{}
	code, err := c.Parse("iost.vote", rawCode, rawAbi)
	if err != nil {
		t.Fatal(err)
	}
	num := len(witnessInfo) / 2

	act := tx.NewAction("iost.system", "InitSetCode", fmt.Sprintf(`["%v", "%v"]`, "iost.vote", code.B64Encode()))
	acts = append(acts, &act)

	for i := 0; i < num; i++ {
		act1 := tx.NewAction("iost.vote", "InitProducer", fmt.Sprintf(`["%v"]`, witnessInfo[2*i]))
		acts = append(acts, &act1)
	}

	// deploy iost.bonus
	act2 := tx.NewAction("iost.system", "InitSetCode", fmt.Sprintf(`["%v", "%v"]`, "iost.bonus", native.BonusABI().B64Encode()))
	acts = append(acts, &act2)

	trx := tx.NewTx(acts, nil, 10000000, 0, 0)
	trx.Time = 0
	acc, err := account.NewAccount(common.Base58Decode("BQd9x7rQk9Y3rVWRrvRxk7DReUJWzX4WeP9H9H4CV8Mt"), crypto.Secp256k1)
	if err != nil {
		t.Fatal(err)
	}
	trx, err = tx.SignTx(trx, acc)
	if err != nil {
		t.Fatal(err)
	}

	blockHead := block.BlockHead{
		Version:    0,
		ParentHash: nil,
		Number:     0,
		Witness:    acc.ID,
		Time:       time.Now().Unix() / common.SlotLength,
	}
	mvccdb, err := db.NewMVCCDB("mvcc")
	defer closeMVCCDB(mvccdb)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(&blockHead, mvccdb)
	engine.SetUp("js_path", os.Getenv("GOPATH")+"/src/github.com/iost-official/Go-IOS-Protocol/vm/v8vm/v8/libjs/")
	var txr *tx.TxReceipt
	ti := watchTime(func() {
		txr, err = engine.Exec(trx)
	})
	if err != nil {
		t.Fatal(fmt.Errorf("exec tx failed, stop the pogram. err: %v", err))
	}
	if txr.Status.Code != 0 {
		t.Fatal(txr.Status.Message)
	}
	if ti > time.Second {
		t.Fatal(ti)
	}
	//pl := database.MustUnmarshal(database.NewVisitor(0, mvccdb).Get("iost.vote" + "-" + "pendingProducerList"))

	if txr.Status.Code != tx.Success {
		t.Fatal("exec trx failed.")
	}
	blk := block.Block{
		Head:     &blockHead,
		Sign:     &crypto.Signature{},
		Txs:      []*tx.Tx{trx},
		Receipts: []*tx.TxReceipt{txr},
	}
	blk.Head.TxsHash = blk.CalculateTxsHash()
	blk.Head.MerkleHash = blk.CalculateMerkleHash()
	err = blk.CalculateHeadHash()
	if err != nil {
		t.Fatal(err)
	}

}
