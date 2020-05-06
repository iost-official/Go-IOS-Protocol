package sdk

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/iost-official/go-iost/account"
	"github.com/iost-official/go-iost/common"
	"github.com/iost-official/go-iost/core/contract"
	rpcpb "github.com/iost-official/go-iost/rpc/pb"
	"google.golang.org/grpc"
)

// IOSTDevSDK ...
type IOSTDevSDK struct {
	// the remote server to connect to
	server string

	// account used for sending tx
	accountName string
	keyPair     *account.KeyPair
	// signing algorithm
	signAlgo string

	// fields used to fill tx
	gasLimit    float64
	gasRatio    float64
	expiration  int64
	amountLimit []*rpcpb.AmountLimit
	delaySecond int64

	// whether to check tx after sending by `SendTx`
	checkResult         bool
	checkResultDelay    float32
	checkResultMaxRetry int32

	// query longest chain when fetching information from blockchain, currently only used in `GetAccountInfo`
	useLongestChain bool

	// if false, be silent
	verbose bool

	// chain id set in tx
	chainID uint32

	// internal connection
	rpcConn *grpc.ClientConn
}

// NewIOSTDevSDK creatimg an SDK with reasonable params
func NewIOSTDevSDK() *IOSTDevSDK {
	return &IOSTDevSDK{
		server:              "localhost:30002",
		checkResult:         true,
		checkResultDelay:    3,
		checkResultMaxRetry: 20,
		signAlgo:            "ed25519",
		gasLimit:            1000000,
		gasRatio:            1.0,
		amountLimit:         []*rpcpb.AmountLimit{{Token: "*", Value: "unlimited"}},
		expiration:          90,
		chainID:             uint32(1024),
	}
}

/////////////////////////////////////// getter/setter ///////////////////////////////////////

// SetChainID sets chainID.
func (s *IOSTDevSDK) SetChainID(chainID uint32) {
	s.chainID = chainID
}

// SetAccount ...
func (s *IOSTDevSDK) SetAccount(name string, kp *account.KeyPair) {
	s.accountName = name
	s.keyPair = kp
}

// SetTxInfo ...
func (s *IOSTDevSDK) SetTxInfo(gasLimit float64, gasRatio float64, expiration int64, delaySecond int64, amountLimit []*rpcpb.AmountLimit) {
	s.gasLimit = gasLimit
	s.gasRatio = gasRatio
	s.expiration = expiration
	s.delaySecond = delaySecond
	s.amountLimit = amountLimit
}

// SetCheckResult ...
func (s *IOSTDevSDK) SetCheckResult(checkResult bool, checkResultDelay float32, checkResultMaxRetry int32) {
	s.checkResult = checkResult
	s.checkResultDelay = checkResultDelay
	s.checkResultMaxRetry = checkResultMaxRetry
}

// SetServer ...
func (s *IOSTDevSDK) SetServer(server string) {
	s.server = server
}

// SetSignAlgo ...
func (s *IOSTDevSDK) SetSignAlgo(signAlgo string) {
	s.signAlgo = signAlgo
}

// SetVerbose ...
func (s *IOSTDevSDK) SetVerbose(verbose bool) {
	s.verbose = verbose
}

// SetUseLongestChain ...
func (s *IOSTDevSDK) SetUseLongestChain(useLongestChain bool) {
	s.useLongestChain = useLongestChain
}

// Connected checks if is connected to grpc server.
func (s *IOSTDevSDK) Connected() bool {
	return s.rpcConn != nil
}

// Connect ...
func (s *IOSTDevSDK) Connect() (err error) {
	if s.rpcConn == nil {
		s.log("Connecting to server", s.server, "...")
		s.rpcConn, err = grpc.Dial(s.server, grpc.WithInsecure())
	}
	return
}

// CloseConn ...
func (s *IOSTDevSDK) CloseConn() {
	if s.rpcConn != nil {
		s.rpcConn.Close()
		s.rpcConn = nil
	}
}

func (s *IOSTDevSDK) log(a ...interface{}) {
	if s.verbose {
		fmt.Println(a...)
	}
}

///////////////////////////////////////// wrapper of rpc ////////////////////////////////

