package integration

import (
	"encoding/json"
	"fmt"
	"github.com/iost-official/go-iost/core/global"
	"testing"

	"github.com/iost-official/go-iost/ilog"
	"github.com/stretchr/testify/assert"

	"github.com/iost-official/go-iost/core/tx"
	. "github.com/iost-official/go-iost/verifier"
	"github.com/iost-official/go-iost/vm/database"
	. "github.com/smartystreets/goconvey/convey"
)

func prepareToken(t *testing.T, s *Simulator, pubAcc *TestAccount) {
	r, err := s.Call("token.iost", "create", fmt.Sprintf(`["%v", "%v", %v, {}]`, global.Token, acc0.ID, "21000000000"), pubAcc.ID, pubAcc.KeyPair)
	if err != nil || r.Status.Code != tx.Success {
		t.Fatal(err, r)
	}
	for _, acc := range testAccounts {
		s.Call("token.iost", "issue", fmt.Sprintf(`["%v", "%v", "%v"]`, global.Token, acc.ID, "2000000000"), pubAcc.ID, pubAcc.KeyPair)
	}
	s.Visitor.Commit()
}

func prepareVote(t *testing.T, s *Simulator, acc *TestAccount) (*tx.TxReceipt, error) {
	// deploy vote.iost
	s.Head.Number = 0
	r, err := setNonNativeContract(s, "vote.iost", "vote_common.js", ContractPath)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)

	r, err = s.Call("vote.iost", "initAdmin", fmt.Sprintf(`["%v"]`, acc.ID), acc.ID, acc.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)

	// deploy voteresult
	r, err = setNonNativeContract(s, "Contractvoteresult", "voteresult.js", "./test_data/")
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)

	s.Visitor.MPut("system.iost-contract_owner", "Contractvoteresult", `s`+acc.ID)
	s.SetGas(acc.ID, 1e12)
	s.SetRAM(acc.ID, 1e12)

	config := make(map[string]interface{})
	config["resultNumber"] = 2
	config["minVote"] = 10
	config["options"] = []string{"option1", "option2", "option3", "option4"}
	config["anyOption"] = false
	config["freezeTime"] = 0
	params := []interface{}{
		acc0.ID,
		"test vote",
		config,
	}
	b, _ := json.Marshal(params)
	r, err = s.Call("vote.iost", "newVote", string(b), acc.ID, acc.KeyPair)
	s.Visitor.Commit()
	return r, err
}

func Test_NewVote(t *testing.T) {
	ilog.Stop()
	Convey("test newVote", t, func() {
		s := NewSimulator()
		defer s.Clear()

		createAccountsWithResource(s)
		prepareToken(t, s, acc0)

		r, err := prepareVote(t, s, acc0)
		So(err, ShouldBeNil)
		So(r.Status.Code, ShouldEqual, tx.Success)
		So(s.Visitor.TokenBalance(global.Token, acc0.ID), ShouldEqual, int64(2000000000*1e8))
		So(database.MustUnmarshal(s.Visitor.Get("vote.iost-current_id")), ShouldEqual, `"1"`)
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-voteInfo", "1")), ShouldEqual, `{"deleted":0,"description":"test vote","resultNumber":2,"minVote":10,"anyOption":false,"freezeTime":0,"deposit":"0","optionNum":4,"canVote":true}`)
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", "option1")), ShouldEqual, `{"votes":"0","deleted":0,"clearTime":-1}`)

		r, err = s.Call("Contractvoteresult", "getResult", `["1"]`, acc0.ID, acc0.KeyPair)

		So(err, ShouldBeNil)
		So(r.Status.Code, ShouldEqual, tx.Success)
		So(database.MustUnmarshal(s.Visitor.MGet("Contractvoteresult-vote_result", "1")), ShouldEqual, `[]`)

	})
}

