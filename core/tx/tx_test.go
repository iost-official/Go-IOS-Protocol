package tx

import (
	"bytes"
	"fmt"
	"github.com/iost-official/go-iost/core/global"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/iost-official/go-iost/account"
	"github.com/iost-official/go-iost/common"
	"github.com/iost-official/go-iost/core/contract"
	"github.com/iost-official/go-iost/core/tx/pb"
	"github.com/iost-official/go-iost/crypto"
	. "github.com/smartystreets/goconvey/convey"
)

func TestAction(t *testing.T) {
	Convey("Test of Action Data Structure", t, func() {
		action := Action{
			Contract:   "contract1",
			ActionName: "actionname1",
			Data:       "{\"num\": 1, \"message\": \"contract1\"}",
		}

		encode := action.Encode()

		var action1 Action
		err := action1.Decode(encode)
		So(err, ShouldBeNil)

		So(action.Contract == action1.Contract, ShouldBeTrue)
		So(action.ActionName == action1.ActionName, ShouldBeTrue)
		So(action.Data == action1.Data, ShouldBeTrue)
	})
}

func TestTx(t *testing.T) {
	Convey("Test of Tx Data Structure", t, func() {
		var actions []*Action
		actions = append(actions, &Action{
			Contract:   "contract1",
			ActionName: "actionname1",
			Data:       "{\"num\": 1, \"message\": \"contract1\"}",
		})
		actions = append(actions, &Action{
			Contract:   "contract2",
			ActionName: "actionname2",
			Data:       "1",
		})
		// seckey := cverifier.Base58Decode("3BZ3HWs2nWucCCvLp7FRFv1K7RR3fAjjEQccf9EJrTv4")
		// acc, err := account.NewKeyPair(seckey)
		// So(err, ShouldEqual, nil)

		a1, _ := account.NewKeyPair(nil, crypto.Secp256k1)
		a2, _ := account.NewKeyPair(nil, crypto.Secp256k1)
		a3, _ := account.NewKeyPair(nil, crypto.Secp256k1)

		Convey("proto marshal", func() {
			tx := &txpb.Tx{
				Time: 99,
				Actions: []*txpb.Action{{
					Contract:   "contract1",
					ActionName: "actionname1",
					Data:       "{\"num\": 1, \"message\": \"contract1\"}",
				}},
				Signers: []string{a1.ReadablePubkey()},
			}
			b, err := proto.Marshal(tx)
			So(err, ShouldEqual, nil)

			var tx1 *txpb.Tx = &txpb.Tx{}

			err = proto.Unmarshal(b, tx1)
			So(err, ShouldEqual, nil)

			So(99, ShouldEqual, tx1.Time)
		})

		Convey("encode and decode", func() {
			tx := NewTx(actions, []string{a1.ReadablePubkey()}, 100000, 100, 11, 0, 0)
			tx1 := NewTx([]*Action{}, []string{}, 0, 0, 0, 0, 0)
			hash := tx.Hash()

			encode := tx.Encode()
			err := tx1.Decode(encode)
			So(err, ShouldEqual, nil)

			hash1 := tx1.Hash()
			So(bytes.Equal(hash, hash1), ShouldEqual, true)

			sig, err := SignTxContent(tx, a1.ReadablePubkey(), a1)
			So(err, ShouldEqual, nil)

			_, err = SignTx(tx, a1.ReadablePubkey(), []*account.KeyPair{a1}, sig)
			So(err, ShouldEqual, nil)

			hash = tx.Hash()
			encode = tx.Encode()
			err = tx1.Decode(encode)
			So(err, ShouldEqual, nil)
			hash1 = tx1.Hash()

			So(bytes.Equal(hash, hash1), ShouldEqual, true)

			So(tx.Time == tx1.Time, ShouldBeTrue)
			So(tx.Expiration == tx1.Expiration, ShouldBeTrue)
			So(tx.GasLimit == tx1.GasLimit, ShouldBeTrue)
			So(tx.GasRatio == tx1.GasRatio, ShouldBeTrue)
			So(len(tx.Actions) == len(tx1.Actions), ShouldBeTrue)
			for i := 0; i < len(tx.Actions); i++ {
				So(tx.Actions[i].Contract == tx1.Actions[i].Contract, ShouldBeTrue)
				So(tx.Actions[i].ActionName == tx1.Actions[i].ActionName, ShouldBeTrue)
				So(tx.Actions[i].Data == tx1.Actions[i].Data, ShouldBeTrue)
			}
			So(len(tx.Signers) == len(tx1.Signers), ShouldBeTrue)
			for i := 0; i < len(tx.Signers); i++ {
				So(tx.Signers[i], ShouldEqual, tx1.Signers[i])
			}
			So(len(tx.Signs) == len(tx1.Signs), ShouldBeTrue)
			for i := 0; i < len(tx.Signs); i++ {
				So(tx.Signs[i].Algorithm == tx1.Signs[i].Algorithm, ShouldBeTrue)
				So(bytes.Equal(tx.Signs[i].Pubkey, tx1.Signs[i].Pubkey), ShouldBeTrue)
				So(bytes.Equal(tx.Signs[i].Sig, tx1.Signs[i].Sig), ShouldBeTrue)
			}
			So(len(tx.PublishSigns), ShouldEqual, len(tx1.PublishSigns))
			for i := 0; i < len(tx.PublishSigns); i++ {
				So(tx.PublishSigns[i].Algorithm, ShouldEqual, tx1.PublishSigns[i].Algorithm)
				So(bytes.Equal(tx.PublishSigns[i].Pubkey, tx1.PublishSigns[i].Pubkey), ShouldBeTrue)
				So(bytes.Equal(tx.PublishSigns[i].Sig, tx1.PublishSigns[i].Sig), ShouldBeTrue)
			}
		})

		Convey("sign and verify", func() {
			tx := NewTx(actions, []string{a1.ReadablePubkey(), a2.ReadablePubkey()}, 100000000, 100, time.Now().Add(time.Minute).UnixNano(), 0, 0)
			sig1, err := SignTxContent(tx, a1.ReadablePubkey(), a1)
			So(tx.VerifySigner(sig1), ShouldBeTrue)
			tx.Signs = append(tx.Signs, sig1)

			sig2, err := SignTxContent(tx, a2.ReadablePubkey(), a2)
			So(tx.VerifySigner(sig2), ShouldBeTrue)
			tx.Signs = append(tx.Signs, sig2)

			err = tx.VerifySelf()
			So(err.Error(), ShouldEqual, "publisher empty error")

			tx3, err := SignTx(tx, a3.ReadablePubkey(), []*account.KeyPair{a3})
			So(err, ShouldBeNil)
			err = tx3.VerifySelf()
			So(err, ShouldBeNil)

			tx.PublishSigns = []*crypto.Signature{{
				Algorithm: crypto.Secp256k1,
				Sig:       []byte("hello"),
				Pubkey:    []byte("world"),
			}}
			err = tx.VerifySelf()
			So(err.Error(), ShouldEqual, "publisher error")

			fmt.Println(tx.String())

			tx.Signs[0] = &crypto.Signature{
				Algorithm: crypto.Secp256k1,
				Sig:       []byte("hello"),
				Pubkey:    []byte("world"),
			}
			err = tx.VerifySelf()
			So(err.Error(), ShouldEqual, "signer error")

			tx = NewTx(actions, nil, 100000000, 100, time.Now().Add(time.Minute).UnixNano(), 0, 0)
			tx.Time = -1
			So(tx.VerifySelf().Error(), ShouldEqual, "invalid time and expiration")
			tx.Time = time.Now().UnixNano()
			tx.Expiration = tx.Time
			So(tx.VerifySelf().Error(), ShouldEqual, "invalid time and expiration")
			tx.Expiration = tx.Time + 1
			tx.Delay = -1
			So(tx.VerifySelf().Error(), ShouldEqual, "invalid delay time")
			tx.Delay = 999999999999999999
			So(tx.VerifySelf().Error(), ShouldEqual, "invalid delay time")
			tx.Delay = 1000
			tx.ReferredTx = []byte("b")
			So(tx.VerifySelf().Error(), ShouldEqual, "invalid tx. including both delay and referredtx field")
			tx.ReferredTx = nil
			tx.Actions = []*Action{{"", "", string(make([]byte, 1000000))}}
			So(tx.VerifySelf().Error(), ShouldContainSubstring, "tx size illegal, should <= 65536")
		})

	})
}

