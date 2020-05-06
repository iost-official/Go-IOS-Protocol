package genesis

import (
	"fmt"
	"github.com/iost-official/go-iost/account"
	"github.com/iost-official/go-iost/crypto"
	"os"
	"testing"

	"github.com/iost-official/go-iost/common/config"
	"github.com/iost-official/go-iost/db"
	"github.com/iost-official/go-iost/ilog"
)

func randWitness(idx int) *config.Witness {
	k := account.EncodePubkey(crypto.Ed25519.GetPubkey(crypto.Ed25519.GenSeckey()))
	return &config.Witness{ID: fmt.Sprintf("a%d", idx), Owner: k, Active: k, SignatureBlock: k, Balance: 3 * 1e8}
}

func TestGenGenesis(t *testing.T) {
	ilog.Stop()

	d, err := db.NewMVCCDB("mvcc")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		d.Close()
		os.RemoveAll("mvcc")
	}()
	k := account.EncodePubkey(crypto.Ed25519.GetPubkey(crypto.Ed25519.GenSeckey()))
	blk, err := GenGenesis(d, &config.GenesisConfig{
		WitnessInfo: []*config.Witness{
			randWitness(1),
			randWitness(2),
			randWitness(3),
			randWitness(4),
			randWitness(5),
			randWitness(6),
			randWitness(7),
		},
		TokenInfo: &config.TokenInfo{
			FoundationAccount: "f8",
			IOSTTotalSupply:   90000000000,
			IOSTDecimal:       8,
		},
		InitialTimestamp: "2006-01-02T15:04:05Z",
		ContractPath:     os.Getenv("GOPATH") + "/src/github.com/iost-official/go-iost/config/genesis/contract/",
		AdminInfo:        randWitness(8),
		FoundationInfo:   &config.Witness{ID: "f8", Owner: k, Active: k, Balance: 0},
	})
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(blk)
	return
}
