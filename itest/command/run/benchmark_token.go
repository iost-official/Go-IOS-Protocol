package run

import (
	"github.com/iost-official/go-iost/core/global"
	"os"
	"os/signal"
	"syscall"
	"time"

	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"sync"

	"github.com/iost-official/go-iost/core/tx"
	"github.com/iost-official/go-iost/ilog"
	"github.com/iost-official/go-iost/itest"
	"github.com/urfave/cli"
)

// BenchmarkTokenCommand is the subcommand for benchmark.
var BenchmarkTokenCommand = cli.Command{
	Name:      "benchmarkToken",
	ShortName: "benchT",
	Usage:     "Run token benchmark by given tps",
	Flags:     BenchmarkTokenFlags,
	Action:    BenchmarkTokenAction,
}

// BenchmarkTokenFlags is the list of flags for benchmark.
var BenchmarkTokenFlags = []cli.Flag{
	cli.IntFlag{
		Name:  "tps",
		Value: 50,
		Usage: "The expected ratio of transactions per second",
	},
	cli.BoolFlag{
		Name:  "check",
		Usage: "if check receipt",
	},
}

const (
	createToken         = "create"
	issueToken          = "issue"
	transferToken       = "transfer"
	transferFreezeToken = "transferFreeze"
	destroyToken        = "destroy"
	balanceOfToken      = "balanceOf"
	supplyToken         = "supply"
	totalSupplyToken    = "totalSupply"
)

type tokenInfo struct {
	sym     string
	issuer  string
	balance map[string]float64
	acclist []string
}

type hashItem struct {
	hash   string
	expire time.Time
}

