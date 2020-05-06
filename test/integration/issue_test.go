package integration

import (
	"encoding/json"
	"testing"

	"github.com/iost-official/go-iost/core/tx"

	"github.com/iost-official/go-iost/ilog"

	"github.com/iost-official/go-iost/common"
	. "github.com/iost-official/go-iost/verifier"
	. "github.com/smartystreets/goconvey/convey"
)

func prepareIssue(s *Simulator, acc *TestAccount) (*tx.TxReceipt, error) {
	s.Head.Number = 0

	// deploy issue.iost
	r, err := setNonNativeContract(s, "issue.iost", "issue.js", ContractPath)
	if err != nil || r.Status.Code != tx.Success {
		return r, err
	}

	witness := common.Witness{
		ID:      acc0.ID,
		Owner:   acc0.KeyPair.ReadablePubkey(),
		Active:  acc0.KeyPair.ReadablePubkey(),
		Balance: 21000000000,
	}
	params := []interface{}{
		acc0.ID,
		common.TokenInfo{
			FoundationAccount: acc1.ID,
			IOSTTotalSupply:   90000000000,
			IOSTDecimal:       8,
		},
		[]interface{}{witness},
	}
	b, _ := json.Marshal(params)
	r, err = s.Call("issue.iost", "initGenesis", string(b), acc.ID, acc.KeyPair)
	s.Visitor.Commit()
	return r, err
}

func Test_IOSTIssue(t *testing.T) {
	ilog.Stop()
	Convey("test issue.iost", t, func() {
		s := NewSimulator()
		defer s.Clear()

		createAccountsWithResource(s)
		r, err := prepareIssue(s, acc0)

		Convey("test init", func() {
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			So(s.Visitor.TokenBalance(global.Token, acc0.ID), ShouldEqual, int64(210*1e16))
		})

		prepareNewProducerVote(t, s, acc0)
		initProducer(t, s)

		Convey("test issueIOST", func() {
			s.Head.Time += 4 * 3 * 1e9
			r, err := s.Call("issue.iost", "issueIOST", `[]`, acc0.ID, acc0.KeyPair)
			s.Visitor.Commit()

			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")

			So(s.Visitor.TokenBalance(global.Token, "bonus.iost"), ShouldEqual, int64(7884322975))
			So(s.Visitor.TokenBalance(global.Token, acc1.ID), ShouldEqual, int64(7884323211))
		})
	})
}
