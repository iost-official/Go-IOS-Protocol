package itest

import (
	"fmt"
	"github.com/iost-official/go-iost/core/global"
	"math"
	"math/rand"
	"strconv"

	"github.com/iost-official/go-iost/core/contract"

	"reflect"

	"github.com/iost-official/go-iost/ilog"
)

// Constant of itest
const (
	Zero          = 1e-6
	concurrentNum = 500
)

type semaphore chan struct{}

func (s semaphore) acquire() {
	s <- struct{}{}
}

func (s semaphore) release() {
	<-s
}

// ITest is the test controller
type ITest struct {
	bank    *Account
	keys    []*Key
	clients []*Client
}

// New will return the itest by config and keys
func New(c *Config, keys []*Key) *ITest {
	return &ITest{
		bank:    c.Bank,
		keys:    keys,
		clients: c.Clients,
	}
}

// GetDefaultAccount return the bank account
func (t *ITest) GetDefaultAccount() *Account {
	return t.bank
}

// GetClients returns the clients
func (t *ITest) GetClients() []*Client {
	return t.clients
}

// GetRandClient return a random client
func (t *ITest) GetRandClient() *Client {
	cIndex := rand.Intn(len(t.clients))
	client := t.clients[cIndex]
	return client
}

// Load will load the itest from file
func Load(keysfile, configfile string) (*ITest, error) {
	ilog.Infof("Load itest from file...")

	keys, err := LoadKeys(keysfile)
	if err != nil {
		return nil, fmt.Errorf("load keys failed: %v", err)
	}

	itc, err := LoadConfig(configfile)
	if err != nil {
		return nil, fmt.Errorf("load itest config failed: %v", err)
	}

	it := New(itc, keys)

	return it, nil
}

// CreateAccountN will create n accounts concurrently
func (t *ITest) CreateAccountN(num int, randName bool, check bool) ([]*Account, error) {
	ilog.Infof("Create %v account...", num)

	res := make(chan interface{})
	go func() {
		sem := make(semaphore, concurrentNum)
		for i := 0; i < num; i++ {
			sem.acquire()
			go func(n int, res chan interface{}) {
				defer sem.release()
				var name string
				if randName {
					name = fmt.Sprintf("acc%08d", rand.Int63n(100000000))
				} else {
					name = fmt.Sprintf("account%04d", n)
				}

				account, err := t.CreateAccount(t.GetDefaultAccount(), name, check)
				if err != nil {
					res <- err
				} else {
					res <- account
				}
			}(i, res)
		}
	}()

	accounts := []*Account{}
	for i := 0; i < num; i++ {
		switch value := (<-res).(type) {
		case error:
			ilog.Errorf("Create account failed: %v", value)
		case *Account:
			accounts = append(accounts, value)
		default:
			return accounts, fmt.Errorf("unexpect res: %v", value)
		}
	}

	if len(accounts) != num {
		return accounts, fmt.Errorf(
			"expect create %v account, but only created %v account",
			num,
			len(accounts),
		)
	}

	ilog.Infof("Create %v account successful!", len(accounts))

	// TODO Get account by rpc, and compare account result

	return accounts, nil
}

// CreateAccountRoundN will create n accounts concurrently
func (t *ITest) CreateAccountRoundN(num int, randName bool, check bool, round int) ([]*Account, error) {
	ilog.Infof("Create %v account... round %v", num, round)

	res := make(chan interface{})
	go func() {
		sem := make(semaphore, 2000)
		for i := 0; i < num; i++ {
			sem.acquire()
			go func(n int, res chan interface{}) {
				defer sem.release()
				var name string
				if randName {
					name = fmt.Sprintf("acc%08d", rand.Int63n(100000000))
				} else {
					name = fmt.Sprintf("acc%08d", round*num+n)
				}

				account, err := t.CreateAccount(t.GetDefaultAccount(), name, check)
				if err != nil {
					res <- err
				} else {
					res <- account
				}
			}(i, res)
		}
	}()

	accounts := []*Account{}
	for i := 0; i < num; i++ {
		switch value := (<-res).(type) {
		case error:
			ilog.Errorf("Create account failed: %v", value)
		case *Account:
			accounts = append(accounts, value)
		default:
			return accounts, fmt.Errorf("unexpect res: %v", value)
		}
	}

	ilog.Infof("Create %v account successful!", len(accounts))

	return accounts, nil
}

