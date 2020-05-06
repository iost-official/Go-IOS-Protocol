package genesis

import (
	"encoding/json"
	"fmt"
	"github.com/iost-official/go-iost/common/config"
	"path/filepath"
	"time"

	"github.com/iost-official/go-iost/account"
	"github.com/iost-official/go-iost/core/block"
	"github.com/iost-official/go-iost/core/contract"
	"github.com/iost-official/go-iost/core/tx"
	"github.com/iost-official/go-iost/crypto"
	"github.com/iost-official/go-iost/db"
	"github.com/iost-official/go-iost/ilog"
	"github.com/iost-official/go-iost/verifier"
	"github.com/iost-official/go-iost/vm/native"
)

// GenesisTxExecTime is the maximum execution time of a transaction in genesis block
var GenesisTxExecTime = 10 * time.Second

// GenGenesisByFile is create a genesis block by config file
func GenGenesisByFile(db db.MVCCDB, path string) (*block.Block, error) {
	v := config.LoadYamlAsViper(filepath.Join(path, "genesis.yml"))
	genesisConfig := &config.GenesisConfig{}
	if err := v.Unmarshal(genesisConfig); err != nil {
		ilog.Fatalf("Unable to decode into struct, %v", err)
	}
	genesisConfig.ContractPath = filepath.Join(path, "contract")
	return GenGenesis(db, genesisConfig)
}

func compile(id string, path string, name string) (*contract.Contract, error) {
	if id == "" || path == "" || name == "" {
		return nil, fmt.Errorf("arguments is error, id:%v, path:%v, name:%v", id, path, name)
	}
	cFilePath := filepath.Join(path, name)
	cAbiPath := filepath.Join(path, name+".abi")
	return contract.Compile(id, cFilePath, cAbiPath)
}

