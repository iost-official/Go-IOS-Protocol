package pob

import (
	"testing"

	"github.com/iost-official/Go-IOS-Protocol/account"
	. "github.com/smartystreets/goconvey/convey"
)

func TestGlobalStaticProperty(t *testing.T) {
	Convey("Test of witness lists of static property", t, func() {
		prop := newStaticProperty(
			account.Account{
				ID:     "id0",
				Pubkey: []byte{},
				Seckey: []byte{},
			},
			[]string{"id1", "id2", "id3"},
		)
		So(prop.NumberOfWitnesses, ShouldEqual, 3)
	})
}