// CreateAccount will create a account by name
func (t *ITest) CreateAccount(creator *Account, name string, check bool) (*Account, error) {
	if len(t.keys) == 0 {
		return nil, fmt.Errorf("keys is empty")
	}
	if len(t.clients) == 0 {
		return nil, fmt.Errorf("clients is empty")
	}
	kIndex := rand.Intn(len(t.keys)) // nolint: golint
	key := t.keys[kIndex]
	cIndex := rand.Intn(len(t.clients))
	client := t.clients[cIndex]

	account, err := client.CreateAccount(creator, name, key, check)
	if err != nil {
		return nil, err
	}

	return account, nil
}

// VoteN will send n vote transaction concurrently
func (t *ITest) VoteN(num, pnum int, accounts []*Account) error {
	ilog.Infof("Send %v vote transaction...", num)

	res := make(chan interface{})
	producers := []string{}
	if pnum == 1 {
		producers = append(producers, "producer00001")
	} else {
		for i := 0; i < pnum; i++ {
			producers = append(producers, fmt.Sprintf("producer%05d", i))
		}
	}
	go func() {
		sem := make(semaphore, concurrentNum)
		for i := 0; i < num; i++ {
			sem.acquire()
			go func(res chan interface{}) {
				defer sem.release()
				A := accounts[rand.Intn(len(accounts))]
				amount := float64(rand.Int63n(10000)+1) / 100
				B := producers[rand.Intn(len(producers))]

				A.AddBalance(-amount)
				ilog.Debugf("VoteProducer %v -> %v, amount: %v", A.ID, B, fmt.Sprintf("%0.8f", amount))

				res <- t.vote(A, B, fmt.Sprintf("%0.8f", amount))
			}(res)
		}
	}()

	for i := 0; i < num; i++ {
		switch value := (<-res).(type) {
		case error:
			return fmt.Errorf("send vote transaction failed: %v", value)
		default:
		}
	}

	ilog.Infof("Send %v vote transaction successful!", num)

	return nil
}

// VoteNode will send n vote transaction concurrently
func (t *ITest) VoteNode(num int, accounts []*Account) error {
	ilog.Infof("Send %v vote transaction...", num)

	res := make(chan interface{})
	times := len(accounts)
	go func() {
		sem := make(semaphore, concurrentNum)
		for i := 0; i < times; i++ {
			sem.acquire()
			go func(res chan interface{}, i int) {
				defer sem.release()
				A := t.bank
				B := accounts[i].ID
				var vote string
				if num == 0 {
					vote = accounts[i].vote
				} else {
					vote = strconv.Itoa(num)
				}
				ilog.Infof("VoteNode %v -> %v, vote: %v", A.ID, B, vote)

				res <- t.vote(A, B, vote)
			}(res, i)
		}
	}()

	for i := 0; i < times; i++ {
		switch value := (<-res).(type) {
		case error:
			return fmt.Errorf("send vote transaction failed: %v", value)
		default:
		}
	}

	ilog.Infof("Send %v vote transaction successful!", times)

	return nil
}

// CancelVoteNode will send n Cancel vote transaction concurrently
func (t *ITest) CancelVoteNode(num int, accounts []*Account) error {
	ilog.Infof("Send %v Cancel vote transaction...", num)

	res := make(chan interface{})
	times := len(accounts)
	go func() {
		sem := make(semaphore, concurrentNum)
		for i := 0; i < times; i++ {
			sem.acquire()
			go func(res chan interface{}, i int) {
				defer sem.release()
				A := t.bank
				B := accounts[i].ID
				var vote string
				if num == 0 {
					vote = accounts[i].vote
				} else {
					vote = strconv.Itoa(num)
				}

				ilog.Infof("CancelVoteNode %v -> %v, cancel vote: %v", A.ID, B, vote)

				res <- t.cancelVote(A, B, vote)
			}(res, i)
		}
	}()

	for i := 0; i < times; i++ {
		switch value := (<-res).(type) {
		case error:
			return fmt.Errorf("send cancel vote transaction failed: %v", value)
		default:
		}
	}

	ilog.Infof("Send %v cancel vote transaction successful!", times)

	return nil
}

