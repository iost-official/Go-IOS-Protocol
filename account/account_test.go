package account

import (
	"testing"

	"bytes"

	"fmt"

	"strings"

	. "github.com/iost-official/Go-IOS-Protocol/common"
	. "github.com/smartystreets/goconvey/convey"
)

func TestMember(t *testing.T) {
	Convey("Test of Account", t, func() {
		m, err := NewAccount(nil)
		Convey("New member: ", func() {
			So(err, ShouldBeNil)
			So(len(m.Pubkey), ShouldEqual, 33)
			So(len(m.Seckey), ShouldEqual, 32)
			So(len(m.ID), ShouldEqual, len(GetIDByPubkey(m.Pubkey)))
		})

		Convey("sign and verify: ", func() {
			info := []byte("hello world")
			sig := SignInSecp256k1(Sha256(info), m.Seckey)
			So(VerifySignInSecp256k1(Sha256(info), m.Pubkey, sig), ShouldBeTrue)

			sig2 := Sign(Secp256k1, Sha256(info), m.Seckey, SavePubkey)
			So(bytes.Equal(sig2.Pubkey, m.Pubkey), ShouldBeTrue)

		})
		Convey("sec to pub", func() {
			m, err := NewAccount(Base58Decode("3BZ3HWs2nWucCCvLp7FRFv1K7RR3fAjjEQccf9EJrTv4"))
			So(err, ShouldBeNil)
			fmt.Println(Base58Encode(m.Pubkey))
		})
	})
}

func TestPubkeyAndID(t *testing.T) {
	for i := 0; i < 10; i++ {
		seckey := randomSeckey()
		pubkey := makePubkey(seckey)
		id := GetIDByPubkey(pubkey)
		fmt.Println(`"`, id, `", "`, Base58Encode(seckey), `"`)
		pub2 := GetPubkeyByID(id)
		id2 := GetIDByPubkey(pub2)
		if !strings.HasPrefix(id, "IOST") {
			t.Failed()
		}
		if id != id2 {
			t.Failed()
		}
	}
}
