package integration

import (
	"encoding/json"
	"fmt"
	"github.com/iost-official/go-iost/core/global"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/iost-official/go-iost/account"
	"github.com/iost-official/go-iost/common"
	"github.com/iost-official/go-iost/core/block"
	"github.com/iost-official/go-iost/core/contract"
	"github.com/iost-official/go-iost/core/tx"
	"github.com/iost-official/go-iost/crypto"
	. "github.com/iost-official/go-iost/verifier"
	"github.com/iost-official/go-iost/vm/native"
)

// TestAccount used for writing test cases
type TestAccount struct {
	ID      string
	KeyPair *account.KeyPair
}

func (t *TestAccount) ToAccount() *account.Account {
	return account.NewInitAccount(t.ID, t.KeyPair.ReadablePubkey(), t.KeyPair.ReadablePubkey())
}

var testAccounts = make([]*TestAccount, 0)
var acc0 *TestAccount
var acc1 *TestAccount
var acc2 *TestAccount
var acc3 *TestAccount
var acc4 *TestAccount
var acc5 *TestAccount
var acc6 *TestAccount
var acc7 *TestAccount
var acc8 *TestAccount
var acc9 *TestAccount

func init() {
	var keys = []string{
		"546aCDG9igGgZqVZeybajaorP5ZeF9ghLu2oLncXk3d6",
		"GJt5WSSv5WZi1axd3qkb1vLEfxCEgKGupcXf45b5tERU",
		"DXNYRwG7dRFkbWzMNEbKfBhuS8Yn51x9J6XuTdNwB11M",
		"7CnwT7BXkEFAVx6QZqC7gkDhQwbvC3d2CkMZvXHZdDMN",
		"Htarc5Sp4trjqY4WrTLtZ85CF6qx87v7CRwtV4RRGnbF",
		"8dJ9YKovJ5E7hkebAQaScaG1BA8snRUHPUbVcArcTVq6",
		"AG8uECmAwFis8uxTdWqcgGD9tGDwoP6CxqhkhpuCdSeC",
		"Bk8bAyG4VLBcrsoRErPuQGhwCy4C1VxfKE4jjX9oLhv",
		"7U3uwEeGc2TF3Xde2oT66eTx1Uw15qRqYuTnMd3NNjai",
		"EhNiaU4DzUmjCrvynV3gaUeuj2VjB1v2DCmbGD5U2nSE",
	}

	for idx, k := range keys {
		kp, err := account.NewKeyPair(common.Base58Decode(k), crypto.Secp256k1)
		if err != nil {
			panic(err)
		}
		testAccounts = append(testAccounts, &TestAccount{fmt.Sprintf("user_%d", idx), kp})
	}
	acc0 = testAccounts[0]
	acc1 = testAccounts[1]
	acc2 = testAccounts[2]
	acc3 = testAccounts[3]
	acc4 = testAccounts[4]
	acc5 = testAccounts[5]
	acc6 = testAccounts[6]
	acc7 = testAccounts[7]
	acc8 = testAccounts[8]
	acc9 = testAccounts[9]
}

var ContractPath = os.Getenv("GOPATH") + "/src/github.com/iost-official/go-iost/config/genesis/contract/"

type fataler interface {
	Fatal(args ...interface{})
}

func array2json(ss []interface{}) string {
	x, err := json.Marshal(ss)
	if err != nil {
		panic(err)
	}
	return string(x)
}

func createAccountsWithResource(s *Simulator) {
	for _, acc := range testAccounts {
		s.SetAccount(acc.ToAccount())
		s.SetGas(acc.ID, 100000000)
		s.SetRAM(acc.ID, 10000)
	}
	// deploy token.iost
	s.SetContract(native.TokenABI())
	// used in ram contract
	s.SetAccount(account.NewInitAccount("deadaddr", "hahaha", "hahaha"))
	s.Visitor.SetTokenBalance(global.Token, "deadaddr", 0)
	s.Visitor.Commit()
}

func createToken(t fataler, s *Simulator, acc *TestAccount) error {
	// create token
	r, err := s.Call("token.iost", "create", fmt.Sprintf(`["%v", "%v", %v, {}]`, global.Token, acc0.ID, 1000000), acc.ID, acc.KeyPair)
	if err != nil || r.Status.Code != tx.Success {
		return fmt.Errorf("err %v, receipt: %v", err, r)
	}
	// issue token
	r, err = s.Call("token.iost", "issue", fmt.Sprintf(`["%v", "%v", "%v"]`, global.Token, acc0.ID, "1000"), acc.ID, acc.KeyPair)
	if err != nil || r.Status.Code != tx.Success {
		return fmt.Errorf("err %v, receipt: %v", err, r)
	}
	if 1e11 != s.Visitor.TokenBalance(global.Token, acc0.ID) {
		return fmt.Errorf("err %v, receipt: %v", err, r)
	}
	s.Visitor.Commit()
	return nil
}

func setNonNativeContract(s *Simulator, name string, filename string, ContractPath string) (*tx.TxReceipt, error) {
	jsPath := filepath.Join(ContractPath, filename)
	abiPath := filepath.Join(ContractPath, filename+".abi")
	fd, err := ioutil.ReadFile(jsPath)
	if err != nil {
		return nil, err
	}
	rawCode := string(fd)
	fd, err = ioutil.ReadFile(abiPath)
	if err != nil {
		return nil, err
	}
	rawAbi := string(fd)
	c := contract.Compiler{}
	code, err := c.Parse(name, rawCode, rawAbi)
	if err != nil {
		return nil, err
	}

	s.SetRAM("admin", 1e6)
	return s.DeploySystemContract(code, acc0.ID, acc0.KeyPair)
}

func prepareAuth(t fataler, s *Simulator) *TestAccount {
	s.SetAccount(acc0.ToAccount())
	return acc0
}

var bh = &block.BlockHead{
	ParentHash: []byte("abc"),
	Number:     200,
	Witness:    "witness",
	Time:       123460 * 1e9,
}