// GetContractStorage ...
func (s *IOSTDevSDK) GetContractStorage(r *rpcpb.GetContractStorageRequest) (*rpcpb.GetContractStorageResponse, error) {
	if s.rpcConn == nil {
		if err := s.Connect(); err != nil {
			return nil, err
		}
		defer s.CloseConn()
	}
	client := rpcpb.NewApiServiceClient(s.rpcConn)
	value, err := client.GetContractStorage(context.Background(), r)
	if err != nil {
		return nil, err
	}
	return value, nil
}

// GetBatchContractStorage ...
func (s *IOSTDevSDK) GetBatchContractStorage(r *rpcpb.GetBatchContractStorageRequest) (*rpcpb.GetBatchContractStorageResponse, error) {
	if s.rpcConn == nil {
		if err := s.Connect(); err != nil {
			return nil, err
		}
		defer s.CloseConn()
	}
	client := rpcpb.NewApiServiceClient(s.rpcConn)
	value, err := client.GetBatchContractStorage(context.Background(), r)
	if err != nil {
		return nil, err
	}
	return value, nil
}

// GetNodeInfo ...
func (s *IOSTDevSDK) GetNodeInfo() (*rpcpb.NodeInfoResponse, error) {
	if s.rpcConn == nil {
		if err := s.Connect(); err != nil {
			return nil, err
		}
		defer s.CloseConn()
	}
	client := rpcpb.NewApiServiceClient(s.rpcConn)
	value, err := client.GetNodeInfo(context.Background(), &rpcpb.EmptyRequest{})
	if err != nil {
		return nil, err
	}
	return value, nil
}

// GetChainInfo ...
func (s *IOSTDevSDK) GetChainInfo() (*rpcpb.ChainInfoResponse, error) {
	if s.rpcConn == nil {
		if err := s.Connect(); err != nil {
			return nil, err
		}
		defer s.CloseConn()
	}
	client := rpcpb.NewApiServiceClient(s.rpcConn)
	value, err := client.GetChainInfo(context.Background(), &rpcpb.EmptyRequest{})
	if err != nil {
		return nil, err
	}
	return value, nil
}

// GetRAMInfo ...
func (s *IOSTDevSDK) GetRAMInfo() (*rpcpb.RAMInfoResponse, error) {
	if s.rpcConn == nil {
		if err := s.Connect(); err != nil {
			return nil, err
		}
		defer s.CloseConn()
	}
	client := rpcpb.NewApiServiceClient(s.rpcConn)
	value, err := client.GetRAMInfo(context.Background(), &rpcpb.EmptyRequest{})
	if err != nil {
		return nil, err
	}
	return value, nil

}

// GetAccountInfo return account info
func (s *IOSTDevSDK) GetAccountInfo(id string) (*rpcpb.Account, error) {
	if s.rpcConn == nil {
		if err := s.Connect(); err != nil {
			return nil, err
		}
		defer s.CloseConn()
	}
	client := rpcpb.NewApiServiceClient(s.rpcConn)
	req := &rpcpb.GetAccountRequest{Name: id, ByLongestChain: s.useLongestChain}
	value, err := client.GetAccount(context.Background(), req)
	if err != nil {
		return nil, err
	}
	return value, nil
}

// GetTokenBalance return token balance
func (s *IOSTDevSDK) GetTokenBalance(account string, token string) (*rpcpb.GetTokenBalanceResponse, error) {
	if s.rpcConn == nil {
		if err := s.Connect(); err != nil {
			return nil, err
		}
		defer s.CloseConn()
	}
	client := rpcpb.NewApiServiceClient(s.rpcConn)
	req := &rpcpb.GetTokenBalanceRequest{
		Account:        account,
		Token:          token,
		ByLongestChain: s.useLongestChain,
	}
	value, err := client.GetTokenBalance(context.Background(), req)
	if err != nil {
		return nil, err
	}
	return value, nil
}

// GetBlockByNum ...
func (s *IOSTDevSDK) GetBlockByNum(num int64, complete bool) (*rpcpb.BlockResponse, error) {
	if s.rpcConn == nil {
		if err := s.Connect(); err != nil {
			return nil, err
		}
		defer s.CloseConn()
	}
	client := rpcpb.NewApiServiceClient(s.rpcConn)
	return client.GetBlockByNumber(context.Background(), &rpcpb.GetBlockByNumberRequest{Number: num, Complete: complete})
}