// BenchmarkTokenAction is the action of benchmark.
var BenchmarkTokenAction = func(c *cli.Context) error {
	itest.Interval = 1000 * time.Millisecond
	itest.InitAmount = "1000"
	itest.InitPledge = "1000"
	itest.InitRAM = "3000"
	logger := ilog.New()
	fileWriter := ilog.NewFileWriter(c.GlobalString("log"))
	fileWriter.SetLevel(ilog.LevelInfo)
	logger.AddWriter(fileWriter)
	ilog.InitLogger(logger)

	//ilog.SetLevel(ilog.LevelDebug)
	it, err := itest.Load(c.GlobalString("keys"), c.GlobalString("config"))
	if err != nil {
		return err
	}
	accountFile := c.GlobalString("account")
	t0 := time.Now()
	accounts, err := itest.LoadAccounts(accountFile)
	if err != nil {
		if err := AccountCaseAction(c); err != nil {
			return err
		}
		if accounts, err = itest.LoadAccounts(accountFile); err != nil {
			return err
		}
	}
	t1 := time.Now()
	ilog.Warnf("load account time: %v, got %v", float64(t1.UnixNano()-t0.UnixNano())/1e9, len(accounts))
	accountMap := make(map[string]*itest.Account)
	for _, acc := range accounts {
		accountMap[acc.ID] = acc
	}
	tps := c.Int("tps")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	startTime := time.Now()
	ticker := time.NewTicker(time.Second)
	counter := 0
	total := 0
	slotTotal := 0
	slotStartTime := startTime

	tokenList := []string{global.Token}
	tokenMap := make(map[string]*tokenInfo)
	tokenMap[global.Token] = &tokenInfo{
		sym:     global.Token,
		issuer:  "",
		balance: make(map[string]float64),
		acclist: make([]string, 0),
	}
	for _, acc := range accounts {
		tokenMap[global.Token].balance[acc.ID] = acc.Balance()
		tokenMap[global.Token].acclist = append(tokenMap[global.Token].acclist, acc.ID)
	}
	tokenPrefix := "t" + strconv.FormatInt(time.Now().UnixNano(), 10)[14:]
	checkReceiptConcurrent := 64
	tokenOffset := 0
	var tokenMutex sync.Mutex

	hashCh := make(chan *hashItem, 4*tps*int(itest.Timeout.Seconds()))
	for c := 0; c < checkReceiptConcurrent; c++ {
		go func(hashCh chan *hashItem) {
			counter := 0
			failedCounter := 0
			for item := range hashCh {
				client := it.GetClients()[rand.Intn(len(it.GetClients()))]
				r, err := client.CheckTransactionWithTimeout(item.hash, item.expire)
				counter++
				if err != nil {
					ilog.Errorf("check transaction failed, %v", err)
					failedCounter++
				} else {
					for i := 0; i < len(r.Receipts); i++ {
						if r.Receipts[i].FuncName == "token.iost/issue" {
							args := make([]string, 3)
							err := json.Unmarshal([]byte(r.Receipts[i].Content), &args)
							if err != nil {
								continue
							}
							ilog.Debugf("got receipt %v %v", r.Receipts[i], args)
							tokenMutex.Lock()
							tokenSym := args[0]
							if !strings.HasPrefix(tokenSym, tokenPrefix) {
								tokenMutex.Unlock()
								continue
							}
							acc := args[1]
							amountStr := args[2]
							amount, _ := strconv.ParseFloat(amountStr, 32)
							if _, ok := tokenMap[tokenSym].balance[acc]; !ok {
								tokenMap[tokenSym].acclist = append(tokenMap[tokenSym].acclist, acc)
							}
							tokenMap[tokenSym].balance[acc] += amount
							tokenMutex.Unlock()
							break
						} else if r.Receipts[i].FuncName == "token.iost/create" {
							args := make([]interface{}, 4)
							err := json.Unmarshal([]byte(r.Receipts[i].Content), &args)
							if err != nil {
								continue
							}
							ilog.Debugf("got receipt %v %v", r.Receipts[i], args)
							tokenMutex.Lock()
							tokenSym := args[0].(string)
							issuer := args[1].(string)
							tokenList = append(tokenList, tokenSym)
							tokenMap[tokenSym] = &tokenInfo{
								sym:     tokenSym,
								issuer:  issuer,
								balance: make(map[string]float64),
								acclist: []string{},
							}
							tokenMutex.Unlock()
							break
						}
					}
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
	contractName := "token.iost"
	for {
		trxs := make([]*itest.Transaction, 0)
		errList := []error{}
		tokenMutex.Lock()
		for num := 0; num < tps; num++ {
			// create 1, issue 1000, transfer 10000, transferFreeze 2000, destroy 100, balanceOf 100, supply 10, totalSupply 10
			tIndex := rand.Intn(10000)
			var abiName string
			switch true {
			case tIndex <= 0 || len(tokenList) < 5:
				abiName = createToken
				tokenSym := tokenPrefix + strconv.FormatInt(int64(tokenOffset), 10)
				tokenOffset++
				from := accounts[rand.Intn(len(accounts))]
				decimal := rand.Intn(5) + 2

				act0 := tx.NewAction("ram.iost", "buy", fmt.Sprintf(`["%v", "%v", %v]`, "admin", from.ID, 10000))
				act1 := tx.NewAction("gas.iost", "pledge", fmt.Sprintf(`["%v", "%v", "%v"]`, "admin", from.ID, 10))
				tx0 := itest.NewTransaction([]*tx.Action{act0, act1})
				trx, err := it.GetDefaultAccount().Sign(tx0)
				if err != nil {
					errList = append(errList, err)
				} else {
					trxs = append(trxs, trx)
				}

				act1 = tx.NewAction(contractName, abiName, fmt.Sprintf(`["%v", "%v", %v, %v]`, tokenSym, from.ID, 100000000000, fmt.Sprintf(`{"decimal": %v}`, decimal)))
				tx1 := itest.NewTransaction([]*tx.Action{act1})
				trx, err = from.Sign(tx1)
				if err != nil {
					errList = append(errList, err)
				} else {
					trxs = append(trxs, trx)
				}
			case tIndex <= 10:
				abiName = supplyToken
				ilog.Infof("supply")
				tokenSym := tokenList[rand.Intn(len(tokenList))]
				from := accounts[rand.Intn(len(accounts))]
				act1 := tx.NewAction("gas.iost", "pledge", fmt.Sprintf(`["%v", "%v", "%v"]`, "admin", from.ID, 10))
				tx0 := itest.NewTransaction([]*tx.Action{act1})
				trx, err := it.GetDefaultAccount().Sign(tx0)
				if err != nil {
					errList = append(errList, err)
				} else {
					trxs = append(trxs, trx)
				}

				act1 = tx.NewAction(contractName, abiName, fmt.Sprintf(`["%v"]`, tokenSym))
				tx1 := itest.NewTransaction([]*tx.Action{act1})
				trx, err = from.Sign(tx1) //if _, ok := tokenMap[tokenSym].balance[to.ID]; !ok {
				//	tokenMap[tokenSym].acclist = append(tokenMap[tokenSym].acclist, to.ID)
				//}
				//tokenMap[tokenSym].balance[to.ID] += amount
				if err != nil {
					errList = append(errList, err)
				} else {
					trxs = append(trxs, trx)
				}
			case tIndex <= 20:
				abiName = totalSupplyToken
				ilog.Infof("total supply")
				tokenSym := tokenList[rand.Intn(len(tokenList))]
				from := accounts[rand.Intn(len(accounts))]
				act1 := tx.NewAction("gas.iost", "pledge", fmt.Sprintf(`["%v", "%v", "%v"]`, "admin", from.ID, 10))
				tx0 := itest.NewTransaction([]*tx.Action{act1})
				trx, err := it.GetDefaultAccount().Sign(tx0)
				if err != nil {
					errList = append(errList, err)
				} else {
					trxs = append(trxs, trx)
				}

				act1 = tx.NewAction(contractName, abiName, fmt.Sprintf(`["%v"]`, tokenSym))
				tx1 := itest.NewTransaction([]*tx.Action{act1})
				trx, err = from.Sign(tx1)
				if err != nil {
					errList = append(errList, err)
				} else {
					trxs = append(trxs, trx)
				}
			case tIndex <= 120:
				abiName = balanceOfToken
				tokenSym := tokenList[rand.Intn(len(tokenList))]
				from := accounts[rand.Intn(len(accounts))]
				act1 := tx.NewAction("gas.iost", "pledge", fmt.Sprintf(`["%v", "%v", "%v"]`, "admin", from.ID, 10))
				tx0 := itest.NewTransaction([]*tx.Action{act1})
				trx, err := it.GetDefaultAccount().Sign(tx0)
				if err != nil {
					errList = append(errList, err)
				} else {
					trxs = append(trxs, trx)
				}

				act1 = tx.NewAction(contractName, abiName, fmt.Sprintf(`["%v", "%v"]`, tokenSym, from.ID))
				tx1 := itest.NewTransaction([]*tx.Action{act1})
				trx, err = from.Sign(tx1)
				if err != nil {
					errList = append(errList, err)
				} else {
					trxs = append(trxs, trx)
				}
			case tIndex <= 220:
				abiName = destroyToken
				tokenSym := tokenList[rand.Intn(len(tokenList))]
				if len(tokenMap[tokenSym].balance) == 0 {
					tokenSym = global.Token
				}
				from := accountMap[tokenMap[tokenSym].acclist[rand.Intn(len(tokenMap[tokenSym].acclist))]]
				balance := tokenMap[tokenSym].balance[from.ID]
				act1 := tx.NewAction("gas.iost", "pledge", fmt.Sprintf(`["%v", "%v", "%v"]`, "admin", from.ID, 10))
				tx0 := itest.NewTransaction([]*tx.Action{act1})
				trx, err := it.GetDefaultAccount().Sign(tx0)
				if err != nil {
					errList = append(errList, err)
				} else {
					trxs = append(trxs, trx)
				}

				amount := math.Max(float64(rand.Intn(int(math.Max(balance, 1))))/100.0, 0.01)
				act1 = tx.NewAction(contractName, abiName, fmt.Sprintf(`["%v", "%v", "%v"]`, tokenSym, from.ID, amount))
				tx1 := itest.NewTransaction([]*tx.Action{act1})
				trx, err = from.Sign(tx1)
				if err != nil {
					errList = append(errList, err)
				} else {
					trxs = append(trxs, trx)
					tokenMap[tokenSym].balance[from.ID] -= amount
				}
			case tIndex <= 1000:
				abiName = issueToken
				if len(tokenList) == 1 {
					break
				}
				tokenSym := tokenList[1+rand.Intn(len(tokenList)-1)]
				issuer := accountMap[tokenMap[tokenSym].issuer]
				to := accounts[rand.Intn(len(accounts))]
				act0 := tx.NewAction("ram.iost", "buy", fmt.Sprintf(`["%v", "%v", %v]`, "admin", issuer.ID, 1000))
				act1 := tx.NewAction("gas.iost", "pledge", fmt.Sprintf(`["%v", "%v", "%v"]`, "admin", issuer.ID, 10))
				tx0 := itest.NewTransaction([]*tx.Action{act0, act1})
				trx, err := it.GetDefaultAccount().Sign(tx0)
				if err != nil {
					errList = append(errList, err)
				} else {
					trxs = append(trxs, trx)
				}
				amount := 1000 + 1000*rand.Float64()
				act1 = tx.NewAction(contractName, abiName, fmt.Sprintf(`["%v", "%v", "%v"]`, tokenSym, to.ID, int64(amount)))
				tx1 := itest.NewTransaction([]*tx.Action{act1})
				trx, err = issuer.Sign(tx1)
				if err != nil {
					errList = append(errList, err)
				} else {
					trxs = append(trxs, trx)
				}
			case tIndex <= 1100:
				abiName = transferFreezeToken
				tokenSym := tokenList[rand.Intn(len(tokenList))]
				if len(tokenMap[tokenSym].balance) == 0 {
					tokenSym = global.Token
				}
				from := accountMap[tokenMap[tokenSym].acclist[rand.Intn(len(tokenMap[tokenSym].acclist))]]
				to := accounts[rand.Intn(len(accounts))]
				balance := tokenMap[tokenSym].balance[from.ID]
				act0 := tx.NewAction("ram.iost", "buy", fmt.Sprintf(`["%v", "%v", %v]`, "admin", from.ID, 10000))
				act1 := tx.NewAction("gas.iost", "pledge", fmt.Sprintf(`["%v", "%v", "%v"]`, "admin", from.ID, 10))
				tx0 := itest.NewTransaction([]*tx.Action{act0, act1})
				trx, err := it.GetDefaultAccount().Sign(tx0)
				if err != nil {
					errList = append(errList, err)
				} else {
					trxs = append(trxs, trx)
				}

				ftime := time.Now().UnixNano() + int64(rand.Intn(100))*1e9
				amount := math.Max(float64(rand.Intn(int(math.Max(balance, 1))))/100.0, 0.01)
				act1 = tx.NewAction(contractName, abiName, fmt.Sprintf(`["%v", "%v", "%v", "%v", %v, "%v"]`, tokenSym, from.ID, to.ID, amount, ftime, ""))
				tx1 := itest.NewTransaction([]*tx.Action{act1})
				trx, err = from.Sign(tx1)
				if err != nil {
					errList = append(errList, err)
				} else {
					trxs = append(trxs, trx)
					tokenMap[tokenSym].balance[from.ID] -= amount
				}
			default:
				abiName = transferToken
				tokenSym := tokenList[rand.Intn(len(tokenList))]
				if len(tokenMap[tokenSym].balance) == 0 {
					tokenSym = global.Token
				}
				from := accountMap[tokenMap[tokenSym].acclist[rand.Intn(len(tokenMap[tokenSym].acclist))]]
				to := accounts[rand.Intn(len(accounts))]
				balance := tokenMap[tokenSym].balance[from.ID]
				act0 := tx.NewAction("ram.iost", "buy", fmt.Sprintf(`["%v", "%v", %v]`, "admin", from.ID, 100))
				act1 := tx.NewAction("gas.iost", "pledge", fmt.Sprintf(`["%v", "%v", "%v"]`, "admin", from.ID, 10))
				tx0 := itest.NewTransaction([]*tx.Action{act0, act1})
				trx, err := it.GetDefaultAccount().Sign(tx0)
				if err != nil {
					errList = append(errList, err)
				} else {
					trxs = append(trxs, trx)
				}

				amount := math.Max(float64(rand.Intn(int(math.Max(balance, 1))))/100.0, 0.01)
				act1 = tx.NewAction(contractName, abiName, fmt.Sprintf(`["%v", "%v", "%v", "%v", "%v"]`, tokenSym, from.ID, to.ID, amount, ""))
				tx1 := itest.NewTransaction([]*tx.Action{act1})
				trx, err = from.Sign(tx1)
				if err != nil {
					errList = append(errList, err)
				} else {
					trxs = append(trxs, trx)
					tokenMap[tokenSym].balance[from.ID] -= amount
				}
			}
		}
		tokenMutex.Unlock()
		hashList, tmpList := it.SendTransactionN(trxs, false)
		errList = append(errList, tmpList...)
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
			ilog.Warnf("Current tps %v, Average tps %v, Total tx %v, token num %v", currentTps, averageTps, total, len(tokenList))
			counter = 0
			slotTotal = 0
			slotStartTime = time.Now()
		}
	}
}