// TransferN will send n transfer transaction concurrently
func (t *ITest) TransferN(num int, accounts []*Account, memoSize int, check bool) (successNum int, firstErr error) {
	ilog.Infof("Sending %v transfer transactions...", num)

	res := make(chan interface{})
	go func() {
		sem := make(semaphore, concurrentNum)
		for i := 0; i < num; i++ {
			sem.acquire()
			go func(res chan interface{}) {
				defer sem.release()
				A := accounts[rand.Intn(len(accounts))]
				balance, _ := strconv.ParseFloat(A.balance, 64)
				for balance < 1 {
					A = accounts[rand.Intn(len(accounts))]
					balance, _ = strconv.ParseFloat(A.balance, 64)
				}
				B := accounts[rand.Intn(len(accounts))]
				amount := float64(rand.Int63n(int64(math.Min(10000, balance*100)))+1) / 100

				ilog.Debugf("Transfer %v -> %v, amount: %v", A.ID, B.ID, fmt.Sprintf("%0.8f", amount))
				err := t.Transfer(A, B, global.Token, fmt.Sprintf("%0.8f", amount), memoSize, check)

				if err == nil {
					A.AddBalance(-amount)
					B.AddBalance(amount)
				}
				res <- err
			}(res)
		}
	}()

	for i := 0; i < num; i++ {
		switch value := (<-res).(type) {
		case error:
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to send transfer transactions: %v", value)
			}
		default:
			successNum++
		}
	}

	ilog.Infof("Sent %v/%v transfer transactions", successNum, num)
	return
}

// PledgeGasN will send n pledge/unpledge transaction concurrently
func (t *ITest) PledgeGasN(actionType string, num int, accounts []*Account, check bool) (successNum int, firstErr error) {
	ilog.Infof("Sending %v gas transaction...", num)

	res := make(chan interface{})
	go func() {
		sem := make(semaphore, concurrentNum)
		for i := 0; i < num; i++ {
			sem.acquire()
			go func(res chan interface{}) {
				defer sem.release()
				A := accounts[rand.Intn(len(accounts))]
				balance, _ := strconv.ParseFloat(A.balance, 64)
				for balance < 1 {
					A = accounts[rand.Intn(len(accounts))]
					balance, _ = strconv.ParseFloat(A.balance, 64)
				}
				amount := float64(rand.Int63n(int64(math.Min(1000, balance*100)))+1)/100 + 1.0

				ilog.Debugf("pledge gas %v, amount: %v", A.ID, fmt.Sprintf("%0.8f", amount))
				var err error
				action := actionType
				if action == "rand" {
					if rand.Int()%2 == 0 {
						action = "pledge"
					} else {
						action = "unpledge"
					}
				}
				if action == "pledge" {
					err = t.Pledge(A, fmt.Sprintf("%0.8f", amount), check)
				} else if action == "unpledge" {
					err = t.Unpledge(A, fmt.Sprintf("%0.8f", amount), check)
				} else {
					panic("invalid action " + action)
				}

				if err == nil {
					A.AddBalance(-amount)
				}
				res <- err
			}(res)
		}
	}()

	for i := 0; i < num; i++ {
		switch value := (<-res).(type) {
		case error:
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to send transfer transactions: %v", value)
			}
		default:
			successNum++
		}
	}

	ilog.Infof("Sent %v/%v gas transactions", successNum, num)
	return
}

