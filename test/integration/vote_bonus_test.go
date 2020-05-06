package integration

import (
	"fmt"
	"github.com/iost-official/go-iost/core/global"
	"testing"

	"github.com/iost-official/go-iost/core/tx"
	"github.com/iost-official/go-iost/ilog"
	. "github.com/iost-official/go-iost/verifier"
	"github.com/iost-official/go-iost/vm/database"
	"github.com/stretchr/testify/assert"
)

func Test_VoteBonus(t *testing.T) {
	ilog.Stop()
	s := NewSimulator()
	defer s.Clear()

	s.Head.Number = 0

	createAccountsWithResource(s)
	prepareFakeBase(t, s)
	prepareIssue(s, acc0)
	prepareNewProducerVote(t, s, acc0)
	initProducer(t, s)

	// deploy bonus.iost
	r, err := setNonNativeContract(s, "bonus.iost", "bonus.js", ContractPath)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)

	s.Head.Number = 1
	for _, acc := range testAccounts[6:] {
		r, err := s.Call("vote_producer.iost", "applyRegister", fmt.Sprintf(`["%v", "%v", "loc", "url", "netId", true]`, acc.ID, acc.KeyPair.ReadablePubkey()), acc.ID, acc.KeyPair)
		assert.Nil(t, err)
		assert.Equal(t, tx.Success, r.Status.Code)
		r, err = s.Call("vote_producer.iost", "approveRegister", fmt.Sprintf(`["%v"]`, acc.ID), acc0.ID, acc0.KeyPair)
		assert.Nil(t, err)
		assert.Equal(t, tx.Success, r.Status.Code)
		r, err = s.Call("vote_producer.iost", "logInProducer", fmt.Sprintf(`["%v"]`, acc.ID), acc.ID, acc.KeyPair)
		assert.Nil(t, err)
		assert.Equal(t, tx.Success, r.Status.Code)
	}
	s.Visitor.SetTokenBalance(global.Token, acc2.ID, 1e17)
	for idx, acc := range testAccounts {
		voter := acc0
		if idx > 0 {
			r, err := s.Call("vote_producer.iost", "vote", fmt.Sprintf(`["%v", "%v", "%v"]`, voter.ID, acc.ID, idx*2e7), voter.ID, voter.KeyPair)
			assert.Nil(t, err)
			assert.Empty(t, r.Status.Message)
			assert.Equal(t, fmt.Sprintf(`{"votes":"%d","deleted":0,"clearTime":-1}`, idx*2e7), database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", acc.ID)))
		}
		voter = acc2
		r, err := s.Call("vote_producer.iost", "vote", fmt.Sprintf(`["%v", "%v", "%v"]`, voter.ID, acc.ID, 2e7), voter.ID, voter.KeyPair)
		assert.Nil(t, err)
		assert.Empty(t, r.Status.Message)
		assert.Equal(t, fmt.Sprintf(`{"votes":"%d","deleted":0,"clearTime":-1}`, (idx+1)*2e7), database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", acc.ID)))
	}

	for idx, acc := range testAccounts {
		s.Head.Witness = acc.KeyPair.ReadablePubkey()
		for i := 0; i <= idx; i++ {
			s.Head.Number++
			r, err := s.Call("base.iost", "issueContribute", fmt.Sprintf(`[{"parent":["%v","%v"]}]`, acc.ID, 1), acc.ID, acc.KeyPair)
			assert.Nil(t, err)
			assert.Empty(t, r.Status.Message)
		}
		assert.Equal(t, int64(328513441*(idx+1)), s.Visitor.TokenBalance("contribute", acc.ID))
	}
	assert.Equal(t, `{"user_1":["20000000",1,"0"],"user_2":["40000000",1,"0"],"user_3":["60000000",1,"0"],"user_4":["80000000",1,"0"],"user_5":["100000000",1,"0"],"user_6":["120000000",1,"0"],"user_7":["140000000",1,"0"],"user_8":["160000000",1,"0"],"user_9":["180000000",1,"0"]}`, database.MustUnmarshal(s.Visitor.MGet("vote.iost-u_1", acc0.ID)))
	assert.Equal(t, `{"user_0":["20000000",1,"0"],"user_1":["20000000",1,"0"],"user_2":["20000000",1,"0"],"user_3":["20000000",1,"0"],"user_4":["20000000",1,"0"],"user_5":["20000000",1,"0"],"user_6":["20000000",1,"0"],"user_7":["20000000",1,"0"],"user_8":["20000000",1,"0"],"user_9":["20000000",1,"0"]}`, database.MustUnmarshal(s.Visitor.MGet("vote.iost-u_1", acc2.ID)))
	s.Head.Time += 5073358980
	r, err = s.Call("issue.iost", "issueIOST", `[]`, acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, int64(103333333410), s.Visitor.TokenBalance(global.Token, "vote_producer.iost"))
	assert.Equal(t, int64(3333333410), s.Visitor.TokenBalance(global.Token, "bonus.iost"))

	for i := 0; i < 10; i++ {
		s.Visitor.SetTokenBalance(global.Token, testAccounts[i].ID, 0)
	}

	// 0. normal withdraw
	r, err = s.Call("bonus.iost", "exchangeIOST", fmt.Sprintf(`["%s","0"]`, acc1.ID), acc1.ID, acc1.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, int64(0), s.Visitor.TokenBalance("contribute", acc1.ID))
	assert.Equal(t, int64(657026882), s.Visitor.TokenBalance(global.Token, acc1.ID))
	assert.Equal(t, int64(103333333410), s.Visitor.TokenBalance(global.Token, "vote_producer.iost"))

	s.Head.Time += 86400 * 1e9
	r, err = s.Call("bonus.iost", "exchangeIOST", fmt.Sprintf(`["%s","%s"]`, acc1.ID, "0.00000001"), acc1.ID, acc1.KeyPair)
	assert.Nil(t, err)
	assert.Contains(t, r.Status.Message, "invalid amount: negative or greater than contribute")

	// withdraw by admin
	r, err = s.Call("vote_producer.iost", "candidateWithdraw", fmt.Sprintf(`["%s"]`, acc1.ID), acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, int64(657026882+60606062), s.Visitor.TokenBalance(global.Token, acc1.ID))                 // 60606062 = (3333333410*(2/55))/2
	assert.Equal(t, int64(103333333410-60606062), s.Visitor.TokenBalance(global.Token, "vote_producer.iost")) // half to voterBonus

	r, err = s.Call("vote_producer.iost", "candidateWithdraw", fmt.Sprintf(`["%s"]`, acc1.ID), acc1.ID, acc1.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, int64(657026882+60606062), s.Visitor.TokenBalance(global.Token, acc1.ID)) // not change
	assert.Equal(t, int64(103333333410-60606062), s.Visitor.TokenBalance(global.Token, "vote_producer.iost"))

	r, err = s.Call("vote_producer.iost", "voterWithdraw", fmt.Sprintf(`["%s"]`, acc0.ID), acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, int64(30303031), s.Visitor.TokenBalance(global.Token, acc0.ID))
	assert.Equal(t, int64(103242424317), s.Visitor.TokenBalance(global.Token, "vote_producer.iost"))

	r, err = s.Call("vote_producer.iost", "voterWithdraw", fmt.Sprintf(`["%s"]`, acc0.ID), acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, int64(30303031), s.Visitor.TokenBalance(global.Token, acc0.ID))
	assert.Equal(t, int64(103242424317), s.Visitor.TokenBalance(global.Token, "vote_producer.iost"))

	// 1. unregistered withdraw
	r, err = s.Call("vote_producer.iost", "forceUnregister", fmt.Sprintf(`["%v"]`, acc3.ID), acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	r, err = s.Call("vote_producer.iost", "unregister", fmt.Sprintf(`["%v"]`, acc3.ID), acc3.ID, acc3.KeyPair)
	assert.Nil(t, err)
	assert.Contains(t, r.Status.Message, "producer in pending list or in current list, can't unregister")

	r, err = s.Call("bonus.iost", "exchangeIOST", fmt.Sprintf(`["%s","0"]`, acc3.ID), acc3.ID, acc3.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, int64(0), s.Visitor.TokenBalance("contribute", acc3.ID))
	assert.Equal(t, int64(1314053764), s.Visitor.TokenBalance(global.Token, acc3.ID))
	assert.Equal(t, int64(103242424317), s.Visitor.TokenBalance(global.Token, "vote_producer.iost"))

	s.Head.Time += 86400 * 1e9
	r, err = s.Call("bonus.iost", "exchangeIOST", fmt.Sprintf(`["%s","%s"]`, acc3.ID, "0.00000001"), acc3.ID, acc3.KeyPair)
	assert.Nil(t, err)
	assert.Contains(t, r.Status.Message, "invalid amount: negative or greater than contribute")

	r, err = s.Call("vote_producer.iost", "candidateWithdraw", fmt.Sprintf(`["%s"]`, acc3.ID), acc3.ID, acc3.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, int64(1314053764+121212124), s.Visitor.TokenBalance(global.Token, acc3.ID))                // 121212124 = (3333333410*(4/55))/2
	assert.Equal(t, int64(103242424317-121212124), s.Visitor.TokenBalance(global.Token, "vote_producer.iost")) // =103121212193. half to voterBonus

	r, err = s.Call("vote_producer.iost", "candidateWithdraw", fmt.Sprintf(`["%s"]`, acc3.ID), acc3.ID, acc3.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, int64(1314053764+121212124), s.Visitor.TokenBalance(global.Token, acc3.ID)) // not change
	assert.Equal(t, int64(103242424317-121212124), s.Visitor.TokenBalance(global.Token, "vote_producer.iost"))

	r, err = s.Call("vote_producer.iost", "voterWithdraw", fmt.Sprintf(`["%s"]`, acc0.ID), acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, int64(30303031+90909093), s.Visitor.TokenBalance(global.Token, acc0.ID))                  // 121212124
	assert.Equal(t, int64(103121212193-90909093), s.Visitor.TokenBalance(global.Token, "vote_producer.iost")) // 103030303100

	r, err = s.Call("vote_producer.iost", "voterWithdraw", fmt.Sprintf(`["%s"]`, acc0.ID), acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, int64(30303031+90909093), s.Visitor.TokenBalance(global.Token, acc0.ID))
	assert.Equal(t, int64(103121212193-90909093), s.Visitor.TokenBalance(global.Token, "vote_producer.iost"))
}

func Test_PartnerBonus(t *testing.T) {
	ilog.Stop()
	s := NewSimulator()
	defer s.Clear()

	s.Head.Number = 0

	createAccountsWithResource(s)
	prepareFakeBase(t, s)
	prepareIssue(s, acc0)
	prepareNewProducerVote(t, s, acc0)
	initProducer(t, s)

	// deploy bonus.iost
	r, err := setNonNativeContract(s, "bonus.iost", "bonus.js", ContractPath)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)

	s.Head.Number = 1
	for _, acc := range testAccounts[6:] {
		r, err := s.Call("vote_producer.iost", "applyRegister", fmt.Sprintf(`["%v", "%v", "loc", "url", "netId", false]`, acc.ID, acc.KeyPair.ReadablePubkey()), acc.ID, acc.KeyPair)
		assert.Nil(t, err)
		assert.Equal(t, tx.Success, r.Status.Code)
		r, err = s.Call("vote_producer.iost", "approveRegister", fmt.Sprintf(`["%v"]`, acc.ID), acc0.ID, acc0.KeyPair)
		assert.Nil(t, err)
		assert.Equal(t, tx.Success, r.Status.Code)
		r, err = s.Call("vote_producer.iost", "logInProducer", fmt.Sprintf(`["%v"]`, acc.ID), acc.ID, acc.KeyPair)
		assert.Nil(t, err)
		assert.Equal(t, tx.Success, r.Status.Code)
	}
	s.Visitor.SetTokenBalance(global.Token, acc2.ID, 1e17)
	for idx, acc := range testAccounts {
		voter := acc0
		if idx > 0 {
			r, err := s.Call("vote_producer.iost", "vote", fmt.Sprintf(`["%v", "%v", "%v"]`, voter.ID, acc.ID, idx*2e7), voter.ID, voter.KeyPair)
			assert.Nil(t, err)
			assert.Empty(t, r.Status.Message)
			assert.Equal(t, fmt.Sprintf(`{"votes":"%d","deleted":0,"clearTime":-1}`, idx*2e7), database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", acc.ID)))
		}
		voter = acc2
		r, err := s.Call("vote_producer.iost", "vote", fmt.Sprintf(`["%v", "%v", "%v"]`, voter.ID, acc.ID, 2e7), voter.ID, voter.KeyPair)
		assert.Nil(t, err)
		assert.Empty(t, r.Status.Message)
		assert.Equal(t, fmt.Sprintf(`{"votes":"%d","deleted":0,"clearTime":-1}`, (idx+1)*2e7), database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", acc.ID)))
	}

	assert.Equal(t, `{"user_1":["20000000",1,"0"],"user_2":["40000000",1,"0"],"user_3":["60000000",1,"0"],"user_4":["80000000",1,"0"],"user_5":["100000000",1,"0"],"user_6":["120000000",1,"0"],"user_7":["140000000",1,"0"],"user_8":["160000000",1,"0"],"user_9":["180000000",1,"0"]}`, database.MustUnmarshal(s.Visitor.MGet("vote.iost-u_1", acc0.ID)))
	assert.Equal(t, `{"user_0":["20000000",1,"0"],"user_1":["20000000",1,"0"],"user_2":["20000000",1,"0"],"user_3":["20000000",1,"0"],"user_4":["20000000",1,"0"],"user_5":["20000000",1,"0"],"user_6":["20000000",1,"0"],"user_7":["20000000",1,"0"],"user_8":["20000000",1,"0"],"user_9":["20000000",1,"0"]}`, database.MustUnmarshal(s.Visitor.MGet("vote.iost-u_1", acc2.ID)))
	s.Head.Time += 5073358980
	r, err = s.Call("issue.iost", "issueIOST", `[]`, acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, int64(103333333410), s.Visitor.TokenBalance(global.Token, "vote_producer.iost"))
	assert.Equal(t, int64(3333333410), s.Visitor.TokenBalance(global.Token, "bonus.iost"))

	for i := 0; i < 10; i++ {
		s.Visitor.SetTokenBalance(global.Token, testAccounts[i].ID, 0)
	}

	// 0. normal withdraw
	r, err = s.Call("vote_producer.iost", "candidateWithdraw", fmt.Sprintf(`["%s"]`, acc6.ID), acc6.ID, acc6.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, int64(212121217), s.Visitor.TokenBalance(global.Token, acc6.ID))                           // 212121217 = (3333333410*(7/55))/2
	assert.Equal(t, int64(103333333410-212121217), s.Visitor.TokenBalance(global.Token, "vote_producer.iost")) // half to voterBonus

	r, err = s.Call("vote_producer.iost", "candidateWithdraw", fmt.Sprintf(`["%s"]`, acc6.ID), acc6.ID, acc6.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, int64(212121217), s.Visitor.TokenBalance(global.Token, acc6.ID)) // not change
	assert.Equal(t, int64(103333333410-212121217), s.Visitor.TokenBalance(global.Token, "vote_producer.iost"))

	// withdraw by admin
	r, err = s.Call("vote_producer.iost", "voterWithdraw", fmt.Sprintf(`["%s"]`, acc2.ID), acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, int64(30303031), s.Visitor.TokenBalance(global.Token, acc2.ID))
	assert.Equal(t, int64(103090909162), s.Visitor.TokenBalance(global.Token, "vote_producer.iost"))

	r, err = s.Call("vote_producer.iost", "voterWithdraw", fmt.Sprintf(`["%s"]`, acc2.ID), acc2.ID, acc2.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, int64(30303031), s.Visitor.TokenBalance(global.Token, acc2.ID))
	assert.Equal(t, int64(103090909162), s.Visitor.TokenBalance(global.Token, "vote_producer.iost"))
}

func TestCriticalVoteCase(t *testing.T) {
	ilog.Stop()
	s := NewSimulator()
	defer s.Clear()

	s.Head.Number = 0

	createAccountsWithResource(s)
	prepareFakeBase(t, s)
	prepareIssue(s, acc0)
	prepareNewProducerVote(t, s, acc0)
	initProducer(t, s)

	// deploy bonus.iost
	r, err := setNonNativeContract(s, "bonus.iost", "bonus.js", ContractPath)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	s.Head.Number = 1

	for _, acc := range testAccounts[6:] {
		r, err := s.Call("vote_producer.iost", "applyRegister", fmt.Sprintf(`["%v", "%v", "loc", "url", "netId", true]`, acc.ID, acc.KeyPair.ReadablePubkey()), acc.ID, acc.KeyPair)
		assert.Nil(t, err)
		assert.Equal(t, tx.Success, r.Status.Code)
	}
	for idx, acc := range testAccounts {
		voter := acc0
		r, err := s.Call("vote_producer.iost", "vote", fmt.Sprintf(`["%v", "%v", "%v"]`, voter.ID, acc.ID, (idx+1)*2e6), voter.ID, voter.KeyPair)
		assert.Nil(t, err)
		assert.Empty(t, r.Status.Message)
		assert.Equal(t, fmt.Sprintf(`{"votes":"%d","deleted":0,"clearTime":-1}`, (idx+1)*2e6), database.MustUnmarshal(s.Visitor.MGet("vote.iost-v_1", acc.ID)))
		if idx == 0 {
			assert.Nil(t, database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-candAllKey")))
		} else {
			assert.Equal(t, fmt.Sprintf(`"%d"`, (idx+1)*(idx+2)*2e6/2-2e6), database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-candAllKey")))
		}
	}
	assert.Equal(t, `{"user_0":["2000000",1,"0"],"user_1":["4000000",1,"0"],"user_2":["6000000",1,"0"],"user_3":["8000000",1,"0"],"user_4":["10000000",1,"0"],"user_5":["12000000",1,"0"],"user_6":["14000000",1,"0"],"user_7":["16000000",1,"0"],"user_8":["18000000",1,"0"],"user_9":["20000000",1,"0"]}`, database.MustUnmarshal(s.Visitor.MGet("vote.iost-u_1", acc0.ID)))
	s.Head.Time += 5073358980
	r, err = s.Call("issue.iost", "issueIOST", `[]`, acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, int64(103333333410), s.Visitor.TokenBalance(global.Token, "vote_producer.iost"))
	assert.Equal(t, int64(3333333410), s.Visitor.TokenBalance(global.Token, "bonus.iost"))

	for i := 0; i < 10; i++ {
		s.Visitor.SetTokenBalance(global.Token, testAccounts[i].ID, 0)
	}
	r, err = s.Call("vote_producer.iost", "candidateWithdraw", fmt.Sprintf(`["%s"]`, acc6.ID), acc6.ID, acc6.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, int64(216049387), s.Visitor.TokenBalance(global.Token, acc6.ID))                           // 216049387 = (3333333410*(7/54))/2
	assert.Equal(t, int64(103333333410-216049387), s.Visitor.TokenBalance(global.Token, "vote_producer.iost")) // half to voterBonus

	r, err = s.Call("vote_producer.iost", "voterWithdraw", fmt.Sprintf(`["%s"]`, acc0.ID), acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, int64(216049387), s.Visitor.TokenBalance(global.Token, acc0.ID))
	assert.Equal(t, int64(103333333410-216049387*2), s.Visitor.TokenBalance(global.Token, "vote_producer.iost"))

	s.Visitor.SetTokenBalance(global.Token, acc2.ID, 1e17)
	r, err = s.Call("vote_producer.iost", "vote", fmt.Sprintf(`["%v", "%v", "%v"]`, acc2.ID, acc0.ID, 1e5), acc2.ID, acc2.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, `"110100000"`, database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-candAllKey")))

	r, err = s.Call("vote_producer.iost", "candidateWithdraw", fmt.Sprintf(`["%s"]`, acc0.ID), acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, int64(216049387), s.Visitor.TokenBalance(global.Token, acc0.ID)) // not changed

	s.Head.Time += 24*3600*1e9 + 1
	r, err = s.Call("issue.iost", "issueIOST", `[]`, acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, int64(102901234636+56767125693515), s.Visitor.TokenBalance(global.Token, "vote_producer.iost"))
	assert.Equal(t, int64(3333333410+56767125693515), s.Visitor.TokenBalance(global.Token, "bonus.iost"))

	r, err = s.Call("vote_producer.iost", "candidateWithdraw", fmt.Sprintf(`["%s"]`, acc0.ID), acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, int64(216049387+541375858112), s.Visitor.TokenBalance(global.Token, acc0.ID)) // 541375858112 = (56767125693515*(21/1101))/2

	r, err = s.Call("vote_producer.iost", "unvote", fmt.Sprintf(`["%v", "%v", "%v"]`, acc2.ID, acc0.ID, 1e5), acc2.ID, acc2.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, `"108000000"`, database.MustUnmarshal(s.Visitor.Get("vote_producer.iost-candAllKey")))

	s.Head.Time += 24*3600*1e9 + 1
	r, err = s.Call("issue.iost", "issueIOST", `[]`, acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)

	r, err = s.Call("vote_producer.iost", "candidateWithdraw", fmt.Sprintf(`["%s"]`, acc0.ID), acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Empty(t, r.Status.Message)
	assert.Equal(t, int64(216049387+541375858112), s.Visitor.TokenBalance(global.Token, acc0.ID)) // not changed
}