func Test_AddOption(t *testing.T) {
	ilog.Stop()
	Convey("test vote", t, func() {
		s := NewSimulator()
		defer s.Clear()

		createAccountsWithResource(s)
		prepareToken(t, s, acc0)
		r, err := prepareVote(t, s, acc0)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")

		r, err = s.Call("vote.iost", "addOption", `["1", "option5", true]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		So(s.Visitor.MKeys("vote.iost-v_1"), ShouldResemble, []string{"option1", "option2", "option3", "option4", "option5"})
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", "option5")), ShouldEqual, `{"votes":"0","deleted":0,"clearTime":-1}`)

		r, err = s.Call("vote.iost", "getOption", `["1", "option5"]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		So(r.Returns[0], ShouldEqual, `["{\"votes\":\"0\",\"deleted\":0,\"clearTime\":-1}"]`)
	})
}

func Test_RemoveOption(t *testing.T) {
	ilog.Stop()
	Convey("test remove option", t, func() {
		s := NewSimulator()
		defer s.Clear()

		createAccountsWithResource(s)
		prepareToken(t, s, acc0)
		r, err := prepareVote(t, s, acc0)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")

		r, err = s.Call("vote.iost", "removeOption", `["1", "option2", true]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Code, ShouldEqual, tx.Success)
		So(s.Visitor.MKeys("vote.iost-v_1"), ShouldResemble, []string{"option1", "option3", "option4"})
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-voteInfo", "1")), ShouldEqual, `{"deleted":0,"description":"test vote","resultNumber":2,"minVote":10,"anyOption":false,"freezeTime":0,"deposit":"0","optionNum":3,"canVote":true}`)
	})
}

func Test_Vote(t *testing.T) {
	ilog.Stop()
	Convey("test vote", t, func() {
		s := NewSimulator()
		defer s.Clear()

		s.Head.Number = 0
		s.GasLimit = 2e8

		createAccountsWithResource(s)
		prepareToken(t, s, acc0)
		rs, err := prepareVote(t, s, acc0)
		So(err, ShouldBeNil)
		So(rs.Status.Message, ShouldEqual, "")

		rs, err = s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option3", "5"]`, acc1.ID), acc1.ID, acc1.KeyPair)
		So(err, ShouldBeNil)
		So(rs.Status.Message, ShouldEqual, "")

		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", "option3")), ShouldEqual, `{"votes":"5","deleted":0,"clearTime":-1}`)
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-u_1", acc1.ID)), ShouldEqual, `{"option3":["5",0,"0"]}`)

		rs, err = s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option3", "5"]`, acc1.ID), acc1.ID, acc1.KeyPair)
		So(err, ShouldBeNil)
		So(rs.Status.Message, ShouldEqual, "")
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", "option3")), ShouldEqual, `{"votes":"10","deleted":0,"clearTime":-1}`)
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-u_1", acc1.ID)), ShouldEqual, `{"option3":["10",0,"0"]}`)
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-preResult", "1")), ShouldContainSubstring, `"option3":"10"`)

		rs, err = s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option1", "20"]`, acc1.ID), acc1.ID, acc1.KeyPair)
		So(err, ShouldBeNil)
		So(rs.Status.Message, ShouldEqual, "")
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", "option1")), ShouldEqual, `{"votes":"20","deleted":0,"clearTime":-1}`)
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-u_1", acc1.ID)), ShouldEqual, `{"option3":["10",0,"0"],"option1":["20",0,"0"]}`)
		info := database.MustUnmarshal(s.Visitor.MGet("vote.iost-preResult", "1"))
		So(info, ShouldContainSubstring, `option3`)
		So(info, ShouldContainSubstring, `option1`)

		rs, err = s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option3", "100"]`, acc0.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(rs.Status.Message, ShouldEqual, "")
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", "option3")), ShouldEqual, `{"votes":"110","deleted":0,"clearTime":-1}`)
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-u_1", acc0.ID)), ShouldEqual, `{"option3":["100",0,"0"]}`)
		info = database.MustUnmarshal(s.Visitor.MGet("vote.iost-preResult", "1"))
		So(info, ShouldContainSubstring, `option3`)
		So(info, ShouldContainSubstring, `option1`)

		rs, err = s.Call("Contractvoteresult", "getResult", `["1"]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(rs.Status.Message, ShouldEqual, "")
		So(database.MustUnmarshal(s.Visitor.MGet("Contractvoteresult-vote_result", "1")), ShouldEqual, `[{"option":"option3","votes":"110"},{"option":"option1","votes":"20"}]`)

		r, err := s.Call("vote.iost", "getOption", `["1", "option3"]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Code, ShouldEqual, tx.Success)
		So(r.Returns[0], ShouldEqual, `["{\"votes\":\"110\",\"deleted\":0,\"clearTime\":-1}"]`)

		r, err = s.Call("vote.iost", "getVote", fmt.Sprintf(`["1", "%v"]`, acc1.ID), acc1.ID, acc1.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Code, ShouldEqual, tx.Success)
		So(r.Returns[0], ShouldEqual, `["[{\"option\":\"option3\",\"votes\":\"10\",\"voteTime\":0,\"clearedVotes\":\"0\"},{\"option\":\"option1\",\"votes\":\"20\",\"voteTime\":0,\"clearedVotes\":\"0\"}]"]`)
	})
}

