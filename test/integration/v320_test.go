package integration

import (
	"fmt"
	"github.com/iost-official/go-iost/common/config"
	"testing"

	"github.com/iost-official/go-iost/account"

	"github.com/iost-official/go-iost/core/version"
	"github.com/iost-official/go-iost/ilog"
	. "github.com/iost-official/go-iost/verifier"
	"github.com/iost-official/go-iost/vm/database"
	"github.com/iost-official/go-iost/vm/native"
	"github.com/stretchr/testify/assert"
)

func Test_Caller(t *testing.T) {
	t.Skip()
	ilog.Stop()
	s := NewSimulator()
	defer s.Clear()

	s.SetContract(native.TokenABI())
	acc := prepareAuth(t, s)
	s.SetGas(acc.ID, 1e10)
	s.SetRAM(acc.ID, 1e5)
	prepareToken(t, s, acc)

	conf := &config.Config{
		P2P: &config.P2PConfig{
			ChainID: 1024,
		},
	}
	version.InitChainConf(conf)
	rules := version.NewRules(0)
	assert.False(t, rules.IsFork3_3_0)
	s.Visitor = database.NewVisitor(0, s.Mvcc, rules)

	ca, err := s.Compile("", "./test_data/v320", "./test_data/v320.js")
	assert.Nil(t, err)
	assert.NotNil(t, ca)

	cname, r, err := s.DeployContract(ca, acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Equal(t, "", r.Status.Message)

	r, err = s.Call(cname, "caller", `[]`, acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Contains(t, r.Status.Message, "blockchain.caller is not a function")

	r, err = s.Call(cname, "caller2", `[]`, acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Contains(t, r.Status.Message, "blockchain.caller is not a function")

	conf.P2P.ChainID = 0
	version.InitChainConf(conf)
	rules = version.NewRules(0)
	assert.True(t, rules.IsFork3_3_0)
	s.Visitor = database.NewVisitor(0, s.Mvcc, rules)

	r, err = s.Call(cname, "caller", `[]`, acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Contains(t, r.Returns, fmt.Sprintf(`["{\"name\":\"%s\",\"is_account\":true}"]`, acc0.ID))

	r, err = s.Call(cname, "caller2", `[]`, acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Contains(t, r.Returns, fmt.Sprintf(`["{\"name\":\"%s\",\"is_account\":false}"]`, cname))
}

func Test_TxInfo(t *testing.T) {
	t.Skip()
	ilog.Stop()
	s := NewSimulator()
	defer s.Clear()

	s.SetContract(native.TokenABI())
	acc := prepareAuth(t, s)
	s.SetGas(acc.ID, 1e10)
	s.SetRAM(acc.ID, 1e5)
	prepareToken(t, s, acc)

	conf := &config.Config{
		P2P: &config.P2PConfig{
			ChainID: 1024,
		},
	}
	version.InitChainConf(conf)
	rules := version.NewRules(0)
	assert.False(t, rules.IsFork3_3_0)
	s.Visitor = database.NewVisitor(0, s.Mvcc, rules)

	ca, err := s.Compile("", "./test_data/v320", "./test_data/v320.js")
	assert.Nil(t, err)
	assert.NotNil(t, ca)

	cname, r, err := s.DeployContract(ca, acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Equal(t, "", r.Status.Message)

	r, err = s.Call(cname, "txInfo", `[]`, acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Contains(t, r.Returns[0], `"undefined-undefined"`)

	conf.P2P.ChainID = 0
	version.InitChainConf(conf)
	rules = version.NewRules(0)
	assert.True(t, rules.IsFork3_3_0)
	s.Visitor = database.NewVisitor(0, s.Mvcc, rules)

	r, err = s.Call(cname, "txInfo", `[]`, acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Contains(t, r.Returns[0], fmt.Sprintf(`"[{\"token\":\"*\",\"val\":\"unlimited\"}]-[{\"contract\":\"%s\",\"actionName\":\"txInfo\",\"data\":\"[]\"}]"`, cname))
}

func Test_Transfer(t *testing.T) {
	t.Skip()
	ilog.Stop()
	s := NewSimulator()
	defer s.Clear()

	createAccountsWithResource(s)
	prepareToken(t, s, acc0)

	conf := &config.Config{
		P2P: &config.P2PConfig{
			ChainID: 1024,
		},
	}
	version.InitChainConf(conf)
	rules := version.NewRules(0)
	assert.False(t, rules.IsFork3_3_0)
	s.Visitor = database.NewVisitor(0, s.Mvcc, rules)

	ca, err := s.Compile("", "./test_data/v320", "./test_data/v320.js")
	assert.Nil(t, err)
	assert.NotNil(t, ca)

	cname, r, err := s.DeployContract(ca, acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Equal(t, "", r.Status.Message)

	r, err = s.Call(cname, "transfer", fmt.Sprintf(`["%s"]`, acc1.ID), acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Contains(t, r.Status.Message, "invalid amount 1e-8 abnormal char in amount")

	conf.P2P.ChainID = 0
	version.InitChainConf(conf)
	rules = version.NewRules(0)
	assert.True(t, rules.IsFork3_3_0)
	s.Visitor = database.NewVisitor(0, s.Mvcc, rules)

	r, err = s.Call(cname, "transfer", fmt.Sprintf(`["%s"]`, acc1.ID), acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Equal(t, "", r.Status.Message)
}

func Test_SignerAuth(t *testing.T) {
	ilog.Stop()

	s := NewSimulator()
	defer s.Clear()

	s.SetContract(native.TokenABI())
	acc := prepareAuth(t, s)
	s.SetGas(acc.ID, 1e10)
	s.SetRAM(acc.ID, 1e5)
	prepareToken(t, s, acc)

	a1 := &TestAccount{"u1", acc.KeyPair}
	s.SetAccount(a1.ToAccount())

	a2 := &TestAccount{"u2", acc.KeyPair}
	a2acc := a2.ToAccount()
	a2acc.Permissions["p20"] = &account.Permission{
		Name:      "p20",
		Threshold: 1,
		Items: []*account.Item{
			{
				ID:         a1.ID,
				Permission: "active",
				IsKeyPair:  false,
				Weight:     1,
			},
		},
	}
	a2acc.Permissions["p21"] = &account.Permission{
		Name:      "p21",
		Threshold: 1,
		Items: []*account.Item{
			{
				ID:         a1.ID,
				Permission: "owner",
				IsKeyPair:  false,
				Weight:     1,
			},
		},
	}

	s.SetAccount(a2acc)

	s.SetGas(a1.ID, 1e10)
	s.SetRAM(a1.ID, 1e5)

	conf := &config.Config{
		P2P: &config.P2PConfig{
			ChainID: 1024,
		},
	}
	version.InitChainConf(conf)
	rules := version.NewRules(0)
	assert.False(t, rules.IsFork3_3_0)
	s.Visitor = database.NewVisitor(0, s.Mvcc, rules)

	ca, err := s.Compile("", "./test_data/v320", "./test_data/v320.js")
	assert.Nil(t, err)
	assert.NotNil(t, ca)

	cname, r, err := s.DeployContract(ca, acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Equal(t, "", r.Status.Message)

	signers := []string{}
	r, err = s.Call(cname, "requireAuth", `[]`, a1.ID, a1.KeyPair, signers)
	assert.Nil(t, err)
	assert.Equal(t, `["[true,true,true,true,true,true,true]"]`, r.Returns[0])

	conf.P2P.ChainID = 0
	version.InitChainConf(conf)
	rules = version.NewRules(0)
	assert.True(t, rules.IsFork3_3_0)
	s.Visitor = database.NewVisitor(0, s.Mvcc, rules)

	signers = []string{}
	r, err = s.Call(cname, "requireAuth", `[]`, a1.ID, a1.KeyPair, signers)
	assert.Nil(t, err)
	assert.Equal(t, `["[true,true,false,true,false,false,false]"]`, r.Returns[0])

	signers = []string{"u1@owner"}
	r, err = s.Call(cname, "requireAuth", `[]`, a1.ID, a1.KeyPair, signers)
	assert.Nil(t, err)
	assert.Equal(t, `["[true,true,true,true,true,false,false]"]`, r.Returns[0])

	signers = []string{"u2@p20"}
	r, err = s.Call(cname, "requireAuth", `[]`, a1.ID, a1.KeyPair, signers)
	assert.Nil(t, err)
	assert.Equal(t, `["[true,true,false,true,false,false,false]"]`, r.Returns[0])

	signers = []string{"u2@p21"}
	r, err = s.Call(cname, "requireAuth", `[]`, a1.ID, a1.KeyPair, signers)
	assert.Nil(t, err)
	assert.Equal(t, `["[true,true,false,true,true,false,false]"]`, r.Returns[0])

	signers = []string{"u1@owner", "u2@p21"}
	r, err = s.Call(cname, "requireAuth", `[]`, a1.ID, a1.KeyPair, signers)
	assert.Nil(t, err)
	assert.Equal(t, `["[true,true,true,true,true,false,false]"]`, r.Returns[0])
}

func Test_SetCode(t *testing.T) {
	ilog.Stop()

	s := NewSimulator()
	defer s.Clear()

	s.SetContract(native.TokenABI())
	acc := prepareAuth(t, s)
	s.SetGas(acc.ID, 1e10)
	s.SetRAM(acc.ID, 1e5)
	prepareToken(t, s, acc)

	conf := &config.Config{
		P2P: &config.P2PConfig{
			ChainID: 1024,
		},
	}
	version.InitChainConf(conf)
	rules := version.NewRules(0)
	assert.False(t, rules.IsFork3_3_0)
	s.Visitor = database.NewVisitor(0, s.Mvcc, rules)

	ca, err := s.Compile("", "./test_data/v320", "./test_data/v320.js")
	assert.Nil(t, err)
	assert.NotNil(t, ca)

	cname, r, err := s.DeployContract(ca, acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Equal(t, "", r.Status.Message)

	r, err = s.Call(cname, "init", `[]`, acc.ID, acc.KeyPair)
	assert.Nil(t, err)
	assert.Equal(t, "", r.Status.Message)

	s.Head.Time += 1000 // for different cname

	conf.P2P.ChainID = 0
	version.InitChainConf(conf)
	rules = version.NewRules(0)
	assert.True(t, rules.IsFork3_3_0)
	s.Visitor = database.NewVisitor(0, s.Mvcc, rules)

	ca, err = s.Compile("", "./test_data/v320", "./test_data/v320.js")
	assert.Nil(t, err)
	assert.NotNil(t, ca)

	cname, r, err = s.DeployContract(ca, acc0.ID, acc0.KeyPair)
	assert.Nil(t, err)
	assert.Equal(t, "", r.Status.Message)

	r, err = s.Call(cname, "init", `[]`, acc.ID, acc.KeyPair)
	assert.Nil(t, err)
	assert.Contains(t, r.Status.Message, "cannot call 'init' manually")
}
