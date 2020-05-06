package run

import (
	"github.com/iost-official/go-iost/account"
	"github.com/iost-official/go-iost/core/global"
	"github.com/iost-official/go-iost/crypto"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fmt"
	"github.com/iost-official/go-iost/core/tx"
	"github.com/iost-official/go-iost/ilog"
	"github.com/iost-official/go-iost/itest"
	"github.com/urfave/cli"
	"math/rand"
)

// BenchmarkAccountCommand is the subcommand for benchmark.
var BenchmarkAccountCommand = cli.Command{
	Name:      "benchmarkAccount",
	ShortName: "benchA",
	Usage:     "Run account benchmark by given tps",
	Flags:     BenchmarkAccountFlags,
	Action:    BenchmarkAccountAction,
}

// BenchmarkAccountFlags is the list of flags for benchmark.
var BenchmarkAccountFlags = []cli.Flag{
	cli.IntFlag{
		Name:  "tps",
		Value: 20,
		Usage: "The expected ratio of transactions per second",
	},
	cli.BoolFlag{
		Name:  "check",
		Usage: "if check receipt",
	},
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randStr(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func generateAccountTxs(it *itest.ITest, accounts []*itest.Account, tps int) ([]*itest.Transaction, error) { // nolint: gocyclo
	contractName := "auth.iost"
	trxs := make([]*itest.Transaction, 0)
	for num := 0; num < tps; num++ {
		//tIndex := rand.Intn(15) // signUp 1/3, each other 1/15
		tIndex := 5 + rand.Intn(10)
		switch true {
		case tIndex < 5:
			// signUp
			from := accounts[rand.Intn(len(accounts))]
			act11 := tx.NewAction("token.iost", "transfer", fmt.Sprintf(`["%v","%v","%v","%v",""]`, global.Token, "admin", from.ID, 10))
			act12 := tx.NewAction("gas.iost", "pledge", fmt.Sprintf(`["%v","%v","%v"]`, "admin", from.ID, 10))
			trx1, err := it.GetDefaultAccount().Sign(itest.NewTransaction([]*tx.Action{act11, act12}))
			if err != nil {
				return nil, err
			}
			trxs = append(trxs, trx1)
			kp, _ := account.NewKeyPair(nil, crypto.Ed25519)
			k := kp.ReadablePubkey()
			randName := fmt.Sprintf("acc%08d", rand.Int63n(100000000))
			act2 := tx.NewAction(contractName, "signUp", fmt.Sprintf(`["%v","%v","%v"]`, randName, k, k))
			trx2, err := from.Sign(itest.NewTransaction([]*tx.Action{act2}))
			if err != nil {
				return nil, err
			}
			trxs = append(trxs, trx2)
		case tIndex < 7:
			// addPermission and dropPermission
			from := accounts[rand.Intn(len(accounts))]
			randPerm := randStr(10)
			act1 := tx.NewAction(contractName, "addPermission", fmt.Sprintf(`["%v","%v",%v]`, from.ID, randPerm, 10+rand.Intn(50)))
			//trx1, err := from.Sign(itest.NewTransaction([]*tx.Action{act1}))
			//if err != nil {
			//	return nil, err
			//}
			//trxs = append(trxs, trx1)
			act2 := tx.NewAction(contractName, "dropPermission", fmt.Sprintf(`["%v","%v"]`, from.ID, randPerm))
			trx2, err := from.Sign(itest.NewTransaction([]*tx.Action{act1, act2}))
			if err != nil {
				return nil, err
			}
			trxs = append(trxs, trx2)
		case tIndex < 9:
			// assignPermission and revokePermission
			from := accounts[rand.Intn(len(accounts))]
			kp, _ := account.NewKeyPair(nil, crypto.Ed25519)
			k := kp.ReadablePubkey()
			act1 := tx.NewAction(contractName, "assignPermission", fmt.Sprintf(`["%v","%v","%v",%v]`, from.ID, "active", k, 10+rand.Intn(50)))
			//trx1, err := from.Sign(itest.NewTransaction([]*tx.Action{act1}))
			//if err != nil {
			//	return nil, err
			//}
			//trxs = append(trxs, trx1)
			act2 := tx.NewAction(contractName, "revokePermission", fmt.Sprintf(`["%v","%v","%v"]`, from.ID, "active", k))
			trx2, err := from.Sign(itest.NewTransaction([]*tx.Action{act1, act2}))
			if err != nil {
				return nil, err
			}
			trxs = append(trxs, trx2)
		case tIndex < 11:
			// addGroup and dropGroup
			from := accounts[rand.Intn(len(accounts))]
			randGroup := randStr(10)
			act1 := tx.NewAction(contractName, "addGroup", fmt.Sprintf(`["%v","%v"]`, from.ID, randGroup))
			//trx1, err := from.Sign(itest.NewTransaction([]*tx.Action{act1}))
			//if err != nil {
			//	return nil, err
			//}
			//trxs = append(trxs, trx1)
			act2 := tx.NewAction(contractName, "dropGroup", fmt.Sprintf(`["%v","%v"]`, from.ID, randGroup))
			trx2, err := from.Sign(itest.NewTransaction([]*tx.Action{act1, act2}))
			if err != nil {
				return nil, err
			}
			trxs = append(trxs, trx2)
		case tIndex < 13:
			// assignGroup and revokeGroup
			from := accounts[rand.Intn(len(accounts))]
			randGroup := randStr(10)
			kp, _ := account.NewKeyPair(nil, crypto.Ed25519)
			k := kp.ReadablePubkey()
			act11 := tx.NewAction(contractName, "addGroup", fmt.Sprintf(`["%v","%v"]`, from.ID, randGroup))
			act12 := tx.NewAction(contractName, "assignGroup", fmt.Sprintf(`["%v","%v","%v",%v]`, from.ID, randGroup, k, 30))
			//trx1, err := from.Sign(itest.NewTransaction([]*tx.Action{act11, act12}))
			//if err != nil {
			//	return nil, err
			//}
			//trxs = append(trxs, trx1)
			act2 := tx.NewAction(contractName, "revokeGroup", fmt.Sprintf(`["%v","%v","%v"]`, from.ID, randGroup, k))
			trx2, err := from.Sign(itest.NewTransaction([]*tx.Action{act11, act12, act2}))
			if err != nil {
				return nil, err
			}
			trxs = append(trxs, trx2)
		case tIndex < 15:
			// assignPermissionToGroup and revokePermissionInGroup
			from := accounts[rand.Intn(len(accounts))]
			randGroup := randStr(10)
			randPerm := randStr(10)
			act11 := tx.NewAction(contractName, "addGroup", fmt.Sprintf(`["%v","%v"]`, from.ID, randGroup))
			act12 := tx.NewAction(contractName, "addPermission", fmt.Sprintf(`["%v","%v",%v]`, from.ID, randPerm, 10+rand.Intn(50)))
			act13 := tx.NewAction(contractName, "assignPermissionToGroup", fmt.Sprintf(`["%v","%v","%v"]`, from.ID, randPerm, randGroup))
			//trx1, err := from.Sign(itest.NewTransaction([]*tx.Action{act11, act12, act13}))
			//if err != nil {
			//	return nil, err
			//}
			//trxs = append(trxs, trx1)
			act2 := tx.NewAction(contractName, "revokePermissionInGroup", fmt.Sprintf(`["%v","%v","%v"]`, from.ID, randPerm, randGroup))
			trx2, err := from.Sign(itest.NewTransaction([]*tx.Action{act11, act12, act13, act2}))
			if err != nil {
				return nil, err
			}
			trxs = append(trxs, trx2)
		}
	}
	return trxs, nil
}

// BenchmarkAccountAction is the action of benchmark.
var BenchmarkAccountAction = func(c *cli.Context) error {
	rand.Seed(time.Now().UTC().UnixNano())
	itest.Interval = 1000 * time.Millisecond
	itest.InitAmount = "1000"
	itest.InitPledge = "1000"
	itest.InitRAM = "3000"
	//ilog.SetLevel(ilog.LevelDebug)
	it, err := itest.Load(c.GlobalString("keys"), c.GlobalString("config"))
	if err != nil {
		return err
	}
	accountFile := c.GlobalString("account")
	accounts, err := itest.LoadAccounts(accountFile)
	if err != nil {
		ilog.Warnf("load accounts from %v failed, creating...%v", accountFile, err)
		if err := AccountCaseAction(c); err != nil {
			return err
		}
		if accounts, err = itest.LoadAccounts(accountFile); err != nil {
			return err
		}
	}
	ilog.Infof("accounts num %v", len(accounts))
	tps := c.Int("tps")
	ilog.Infof("target tps %v", tps)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	startTime := time.Now()
	ticker := time.NewTicker(time.Second)
	counter := 0
	total := 0
	slotTotal := 0
	slotStartTime := startTime

	checkReceiptConcurrent := 64

	hashCh := make(chan *hashItem, 4*tps*int(itest.Timeout.Seconds()))
	for c := 0; c < checkReceiptConcurrent; c++ {
		go func(hashCh chan *hashItem) {
			counter := 0
			failedCounter := 0
			for item := range hashCh {
				client := it.GetClients()[rand.Intn(len(it.GetClients()))]
				_, err := client.CheckTransactionWithTimeout(item.hash, item.expire)
				counter++
				if err != nil {
					ilog.Errorf("check transaction failed, %v", err)
					failedCounter++
				}

				if counter%1000 == 0 {
					ilog.Warnf("check %v transaction, %v successful, %v failed.", counter, counter-failedCounter, failedCounter)
				}
				if len(hashCh) > 3*tps*int(itest.Timeout.Seconds()) {
					ilog.Infof("hash ch size too large %v", len(hashCh))
				}
			}
		}(hashCh)
	}
	check := c.Bool("check")

	for {
		trxs, err := generateAccountTxs(it, accounts, tps)
		if err != nil {
			ilog.Errorf("generateAccountTxs error %v", err)
			continue
		}
		hashList, errList := it.SendTransactionN(trxs, false)
		ilog.Warnf("Send %v trxs, got %v hash, %v err", len(trxs), len(hashList), len(errList))

		if check {
			expire := time.Now().Add(itest.Timeout)
			for _, hash := range hashList {
				select {
				case hashCh <- &hashItem{hash: hash, expire: expire}:
				case <-time.After(1 * time.Millisecond):
				}
			}
		}

		select {
		case <-sig:
			return fmt.Errorf("signal %v", sig)
		case <-ticker.C:
		}

		counter++
		slotTotal += len(trxs)
		if counter == 10 {
			total += slotTotal
			currentTps := float64(slotTotal) / time.Since(slotStartTime).Seconds()
			averageTps := float64(total) / time.Since(startTime).Seconds()
			ilog.Warnf("Current tps %v, Average tps %v, Total tx %v", currentTps, averageTps, total)
			counter = 0
			slotTotal = 0
			slotStartTime = time.Now()
		}
	}
}