func Test_Unvote(t *testing.T) {
	ilog.Stop()
	Convey("test unvote", t, func() {
		s := NewSimulator()
		defer s.Clear()

		s.Head.Number = 0

		createAccountsWithResource(s)
		prepareToken(t, s, acc0)
		prepareVote(t, s, acc0)

		// vote
		r, err := s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option1", "100"]`, acc1.ID), acc1.ID, acc1.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		info := database.MustUnmarshal(s.Visitor.MGet("vote.iost-preResult", "1"))
		So(info, ShouldContainSubstring, `option1`)

		r, err = s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option2", "100"]`, acc1.ID), acc1.ID, acc1.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option2", "100"]`, acc0.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		info = database.MustUnmarshal(s.Visitor.MGet("vote.iost-preResult", "1"))
		So(info, ShouldContainSubstring, `option1`)
		So(info, ShouldContainSubstring, `option2`)

		r, err = s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option3", "300"]`, acc0.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", "option3")), ShouldEqual, `{"votes":"300","deleted":0,"clearTime":-1}`)
		info = database.MustUnmarshal(s.Visitor.MGet("vote.iost-preResult", "1"))
		So(info, ShouldContainSubstring, `option2`)
		So(info, ShouldContainSubstring, `option3`)

		r, err = s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option4", "400"]`, acc2.ID), acc2.ID, acc2.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		info = database.MustUnmarshal(s.Visitor.MGet("vote.iost-preResult", "1"))
		So(info, ShouldContainSubstring, `option3`)
		So(info, ShouldContainSubstring, `option4`)

		// get result
		r, err = s.Call("Contractvoteresult", "getResult", `["1"]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		So(database.MustUnmarshal(s.Visitor.MGet("Contractvoteresult-vote_result", "1")), ShouldEqual, `[{"option":"option4","votes":"400"},{"option":"option3","votes":"300"}]`)

		// unvote
		r, err = s.Call("vote.iost", "unvote", fmt.Sprintf(`["1", "%v", "option3", "100"]`, acc0.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", "option3")), ShouldEqual, `{"votes":"200","deleted":0,"clearTime":-1}`)
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-u_1", acc0.ID)), ShouldEqual, `{"option2":["100",0,"0"],"option3":["200",0,"0"]}`)
		info = database.MustUnmarshal(s.Visitor.MGet("vote.iost-preResult", "1"))
		So(info, ShouldContainSubstring, `option3`)
		So(info, ShouldContainSubstring, `option4`)

		// get result
		r, err = s.Call("Contractvoteresult", "getResult", `["1"]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		So(database.MustUnmarshal(s.Visitor.MGet("Contractvoteresult-vote_result", "1")), ShouldEqual, `[{"option":"option4","votes":"400"},{"option":"option2","votes":"200"}]`)

		// unvote again
		r, err = s.Call("vote.iost", "unvote", fmt.Sprintf(`["1", "%v", "option2", "95"]`, acc0.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", "option2")), ShouldEqual, `{"votes":"105","deleted":0,"clearTime":-1}`)
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-u_1", acc0.ID)), ShouldEqual, `{"option2":["5",0,"0"],"option3":["200",0,"0"]}`)
		info = database.MustUnmarshal(s.Visitor.MGet("vote.iost-preResult", "1"))
		So(info, ShouldContainSubstring, `option3`)
		So(info, ShouldContainSubstring, `option4`)

		r, err = s.Call("vote.iost", "unvote", fmt.Sprintf(`["1", "%v", "option2", "100"]`, acc1.ID), acc1.ID, acc1.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", "option2")), ShouldEqual, `{"votes":"5","deleted":0,"clearTime":-1}`)
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-u_1", acc1.ID)), ShouldEqual, `{"option1":["100",0,"0"]}`)
		info = database.MustUnmarshal(s.Visitor.MGet("vote.iost-preResult", "1"))
		So(info, ShouldContainSubstring, `option3`)
		So(info, ShouldContainSubstring, `option4`)

		// get result
		r, err = s.Call("Contractvoteresult", "getResult", `["1"]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		So(database.MustUnmarshal(s.Visitor.MGet("Contractvoteresult-vote_result", "1")), ShouldEqual, `[{"option":"option4","votes":"400"},{"option":"option3","votes":"200"}]`)
	})
}

func Test_DelVote(t *testing.T) {
	ilog.Stop()
	Convey("test delVote", t, func() {
		s := NewSimulator()
		defer s.Clear()

		s.Head.Number = 0

		createAccountsWithResource(s)
		prepareToken(t, s, acc0)
		prepareVote(t, s, acc0)

		// vote
		s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option1", "100"]`, acc1.ID), acc1.ID, acc1.KeyPair)
		s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option2", "100"]`, acc1.ID), acc1.ID, acc1.KeyPair)
		s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option2", "100"]`, acc0.ID), acc0.ID, acc0.KeyPair)
		s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option3", "300"]`, acc0.ID), acc0.ID, acc0.KeyPair)
		s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option4", "400"]`, acc2.ID), acc2.ID, acc2.KeyPair)

		// del vote
		r, err := s.Call("vote.iost", "delVote", `["1"]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")

		// unvote part
		r, err = s.Call("vote.iost", "unvote", fmt.Sprintf(`["1", "%v", "option2", "95"]`, acc0.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-u_1", acc0.ID)), ShouldEqual, `{"option2":["5",0,"0"],"option3":["300",0,"0"]}`)

		// unvote all
		r, err = s.Call("vote.iost", "unvote", fmt.Sprintf(`["1", "%v", "option2", "5"]`, acc0.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote.iost", "unvote", fmt.Sprintf(`["1", "%v", "option3", "300"]`, acc0.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		So(s.Visitor.MHas("vote.iost-u_1", acc0.ID), ShouldEqual, false)
		So(s.Visitor.MKeys("vote.iost-v_1"), ShouldResemble, []string{"option1", "option2", "option4"})

		// remove option
		r, err = s.Call("vote.iost", "removeOption", `["1", "option2", true]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		So(s.Visitor.MKeys("vote.iost-v_1"), ShouldResemble, []string{"option1", "option4"})
	})
}

func Test_MixVoteOption(t *testing.T) {
	ilog.Stop()
	Convey("test mixed", t, func() {
		s := NewSimulator()
		defer s.Clear()

		s.Head.Number = 0

		createAccountsWithResource(s)
		prepareToken(t, s, acc0)
		prepareVote(t, s, acc0)

		Convey("test addOption not clear", func() {
			// vote
			r, err := s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option1", "100"]`, acc1.ID), acc1.ID, acc1.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			r, err = s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option2", "100"]`, acc1.ID), acc1.ID, acc1.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			r, err = s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option2", "100"]`, acc0.ID), acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			r, err = s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option3", "300"]`, acc0.ID), acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			r, err = s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option4", "400"]`, acc2.ID), acc2.ID, acc2.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")

			// add option
			r, err = s.Call("vote.iost", "addOption", `["1", "option5", false]`, acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			So(s.Visitor.MKeys("vote.iost-v_1"), ShouldResemble, []string{"option1", "option2", "option3", "option4", "option5"})

			// get result
			r, err = s.Call("Contractvoteresult", "getResult", `["1"]`, acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			So(database.MustUnmarshal(s.Visitor.MGet("Contractvoteresult-vote_result", "1")), ShouldEqual, `[{"option":"option4","votes":"400"},{"option":"option3","votes":"300"}]`)

			s.Head.Number++
			// remove option
			r, err = s.Call("vote.iost", "removeOption", `["1", "option1", false]`, acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", "option1")), ShouldEqual, `{"votes":"100","deleted":1,"clearTime":-1}`)
			info := database.MustUnmarshal(s.Visitor.MGet("vote.iost-preResult", "1"))
			So(info, ShouldNotContainSubstring, `option1`)

			// add option
			r, err = s.Call("vote.iost", "addOption", `["1", "option1", false]`, acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", "option1")), ShouldEqual, `{"votes":"100","deleted":0,"clearTime":-1}`)
		})

		Convey("test addOption and clear", func() {
			// vote
			rs, err := s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option1", "100"]`, acc1.ID), acc1.ID, acc1.KeyPair)
			So(err, ShouldBeNil)
			So(rs.Status.Message, ShouldEqual, "")
			rs, err = s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option2", "100"]`, acc1.ID), acc1.ID, acc1.KeyPair)
			So(err, ShouldBeNil)
			So(rs.Status.Message, ShouldEqual, "")
			rs, err = s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option2", "100"]`, acc0.ID), acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(rs.Status.Message, ShouldEqual, "")
			rs, err = s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option3", "200"]`, acc0.ID), acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(rs.Status.Message, ShouldEqual, "")
			rs, err = s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option4", "400"]`, acc2.ID), acc2.ID, acc2.KeyPair)
			So(err, ShouldBeNil)
			So(rs.Status.Message, ShouldEqual, "")

			s.Head.Number++
			// remove option
			rs, err = s.Call("vote.iost", "removeOption", `["1", "option1", false]`, acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(rs.Status.Message, ShouldEqual, "")
			// add option
			rs, err = s.Call("vote.iost", "addOption", `["1", "option1", true]`, acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(rs.Status.Message, ShouldEqual, "")

			So(s.Visitor.MKeys("vote.iost-v_1"), ShouldResemble, []string{"option1", "option2", "option3", "option4"})
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", "option1")), ShouldEqual, `{"votes":"0","deleted":0,"clearTime":1}`)
			info := database.MustUnmarshal(s.Visitor.MGet("vote.iost-preResult", "1"))
			So(info, ShouldContainSubstring, `option2`)
			So(info, ShouldContainSubstring, `option4`)

			// vote after clear in same block
			rs, err = s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option1", "100"]`, acc1.ID), acc1.ID, acc1.KeyPair)
			So(err, ShouldBeNil)
			So(rs.Status.Message, ShouldEqual, "")
			info = database.MustUnmarshal(s.Visitor.MGet("vote.iost-preResult", "1"))
			So(info, ShouldNotContainSubstring, `option1`)
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-u_1", acc1.ID)), ShouldEqual, `{"option1":["200",1,"100"],"option2":["100",0,"0"]}`)

			// vote after the clear block
			s.Head.Number++
			rs, err = s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option1", "100"]`, acc1.ID), acc1.ID, acc1.KeyPair)
			So(err, ShouldBeNil)
			So(rs.Status.Message, ShouldEqual, "")
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-u_1", acc1.ID)), ShouldEqual, `{"option1":["300",2,"200"],"option2":["100",0,"0"]}`)

			// vote again
			rs, err = s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option1", "100"]`, acc1.ID), acc1.ID, acc1.KeyPair)
			So(err, ShouldBeNil)
			So(rs.Status.Message, ShouldEqual, "")
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-u_1", acc1.ID)), ShouldEqual, `{"option1":["400",2,"200"],"option2":["100",0,"0"]}`)

			// get result
			rs, err = s.Call("Contractvoteresult", "getResult", `["1"]`, acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(rs.Status.Message, ShouldEqual, "")
			So(database.MustUnmarshal(s.Visitor.MGet("Contractvoteresult-vote_result", "1")), ShouldEqual, `[{"option":"option4","votes":"400"},{"option":"option2","votes":"200"}]`)

			// unvote
			rs, err = s.Call("vote.iost", "unvote", fmt.Sprintf(`["1", "%v", "option1", "50"]`, acc1.ID), acc1.ID, acc1.KeyPair)
			So(err, ShouldBeNil)
			So(rs.Status.Message, ShouldEqual, "")
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-u_1", acc1.ID)), ShouldEqual, `{"option1":["350",2,"150"],"option2":["100",0,"0"]}`)

			// unvote again
			rs, err = s.Call("vote.iost", "unvote", fmt.Sprintf(`["1", "%v", "option1", "200"]`, acc1.ID), acc1.ID, acc1.KeyPair)
			So(err, ShouldBeNil)
			So(rs.Status.Message, ShouldEqual, "")
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-u_1", acc1.ID)), ShouldEqual, `{"option1":["150",2,"0"],"option2":["100",0,"0"]}`)
		})

		Convey("test removeOption not force", func() {
			// vote
			s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option1", "100"]`, acc1.ID), acc1.ID, acc1.KeyPair)
			s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option2", "200"]`, acc1.ID), acc1.ID, acc1.KeyPair)
			s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option2", "100"]`, acc0.ID), acc0.ID, acc0.KeyPair)
			s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option3", "300"]`, acc0.ID), acc0.ID, acc0.KeyPair)
			s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option4", "400"]`, acc2.ID), acc2.ID, acc2.KeyPair)

			// add option
			s.Call("vote.iost", "addOption", `["1", "option5", false]`, acc0.ID, acc0.KeyPair)
			So(s.Visitor.MKeys("vote.iost-v_1"), ShouldResemble, []string{"option1", "option2", "option3", "option4", "option5"})

			// remove option
			s.Call("vote.iost", "removeOption", `["1", "option5", false]`, acc0.ID, acc0.KeyPair)
			// s.Call("vote.iost", "removeOption", `["1", "option4", false]`,acc0.ID, acc0.KeyPair) // should fail
			// s.Call("vote.iost", "removeOption", `["1", "option3", false]`,acc0.ID, acc0.KeyPair) // should fail
			// s.Call("vote.iost", "removeOption", `["1", "option2", false]`,acc0.ID, acc0.KeyPair) // should fail
			s.Call("vote.iost", "removeOption", `["1", "option1", false]`, acc0.ID, acc0.KeyPair)
			So(s.Visitor.MKeys("vote.iost-v_1"), ShouldResemble, []string{"option1", "option2", "option3", "option4"})
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", "option5")), ShouldBeNil)
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", "option4")), ShouldEqual, `{"votes":"400","deleted":0,"clearTime":-1}`)
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", "option3")), ShouldEqual, `{"votes":"300","deleted":0,"clearTime":-1}`)
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", "option2")), ShouldEqual, `{"votes":"300","deleted":0,"clearTime":-1}`)
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", "option1")), ShouldEqual, `{"votes":"100","deleted":1,"clearTime":-1}`)
			info := database.MustUnmarshal(s.Visitor.MGet("vote.iost-preResult", "1"))
			So(info, ShouldContainSubstring, `option2`)
			So(info, ShouldContainSubstring, `option4`)
		})

		Convey("test removeOption with force", func() {
			// vote
			s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option1", "100"]`, acc1.ID), acc1.ID, acc1.KeyPair)
			s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option2", "200"]`, acc1.ID), acc1.ID, acc1.KeyPair)
			s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option2", "100"]`, acc0.ID), acc0.ID, acc0.KeyPair)
			s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option3", "300"]`, acc0.ID), acc0.ID, acc0.KeyPair)
			s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option4", "400"]`, acc2.ID), acc2.ID, acc2.KeyPair)

			// add option
			s.Call("vote.iost", "addOption", `["1", "option5", false]`, acc0.ID, acc0.KeyPair)
			So(s.Visitor.MKeys("vote.iost-v_1"), ShouldResemble, []string{"option1", "option2", "option3", "option4", "option5"})

			// remove option
			s.Call("vote.iost", "removeOption", `["1", "option5", true]`, acc0.ID, acc0.KeyPair)
			s.Call("vote.iost", "removeOption", `["1", "option4", true]`, acc0.ID, acc0.KeyPair)
			s.Call("vote.iost", "removeOption", `["1", "option3", true]`, acc0.ID, acc0.KeyPair)
			s.Call("vote.iost", "removeOption", `["1", "option2", true]`, acc0.ID, acc0.KeyPair)
			s.Call("vote.iost", "removeOption", `["1", "option1", true]`, acc0.ID, acc0.KeyPair)
			So(s.Visitor.MKeys("vote.iost-v_1"), ShouldResemble, []string{"option1", "option2", "option3", "option4"})
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", "option5")), ShouldBeNil)
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", "option4")), ShouldEqual, `{"votes":"400","deleted":1,"clearTime":-1}`)
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", "option3")), ShouldEqual, `{"votes":"300","deleted":1,"clearTime":-1}`)
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", "option2")), ShouldEqual, `{"votes":"300","deleted":1,"clearTime":-1}`)
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", "option1")), ShouldEqual, `{"votes":"100","deleted":1,"clearTime":-1}`)
			info := database.MustUnmarshal(s.Visitor.MGet("vote.iost-preResult", "1"))
			So(info, ShouldEqual, `{}`)
		})
	})
}

func Test_LargeVote(t *testing.T) {
	ilog.Stop()
	Convey("test latge vote", t, func() {
		s := NewSimulator()
		defer s.Clear()

		createAccountsWithResource(s)
		prepareToken(t, s, acc0)
		prepareVote(t, s, acc0)
		s.GasLimit = 2e8

		for i := 5; i < 1000; i++ {
			s.SetGas(acc0.ID, 1e9)
			r, err := s.Call("vote.iost", "addOption", fmt.Sprintf(`["1", "option%d", true]`, i), acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(fmt.Sprintf("%s,%d,%d", r.Status.Message, i, r.GasUsage), ShouldEqual, fmt.Sprintf(",%d,%d", i, r.GasUsage))
		}

		for i := 1; i < 1000; i++ {
			s.SetGas(acc0.ID, 1e9)
			r, err := s.Call("vote.iost", "vote", fmt.Sprintf(`["1", "%v", "option%d", "%d"]`, acc0.ID, i, i), acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
		}

		s.SetGas(acc0.ID, 1e9)
		r, err := s.Call("vote.iost", "getResult", `["1"]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		So(r.Returns[0], ShouldEqual, `["[{\"option\":\"option999\",\"votes\":\"999\"},{\"option\":\"option998\",\"votes\":\"998\"}]"]`)
	})
}

func Test_CanVote(t *testing.T) {
	ilog.Stop()
	Convey("test latge vote", t, func() {
		s := NewSimulator()
		defer s.Clear()

		createAccountsWithResource(s)
		prepareToken(t, s, acc0)
		prepareVote(t, s, acc0)

		s.Head.Number = 1
		config := make(map[string]interface{})
		config["resultNumber"] = 2
		config["minVote"] = 10
		config["options"] = []string{"option1", "option2", "option3", "option4"}
		config["anyOption"] = false
		config["freezeTime"] = 0
		config["canVote"] = false
		params := []interface{}{
			acc0.ID,
			"test vote",
			config,
		}
		b, _ := json.Marshal(params)
		r, err := s.Call("vote.iost", "newVote", string(b), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		So(r.Returns[0], ShouldEqual, `["2"]`)

		r, err = s.Call("vote.iost", "vote", fmt.Sprintf(`["2", "%v", "option1", "1"]`, acc1.ID), acc1.ID, acc1.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldContainSubstring, "require auth failed")

		r, err = s.Call("vote.iost", "vote", fmt.Sprintf(`["2", "%v", "option1", "1"]`, acc0.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")

		r, err = s.Call("vote.iost", "setCanVote", `["2", true]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")

		r, err = s.Call("vote.iost", "vote", fmt.Sprintf(`["2", "%v", "option1", "1"]`, acc1.ID), acc1.ID, acc1.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")

		r, err = s.Call("vote.iost", "setCanVote", `["2", false]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")

		r, err = s.Call("vote.iost", "unvote", fmt.Sprintf(`["2", "%v", "option1", "1"]`, acc1.ID), acc1.ID, acc1.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldContainSubstring, "require auth failed")
	})
}
