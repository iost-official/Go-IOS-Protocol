package itest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/iost-official/go-iost/core/global"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/iost-official/go-iost/core/tx"
	"github.com/iost-official/go-iost/ilog"
	"github.com/iost-official/go-iost/rpc/pb"
	"google.golang.org/grpc"
)

// Constant of Client
var (
	Interval   = 15 * time.Second
	Timeout    = (90 + 30) * time.Second
	InitToken  = global.Token
	InitAmount = "1000000"
	InitPledge = "1000000"
	InitRAM    = "1000000"
)

// Client is a grpc client for iserver
type Client struct {
	grpc rpcpb.ApiServiceClient
	o    sync.Once
	Name string
	Addr string
}

// GetGRPC return the underlying grpc client
func (c *Client) GetGRPC() (rpcpb.ApiServiceClient, error) {
	c.o.Do(func() {
		conn, err := grpc.Dial(c.Addr, grpc.WithInsecure())
		if err != nil {
			panic(err)
		}
		c.grpc = rpcpb.NewApiServiceClient(conn)
		ilog.Infof("Create grpc connection with %v successful", c.Addr)
	})
	return c.grpc, nil
}

// GetTransaction will get transaction by tx hash
func (c *Client) GetTransaction(hash string) (*Transaction, error) {
	grpc, err := c.GetGRPC()
	if err != nil {
		return nil, err
	}

	resp, err := grpc.GetTxByHash(
		context.Background(),
		&rpcpb.TxHashRequest{
			Hash: hash,
		},
	)
	if err != nil {
		return nil, err
	}

	return NewTransactionFromPb(resp.Transaction), nil
}

// GetReceipt will get receipt by tx hash
func (c *Client) GetReceipt(hash string) (*Receipt, error) {
	grpc, err := c.GetGRPC()
	if err != nil {
		return nil, err
	}

	resp, err := grpc.GetTxReceiptByTxHash(
		context.Background(),
		&rpcpb.TxHashRequest{
			Hash: hash,
		},
	)
	if err != nil {
		return nil, err
	}

	return NewReceiptFromPb(resp), nil
}

// GetAccount will get account by name
func (c *Client) GetAccount(name string) (*Account, error) {
	grpc, err := c.GetGRPC()
	if err != nil {
		return nil, err
	}

	resp, err := grpc.GetAccount(
		context.Background(),
		&rpcpb.GetAccountRequest{
			Name:           name,
			ByLongestChain: true,
		},
	)
	if err != nil {
		return nil, err
	}

	// TODO: Get account permission by resp
	account := &Account{
		ID:      name,
		balance: strconv.FormatFloat(resp.GetBalance(), 'f', -1, 64),
	}

	return account, nil
}

// GetContractStorage will get contract storage by contract id, key and field
func (c *Client) GetContractStorage(id, key, field string) (data string, hash string, number int64, err error) {
	data, hash, number = "", "", 0
	grpc, err := c.GetGRPC()
	if err != nil {
		return
	}

	resp, err := grpc.GetContractStorage(
		context.Background(),
		&rpcpb.GetContractStorageRequest{
			Id:    id,
			Key:   key,
			Field: field,
		},
	)
	if err != nil {
		return
	}
	return resp.Data, resp.BlockHash, resp.BlockNumber, nil
}

// GetBlockByNumber will get block by number
func (c *Client) GetBlockByNumber(number int64) (*Block, error) {
	grpc, err := c.GetGRPC()
	if err != nil {
		return nil, err
	}

	resp, err := grpc.GetBlockByNumber(
		context.Background(),
		&rpcpb.GetBlockByNumberRequest{
			Number: number,
		},
	)
	if err != nil {
		return nil, err
	}
	return NewBlockFromPb(resp.Block), nil
}

// SendTransaction will send transaction to blockchain
func (c *Client) SendTransaction(transaction *Transaction, check bool) (string, error) {
	grpc, err := c.GetGRPC()
	if err != nil {
		return "", err
	}

	resp, err := grpc.SendTransaction(
		context.Background(),
		transaction.ToTxRequest(),
	)
	if err != nil {
		return "", err
	}
	if check {
		ilog.Debugf("transaction size: %v bytes", len(transaction.ToBytes(tx.Full)))

		ilog.Debugf("Check transaction receipt for %v...", resp.GetHash())
		if err := c.checkTransaction(resp.GetHash()); err != nil {
			return "", err
		}
		ilog.Debugf("Check transaction receipt for %v successful!", resp.GetHash())
	}

	return resp.GetHash(), nil
}

// CheckTransactionWithTimeout will check transaction receipt with expire time
func (c *Client) CheckTransactionWithTimeout(hash string, expire time.Time) (*Receipt, error) {
	ticker := time.NewTicker(Interval)
	defer ticker.Stop()
	var afterTimeout <-chan time.Time
	now := time.Now()

	var timer *time.Timer
	if expire.Before(now) {
		timer = time.NewTimer(2 * time.Millisecond)
	} else {
		timer = time.NewTimer(time.Until(expire))
	}
	afterTimeout = timer.C
	defer timer.Stop()

	for {
		select {
		case <-afterTimeout:
			return nil, fmt.Errorf("transaction be on chain timeout: %v", hash)
		case <-ticker.C:
			ilog.Debugf("Get receipt for %v...", hash)
			r, err := c.GetReceipt(hash)
			if err != nil {
				break
			}
			ilog.Debugf("Get receipt for %v successful!", hash)

			if !r.Success() {
				return nil, fmt.Errorf("%v: %v", r.Status.Code, r.Status.Message)
			}
			return r, nil
		}
	}
}
func (c *Client) checkTransaction(hash string) error {
	ticker := time.NewTicker(Interval)
	defer ticker.Stop()
	timer := time.NewTimer(Timeout)
	afterTimeout := timer.C
	defer timer.Stop()
	for {
		select {
		case <-afterTimeout:
			return fmt.Errorf("transaction be on chain timeout: %v", hash)
		case <-ticker.C:
			ilog.Debugf("Get receipt for %v...", hash)
			r, err := c.GetReceipt(hash)
			if err != nil {
				break
			}
			ilog.Debugf("Get receipt for %v successful!", hash)

			if !r.Success() {
				return fmt.Errorf("%v: %v", r.Status.Code, r.Status.Message)
			}
			return nil
		}
	}
}