func TestTx_Platform(t *testing.T) {
	//t.Skip()
	//var sep = `\` + "`" + "^" + "/" + "<"
	//fmt.Println(sep, "is", []byte(sep))
	txx := &Tx{
		Time:       1544013436179000000,
		Expiration: 1544013526179000000,
		GasRatio:   100,
		GasLimit:   123400,
		Delay:      0,
	}
	txx.Signers = []string{"abc"}
	txx.Actions = []*Action{{
		Contract:   "cont",
		ActionName: "abi",
		Data:       "[]",
	},
	}
	txx.AmountLimit = []*contract.Amount{
		{
			Token: global.Token,
			Val:   "123",
		},
	}
	by := txx.ToBytes(0)

	fmt.Println("{")

	fmt.Printf(`"tx_bytes_0" : "%x",`+"\n", by)

	fmt.Printf(`"tx_base_hash" : "%x",`+"\n", txx.baseHash())

	kp, err := account.NewKeyPair(common.Base58Decode("1rANSfcRzr4HkhbUFZ7L1Zp69JZZHiDDq5v7dNSbbEqeU4jxy3fszV4HGiaLQEyqVpS1dKT9g7zCVRxBVzuiUzB"), crypto.Ed25519)
	if err != nil {
		t.Fatal(err)
	}

	sig, err := SignTxContent(txx, "abc", kp)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf(`"sig_bytes" : "%x",`+"\n", sig.ToBytes())
	fmt.Printf(`"sig_pubkey" : "%x",`+"\n", sig.Pubkey)
	fmt.Printf(`"sig_sig" : "%x",`+"\n", sig.Sig)

	txx.Signs = append(txx.Signs, sig)

	fmt.Printf(`"tx_bytes_1" : "%x",`+"\n", txx.ToBytes(1))

	fmt.Printf(`"tx_publish_hash" : "%x",`+"\n", txx.publishHash())

	tx2, err := SignTx(txx, "def", []*account.KeyPair{kp})

	fmt.Printf(`"tx_publish_sign" : "%x"`+"\n", tx2.PublishSigns[0].ToBytes())
	fmt.Println("}")
}

func BenchmarkHash(b *testing.B) {
	tx := &Tx{
		Time:       1234567890,
		Expiration: 9876543210,
		GasRatio:   100,
		GasLimit:   10000,
		Delay:      0,
		Publisher:  "root",
		Actions: []*Action{
			{
				Contract:   "contract",
				ActionName: "actionname",
				Data:       "data",
			},
		},
		Signers: []string{"signer1", "signer2"},
		Signs: []*crypto.Signature{
			{
				Algorithm: crypto.Secp256k1,
				Sig:       []byte("hello"),
				Pubkey:    []byte("world"),
			},
			{
				Algorithm: crypto.Ed25519,
				Sig:       []byte("foo"),
				Pubkey:    []byte("bar"),
			},
		},
		PublishSigns: []*crypto.Signature{
			{
				Algorithm: crypto.Ed25519,
				Sig:       []byte("aaa"),
				Pubkey:    []byte("bbb"),
			},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx.Hash()
	}
}