func genGenesisTx(gConf *config.GenesisConfig) (*tx.Tx, *account.Account, error) {
	witnessInfo := gConf.WitnessInfo
	// prepare actions
	var acts []*tx.Action
	adminInfo := gConf.AdminInfo

	// deploy token.iost
	acts = append(acts, tx.NewAction("system.iost", "initSetCode",
		fmt.Sprintf(`["%v", "%v"]`, "token.iost", native.SystemContractABI("token.iost", "1.0.0").B64Encode())))
	acts = append(acts, tx.NewAction("system.iost", "initSetCode",
		fmt.Sprintf(`["%v", "%v"]`, "token721.iost", native.SystemContractABI("token721.iost", "1.0.0").B64Encode())))
	// deploy iost.gas
	acts = append(acts, tx.NewAction("system.iost", "initSetCode",
		fmt.Sprintf(`["%v", "%v"]`, "gas.iost", native.SystemContractABI("gas.iost", "1.0.0").B64Encode())))
	// deploy issue.iost and create iost
	code, err := compile("issue.iost", gConf.ContractPath, "issue.js")
	if err != nil {
		return nil, nil, err
	}
	acts = append(acts, tx.NewAction("system.iost", "initSetCode", fmt.Sprintf(`["%v", "%v"]`, "issue.iost", code.B64Encode())))
	tokenInfo := gConf.TokenInfo
	tokenHolder := append(witnessInfo, adminInfo)
	params := []interface{}{
		adminInfo.ID,
		tokenInfo,
		tokenHolder,
	}
	b, _ := json.Marshal(params)
	acts = append(acts, tx.NewAction("issue.iost", "initGenesis", string(b)))
	// deploy account.iost
	code, err = compile("auth.iost", gConf.ContractPath, "account.js")
	if err != nil {
		return nil, nil, err
	}
	acts = append(acts, tx.NewAction("system.iost", "initSetCode", fmt.Sprintf(`["%v", "%v"]`, "auth.iost", code.B64Encode())))
	acts = append(acts, tx.NewAction("auth.iost", "initAdmin", fmt.Sprintf(`["%v"]`, adminInfo.ID)))

	// deploy domain.iost
	acts = append(acts, tx.NewAction("system.iost", "initSetCode",
		fmt.Sprintf(`["%v", "%v"]`, "domain.iost", native.SystemContractABI("domain.iost", "0.0.0").B64Encode())))

	// new account
	acts = append(acts, tx.NewAction("auth.iost", "signUp", fmt.Sprintf(`["%v", "%v", "%v"]`, adminInfo.ID, adminInfo.Owner, adminInfo.Active)))
	// new account
	foundationInfo := gConf.FoundationInfo
	acts = append(acts, tx.NewAction("auth.iost", "signUp", fmt.Sprintf(`["%v", "%v", "%v"]`, foundationInfo.ID, foundationInfo.Owner, foundationInfo.Active)))

	for _, v := range witnessInfo {
		acts = append(acts, tx.NewAction("auth.iost", "signUp", fmt.Sprintf(`["%v", "%v", "%v"]`, v.ID, v.Owner, v.Active)))
	}
	invalidPubKey := "0"
	deadAccount := account.NewAccount("deadaddr")
	acts = append(acts, tx.NewAction("auth.iost", "signUp", fmt.Sprintf(`["%v", "%v", "%v"]`, deadAccount.ID, invalidPubKey, invalidPubKey)))

	// deploy bonus.iost
	code, err = compile("bonus.iost", gConf.ContractPath, "bonus.js")
	if err != nil {
		return nil, nil, err
	}
	acts = append(acts, tx.NewAction("system.iost", "initSetCode", fmt.Sprintf(`["%v", "%v"]`, "bonus.iost", code.B64Encode())))
	acts = append(acts, tx.NewAction("bonus.iost", "initAdmin", fmt.Sprintf(`["%v"]`, adminInfo.ID)))

	// deploy vote.iost
	code, err = compile("vote.iost", gConf.ContractPath, "vote_common.js")
	if err != nil {
		return nil, nil, err
	}
	acts = append(acts, tx.NewAction("system.iost", "initSetCode", fmt.Sprintf(`["%v", "%v"]`, "vote.iost", code.B64Encode())))
	acts = append(acts, tx.NewAction("vote.iost", "initAdmin", fmt.Sprintf(`["%v"]`, adminInfo.ID)))

	// deploy vote_producer.iost
	code, err = compile("vote_producer.iost", gConf.ContractPath, "vote_producer.js")
	if err != nil {
		return nil, nil, err
	}
	acts = append(acts, tx.NewAction("system.iost", "initSetCode", fmt.Sprintf(`["%v", "%v"]`, "vote_producer.iost", code.B64Encode())))
	acts = append(acts, tx.NewAction("vote_producer.iost", "initAdmin", fmt.Sprintf(`["%v"]`, adminInfo.ID)))

	// deploy base.iost
	code, err = compile("base.iost", gConf.ContractPath, "base.js")
	if err != nil {
		return nil, nil, err
	}
	acts = append(acts, tx.NewAction("system.iost", "initSetCode", fmt.Sprintf(`["%v", "%v"]`, "base.iost", code.B64Encode())))
	acts = append(acts, tx.NewAction("base.iost", "initAdmin", fmt.Sprintf(`["%v"]`, adminInfo.ID)))

	// deploy exchange.iost
	code, err = compile("exchange.iost", gConf.ContractPath, "exchange.js")
	if err != nil {
		return nil, nil, err
	}
	acts = append(acts, tx.NewAction("system.iost", "initSetCode", fmt.Sprintf(`["%v", "%v"]`, "exchange.iost", code.B64Encode())))

	for _, v := range witnessInfo {
		acts = append(acts, tx.NewAction("vote_producer.iost", "initProducer", fmt.Sprintf(`["%v", "%v"]`, v.ID, v.SignatureBlock)))
	}

	// pledge gas for admin
	gasPledgeAmount := 100
	acts = append(acts, tx.NewAction("gas.iost", "pledge", fmt.Sprintf(`["%v", "%v", "%v"]`, adminInfo.ID, adminInfo.ID, gasPledgeAmount)))

	// deploy ram.iost
	code, err = compile("ram.iost", gConf.ContractPath, "ram.js")
	if err != nil {
		return nil, nil, err
	}
	acts = append(acts, tx.NewAction("system.iost", "initSetCode", fmt.Sprintf(`["%v", "%v"]`, "ram.iost", code.B64Encode())))
	acts = append(acts, tx.NewAction("ram.iost", "initAdmin", fmt.Sprintf(`["%v"]`, adminInfo.ID)))
	var initialTotal int64 = 128 * 1024 * 1024 * 1024                           // 128GB at first
	var increaseInterval int64 = 10 * 60                                        // increase every 10 mins
	var increaseAmount int64 = 10 * (64 * 1024 * 1024 * 1024) / (365 * 24 * 60) // 64GB per year
	var reserveRAM = initialTotal * 3 / 10                                      // reserve for foundation
	acts = append(acts, tx.NewAction("ram.iost", "issue", fmt.Sprintf(`[%v, %v, %v, %v]`, initialTotal, increaseInterval, increaseAmount, reserveRAM)))

	adminInitialRAM := 100000
	acts = append(acts, tx.NewAction("ram.iost", "buy", fmt.Sprintf(`["%v", "%v", %v]`, adminInfo.ID, adminInfo.ID, adminInitialRAM)))
	acts = append(acts, tx.NewAction("token.iost", "transfer", fmt.Sprintf(`["ram","ram.iost", "%v", "%v", ""]`, foundationInfo.ID, reserveRAM)))

	for _, v := range witnessInfo {
		acts = append(acts, tx.NewAction("ram.iost", "buy", fmt.Sprintf(`["%v", "%v", %v]`, adminInfo.ID, v.ID, adminInitialRAM)))
	}

	acts = append(acts, tx.NewAction("gas.iost", "pledge", fmt.Sprintf(`["%v", "%v", "%v"]`, adminInfo.ID, foundationInfo.ID, gasPledgeAmount)))
	for _, v := range witnessInfo {
		acts = append(acts, tx.NewAction("gas.iost", "pledge", fmt.Sprintf(`["%v", "%v", "%v"]`, adminInfo.ID, v.ID, gasPledgeAmount)))
	}

	trx := tx.NewTx(acts, nil, 1000000000, 100, 0, 0, tx.ChainID)
	trx.Time = 0
	trx, err = tx.SignTx(trx, deadAccount.ID, []*account.KeyPair{})
	if err != nil {
		return nil, nil, err
	}
	trx.AmountLimit = append(trx.AmountLimit, &contract.Amount{Token: "*", Val: "unlimited"})
	return trx, deadAccount, nil
}