// GetBlockByHash ...
func (s *IOSTDevSDK) GetBlockByHash(hash string, complete bool) (*rpcpb.BlockResponse, error) {
	if s.rpcConn == nil {
		if err := s.Connect(); err != nil {
			return nil, err
		}
		defer s.CloseConn()
	}
	client := rpcpb.NewApiServiceClient(s.rpcConn)
	return client.GetBlockByHash(context.Background(), &rpcpb.GetBlockByHashRequest{Hash: hash, Complete: complete})
}

// GetTxByHash ...
func (s *IOSTDevSDK) GetTxByHash(hash string) (*rpcpb.TransactionResponse, error) {
	if s.rpcConn == nil {
		if err := s.Connect(); err != nil {
			return nil, err
		}
		defer s.CloseConn()
	}
	client := rpcpb.NewApiServiceClient(s.rpcConn)
	return client.GetTxByHash(context.Background(), &rpcpb.TxHashRequest{Hash: hash})
}

// GetTxReceiptByTxHash ...
func (s *IOSTDevSDK) GetTxReceiptByTxHash(txHashStr string) (*rpcpb.TxReceipt, error) {
	if s.rpcConn == nil {
		if err := s.Connect(); err != nil {
			return nil, err
		}
		defer s.CloseConn()
	}
	client := rpcpb.NewApiServiceClient(s.rpcConn)
	return client.GetTxReceiptByTxHash(context.Background(), &rpcpb.TxHashRequest{Hash: txHashStr})
}

// GetTokenInfo ...
func (s *IOSTDevSDK) GetTokenInfo(token string) (*rpcpb.TokenInfo, error) {
	if s.rpcConn == nil {
		if err := s.Connect(); err != nil {
			return nil, err
		}
		defer s.CloseConn()
	}
	client := rpcpb.NewApiServiceClient(s.rpcConn)
	return client.GetTokenInfo(context.Background(), &rpcpb.GetTokenInfoRequest{
		Symbol:         token,
		ByLongestChain: s.useLongestChain,
	})

}

// SendTransaction send raw transaction to server
func (s *IOSTDevSDK) SendTransaction(signedTx *rpcpb.TransactionRequest) (string, error) {
	if s.rpcConn == nil {
		if err := s.Connect(); err != nil {
			return "", err
		}
		defer s.CloseConn()
	}
	client := rpcpb.NewApiServiceClient(s.rpcConn)
	resp, err := client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return "", err
	}
	return resp.Hash, nil
}