// CreateAccount will create account by sending transaction
func (c *Client) CreateAccount(creator *Account, name string, key *Key, check bool) (*Account, error) {
	k := key.ReadablePubkey()
	action1 := tx.NewAction(
		"auth.iost",
		"signUp",
		fmt.Sprintf(`["%v", "%v", "%v"]`, name, k, k),
	)

	action2 := tx.NewAction(
		"ram.iost",
		"buy",
		fmt.Sprintf(`["%v", "%v", %v]`, creator.ID, name, InitRAM),
	)

	action3 := tx.NewAction(
		"gas.iost",
		"pledge",
		fmt.Sprintf(`["%v", "%v", "%v"]`, creator.ID, name, InitPledge),
	)

	action4 := tx.NewAction(
		"token.iost",
		"transfer",
		fmt.Sprintf(`["%v", "%v", "%v", "%v", ""]`, InitToken, creator.ID, name, InitAmount),
	)

	actions := []*tx.Action{action1, action2, action3, action4}
	transaction := NewTransaction(actions)

	st, err := creator.Sign(transaction)
	if err != nil {
		return nil, err
	}

	ilog.Debugf("Sending create account transaction for %v...", name)
	if _, err := c.SendTransaction(st, check); err != nil {
		return nil, err
	}
	ilog.Debugf("Sent create account transaction for %v!", name)

	account := &Account{
		ID:      name,
		balance: InitAmount,
		key:     key,
	}

	return account, nil
}

// ContractTransfer will contract transfer token by sending transaction
func (c *Client) ContractTransfer(cid string, sender, recipient *Account, amount string, memoSize int, check bool) error {
	memo := make([]byte, memoSize)
	rand.Read(memo)
	memoStr := base64.StdEncoding.EncodeToString(memo)
	_, err := c.CallAction(check, sender, cid, "transfer", sender.ID, recipient.ID, amount, memoStr[:memoSize])
	return err
}

// ExchangeTransfer will contract transfer token by sending transaction
func (c *Client) ExchangeTransfer(sender, recipient *Account, token, amount string, memoSize int, check bool) error {
	memo := make([]byte, memoSize)
	rand.Read(memo)
	memoStr := base64.StdEncoding.EncodeToString(memo)
	_, err := c.CallAction(check, sender, "exchange.iost", "transfer", token, recipient.ID, amount, memoStr[:memoSize])
	return err
}

// CallAction send a tx with given actions
func (c *Client) CallAction(check bool, sender *Account, contractName, actionName string, args ...interface{}) (string, error) {
	argsBytes, err := json.Marshal(args)
	if err != nil {
		return "", err
	}
	action := tx.NewAction(
		contractName,
		actionName,
		string(argsBytes),
	)
	//fmt.Printf("in call action %v -> %v\n", args, action.Data)
	actions := []*tx.Action{action}
	transaction := NewTransaction(actions)

	st, err := sender.Sign(transaction)
	if err != nil {
		return "", err
	}

	hash, err := c.SendTransaction(st, check)
	if err != nil {
		return "", err
	}

	return hash, nil
}

// VoteProducer will vote producer by sending transaction
func (c *Client) VoteProducer(sender *Account, recipient, amount string) error {
	_, err := c.CallAction(true, sender, "vote_producer.iost", "vote", sender.ID, recipient, amount)
	return err
}

// CancelVoteProducer will vote producer by sending transaction
func (c *Client) CancelVoteProducer(sender *Account, recipient, amount string) error {
	_, err := c.CallAction(true, sender, "vote_producer.iost", "unvote", sender.ID, recipient, amount)
	return err
}

// Pledge ...
func (c *Client) Pledge(sender *Account, amount string, check bool) error {
	_, err := c.CallAction(check, sender, "gas.iost", "pledge", sender.ID, sender.ID, amount)
	return err
}

// Unpledge ...
func (c *Client) Unpledge(sender *Account, amount string, check bool) error {
	_, err := c.CallAction(check, sender, "gas.iost", "unpledge", sender.ID, sender.ID, amount)
	return err
}

// BuyRAM ...
func (c *Client) BuyRAM(sender *Account, amount int64, check bool) error {
	_, err := c.CallAction(check, sender, "ram.iost", "buy", sender.ID, sender.ID, amount)
	return err
}

// SellRAM ...
func (c *Client) SellRAM(sender *Account, amount int64, check bool) error {
	_, err := c.CallAction(check, sender, "ram.iost", "sell", sender.ID, sender.ID, amount)
	return err
}

// Transfer will transfer token by sending transaction
func (c *Client) Transfer(sender, recipient *Account, token, amount string, memoSize int, check bool) error {
	memo := make([]byte, memoSize)
	rand.Read(memo)
	memoStr := base64.StdEncoding.EncodeToString(memo)
	_, err := c.CallAction(check, sender, "token.iost", "transfer", token, sender.ID, recipient.ID, amount, memoStr[:memoSize])
	return err
}

// SetContract will set the contract by sending transaction
func (c *Client) SetContract(creator *Account, contract *Contract) (string, error) {
	hash, err := c.CallAction(true, creator, "system.iost", "setCode", contract.String())
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Contract%v", hash), nil
}