// GenGenesis is create a genesis block
func GenGenesis(db db.MVCCDB, gConf *config.GenesisConfig) (*block.Block, error) {
	t, err := time.Parse(time.RFC3339, gConf.InitialTimestamp)
	if err != nil {
		ilog.Fatalf("invalid genesis initial time string %v (%v).", gConf.InitialTimestamp, err)
		return nil, err
	}
	trx, _, err := genGenesisTx(gConf)
	if err != nil {
		return nil, err
	}

	blockHead := block.BlockHead{
		Version:    block.V0,
		ParentHash: nil,
		Number:     0,
		Witness:    "0",
		Time:       t.UnixNano(),
	}
	v := verifier.Verifier{}
	txr, err := v.Exec(&blockHead, db, trx, GenesisTxExecTime)
	if err != nil || txr.Status.Code != tx.Success {
		return nil, fmt.Errorf("exec tx failed, stop the pogram. err: %v, receipt: %v", err, txr)
	}
	blk := &block.Block{
		Head:     &blockHead,
		Sign:     &crypto.Signature{},
		Txs:      []*tx.Tx{trx},
		Receipts: []*tx.TxReceipt{txr},
	}
	blk.Head.TxMerkleHash = blk.CalculateTxMerkleHash()
	blk.Head.TxReceiptMerkleHash = blk.CalculateTxReceiptMerkleHash()
	blk.CalculateHeadHash()
	db.Commit(string(blk.HeadHash()))
	return blk, nil
}