// BuyRAMN will send n buy/sell ram transaction concurrently
func (t *ITest) BuyRAMN(actionType string, num int, accounts []*Account, check bool) (successNum int, firstErr error) {
	ilog.Infof("Sending %v ram transaction...", num)

	AmountLimit = []*contract.Amount{{Token: global.Token, Val: "1000"}, {Token: "ram", Val: "1000"}}

	res := make(chan interface{})
	go func() {
		sem := make(semaphore, concurrentNum)
		for i := 0; i < num; i++ {
			sem.acquire()
			go func(res chan interface{}) {
				defer sem.release()
				A := accounts[rand.Intn(len(accounts))]
				amount := rand.Int63n(10) + 10

				ilog.Debugf("buy/sell ram %v, amount: %v", A.ID, amount)
				var err error
				action := actionType
				if action == "rand" {
					if rand.Int()%2 == 0 {
						action = "buy"
					} else {
						action = "sell"
					}
				}
				if action == "buy" {
					err = t.BuyRAM(A, amount, check)
				} else if action == "sell" {
					err = t.SellRAM(A, amount, check)
				} else {
					panic("invalid action " + action)
				}

				res <- err
			}(res)
		}
	}()

	for i := 0; i < num; i++ {
		switch value := (<-res).(type) {
		case error:
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to send transfer transactions: %v", value)
			}
		default:
			successNum++
		}
	}

	ilog.Infof("Sent %v/%v ram transactions", successNum, num)
	return
}

// ContractTransferN will send n contract transfer transaction concurrently
func (t *ITest) ContractTransferN(cid string, num int, accounts []*Account, memoSize int, check bool) (successNum int, firstErr error) {
	ilog.Infof("Sending %v contract transfer transaction...", num)

	res := make(chan interface{})
	go func() {
		sem := make(semaphore, concurrentNum)
		for i := 0; i < num; i++ {
			sem.acquire()
			go func(res chan interface{}) {
				defer sem.release()
				A := accounts[rand.Intn(len(accounts))]
				balance, _ := strconv.ParseFloat(A.balance, 64)
				for balance < 1 {
					A = accounts[rand.Intn(len(accounts))]
					balance, _ = strconv.ParseFloat(A.balance, 64)
				}
				B := accounts[rand.Intn(len(accounts))]
				amount := float64(rand.Int63n(int64(math.Min(10000, balance*100)))+1) / 100

				ilog.Debugf("Contract transfer %v -> %v, amount: %v", A.ID, B.ID, fmt.Sprintf("%0.8f", amount))
				err := t.ContractTransfer(cid, A, B, fmt.Sprintf("%0.8f", amount), memoSize, check)

				if err == nil {
					A.AddBalance(-amount)
					B.AddBalance(amount)
				}
				res <- err
			}(res)
		}
	}()

	for i := 0; i < num; i++ {
		switch value := (<-res).(type) {
		case error:
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to send contract transfer transactions: %v", value)
			}
		default:
			successNum++
		}
	}

	ilog.Infof("Sent %v/%v contract transfer transactions", successNum, num)
	return
}

// ExchangeTransferN will send n contract transfer transaction concurrently
func (t *ITest) ExchangeTransferN(num int, accounts []*Account, memoSize int, check bool) (successNum int, firstErr error) {
	ilog.Infof("Sending %v exchange transfer transaction...", num)

	res := make(chan interface{})
	go func() {
		sem := make(semaphore, concurrentNum)
		for i := 0; i < num; i++ {
			sem.acquire()
			go func(res chan interface{}) {
				defer sem.release()
				A := accounts[rand.Intn(len(accounts))]
				balance, _ := strconv.ParseFloat(A.balance, 64)
				for balance < 1 {
					A = accounts[rand.Intn(len(accounts))]
					balance, _ = strconv.ParseFloat(A.balance, 64)
				}
				B := accounts[rand.Intn(len(accounts))]
				amount := float64(rand.Int63n(int64(math.Min(10000, balance*100)))+1) / 100

				ilog.Debugf("Contract transfer %v -> %v, amount: %v", A.ID, B.ID, fmt.Sprintf("%0.8f", amount))
				err := t.ExchangeTransfer(A, B, fmt.Sprintf("%0.8f", amount), memoSize, check)

				if err == nil {
					A.AddBalance(-amount)
					B.AddBalance(amount)
				}
				res <- err
			}(res)
		}
	}()

	for i := 0; i < num; i++ {
		switch value := (<-res).(type) {
		case error:
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to send contract transfer transactions: %v", value)
			}
		default:
			successNum++
		}
	}

	ilog.Infof("Sent %v/%v contract transfer transactions", successNum, num)
	return
}