// ExecTransaction send raw transaction to server
func (s *IOSTDevSDK) ExecTransaction(signedTx *rpcpb.TransactionRequest) (*rpcpb.TxReceipt, error) {
	if s.rpcConn == nil {
		if err := s.Connect(); err != nil {
			return nil, err
		}
		defer s.CloseConn()
	}
	client := rpcpb.NewApiServiceClient(s.rpcConn)
	resp, err := client.ExecTransaction(context.Background(), signedTx)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

////////////////////////////////////// transaction related /////////////////////////////////

// CreateTxFromActions ...
func (s *IOSTDevSDK) CreateTxFromActions(actions []*rpcpb.Action) (*rpcpb.TransactionRequest, error) {

	txTime := time.Now().UnixNano()
	expiration := txTime + s.expiration*1e9

	ret := &rpcpb.TransactionRequest{
		Time:          txTime,
		Actions:       actions,
		Signers:       []string{},
		GasLimit:      s.gasLimit,
		GasRatio:      s.gasRatio,
		Expiration:    expiration,
		PublisherSigs: []*rpcpb.Signature{},
		Delay:         s.delaySecond * 1e9,
		ChainId:       s.chainID,
		AmountLimit:   s.amountLimit,
		Signatures:    []*rpcpb.Signature{},
	}
	return ret, nil
}

// SignTx ...
func (s *IOSTDevSDK) SignTx(t *rpcpb.TransactionRequest, signAlgo string) (*rpcpb.TransactionRequest, error) {
	t.Publisher = s.accountName
	if len(t.PublisherSigs) == 0 {
		signAlgorithm := GetSignAlgoByName(signAlgo)
		txHashBytes := common.Sha3(txToBytes(t, true))
		publishSig := &rpcpb.Signature{
			Algorithm: rpcpb.Signature_Algorithm(signAlgorithm),
			Signature: signAlgorithm.Sign(txHashBytes, s.keyPair.Seckey),
			PublicKey: signAlgorithm.GetPubkey(s.keyPair.Seckey),
		}
		t.PublisherSigs = []*rpcpb.Signature{publishSig}
	}
	return t, nil
}

func (s *IOSTDevSDK) checkTransaction(txHash string) error {
	s.log("Checking transaction receipt...")
	receiptPrinted := false
	packedPrinted := false
	irreversiblePrinted := false
	for i := int32(0); i < s.checkResultMaxRetry; i++ {
		time.Sleep(time.Duration(s.checkResultDelay*1000) * time.Millisecond)
		r, err := s.GetTxByHash(txHash)
		if err != nil {
			return err
		}
		if r.Status == rpcpb.TransactionResponse_PENDING {
			if s.verbose {
				if !packedPrinted {
					fmt.Print("Transaction has been sent! Waiting for being packed...")
					packedPrinted = true
				} else {
					fmt.Print("...")
				}
			}
			continue
		}
		txReceipt := r.Transaction.TxReceipt
		if !receiptPrinted {
			s.log("Transaction receipt:")
			s.log(MarshalTextString(txReceipt))
			receiptPrinted = true
		}
		if txReceipt.StatusCode != rpcpb.TxReceipt_SUCCESS {
			s.log("Transaction executed err")
			return fmt.Errorf(txReceipt.Message)
		}
		if r.Status == rpcpb.TransactionResponse_PACKED {
			if s.verbose {
				if !irreversiblePrinted {
					fmt.Print("Transaction has been packed! Waiting for being irreversible...")
					irreversiblePrinted = true
				} else {
					fmt.Print("...")
				}
			}
			continue
		}
		if r.Status == rpcpb.TransactionResponse_IRREVERSIBLE {
			s.log("\nSUCCESS! Transaction has been irreversible")
			return nil
		}
	}
	return fmt.Errorf("exceeded max retry times")
}

func (s *IOSTDevSDK) TryTx(tx *rpcpb.TransactionRequest) (*rpcpb.TxReceipt, error) {
	signedTx, err := s.SignTx(tx, s.signAlgo)
	if err != nil {
		return nil, fmt.Errorf("sign tx error %v", err)
	}
	err = VerifySignature(signedTx)
	if err != nil {
		return nil, err
	}
	s.log("Trying transaction:")
	s.log(MarshalTextString(signedTx))
	return s.ExecTransaction(signedTx)
}

// SendTx send transaction and check result if sdk.checkResult is set
func (s *IOSTDevSDK) SendTx(tx *rpcpb.TransactionRequest) (string, error) {
	signedTx, err := s.SignTx(tx, s.signAlgo)
	if err != nil {
		return "", fmt.Errorf("sign tx error %v", err)
	}
	err = VerifySignature(signedTx)
	if err != nil {
		return "", err
	}
	s.log("Sending transaction...")
	s.log("Transaction:")
	s.log(MarshalTextString(signedTx))
	txHash, err := s.SendTransaction(signedTx)
	if err != nil {
		return "", fmt.Errorf("send tx error %v", err)
	}
	s.log("Transaction has been sent.")
	s.log("The transaction hash is:", txHash)
	if s.checkResult {
		if err = s.checkTransaction(txHash); err != nil {
			return txHash, err
		}
	}
	return txHash, nil
}

// SendTxFromActions send transaction and check result if sdk.checkResult is set
func (s *IOSTDevSDK) SendTxFromActions(actions []*rpcpb.Action) (txHash string, err error) {
	trx, err := s.CreateTxFromActions(actions)
	if err != nil {
		return "", err
	}
	return s.SendTx(trx)
}

////////////////////////////////////// some common used contract calling /////////////////////////////////////////////

// PledgeForGasAndRAM ...
func (s *IOSTDevSDK) PledgeForGasAndRAM(gasPledged int64, ram int64) error {
	var acts []*rpcpb.Action
	acts = append(acts, NewAction("gas.iost", "pledge", fmt.Sprintf(`["%v", "%v", "%v"]`, s.accountName, s.accountName, gasPledged)))
	if ram > 0 {
		acts = append(acts, NewAction("ram.iost", "buy", fmt.Sprintf(`["%v", "%v", %v]`, s.accountName, s.accountName, ram)))
	}
	_, err := s.SendTxFromActions(acts)
	if err != nil {
		return err
	}
	return nil
}

// CreateNewAccountActions makes actions for creating new account.
func (s *IOSTDevSDK) CreateNewAccountActions(newID string, ownerKey string, activeKey string, initialGasPledge int64, initialRAM int64, initialCoins int64) ([]*rpcpb.Action, error) {
	var acts []*rpcpb.Action
	acts = append(acts, NewAction("auth.iost", "signUp", fmt.Sprintf(`["%v", "%v", "%v"]`, newID, ownerKey, activeKey)))
	if initialRAM > 0 {
		acts = append(acts, NewAction("ram.iost", "buy", fmt.Sprintf(`["%v", "%v", %v]`, s.accountName, newID, initialRAM)))
	}
	var registerInitialPledge int64 = 10
	initialGasPledge -= registerInitialPledge
	if initialGasPledge < 0 {
		return nil, fmt.Errorf("min gas pledge is 10")
	}
	if initialGasPledge > 0 {
		acts = append(acts, NewAction("gas.iost", "pledge", fmt.Sprintf(`["%v", "%v", "%v"]`, s.accountName, newID, initialGasPledge)))
	}
	if initialCoins > 0 {
		acts = append(acts, NewAction("token.iost", "transfer", fmt.Sprintf(`[global.Token, "%v", "%v", "%v", ""]`, s.accountName, newID, initialCoins)))
	}
	return acts, nil
}

// CreateNewAccount ... return txHash
func (s *IOSTDevSDK) CreateNewAccount(newID string, ownerKey string, activeKey string, initialGasPledge int64, initialRAM int64, initialCoins int64) (string, error) {
	acts, err := s.CreateNewAccountActions(newID, ownerKey, activeKey, initialGasPledge, initialRAM, initialCoins)
	if err != nil {
		return "", err
	}
	txHash, err := s.SendTxFromActions(acts)
	if err != nil {
		return txHash, err
	}
	return txHash, nil
}

// PublishContractActions makes actions for publishing contract.
func (s *IOSTDevSDK) PublishContractActions(codePath string, abiPath string, conID string, update bool, updateID string) ([]*rpcpb.Action, error) {
	fd, err := ioutil.ReadFile(codePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read source code file: %v", err)
	}
	code := string(fd)

	fd, err = ioutil.ReadFile(abiPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read abi file: %v", err)
	}
	abi := string(fd)

	var info *contract.Info
	err = json.Unmarshal([]byte(abi), &info)
	if err != nil {
		return nil, err
	}
	c := &contract.Contract{
		ID:   conID,
		Code: code,
		Info: info,
	}
	methodName := "setCode"
	if update {
		methodName = "updateCode"
	}
	marshalMethod := "json"
	var contractStr string
	if marshalMethod == "json" {
		buf, err := json.Marshal(c)
		if err != nil {
			return nil, err
		}
		contractStr = string(buf)
	} else {
		buf, err := proto.Marshal(c)
		if err != nil {
			return nil, err
		}
		contractStr = base64.StdEncoding.EncodeToString(buf)
	}
	arr := []string{contractStr}
	if update {
		arr = append(arr, updateID)
	}
	data, err := json.Marshal(arr)
	if err != nil {
		return nil, err
	}
	action := NewAction("system.iost", methodName, string(data))
	return []*rpcpb.Action{action}, nil
}

// PublishContract converts contract js code to transaction. If 'send', also send it to chain.
func (s *IOSTDevSDK) PublishContract(codePath string, abiPath string, conID string, update bool, updateID string) (*rpcpb.TransactionRequest, string, error) {
	acts, err := s.PublishContractActions(codePath, abiPath, conID, update, updateID)
	if err != nil {
		return nil, "", err
	}
	trx, err := s.CreateTxFromActions(acts)
	if err != nil {
		return nil, "", err
	}
	txHash, err := s.SendTx(trx)
	if err != nil {
		return nil, "", err
	}
	return trx, txHash, nil
}

// GetProducerVoteInfo ...
func (s *IOSTDevSDK) GetProducerVoteInfo(r *rpcpb.GetProducerVoteInfoRequest) (*rpcpb.GetProducerVoteInfoResponse, error) {
	if s.rpcConn == nil {
		if err := s.Connect(); err != nil {
			return nil, err
		}
		defer s.CloseConn()
	}
	client := rpcpb.NewApiServiceClient(s.rpcConn)
	value, err := client.GetProducerVoteInfo(context.Background(), r)
	if err != nil {
		return nil, err
	}
	return value, nil
}
