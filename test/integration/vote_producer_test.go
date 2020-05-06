package integration

import (
	"encoding/json"
	"fmt"
	"github.com/iost-official/go-iost/core/global"
	"testing"

	"github.com/iost-official/go-iost/crypto"
	"github.com/stretchr/testify/assert"

	"github.com/iost-official/go-iost/account"
	"github.com/iost-official/go-iost/common"
	"github.com/iost-official/go-iost/core/tx"
	"github.com/iost-official/go-iost/ilog"
	. "github.com/iost-official/go-iost/verifier"
	"github.com/iost-official/go-iost/vm/database"
	. "github.com/smartystreets/goconvey/convey"
)

func initProducer(t *testing.T, s *Simulator) {
	for _, acc := range testAccounts[:6] {
		r, err := s.Call("vote_producer.iost", "initProducer", fmt.Sprintf(`["%v", "%v"]`, acc.ID, acc.KeyPair.ReadablePubkey()), acc0.ID, acc0.KeyPair)
		assert.Nil(t, err)
		assert.Empty(t, r.Status.Message)
	}
}

func prepareFakeBase(t *testing.T, s *Simulator) {
	// deploy fake base.iost
	r, err := setNonNativeContract(s, "base.iost", "base.js", "./test_data/")
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	lst := []string{}
	for _, acc := range testAccounts {
		lst = append(lst, acc.KeyPair.ReadablePubkey())
	}
	jsonStr, err := json.Marshal(lst)
	assert.Nil(t, err)
	_, err = s.Call("base.iost", "initWitness", fmt.Sprintf(`[%v]`, string(jsonStr)), acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
}

func prepareNewProducerVote(t *testing.T, s *Simulator, acc1 *TestAccount) {
	s.Head.Number = 0
	// deploy vote.iost
	r, err := setNonNativeContract(s, "vote.iost", "vote_common.js", ContractPath)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)

	r, err = s.Call("vote.iost", "initAdmin", fmt.Sprintf(`["%s"]`, acc1.ID), acc1.ID, acc1.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)

	// deploy vote_producer.iost
	r, err = setNonNativeContract(s, "vote_producer.iost", "vote_producer.js", ContractPath)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)

	r, err = s.Call("token.iost", "issue", fmt.Sprintf(`["%v", "%v", "%v"]`, global.Token, "vote_producer.iost", "1000"), acc1.ID, acc1.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)

	r, err = s.Call("vote_producer.iost", "initAdmin", fmt.Sprintf(`["%s"]`, acc1.ID), acc1.ID, acc1.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)

	s.Visitor.Commit()
}

func Test_InitProducer(t *testing.T) {
	ilog.Stop()
	Convey("test initProducer", t, func() {
		s := NewSimulator()
		defer s.Clear()

		s.Head.Number = 0

		createAccountsWithResource(s)
		prepareToken(t, s, acc0)
		prepareNewProducerVote(t, s, acc0)

		So(database.MustUnmarshal(s.Visitor.Get("vote.iost-current_id")), ShouldEqual, `"1"`)
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-voteId")), ShouldEqual, `1`)
		Convey("test init producer", func() {
			initProducer(t, s)
			list, _ := json.Marshal([]string{acc0.KeyPair.ReadablePubkey(), acc3.KeyPair.ReadablePubkey(), acc1.KeyPair.ReadablePubkey(), acc4.KeyPair.ReadablePubkey(), acc5.KeyPair.ReadablePubkey(), acc2.KeyPair.ReadablePubkey()})
			So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-pendingProducerList")), ShouldEqual, string(list))

			So(s.Visitor.MKeys("vote.iost-v_1"), ShouldResemble, []string{"user_0", "user_1", "user_2", "user_3", "user_4", "user_5"})
		})
	})
}