// CheckAccounts will check account info by getting account info
func (t *ITest) CheckAccounts(a []*Account) error {
	ilog.Infof("Get %v accounts info...", len(a))

	res := make(chan interface{})
	for _, i := range a {
		go func(name string, res chan interface{}) {
			account, err := t.GetAccount(name)
			if err != nil {
				res <- err
			} else {
				res <- account
			}
		}(i.ID, res)
	}

	aMap := make(map[string]*Account)
	for i := 0; i < len(a); i++ {
		switch value := (<-res).(type) {
		case error:
			ilog.Errorf("Get account failed: %v", value)
		case *Account:
			aMap[value.ID] = value
		default:
			return fmt.Errorf("unexpect res: %v", value)
		}
	}

	if len(aMap) != len(a) {
		return fmt.Errorf(
			"expect get %v account, but only ge %v account",
			len(a),
			len(aMap),
		)
	}

	ilog.Infof("Get %v accounts info successful!", len(aMap))

	ilog.Infof("Check %v accounts info...", len(a))

	for _, i := range a {
		expect := i.Balance()
		actual := aMap[i.ID].Balance()
		if math.Abs(expect-actual) > Zero {
			return fmt.Errorf(
				"expect account %v's balance is %0.8f, but balance is %0.8f",
				i.ID,
				expect,
				actual,
			)
		}
	}
	ilog.Infof("Check %v accounts info successful!", len(a))

	return nil
}

// ContractTransfer will contract transfer token from sender to recipient
func (t *ITest) ContractTransfer(cid string, sender, recipient *Account, amount string, memoSize int, check bool) error {
	cIndex := rand.Intn(len(t.clients))
	client := t.clients[cIndex]

	err := client.ContractTransfer(cid, sender, recipient, amount, memoSize, check)
	if err != nil {
		return err
	}

	return nil
}

// ExchangeTransfer will contract transfer token from sender to recipient
func (t *ITest) ExchangeTransfer(sender, recipient *Account, amount string, memoSize int, check bool) error {
	cIndex := rand.Intn(len(t.clients))
	client := t.clients[cIndex]

	err := client.ExchangeTransfer(sender, recipient, global.Token, amount, memoSize, check)
	if err != nil {
		return err
	}

	return nil
}

// vote will vote producer from sender to recipient
func (t *ITest) vote(sender *Account, recipient, amount string) error {
	cIndex := rand.Intn(len(t.clients))
	client := t.clients[cIndex]

	err := client.VoteProducer(sender, recipient, amount)
	if err != nil {
		return err
	}

	return nil
}

// vote will cancel vote producer from sender to recipient
func (t *ITest) cancelVote(sender *Account, recipient, amount string) error {
	cIndex := rand.Intn(len(t.clients))
	client := t.clients[cIndex]

	err := client.CancelVoteProducer(sender, recipient, amount)
	if err != nil {
		return err
	}

	return nil
}

// CallActionWithRandClient randomly select one client and use it to send a tx
func (t *ITest) CallActionWithRandClient(sender *Account, contractName, actionName string, args ...interface{}) (string, error) {
	cIndex := rand.Intn(len(t.clients))
	client := t.clients[cIndex]
	return client.CallAction(true, sender, contractName, actionName, args...)
}

// Transfer will transfer token from sender to recipient
func (t *ITest) Transfer(sender, recipient *Account, token, amount string, memoSize int, check bool) error {
	cIndex := rand.Intn(len(t.clients))
	client := t.clients[cIndex]

	err := client.Transfer(sender, recipient, token, amount, memoSize, check)
	if err != nil {
		return err
	}

	return nil
}

// Pledge will pledge gas for sender
func (t *ITest) Pledge(sender *Account, amount string, check bool) error {
	cIndex := rand.Intn(len(t.clients))
	client := t.clients[cIndex]

	err := client.Pledge(sender, amount, check)
	if err != nil {
		return err
	}

	return nil
}

