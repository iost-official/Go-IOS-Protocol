package integration

import (
	"fmt"
	"testing"

	"github.com/iost-official/go-iost/ilog"
	"github.com/iost-official/go-iost/vm/database"

	"github.com/iost-official/go-iost/core/tx"
	. "github.com/iost-official/go-iost/verifier"
	. "github.com/smartystreets/goconvey/convey"
)

func Test_IssueBonus(t *testing.T) {
	ilog.Stop()
	Convey("test bonus.iost", t, func() {
		s := NewSimulator()
		defer s.Clear()

		createAccountsWithResource(s)
		prepareFakeBase(t, s)

		// deploy issue.iost
		r, err := setNonNativeContract(s, "bonus.iost", "bonus.js", ContractPath)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")

		Convey("test issueContribute", func() {
			s.Head.Witness = acc1.KeyPair.ReadablePubkey()
			s.Head.Number = 1

			r, err := s.Call("base.iost", "issueContribute", fmt.Sprintf(`[{"parent":["%v","12345678"]}]`, acc1.ID), acc1.ID, acc1.KeyPair)
			s.Visitor.Commit()

			So(err, ShouldBeNil)
			So(r.Status.Code, ShouldEqual, tx.Success)
			So(s.Visitor.TokenBalance("contribute", acc1.ID), ShouldEqual, int64(328513441))
		})
	})
}

func Test_ExchangeIOST(t *testing.T) {
	ilog.Stop()
	Convey("test bonus.iost", t, func() {
		s := NewSimulator()
		defer s.Clear()

		s.Head.Number = 0
		createAccountsWithResource(s)
		prepareIssue(s, acc0)
		prepareNewProducerVote(t, s, acc0)
		initProducer(t, s)
		prepareFakeBase(t, s)

		// deploy bonus.iost
		r, err := setNonNativeContract(s, "bonus.iost", "bonus.js", ContractPath)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")

		Convey("test exchangeIOST", func() {
			createToken(t, s, acc0)

			// set bonus pool
			s.Call("token.iost", "issue", fmt.Sprintf(`["%v", "%v", "%v"]`, global.Token, "bonus.iost", "1000"), acc0.ID, acc0.KeyPair)

			// gain contribute
			s.Head.Witness = acc1.KeyPair.ReadablePubkey()
			s.Head.Number = 1
			r, err := s.Call("base.iost", "issueContribute", fmt.Sprintf(`[{"parent":["%v","%v"]}]`, acc1.ID, 1), acc1.ID, acc1.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			s.Visitor.Commit()

			So(s.Visitor.TokenBalance("contribute", acc1.ID), ShouldEqual, int64(328513441))

			s.Head.Witness = acc2.KeyPair.ReadablePubkey()
			s.Head.Number = 2
			r, err = s.Call("base.iost", "issueContribute", fmt.Sprintf(`[{"parent":["%v","%v"]}]`, acc2.ID, 123456789), acc2.ID, acc2.KeyPair)
			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			s.Visitor.Commit()

			So(s.Visitor.TokenBalance("contribute", acc2.ID), ShouldEqual, int64(328513441))

			r, err = s.Call("bonus.iost", "exchangeIOST", fmt.Sprintf(`["%v", "%v"]`, acc1.ID, "1.9"), acc1.ID, acc1.KeyPair)
			s.Visitor.Commit()

			So(err, ShouldBeNil)
			So(r.Status.Message, ShouldEqual, "")
			So(s.Visitor.TokenBalance("contribute", acc1.ID), ShouldEqual, int64(138513441))
			So(s.Visitor.TokenBalance(global.Token, acc1.ID), ShouldEqual, int64(190000000))
			So(s.Visitor.TokenBalance(global.Token, "bonus.iost"), ShouldEqual, int64(99810000000))
		})
	})
}

func Test_UpdateBonus(t *testing.T) {
	ilog.Stop()
	Convey("test update bonus", t, func() {
		s := NewSimulator()
		defer s.Clear()

		createAccountsWithResource(s)
		prepareFakeBase(t, s)
		r, err := prepareIssue(s, acc0)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")

		// deploy issue.iost
		r, err = setNonNativeContract(s, "bonus.iost", "bonus.js", ContractPath)
		So(err, ShouldBeNil)
		So(r.Status.Message, ShouldEqual, "")

		prepareNewProducerVote(t, s, acc0)
		initProducer(t, s)

		Convey("test update bonus 1", func() {
			s.Head.Witness = acc1.KeyPair.ReadablePubkey()
			s.Head.Number = 1

			So(database.MustUnmarshal(s.Visitor.Get("bonus.iost-blockContrib")), ShouldEqual, `"3.28513441"`)
			for i := 0; i < 7; i++ {
				s.Head.Time += 86400 * 1e9
				r, err = s.Call("issue.iost", "issueIOST", `[]`, acc0.ID, acc0.KeyPair)
				So(err, ShouldBeNil)
				So(r.Status.Message, ShouldEqual, "")

				r, err = s.Call("base.iost", "issueContribute", fmt.Sprintf(`[{"parent":["%v","12345678"]}]`, acc1.ID), acc1.ID, acc1.KeyPair)
				So(err, ShouldBeNil)
				So(r.Status.Message, ShouldEqual, "")
			}

			So(s.Visitor.TokenBalance(global.Token, "bonus.iost"), ShouldEqual, int64(397466566222260))
			So(s.Visitor.TokenBalance("contribute", acc1.ID), ShouldEqual, int64(2299780636))
			So(database.MustUnmarshal(s.Visitor.Get("bonus.iost-blockContrib")), ShouldEqual, `"3.28699990"`)

			for i := 0; i < 7; i++ {
				s.Head.Time += 86400 * 1e9
				r, err = s.Call("issue.iost", "issueIOST", `[]`, acc0.ID, acc0.KeyPair)
				So(err, ShouldBeNil)
				So(r.Status.Message, ShouldEqual, "")

				r, err = s.Call("base.iost", "issueContribute", fmt.Sprintf(`[{"parent":["%v","12345678"]}]`, acc1.ID), acc1.ID, acc1.KeyPair)
				So(err, ShouldBeNil)
				So(r.Status.Message, ShouldEqual, "")
			}

			So(s.Visitor.TokenBalance(global.Token, "bonus.iost"), ShouldEqual, int64(795158817680693))
			So(s.Visitor.TokenBalance("contribute", acc1.ID), ShouldEqual, int64(4600867205))
			So(database.MustUnmarshal(s.Visitor.Get("bonus.iost-blockContrib")), ShouldEqual, `"3.28886629"`)
		})
	})
}