func Test_Register(t *testing.T) {
	ilog.Stop()
	Convey("test register 1", t, func() {
		s := NewSimulator()
		defer s.Clear()

		s.Head.Number = 0

		createAccountsWithResource(s)
		prepareFakeBase(t, s)
		prepareToken(t, s, acc0)
		prepareNewProducerVote(t, s, acc0)
		initProducer(t, s)

		s.Head.Number = 1
		r, err := s.Call("vote_producer.iost", "applyRegister", fmt.Sprintf(`["%v", "%v", "loc", "url", "netId", true]`, acc6.ID, acc6.KeyPair.ReadablePubkey()), acc6.ID, acc6.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "logInProducer", fmt.Sprintf(`["%v"]`, acc6.ID), acc6.ID, acc6.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "applyUnregister", fmt.Sprintf(`["%v"]`, acc6.ID), acc6.ID, acc6.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "unregister", fmt.Sprintf(`["%v"]`, acc6.ID), acc6.ID, acc6.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "applyRegister", fmt.Sprintf(`["%v", "%v", "loc", "url", "netId", true]`, acc6.ID, acc6.KeyPair.ReadablePubkey()), acc6.ID, acc6.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "approveRegister", fmt.Sprintf(`["%v"]`, acc6.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
	})

	Convey("test register 2", t, func() {
		s := NewSimulator()
		defer s.Clear()

		s.Head.Number = 0

		createAccountsWithResource(s)
		prepareFakeBase(t, s)
		prepareToken(t, s, acc0)
		prepareNewProducerVote(t, s, acc0)
		initProducer(t, s)

		s.Head.Number = 1
		r, err := s.Call("vote_producer.iost", "applyRegister", fmt.Sprintf(`["%v", "%v", "loc", "url", "netId", true]`, acc6.ID, acc6.KeyPair.ReadablePubkey()), acc6.ID, acc6.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "logInProducer", fmt.Sprintf(`["%v"]`, acc6.ID), acc6.ID, acc6.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "approveRegister", fmt.Sprintf(`["%v"]`, acc6.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "applyUnregister", fmt.Sprintf(`["%v"]`, acc6.ID), acc6.ID, acc6.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "approveUnregister", fmt.Sprintf(`["%v"]`, acc6.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "unregister", fmt.Sprintf(`["%v"]`, acc6.ID), acc6.ID, acc6.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "applyRegister", fmt.Sprintf(`["%v", "%v", "loc", "url", "netId", true]`, acc6.ID, acc6.KeyPair.ReadablePubkey()), acc6.ID, acc6.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "approveRegister", fmt.Sprintf(`["%v"]`, acc6.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
	})

	Convey("test register 3", t, func() {
		s := NewSimulator()
		defer s.Clear()

		s.Head.Number = 0

		createAccountsWithResource(s)
		prepareFakeBase(t, s)
		prepareToken(t, s, acc0)
		prepareNewProducerVote(t, s, acc0)
		initProducer(t, s)

		s.Head.Number = 1
		r, err := s.Call("vote_producer.iost", "applyRegister", fmt.Sprintf(`["%v", "%v", "loc", "url", "netId", true]`, acc6.ID, acc6.KeyPair.ReadablePubkey()), acc6.ID, acc6.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "logInProducer", fmt.Sprintf(`["%v"]`, acc6.ID), acc6.ID, acc6.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "approveRegister", fmt.Sprintf(`["%v"]`, acc6.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "forceUnregister", fmt.Sprintf(`["%v"]`, acc6.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "unregister", fmt.Sprintf(`["%v"]`, acc6.ID), acc6.ID, acc6.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "applyRegister", fmt.Sprintf(`["%v", "%v", "loc", "url", "netId", true]`, acc6.ID, acc6.KeyPair.ReadablePubkey()), acc6.ID, acc6.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "approveRegister", fmt.Sprintf(`["%v"]`, acc6.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
	})

	Convey("test register 4", t, func() {
		s := NewSimulator()
		defer s.Clear()

		s.Head.Number = 0

		createAccountsWithResource(s)
		prepareFakeBase(t, s)
		prepareToken(t, s, acc0)
		prepareNewProducerVote(t, s, acc0)
		initProducer(t, s)

		s.Head.Number = 1
		r, err := s.Call("vote_producer.iost", "applyRegister", fmt.Sprintf(`["%v", "%v", "loc", "url", "netId", true]`, acc6.ID, acc6.KeyPair.ReadablePubkey()), acc6.ID, acc6.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "logInProducer", fmt.Sprintf(`["%v"]`, acc6.ID), acc6.ID, acc6.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "forceUnregister", fmt.Sprintf(`["%v"]`, acc6.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "unregister", fmt.Sprintf(`["%v"]`, acc6.ID), acc6.ID, acc6.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "applyRegister", fmt.Sprintf(`["%v", "%v", "loc", "url", "netId", true]`, acc6.ID, acc6.KeyPair.ReadablePubkey()), acc6.ID, acc6.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "approveRegister", fmt.Sprintf(`["%v"]`, acc6.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
	})

	Convey("test register 5", t, func() {
		s := NewSimulator()
		defer s.Clear()

		s.Head.Number = 0

		createAccountsWithResource(s)
		prepareFakeBase(t, s)
		prepareToken(t, s, acc0)
		prepareNewProducerVote(t, s, acc0)
		initProducer(t, s)

		s.Head.Number = 1
		r, err := s.Call("vote_producer.iost", "applyRegister", fmt.Sprintf(`["%v", "%v", "loc", "url", "netId", true]`, acc6.ID, acc6.KeyPair.ReadablePubkey()), acc6.ID, acc6.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "unregister", fmt.Sprintf(`["%v"]`, acc6.ID), acc6.ID, acc6.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "unregister", fmt.Sprintf(`["%v"]`, acc6.ID), acc6.ID, acc6.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldContainSubstring, "option not exist")
		r, err = s.Call("vote_producer.iost", "applyRegister", fmt.Sprintf(`["%v", "%v", "loc", "url", "netId", true]`, acc6.ID, acc6.KeyPair.ReadablePubkey()), acc6.ID, acc6.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "approveRegister", fmt.Sprintf(`["%v"]`, acc6.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
	})

	Convey("test register 6", t, func() {
		s := NewSimulator()
		defer s.Clear()

		s.Head.Number = 0

		createAccountsWithResource(s)
		prepareFakeBase(t, s)
		prepareToken(t, s, acc0)
		prepareNewProducerVote(t, s, acc0)
		initProducer(t, s)

		s.Head.Number = 1
		r, err := s.Call("vote_producer.iost", "applyRegister", fmt.Sprintf(`["%v", "%v", "loc", "url", "netId", true]`, acc6.ID, acc6.KeyPair.ReadablePubkey()), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "logInProducer", fmt.Sprintf(`["%v"]`, acc6.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "forceUnregister", fmt.Sprintf(`["%v"]`, acc6.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "unregister", fmt.Sprintf(`["%v"]`, acc6.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "applyRegister", fmt.Sprintf(`["%v", "%v", "loc", "url", "netId", true]`, acc6.ID, acc6.KeyPair.ReadablePubkey()), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "approveRegister", fmt.Sprintf(`["%v"]`, acc6.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "logOutProducer", fmt.Sprintf(`["%v"]`, acc6.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "applyUnregister", fmt.Sprintf(`["%v"]`, acc6.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
	})

	Convey("test register 7", t, func() {
		s := NewSimulator()
		defer s.Clear()

		s.Head.Number = 0

		kp, _ := account.NewKeyPair(nil, crypto.Ed25519)
		operator := account.NewInitAccount("operator", kp.ReadablePubkey(), kp.ReadablePubkey())
		s.SetAccount(operator)
		s.SetGas(operator.ID, 1e12)
		s.SetRAM(operator.ID, 1e12)

		createAccountsWithResource(s)
		prepareFakeBase(t, s)
		prepareToken(t, s, acc0)
		prepareNewProducerVote(t, s, acc0)
		initProducer(t, s)

		s.Head.Number = 1
		r, err := s.Call("vote_producer.iost", "applyRegister", fmt.Sprintf(`["%v", "%v", "loc", "url", "netId", true]`, acc6.ID, acc6.KeyPair.ReadablePubkey()), operator.ID, kp)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "logInProducer", fmt.Sprintf(`["%v"]`, acc6.ID), operator.ID, kp)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "forceUnregister", fmt.Sprintf(`["%v"]`, acc6.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "unregister", fmt.Sprintf(`["%v"]`, acc6.ID), operator.ID, kp)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "applyRegister", fmt.Sprintf(`["%v", "%v", "loc", "url", "netId", true]`, acc6.ID, acc6.KeyPair.ReadablePubkey()), operator.ID, kp)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "approveRegister", fmt.Sprintf(`["%v"]`, acc6.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "logOutProducer", fmt.Sprintf(`["%v"]`, acc6.ID), operator.ID, kp)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "applyUnregister", fmt.Sprintf(`["%v"]`, acc6.ID), operator.ID, kp)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
	})
}

func Test_SwitchOff(t *testing.T) {
	t.Skip()
	ilog.Stop()
	Convey("test switch off", t, func() {
		s := NewSimulator()
		defer s.Clear()

		s.Head.Number = 0

		createAccountsWithResource(s)
		prepareFakeBase(t, s)
		prepareToken(t, s, acc0)
		prepareNewProducerVote(t, s, acc0)
		initProducer(t, s)

		s.Head.Number = 1

		r, err := s.Call("vote_producer.iost", "vote", fmt.Sprintf(`["%v", "%v", "%v"]`, acc0.ID, acc1.ID, 3), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", acc1.ID)), ShouldEqual, fmt.Sprintf(`{"votes":"%v","deleted":0,"clearTime":-1}`, 3))

		r, err = s.Call("vote_producer.iost", "unvote", fmt.Sprintf(`["%v", "%v", "%v"]`, acc0.ID, acc1.ID, 1), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", acc1.ID)), ShouldEqual, fmt.Sprintf(`{"votes":"%v","deleted":0,"clearTime":-1}`, 2))

		r, err = s.Call("vote_producer.iost", "switchOff", `[true]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")

		r, err = s.Call("vote_producer.iost", "vote", fmt.Sprintf(`["%v", "%v", "%v"]`, acc0.ID, acc1.ID, 3), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldContainSubstring, "can't vote for now")

		r, err = s.Call("vote_producer.iost", "unvote", fmt.Sprintf(`["%v", "%v", "%v"]`, acc0.ID, acc1.ID, 1), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldContainSubstring, "can't unvote for now")

		r, err = s.Call("vote_producer.iost", "topupVoterBonus", fmt.Sprintf(`["%v", "%v", "%v"]`, acc1.ID, 1, acc0.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldContainSubstring, "can't topup for now")

		r, err = s.Call("vote_producer.iost", "voterWithdraw", fmt.Sprintf(`["%v"]`, acc0.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldContainSubstring, "can't withdraw for now")

		r, err = s.Call("vote_producer.iost", "candidateWithdraw", fmt.Sprintf(`["%v"]`, acc0.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldContainSubstring, "can't withdraw for now")

		r, err = s.Call("vote_producer.iost", "switchOff", `[false]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")

		r, err = s.Call("vote_producer.iost", "vote", fmt.Sprintf(`["%v", "%v", "%v"]`, acc0.ID, acc1.ID, 3), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", acc1.ID)), ShouldEqual, fmt.Sprintf(`{"votes":"%v","deleted":0,"clearTime":-1}`, 5))

		r, err = s.Call("vote_producer.iost", "unvote", fmt.Sprintf(`["%v", "%v", "%v"]`, acc0.ID, acc1.ID, 1), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", acc1.ID)), ShouldEqual, fmt.Sprintf(`{"votes":"%v","deleted":0,"clearTime":-1}`, 4))
	})
}

func Test_Unregister2(t *testing.T) {
	ilog.Stop()
	Convey("test Unregister2", t, func() {
		s := NewSimulator()
		defer s.Clear()

		s.Head.Number = 0

		createAccountsWithResource(s)
		prepareFakeBase(t, s)
		prepareToken(t, s, acc0)
		prepareNewProducerVote(t, s, acc0)
		initProducer(t, s)

		s.Head.Number = 1
		for _, acc := range testAccounts[6:] {
			r, err := s.Call("vote_producer.iost", "applyRegister", fmt.Sprintf(`["%v", "%v", "loc", "url", "netId", true]`, acc.ID, acc.KeyPair.ReadablePubkey()), acc.ID, acc.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			r, err = s.Call("vote_producer.iost", "approveRegister", fmt.Sprintf(`["%v"]`, acc.ID), acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			r, err = s.Call("vote_producer.iost", "logInProducer", fmt.Sprintf(`["%v"]`, acc.ID), acc.ID, acc.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
		}
		// So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-voteInfo", fmt.Sprintf(`%d`, 1))), ShouldEqual, "")
		s.Visitor.SetTokenBalance(global.Token, acc1.ID, 2e18)
		for idx, acc := range testAccounts {
			r, err := s.Call("vote_producer.iost", "vote", fmt.Sprintf(`["%v", "%v", "%v"]`, acc1.ID, acc.ID, (idx+2)*1e7), acc1.ID, acc1.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", acc.ID)), ShouldEqual, fmt.Sprintf(`{"votes":"%d","deleted":0,"clearTime":-1}`, (idx+2)*1e7))
		}

		// do stat
		s.Head.Number = common.VoteInterval
		r, err := s.Call("base.iost", "stat", `[]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		// acc	: score			, votes
		// 0	: 2				, 2
		// 1	: 3				, 3
		// 2	: 4				, 4
		// 3	: 5				, 5
		// 4	: 6 - 0.65		, 6
		// 5	: 7 - 0.65		, 7
		// 6	: 8	- 0.65		, 8
		// 7	: 9 - 0.65		, 9
		// 8	: 10 - 0.65		, 10
		// 9	: 11 - 0.65		, 11
		// 0, 3, 1, 4, 5, 2
		currentList, _ := json.Marshal([]string{acc0.KeyPair.ReadablePubkey(), acc3.KeyPair.ReadablePubkey(), acc1.KeyPair.ReadablePubkey(), acc4.KeyPair.ReadablePubkey(), acc5.KeyPair.ReadablePubkey(), acc2.KeyPair.ReadablePubkey()})
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-currentProducerList")), ShouldEqual, string(currentList))
		// 9, 8, 7, 6, 5, 4
		pendingList, _ := json.Marshal([]string{acc9.KeyPair.ReadablePubkey(), acc8.KeyPair.ReadablePubkey(), acc7.KeyPair.ReadablePubkey(), acc6.KeyPair.ReadablePubkey(), acc5.KeyPair.ReadablePubkey(), acc4.KeyPair.ReadablePubkey()})
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-pendingProducerList")), ShouldEqual, string(pendingList))
		scores := `{"user_9":"103500000.00000000","user_8":"93500000.00000000","user_7":"83500000.00000000","user_6":"73500000.00000000","user_5":"63500000.00000000","user_4":"53500000.00000000","user_3":"50000000","user_2":"40000000","user_1":"30000000","user_0":"20000000"}`
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-producerScores")), ShouldEqual, scores)

		r, err = s.Call("vote_producer.iost", "applyUnregister", fmt.Sprintf(`["%v"]`, acc9.ID), acc9.ID, acc9.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Code, ShouldEqual, tx.Success)

		// do stat
		s.Head.Number += common.VoteInterval
		r, err = s.Call("base.iost", "stat", `[]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		// acc	: score			, votes
		// 0	: 4				, 2
		// 1	: 6				, 3
		// 2	: 8				, 4
		// 3	: 10			, 5
		// 4	: 12 - 1.911	, 6
		// 5	: 14 - 1.911	, 7
		// 6	: 16 - 1.911	, 8
		// 7	: 18 - 1.911	, 9
		// 8	: 20 - 1.911	, 10
		// 9	: 22 - 1.911	, 11
		// 9, 8, 7, 6, 5, 4
		currentList = pendingList
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-currentProducerList")), ShouldEqual, string(currentList))
		// 9, 8, 7, 6, 5, 4
		pendingList, _ = json.Marshal([]string{acc9.KeyPair.ReadablePubkey(), acc8.KeyPair.ReadablePubkey(), acc7.KeyPair.ReadablePubkey(), acc6.KeyPair.ReadablePubkey(), acc5.KeyPair.ReadablePubkey(), acc4.KeyPair.ReadablePubkey()})
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-pendingProducerList")), ShouldEqual, string(pendingList))
		scores = `{"user_9":"200890000.00000000","user_8":"180890000.00000000","user_7":"160890000.00000000","user_6":"140890000.00000000","user_5":"120890000.00000000","user_4":"100890000.00000000","user_3":"100000000","user_2":"80000000","user_1":"60000000","user_0":"40000000"}`
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-producerScores")), ShouldEqual, scores)

		r, err = s.Call("vote_producer.iost", "approveUnregister", fmt.Sprintf(`["%v"]`, acc9.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "unregister", fmt.Sprintf(`["%v"]`, acc9.ID), acc9.ID, acc9.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldContainSubstring, "producer in pending list or in current list, can't unregister")

		// do stat
		s.Head.Number += common.VoteInterval
		r, err = s.Call("base.iost", "stat", `[]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		// acc	: score				, votes
		// 0	: 6					, 2
		// 1	: 9					, 3
		// 2	: 12				, 4
		// 3	: 15 - 1.69383333	, 5
		// 4	: 18 - 3.60483333	, 6
		// 5	: 21 - 3.60483333	, 7
		// 6	: 24 - 3.60483333	, 8
		// 7	: 27 - 3.60483333	, 9
		// 8	: 30 - 3.60483333	, 10
		// 9 X	: X					, 11
		// 9, 8, 7, 6, 5, 4
		currentList = pendingList
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-currentProducerList")), ShouldEqual, string(currentList))
		// 8, 7, 6, 5, 4, 3
		pendingList, _ = json.Marshal([]string{acc8.KeyPair.ReadablePubkey(), acc7.KeyPair.ReadablePubkey(), acc6.KeyPair.ReadablePubkey(), acc5.KeyPair.ReadablePubkey(), acc4.KeyPair.ReadablePubkey(), acc3.KeyPair.ReadablePubkey()})
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-pendingProducerList")), ShouldEqual, string(pendingList))
		scores = `{"user_8":"263951666.66666666","user_7":"233951666.66666666","user_6":"203951666.66666666","user_5":"173951666.66666666","user_4":"143951666.66666666","user_3":"133061666.66666666","user_2":"120000000","user_1":"90000000","user_0":"60000000"}`
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-producerScores")), ShouldEqual, scores)

		// unregister 8
		r, err = s.Call("vote_producer.iost", "applyUnregister", fmt.Sprintf(`["%v"]`, acc8.ID), acc8.ID, acc8.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "approveUnregister", fmt.Sprintf(`["%v"]`, acc8.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "unregister", fmt.Sprintf(`["%v"]`, acc8.ID), acc8.ID, acc8.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldContainSubstring, "producer in pending list or in current list, can't unregister")

		// unregister 3
		r, err = s.Call("vote_producer.iost", "applyUnregister", fmt.Sprintf(`["%v"]`, acc3.ID), acc3.ID, acc3.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "approveUnregister", fmt.Sprintf(`["%v"]`, acc3.ID), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		r, err = s.Call("vote_producer.iost", "unregister", fmt.Sprintf(`["%v"]`, acc3.ID), acc3.ID, acc3.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldContainSubstring, "producer in pending list or in current list, can't unregister")

		// do stat
		s.Head.Number += common.VoteInterval
		r, err = s.Call("base.iost", "stat", `[]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		// acc	: score			, votes
		// 0	: 8					, 2
		// 1	: 12 - 2.02258095	, 3
		// 2	: 16 - 2.02258095	, 4
		// 3 X	: X					, 5
		// 4	: 24 - 5.62741428	, 6
		// 5	: 28 - 5.62741428	, 7
		// 6	: 32 - 5.62741428	, 8
		// 7	: 36 - 5.62741428	, 9
		// 8 X	: X					, 10
		// 9 X	: X					, 11
		// 8, 7, 6, 5, 4, 3
		currentList = pendingList
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-currentProducerList")), ShouldEqual, string(currentList))
		// 7, 6, 5, 4, 2, 1
		pendingList, _ = json.Marshal([]string{acc7.KeyPair.ReadablePubkey(), acc6.KeyPair.ReadablePubkey(), acc5.KeyPair.ReadablePubkey(), acc4.KeyPair.ReadablePubkey(), acc2.KeyPair.ReadablePubkey(), acc1.KeyPair.ReadablePubkey()})
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-pendingProducerList")), ShouldEqual, string(pendingList))
		scores = `{"user_7":"303725857.14285713","user_6":"263725857.14285713","user_5":"223725857.14285713","user_4":"183725857.14285713","user_2":"139774190.47619047","user_1":"99774190.47619047","user_0":"80000000"}`
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-producerScores")), ShouldEqual, scores)

		// force unregister all left except for 2 (or acc2.ID)
		for _, acc := range []*TestAccount{acc0, acc1, acc4, acc5, acc6, acc7} {
			r, err = s.Call("vote_producer.iost", "forceUnregister", fmt.Sprintf(`["%v"]`, acc.ID), acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
		}

		// do stat
		s.Head.Number += common.VoteInterval
		r, err = s.Call("base.iost", "stat", `[]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		// acc	: score				, votes
		// 0 X	: X					, 2
		// 1 W	: X					, 3
		// 2	: 20 - 3.82032285	, 4
		// 3 X	: X					, 5
		// 4 W	: X					, 6
		// 5 W	: X					, 7
		// 6 W	: X					, 8
		// 7 W	: X					, 9
		// 8 X	: X					, 10
		// 9 X	: X					, 11
		// 7, 6, 5, 4, 2, 1
		currentList = pendingList
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-currentProducerList")), ShouldEqual, string(currentList))
		// 2, 7, 6, 5, 4, 1
		pendingList, _ = json.Marshal([]string{acc2.KeyPair.ReadablePubkey(), acc7.KeyPair.ReadablePubkey(), acc6.KeyPair.ReadablePubkey(), acc5.KeyPair.ReadablePubkey(), acc4.KeyPair.ReadablePubkey(), acc1.KeyPair.ReadablePubkey()})
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-pendingProducerList")), ShouldEqual, string(pendingList))
		scores = `{"user_2":"161796771.42857142"}`
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-producerScores")), ShouldEqual, scores)

		for _, acc := range []*TestAccount{acc3, acc4, acc8, acc9} {
			r, err = s.Call("vote_producer.iost", "approveRegister", fmt.Sprintf(`["%v"]`, acc.ID), acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			r, err = s.Call("vote_producer.iost", "logInProducer", fmt.Sprintf(`["%v"]`, acc.ID), acc.ID, acc.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
		}

		// do stat
		s.Head.Number += common.VoteInterval
		r, err = s.Call("base.iost", "stat", `[]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		// acc	: score				, votes
		// 0 X	: X					, 2
		// 1 W	: X					, 3
		// 2	: 24 - 4.86391639	, 4
		// 3 	: 5 - 1.04359354	, 5
		// 4 	: 6 - 1.04359354	, 6
		// 5 W	: X					, 7
		// 6 W	: X					, 8
		// 7 W	: X					, 9
		// 8 	: 10 - 1.04359354	, 10
		// 9 	: 11 - 1.04359354	, 11
		// 2, 7, 6, 5, 4, 1
		currentList = pendingList
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-currentProducerList")), ShouldEqual, string(currentList))
		// 2, 9, 8, 4, 3, 7
		pendingList, _ = json.Marshal([]string{acc2.KeyPair.ReadablePubkey(), acc9.KeyPair.ReadablePubkey(), acc8.KeyPair.ReadablePubkey(), acc4.KeyPair.ReadablePubkey(), acc3.KeyPair.ReadablePubkey(), acc7.KeyPair.ReadablePubkey()})
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-pendingProducerList")), ShouldEqual, string(pendingList))
		scores = `{"user_9":"99564064.57142857","user_8":"89564064.57142857","user_4":"49564064.57142857","user_3":"39564064.57142857","user_2":"191360835.99999999"}`
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-producerScores")), ShouldEqual, scores)
	})
}

func Test_TakeTurns(t *testing.T) {
	ilog.Stop()
	Convey("test take turns", t, func() {
		s := NewSimulator()
		defer s.Clear()

		s.Head.Number = 0

		createAccountsWithResource(s)
		prepareFakeBase(t, s)
		prepareToken(t, s, acc0)
		prepareNewProducerVote(t, s, acc0)
		initProducer(t, s)

		s.Head.Number = 1
		for _, acc := range testAccounts[6:] {
			r, err := s.Call("vote_producer.iost", "applyRegister", fmt.Sprintf(`["%v", "%v", "loc", "url", "netId", true]`, acc.ID, acc.KeyPair.ReadablePubkey()), acc.ID, acc.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			r, err = s.Call("vote_producer.iost", "approveRegister", fmt.Sprintf(`["%v"]`, acc.ID), acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			r, err = s.Call("vote_producer.iost", "logInProducer", fmt.Sprintf(`["%v"]`, acc.ID), acc.ID, acc.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
		}
		for idx, acc := range testAccounts {
			r, err := s.Call("vote_producer.iost", "vote", fmt.Sprintf(`["%v", "%v", "%v"]`, acc0.ID, acc.ID, (idx+2)*1e7), acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", acc.ID)), ShouldEqual, fmt.Sprintf(`{"votes":"%d","deleted":0,"clearTime":-1}`, (idx+2)*1e7))
		}

		// do stat
		s.Head.Number = common.VoteInterval
		r, err := s.Call("base.iost", "stat", `[]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		// acc	: score			, votes
		// 0	: 2				, 2
		// 1	: 3				, 3
		// 2	: 4				, 4
		// 3	: 5				, 5
		// 4	: 6 - 0.65		, 6
		// 5	: 7 - 0.65		, 7
		// 6	: 8 - 0.65		, 8
		// 7	: 9 - 0.65		, 9
		// 8	: 10 - 0.65		, 10
		// 9	: 11 - 0.65		, 11
		// 0, 3, 1, 4, 5, 2
		currentList, _ := json.Marshal([]string{acc0.KeyPair.ReadablePubkey(), acc3.KeyPair.ReadablePubkey(), acc1.KeyPair.ReadablePubkey(), acc4.KeyPair.ReadablePubkey(), acc5.KeyPair.ReadablePubkey(), acc2.KeyPair.ReadablePubkey()})
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-currentProducerList")), ShouldEqual, string(currentList))
		// 9, 8, 7, 6, 5, 4
		pendingList, _ := json.Marshal([]string{acc9.KeyPair.ReadablePubkey(), acc8.KeyPair.ReadablePubkey(), acc7.KeyPair.ReadablePubkey(), acc6.KeyPair.ReadablePubkey(), acc5.KeyPair.ReadablePubkey(), acc4.KeyPair.ReadablePubkey()})
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-pendingProducerList")), ShouldEqual, string(pendingList))
		scores := `{"user_9":"103500000.00000000","user_8":"93500000.00000000","user_7":"83500000.00000000","user_6":"73500000.00000000","user_5":"63500000.00000000","user_4":"53500000.00000000","user_3":"50000000","user_2":"40000000","user_1":"30000000","user_0":"20000000"}`
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-producerScores")), ShouldEqual, scores)

		for i := 7; i < 10; i++ {
			acc := testAccounts[i]
			r, err := s.Call("vote_producer.iost", "unvote", fmt.Sprintf(`["%v", "%v", "%v"]`, acc0.ID, acc.ID, (i+2)*1e7), acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", acc.ID)), ShouldEqual, `{"votes":"0","deleted":0,"clearTime":-1}`)
		}

		// do stat
		s.Head.Number += common.VoteInterval
		r, err = s.Call("base.iost", "stat", `[]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		// acc	: score			, votes
		// 0	: 4				, 2
		// 1	: 6 - 0.97214	, 3
		// 2	: 8 - 0.97214	, 4
		// 3	: 10 - 0.97214	, 5
		// 4	: 12 - 1.62214	, 6
		// 5	: 14 - 1.62214	, 7
		// 6	: 16 - 1.62214	, 8
		// 7	: 0				, 0
		// 8	: 0				, 0
		// 9	: 0				, 0
		// 9, 8, 7, 6, 5, 4
		currentList = pendingList
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-currentProducerList")), ShouldEqual, string(currentList))
		// 6, 5, 4, 3, 2, 1
		pendingList, _ = json.Marshal([]string{acc6.KeyPair.ReadablePubkey(), acc5.KeyPair.ReadablePubkey(), acc4.KeyPair.ReadablePubkey(), acc3.KeyPair.ReadablePubkey(), acc2.KeyPair.ReadablePubkey(), acc1.KeyPair.ReadablePubkey()})
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-pendingProducerList")), ShouldEqual, string(pendingList))
		scores = `{"user_6":"143778571.42857142","user_5":"123778571.42857142","user_4":"103778571.42857142","user_3":"90278571.42857142","user_2":"70278571.42857142","user_1":"50278571.42857142","user_0":"40000000"}`
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-producerScores")), ShouldEqual, scores)
	})
}

func Test_KickOut(t *testing.T) {
	ilog.Stop()
	Convey("test kick out", t, func() {
		s := NewSimulator()
		defer s.Clear()

		s.Head.Number = 0

		createAccountsWithResource(s)
		prepareFakeBase(t, s)
		prepareToken(t, s, acc0)
		prepareNewProducerVote(t, s, acc0)
		initProducer(t, s)

		s.Head.Number = 1
		for _, acc := range testAccounts[6:] {
			r, err := s.Call("vote_producer.iost", "applyRegister", fmt.Sprintf(`["%v", "%v", "loc", "url", "netId", true]`, acc.ID, acc.KeyPair.ReadablePubkey()), acc.ID, acc.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			r, err = s.Call("vote_producer.iost", "approveRegister", fmt.Sprintf(`["%v"]`, acc.ID), acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			r, err = s.Call("vote_producer.iost", "logInProducer", fmt.Sprintf(`["%v"]`, acc.ID), acc.ID, acc.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
		}
		for idx, acc := range testAccounts {
			r, err := s.Call("vote_producer.iost", "vote", fmt.Sprintf(`["%v", "%v", "%v"]`, acc0.ID, acc.ID, (idx+2)*1e7), acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", acc.ID)), ShouldEqual, fmt.Sprintf(`{"votes":"%d","deleted":0,"clearTime":-1}`, (idx+2)*1e7))
		}

		// do stat
		s.Head.Number = common.VoteInterval
		r, err := s.Call("base.iost", "stat", `[]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		// acc	: score			, votes
		// 0	: 2				, 2
		// 1	: 3				, 3
		// 2	: 4				, 4
		// 3	: 5				, 5
		// 4	: 6 - 0.65		, 6
		// 5	: 7 - 0.65		, 7
		// 6	: 8 - 0.65		, 8
		// 7	: 9 - 0.65		, 9
		// 8	: 10 - 0.65		, 10
		// 9	: 11 - 0.65		, 11
		// 0, 3, 1, 4, 5, 2
		currentList, _ := json.Marshal([]string{acc0.KeyPair.ReadablePubkey(), acc3.KeyPair.ReadablePubkey(), acc1.KeyPair.ReadablePubkey(), acc4.KeyPair.ReadablePubkey(), acc5.KeyPair.ReadablePubkey(), acc2.KeyPair.ReadablePubkey()})
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-currentProducerList")), ShouldEqual, string(currentList))
		// 9, 8, 7, 6, 5, 4
		pendingList, _ := json.Marshal([]string{acc9.KeyPair.ReadablePubkey(), acc8.KeyPair.ReadablePubkey(), acc7.KeyPair.ReadablePubkey(), acc6.KeyPair.ReadablePubkey(), acc5.KeyPair.ReadablePubkey(), acc4.KeyPair.ReadablePubkey()})
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-pendingProducerList")), ShouldEqual, string(pendingList))
		scores := `{"user_9":"103500000.00000000","user_8":"93500000.00000000","user_7":"83500000.00000000","user_6":"73500000.00000000","user_5":"63500000.00000000","user_4":"53500000.00000000","user_3":"50000000","user_2":"40000000","user_1":"30000000","user_0":"20000000"}`
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-producerScores")), ShouldEqual, scores)

		lst := []string{}
		for idx, acc := range testAccounts {
			if idx == 4 || idx == 5 {
				continue
			}
			lst = append(lst, acc.KeyPair.ReadablePubkey())
		}
		jsonStr, err := json.Marshal(lst)
		So(err, ShouldBeNil)
		r, err = s.Call("base.iost", "initWitness", fmt.Sprintf(`[%v]`, string(jsonStr)), acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")

		// do stat
		s.Head.Number += common.VoteInterval
		r, err = s.Call("base.iost", "stat", `[]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		// acc	: score			, votes
		// 0	: 4				, 2
		// 1	: 6				, 3
		// 2	: 8 - 1.261		, 4
		// 3	: 10 - 1.261	, 5
		// 4	: (12-0.65)/2	, 6
		// 5	: (14-0.65)/2	, 7
		// 6	: 16 - 1.911	, 8
		// 7	: 18 - 1.911	, 9
		// 8	: 20 - 1.911	, 10
		// 9	: 22 - 1.911	, 11
		// 9, 8, 7, 6, 5, 4
		currentList = pendingList
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-currentProducerList")), ShouldEqual, string(currentList))
		// 9, 8, 7, 6, 3, 2
		pendingList, _ = json.Marshal([]string{acc9.KeyPair.ReadablePubkey(), acc8.KeyPair.ReadablePubkey(), acc7.KeyPair.ReadablePubkey(), acc6.KeyPair.ReadablePubkey(), acc3.KeyPair.ReadablePubkey(), acc2.KeyPair.ReadablePubkey()})
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-pendingProducerList")), ShouldEqual, string(pendingList))
		scores = `{"user_9":"200890000.00000000","user_8":"180890000.00000000","user_7":"160890000.00000000","user_6":"140890000.00000000","user_5":"66750000.00000000","user_4":"56750000.00000000","user_3":"87390000.00000000","user_2":"67390000.00000000","user_1":"60000000","user_0":"40000000"}`
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-producerScores")), ShouldEqual, scores)
	})
}

func Test_UpdatePubkey(t *testing.T) {
	ilog.Stop()
	Convey("test update pubkey", t, func() {
		s := NewSimulator()
		defer s.Clear()

		s.Head.Number = 0

		createAccountsWithResource(s)
		prepareFakeBase(t, s)
		prepareToken(t, s, acc0)
		prepareNewProducerVote(t, s, acc0)
		initProducer(t, s)

		s.Head.Number = 1
		for _, acc := range testAccounts[6:9] {
			r, err := s.Call("vote_producer.iost", "applyRegister", fmt.Sprintf(`["%v", "%v", "loc", "url", "netId", true]`, acc.ID, acc.KeyPair.ReadablePubkey()), acc.ID, acc.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			r, err = s.Call("vote_producer.iost", "approveRegister", fmt.Sprintf(`["%v"]`, acc.ID), acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			r, err = s.Call("vote_producer.iost", "logInProducer", fmt.Sprintf(`["%v"]`, acc.ID), acc.ID, acc.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
		}
		for idx, acc := range testAccounts[:9] {
			r, err := s.Call("vote_producer.iost", "vote", fmt.Sprintf(`["%v", "%v", "%v"]`, acc0.ID, acc.ID, (idx+2)*1e7), acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", acc.ID)), ShouldEqual, fmt.Sprintf(`{"votes":"%d","deleted":0,"clearTime":-1}`, (idx+2)*1e7))
		}

		r, err := s.Call("vote_producer.iost", "updateProducer", fmt.Sprintf(`["%v","%v","loc","url","netId"]`, acc8.ID, acc9.KeyPair.ReadablePubkey()), acc8.ID, acc8.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")

		// do stat
		s.Head.Number = common.VoteInterval
		r, err = s.Call("base.iost", "stat", `[]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		// acc	: score			, votes
		// 0	: 2				, 2
		// 1	: 3				, 3
		// 2	: 4				, 4
		// 3	: 5 - 0.6		, 5
		// 4	: 6 - 0.6		, 6
		// 5	: 7 - 0.6		, 7
		// 6	: 8 - 0.6		, 8
		// 7	: 9 - 0.6		, 9
		// 8	: 10 - 0.6		, 10
		// 0, 3, 1, 4, 5, 2
		currentList, _ := json.Marshal([]string{acc0.KeyPair.ReadablePubkey(), acc3.KeyPair.ReadablePubkey(), acc1.KeyPair.ReadablePubkey(), acc4.KeyPair.ReadablePubkey(), acc5.KeyPair.ReadablePubkey(), acc2.KeyPair.ReadablePubkey()})
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-currentProducerList")), ShouldEqual, string(currentList))
		// 8(9), 7, 6, 5, 4, 3
		pendingList, _ := json.Marshal([]string{acc9.KeyPair.ReadablePubkey(), acc7.KeyPair.ReadablePubkey(), acc6.KeyPair.ReadablePubkey(), acc5.KeyPair.ReadablePubkey(), acc4.KeyPair.ReadablePubkey(), acc3.KeyPair.ReadablePubkey()})
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-pendingProducerList")), ShouldEqual, string(pendingList))
		scores := `{"user_8":"94000000.00000000","user_7":"84000000.00000000","user_6":"74000000.00000000","user_5":"64000000.00000000","user_4":"54000000.00000000","user_3":"44000000.00000000","user_2":"40000000","user_1":"30000000","user_0":"20000000"}`
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-producerScores")), ShouldEqual, scores)

		r, err = s.Call("vote_producer.iost", "updateProducer", fmt.Sprintf(`["%v","%v","loc","url","netId"]`, acc8.ID, acc8.KeyPair.ReadablePubkey()), acc8.ID, acc8.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldContainSubstring, "account in producerList, can't change pubkey")
	})
}

func Test_UnvoteCommon(t *testing.T) {
	ilog.Stop()
	Convey("test unvote common", t, func() {
		s := NewSimulator()
		defer s.Clear()

		s.Head.Number = 0

		createAccountsWithResource(s)
		prepareFakeBase(t, s)
		prepareToken(t, s, acc0)
		prepareNewProducerVote(t, s, acc0)
		initProducer(t, s)

		s.Head.Number = 1
		s.Visitor.SetTokenBalance(global.Token, acc1.ID, 1e15)
		for idx, acc := range testAccounts[:6] {
			r, err := s.Call("vote_producer.iost", "vote", fmt.Sprintf(`["%v", "%v", "%v"]`, acc1.ID, acc.ID, idx+2), acc1.ID, acc1.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", acc.ID)), ShouldEqual, fmt.Sprintf(`{"votes":"%d","deleted":0,"clearTime":-1}`, idx+2))
			r, err = s.Call("vote.iost", "unvote", fmt.Sprintf(`["1","%v", "%v", "%v"]`, acc1.ID, acc.ID, idx+2), acc1.ID, acc1.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldContainSubstring, "require auth failed")
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", acc.ID)), ShouldEqual, fmt.Sprintf(`{"votes":"%d","deleted":0,"clearTime":-1}`, idx+2))
		}
	})
}

func Test_LogOutInPending(t *testing.T) {
	ilog.Stop()
	Convey("test log out in pending", t, func() {
		s := NewSimulator()
		defer s.Clear()

		s.Head.Number = 0

		createAccountsWithResource(s)
		prepareFakeBase(t, s)
		prepareToken(t, s, acc0)
		prepareNewProducerVote(t, s, acc0)
		initProducer(t, s)

		s.Head.Number = 1
		for _, acc := range testAccounts[6:] {
			r, err := s.Call("vote_producer.iost", "applyRegister", fmt.Sprintf(`["%v", "%v", "loc", "url", "netId", true]`, acc.ID, acc.KeyPair.ReadablePubkey()), acc.ID, acc.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			r, err = s.Call("vote_producer.iost", "approveRegister", fmt.Sprintf(`["%v"]`, acc.ID), acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			r, err = s.Call("vote_producer.iost", "logInProducer", fmt.Sprintf(`["%v"]`, acc.ID), acc.ID, acc.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
		}
		for idx, acc := range testAccounts {
			r, err := s.Call("vote_producer.iost", "vote", fmt.Sprintf(`["%v", "%v", "%v"]`, acc0.ID, acc.ID, (idx+2)*1e7), acc0.ID, acc0.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			So(database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", acc.ID)), ShouldEqual, fmt.Sprintf(`{"votes":"%d","deleted":0,"clearTime":-1}`, (idx+2)*1e7))
		}

		// do stat
		s.Head.Number = common.VoteInterval
		r, err := s.Call("base.iost", "stat", `[]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		// acc	: score			, votes
		// 0	: 2				, 2
		// 1	: 3				, 3
		// 2	: 4				, 4
		// 3	: 5				, 5
		// 4	: 6 - 0.65		, 6
		// 5	: 7 - 0.65		, 7
		// 6	: 8 - 0.65		, 8
		// 7	: 9 - 0.65		, 9
		// 8	: 10 - 0.65		, 10
		// 9	: 11 - 0.65		, 11
		// 0, 3, 1, 4, 5, 2
		currentList, _ := json.Marshal([]string{acc0.KeyPair.ReadablePubkey(), acc3.KeyPair.ReadablePubkey(), acc1.KeyPair.ReadablePubkey(), acc4.KeyPair.ReadablePubkey(), acc5.KeyPair.ReadablePubkey(), acc2.KeyPair.ReadablePubkey()})
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-currentProducerList")), ShouldEqual, string(currentList))
		// 9, 8, 7, 6, 5, 4
		pendingList, _ := json.Marshal([]string{acc9.KeyPair.ReadablePubkey(), acc8.KeyPair.ReadablePubkey(), acc7.KeyPair.ReadablePubkey(), acc6.KeyPair.ReadablePubkey(), acc5.KeyPair.ReadablePubkey(), acc4.KeyPair.ReadablePubkey()})
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-pendingProducerList")), ShouldEqual, string(pendingList))
		scores := `{"user_9":"103500000.00000000","user_8":"93500000.00000000","user_7":"83500000.00000000","user_6":"73500000.00000000","user_5":"63500000.00000000","user_4":"53500000.00000000","user_3":"50000000","user_2":"40000000","user_1":"30000000","user_0":"20000000"}`
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-producerScores")), ShouldEqual, scores)

		r, err = s.Call("vote_producer.iost", "logOutProducer", fmt.Sprintf(`["%v"]`, acc9.ID), acc9.ID, acc9.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")

		// do stat
		s.Head.Number += common.VoteInterval
		r, err = s.Call("base.iost", "stat", `[]`, acc0.ID, acc0.KeyPair)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")
		// acc	: score			, votes
		// 0	: 4				, 2
		// 1	: 6				, 3
		// 2	: 8				, 4
		// 3	: 10 - 1.151	, 5
		// 4	: 12 - 1.801	, 6
		// 5	: 14 - 1.801	, 7
		// 6	: 16 - 1.801	, 8
		// 7	: 18 - 1.801	, 9
		// 8	: 20 - 1.801	, 10
		// 9	: 11 - 0.65		, 11
		// 9, 8, 7, 6, 5, 4
		currentList = pendingList
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-currentProducerList")), ShouldEqual, string(currentList))
		// 8, 7, 6, 5, 4, 3
		pendingList, _ = json.Marshal([]string{acc8.KeyPair.ReadablePubkey(), acc7.KeyPair.ReadablePubkey(), acc6.KeyPair.ReadablePubkey(), acc5.KeyPair.ReadablePubkey(), acc4.KeyPair.ReadablePubkey(), acc3.KeyPair.ReadablePubkey()})
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-pendingProducerList")), ShouldEqual, string(pendingList))
		scores = `{"user_9":"103500000","user_8":"181990000.00000000","user_7":"161990000.00000000","user_6":"141990000.00000000","user_5":"121990000.00000000","user_4":"101990000.00000000","user_3":"88490000.00000000","user_2":"80000000","user_1":"60000000","user_0":"40000000"}`
		So(database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-producerScores")), ShouldEqual, scores)
	})
}