// Unpledge will unpledge gas for sender
func (t *ITest) Unpledge(sender *Account, amount string, check bool) error {
	cIndex := rand.Intn(len(t.clients))
	client := t.clients[cIndex]

	err := client.Unpledge(sender, amount, check)
	if err != nil {
		return err
	}

	return nil
}

// BuyRAM will buy ram for sender
func (t *ITest) BuyRAM(sender *Account, amount int64, check bool) error {
	cIndex := rand.Intn(len(t.clients))
	client := t.clients[cIndex]

	err := client.BuyRAM(sender, amount, check)
	if err != nil {
		return err
	}

	return nil
}

// SellRAM will sell ram for sender
func (t *ITest) SellRAM(sender *Account, amount int64, check bool) error {
	cIndex := rand.Intn(len(t.clients))
	client := t.clients[cIndex]

	err := client.SellRAM(sender, amount, check)
	if err != nil {
		return err
	}

	return nil
}

// SetContract will set the contract on blockchain
func (t *ITest) SetContract(contract *Contract) (string, error) {
	ilog.Infof("Set transfer contract...")
	cIndex := rand.Intn(len(t.clients))
	client := t.clients[cIndex]

	hash, err := client.SetContract(t.bank, contract)
	if err != nil {
		return "", err
	}
	ilog.Infof("Set transfer contract successful!")

	return hash, nil
}

// GetTransaction will get transaction by tx hash
func (t *ITest) GetTransaction(hash string) (*Transaction, error) {
	cIndex := rand.Intn(len(t.clients))
	client := t.clients[cIndex]

	transaction, err := client.GetTransaction(hash)
	if err != nil {
		return nil, err
	}

	return transaction, nil
}

// GetAccount will get account by name
func (t *ITest) GetAccount(name string) (*Account, error) {
	cIndex := rand.Intn(len(t.clients))
	client := t.clients[cIndex]

	account, err := client.GetAccount(name)
	if err != nil {
		return nil, err
	}

	return account, nil
}

// GetContractStorage will get contract storage by contract id, key and field
func (t *ITest) GetContractStorage(id, key, field string) (data string, hash string, number int64, err error) {
	cIndex := rand.Intn(len(t.clients))
	client := t.clients[cIndex]

	return client.GetContractStorage(id, key, field)
}

// GetBlockByNumber will get block by number
func (t *ITest) GetBlockByNumber(number int64) (*Block, error) {
	cIndex := rand.Intn(len(t.clients))
	client := t.clients[cIndex]

	return client.GetBlockByNumber(number)
}

// SendTransactionN will send n transaction to blockchain concurrently
func (t *ITest) SendTransactionN(trxs []*Transaction, check bool) ([]string, []error) {
	ilog.Infof("Send %v transaction...", len(trxs))

	res := make(chan interface{})
	go func() {
		sem := make(semaphore, concurrentNum)
		for i := 0; i < len(trxs); i++ {
			sem.acquire()
			go func(idx int, res chan interface{}) {
				defer sem.release()
				cIndex := rand.Intn(len(t.clients))
				client := t.clients[cIndex]
				hash, err := client.SendTransaction(trxs[idx], check)
				if err != nil {
					res <- err
				} else {
					res <- hash
				}
			}(i, res)
		}
	}()

	hashList := []string{}
	errList := []error{}
	for i := 0; i < len(trxs); i++ {
		switch value := (<-res).(type) {
		case error:
			errList = append(errList, value)
		case string:
			hashList = append(hashList, value)
		default:
			ilog.Errorf("unexpected send transaction value type. %v %v", value, reflect.TypeOf(value))
		}
	}
	ilog.Infof("Send %v transaction successful! %v failed.", len(hashList), len(errList))
	return hashList, errList
}

// SendTransaction will send transaction to blockchain
func (t *ITest) SendTransaction(transaction *Transaction, check bool) (string, error) {
	cIndex := rand.Intn(len(t.clients))
	client := t.clients[cIndex]

	hash, err := client.SendTransaction(transaction, check)
	if err != nil {
		return "", err
	}

	return hash, nil
}
