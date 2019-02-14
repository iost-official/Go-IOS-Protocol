package iwallet

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/iost-official/go-iost/account"
	"github.com/iost-official/go-iost/common"
	"github.com/iost-official/go-iost/core/contract"
	"github.com/iost-official/go-iost/crypto"
	"github.com/iost-official/go-iost/rpc/pb"
	"github.com/mitchellh/go-homedir"
	"google.golang.org/grpc"
)

// SDK ...
type SDK struct {
	server      string
	accountName string
	keyPair     *account.KeyPair
	signAlgo    string
	signKeys    []string
	signers     string
	withSigns   []string

	gasLimit    float64
	gasRatio    float64
	expiration  int64
	amountLimit string
	delaySecond int64
	txTime      string

	checkResult         bool
	checkResultDelay    float32
	checkResultMaxRetry int32
	useLongestChain     bool

	verbose bool

	chainID uint32

	rpcConn *grpc.ClientConn
}

var sdk = &SDK{}

// SetChainID sets chainID.
func (s *SDK) SetChainID(chainID uint32) {
	s.chainID = chainID
}

// SetAccount ...
func (s *SDK) SetAccount(name string, kp *account.KeyPair) {
	s.accountName = name
	s.keyPair = kp
}

// SetTxInfo ...
func (s *SDK) SetTxInfo(gasLimit float64, gasRatio float64, expiration int64, delaySecond int64) {
	s.gasLimit = gasLimit
	s.gasRatio = gasRatio
	s.expiration = expiration
	s.delaySecond = delaySecond
}

// SetCheckResult ...
func (s *SDK) SetCheckResult(checkResult bool, checkResultDelay float32, checkResultMaxRetry int32) {
	s.checkResult = checkResult
	s.checkResultDelay = checkResultDelay
	s.checkResultMaxRetry = checkResultMaxRetry
}

// SetServer ...
func (s *SDK) SetServer(server string) {
	s.server = server
}

// SetAmountLimit ...
func (s *SDK) SetAmountLimit(amountLimit string) {
	s.amountLimit = amountLimit
}

// SetSignAlgo ...
func (s *SDK) SetSignAlgo(signAlgo string) {
	s.signAlgo = signAlgo
}

// SetVerbose ...
func (s *SDK) SetVerbose(verbose bool) {
	s.verbose = verbose
}

func (s *SDK) parseAmountLimit(limitStr string) ([]*rpcpb.AmountLimit, error) {
	result := make([]*rpcpb.AmountLimit, 0)
	if limitStr == "" {
		return result, nil
	}
	splits := strings.Split(limitStr, "|")
	for _, gram := range splits {
		limit := strings.Split(gram, ":")
		if len(limit) != 2 {
			return nil, fmt.Errorf("invalid amount limit %v", gram)
		}
		token := limit[0]
		if limit[1] != "unlimited" {
			amountLimit, err := strconv.ParseFloat(limit[1], 64)
			if err != nil {
				return nil, fmt.Errorf("invalid amount limit %v %v", amountLimit, err)
			}
		}
		tokenLimit := &rpcpb.AmountLimit{}
		tokenLimit.Token = token
		tokenLimit.Value = limit[1]
		result = append(result, tokenLimit)
	}
	return result, nil
}

func (s *SDK) createTx(actions []*rpcpb.Action) (*rpcpb.TransactionRequest, error) {
	if s.amountLimit == "" {
		return nil, fmt.Errorf("cmdline flag --amount_limit must be set like `iost:300.00|ram:2000`. You can set to `*:unlimited` to disable any limit")
	}
	amountLimits, err := s.parseAmountLimit(s.amountLimit)
	if err != nil {
		return nil, err
	}

	var txTime int64
	if s.txTime != "" {
		t, err := time.Parse(time.RFC3339, s.txTime)
		if err != nil {
			return nil, fmt.Errorf("invalid time %v, should in format %v", s.txTime, time.RFC3339)
		}
		txTime = t.UnixNano()
	} else {
		txTime = time.Now().UnixNano()
	}
	expiration := txTime + s.expiration*1e9

	signers := make([]string, 0)
	if s.signers != "" {
		strings.Split(s.signers, ",")
		for _, s := range signers {
			if !(len(strings.Split(s, "@")) == 2) {
				return nil, fmt.Errorf("signer %v should contrain '@'", s)
			}
		}
	}

	ret := &rpcpb.TransactionRequest{
		Time:          txTime,
		Actions:       actions,
		Signers:       signers,
		GasLimit:      s.gasLimit,
		GasRatio:      s.gasRatio,
		Expiration:    expiration,
		PublisherSigs: []*rpcpb.Signature{},
		Delay:         s.delaySecond * 1e9,
		ChainId:       s.chainID,
		AmountLimit:   amountLimits,
	}
	return ret, nil
}

func (s *SDK) toRPCSign(sig *crypto.Signature) *rpcpb.Signature {
	return &rpcpb.Signature{
		Algorithm: rpcpb.Signature_Algorithm(sig.Algorithm),
		Signature: sig.Sig,
		PublicKey: sig.Pubkey,
	}
}

func (s *SDK) getSignatureOfTx(t *rpcpb.TransactionRequest, kp *account.KeyPair) *rpcpb.Signature {
	hash := common.Sha3(txToBytes(t, false))
	sig := s.toRPCSign(kp.Sign(hash))
	return sig
}

func (s *SDK) signTx(t *rpcpb.TransactionRequest) (*rpcpb.TransactionRequest, error) {
	sigs := make([]*rpcpb.Signature, 0)
	if len(s.withSigns) != 0 && len(s.signKeys) != 0 {
		return nil, fmt.Errorf("at least one of --sign_keys and --with_signs should be empty")
	}
	if len(s.signKeys) > 0 {
		for _, f := range s.signKeys {
			kp, err := loadKeyPair(f, s.GetSignAlgo())
			if err != nil {
				return nil, fmt.Errorf("sign tx with priv key %v err %v", f, err)
			}
			sigs = append(sigs, s.getSignatureOfTx(t, kp))
		}
	} else if len(s.withSigns) > 0 {
		hash := common.Sha3(txToBytes(t, false))
		for _, f := range s.withSigns {
			sig := &rpcpb.Signature{}
			err := loadProto(f, sig)
			if err != nil {
				return nil, fmt.Errorf("invalid signature file %v", f)
			}
			if !s.GetSignAlgoByEnum(sig.Algorithm).Verify(hash, sig.PublicKey, sig.Signature) {
				return nil, fmt.Errorf("sign verify error %v", f)
			}
			sigs = append(sigs, sig)
		}
	}

	t.Signatures = sigs
	txHashBytes := common.Sha3(txToBytes(t, true))
	publishSig := &rpcpb.Signature{
		Algorithm: rpcpb.Signature_Algorithm(s.GetSignAlgo()),
		Signature: s.GetSignAlgo().Sign(txHashBytes, s.keyPair.Seckey),
		PublicKey: s.GetSignAlgo().GetPubkey(s.keyPair.Seckey),
	}
	t.PublisherSigs = []*rpcpb.Signature{publishSig}
	t.Publisher = s.accountName
	return t, nil
}

func (s *SDK) getSignAlgoName() string {
	return s.signAlgo
}

// GetSignAlgo ...
func (s *SDK) GetSignAlgo() crypto.Algorithm {
	return s.GetSignAlgoByName(s.getSignAlgoName())
}

// GetSignAlgoByName ...
func (s *SDK) GetSignAlgoByName(name string) crypto.Algorithm {
	switch name {
	case "secp256k1":
		return crypto.Secp256k1
	case "ed25519":
		return crypto.Ed25519
	default:
		return crypto.Ed25519
	}
}

// GetSignAlgoByEnum ...
func (s *SDK) GetSignAlgoByEnum(enum rpcpb.Signature_Algorithm) crypto.Algorithm {
	switch enum {
	case rpcpb.Signature_SECP256K1:
		return crypto.Secp256k1
	case rpcpb.Signature_ED25519:
		return crypto.Ed25519
	default:
		return crypto.Ed25519
	}
}

func (s *SDK) checkPubKey(k string) bool {
	if k == "" {
		return false
	}
	return true
}

// Connect ...
func (s *SDK) Connect() (err error) {
	if s.rpcConn == nil {
		if s.verbose {
			fmt.Println("Connecting to server", s.server, "...")
		}
		s.rpcConn, err = grpc.Dial(s.server, grpc.WithInsecure())
	}
	return
}

// CloseConn ...
func (s *SDK) CloseConn() {
	if s.rpcConn != nil {
		s.rpcConn.Close()
		s.rpcConn = nil
	}
}

// GetContractStorage ...
func (s *SDK) GetContractStorage(r *rpcpb.GetContractStorageRequest) (*rpcpb.GetContractStorageResponse, error) {
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

// GetProducerVoteInfo ...
func (s *SDK) GetProducerVoteInfo(r *rpcpb.GetProducerVoteInfoRequest) (*rpcpb.GetProducerVoteInfoResponse, error) {
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

func (s *SDK) getNodeInfo() (*rpcpb.NodeInfoResponse, error) {
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

func (s *SDK) getChainInfo() (*rpcpb.ChainInfoResponse, error) {
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

// getAccountInfo return account info
func (s *SDK) getAccountInfo(id string) (*rpcpb.Account, error) {
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
func (s *SDK) getGetBlockByNum(num int64, complete bool) (*rpcpb.BlockResponse, error) {
	if s.rpcConn == nil {
		if err := s.Connect(); err != nil {
			return nil, err
		}
		defer s.CloseConn()
	}
	client := rpcpb.NewApiServiceClient(s.rpcConn)
	return client.GetBlockByNumber(context.Background(), &rpcpb.GetBlockByNumberRequest{Number: num, Complete: complete})
}

func (s *SDK) getGetBlockByHash(hash string, complete bool) (*rpcpb.BlockResponse, error) {
	if s.rpcConn == nil {
		if err := s.Connect(); err != nil {
			return nil, err
		}
		defer s.CloseConn()
	}
	client := rpcpb.NewApiServiceClient(s.rpcConn)
	return client.GetBlockByHash(context.Background(), &rpcpb.GetBlockByHashRequest{Hash: hash, Complete: complete})
}

func (s *SDK) getTxByHash(hash string) (*rpcpb.TransactionResponse, error) {
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
func (s *SDK) GetTxReceiptByTxHash(txHashStr string) (*rpcpb.TxReceipt, error) {
	if s.rpcConn == nil {
		if err := s.Connect(); err != nil {
			return nil, err
		}
		defer s.CloseConn()
	}
	client := rpcpb.NewApiServiceClient(s.rpcConn)
	return client.GetTxReceiptByTxHash(context.Background(), &rpcpb.TxHashRequest{Hash: txHashStr})
}

func (s *SDK) sendTx(signedTx *rpcpb.TransactionRequest) (string, error) {
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

func (s *SDK) checkTransaction(txHash string) error {
	fmt.Println("Checking transaction receipt...")
	for i := int32(0); i < s.checkResultMaxRetry; i++ {
		time.Sleep(time.Duration(s.checkResultDelay*1000) * time.Millisecond)
		txReceipt, err := s.GetTxReceiptByTxHash(txHash)
		if err != nil {
			fmt.Println("...", err)
			continue
		}
		if txReceipt == nil {
			fmt.Println("...")
			continue
		}
		if txReceipt.StatusCode != rpcpb.TxReceipt_SUCCESS {
			if s.verbose {
				fmt.Println("Transaction receipt:")
				fmt.Println(marshalTextString(txReceipt))
			}
			return fmt.Errorf(txReceipt.Message)
		}

		fmt.Println("SUCCESS!")
		if s.verbose {
			fmt.Println("Transaction receipt:")
			fmt.Println(marshalTextString(txReceipt))
		}
		return nil
	}
	return fmt.Errorf("exceeded max retry times")
}

func (s *SDK) getAccountDir() (string, error) {
	home, err := homedir.Dir()
	if err != nil {
		return "", err
	}
	return home + "/.iwallet", nil
}

// LoadAccount load account from file
func (s *SDK) LoadAccount() error {
	if s.accountName == "" {
		return fmt.Errorf("you must provide account name")
	}
	if s.keyPair != nil {
		return nil
	}
	dir, err := s.getAccountDir()
	if err != nil {
		return err
	}
	privKeyFile := fmt.Sprintf("%s/%s_%s", dir, s.accountName, s.getSignAlgoName())
	s.keyPair, err = loadKeyPair(privKeyFile, s.GetSignAlgo())
	if err != nil {
		return err
	}
	return nil
}

// SaveAccount save account to file
func (s *SDK) SaveAccount(name string, kp *account.KeyPair) error {
	dir, err := s.getAccountDir()
	if err != nil {
		return err
	}
	err = os.MkdirAll(dir, 0700)
	if err != nil {
		return err
	}
	fileName := dir + "/" + name
	if kp.Algorithm == crypto.Ed25519 {
		fileName += "_ed25519"
	}
	if kp.Algorithm == crypto.Secp256k1 {
		fileName += "_secp256k1"
	}

	pubfile, err := os.Create(fileName + ".pub")
	if err != nil {
		return err
	}
	defer pubfile.Close()

	_, err = pubfile.WriteString(common.Base58Encode(kp.Pubkey))
	if err != nil {
		return err
	}

	secFile, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0400)
	if err != nil {
		return err
	}
	defer secFile.Close()

	_, err = secFile.WriteString(common.Base58Encode(kp.Seckey))
	if err != nil {
		return err
	}

	fmt.Println("Your account private key is saved at:", fileName)
	return nil
}

// PledgeForGasAndRAM ...
func (s *SDK) PledgeForGasAndRAM(gasPledged int64, ram int64) error {
	var acts []*rpcpb.Action
	acts = append(acts, NewAction("gas.iost", "pledge", fmt.Sprintf(`["%v", "%v", "%v"]`, s.accountName, s.accountName, gasPledged)))
	if ram > 0 {
		acts = append(acts, NewAction("ram.iost", "buy", fmt.Sprintf(`["%v", "%v", %v]`, s.accountName, s.accountName, ram)))
	}
	_, err := s.SendTxFromActions(acts)
	if err != nil {
		return err
	}

	info, err := s.getAccountInfo(s.accountName)
	if err != nil {
		return fmt.Errorf("failed to get account info: %v", err)
	}
	fmt.Println("Account info of <", s.accountName, ">:")
	fmt.Println(marshalTextString(info))
	return nil
}

// CreateNewAccount ... return txHash
func (s *SDK) CreateNewAccount(newID string, ownerKey string, activeKey string, initialGasPledge int64, initialRAM int64, initialCoins int64) (string, error) {
	var acts []*rpcpb.Action
	acts = append(acts, NewAction("auth.iost", "signUp", fmt.Sprintf(`["%v", "%v", "%v"]`, newID, ownerKey, activeKey)))
	if initialRAM > 0 {
		acts = append(acts, NewAction("ram.iost", "buy", fmt.Sprintf(`["%v", "%v", %v]`, s.accountName, newID, initialRAM)))
	}
	var registerInitialPledge int64 = 10
	initialGasPledge -= registerInitialPledge
	if initialGasPledge < 0 {
		return "", fmt.Errorf("min gas pledge is 10")
	}
	if initialGasPledge > 0 {
		acts = append(acts, NewAction("gas.iost", "pledge", fmt.Sprintf(`["%v", "%v", "%v"]`, s.accountName, newID, initialGasPledge)))
	}
	if initialCoins > 0 {
		acts = append(acts, NewAction("token.iost", "transfer", fmt.Sprintf(`["iost", "%v", "%v", "%v", ""]`, s.accountName, newID, initialCoins)))
	}
	txHash, err := s.SendTxFromActions(acts)
	if err != nil {
		return txHash, err
	}
	info, err := s.getAccountInfo(newID)
	if err != nil {
		return txHash, fmt.Errorf("failed to get account info: %v", err)
	}
	fmt.Println("Account info of <", newID, ">:")
	fmt.Println(marshalTextString(info))
	return txHash, nil
}

// PublishContract converts contract js code to transaction. If 'send', also send it to chain.
func (s *SDK) PublishContract(codePath string, abiPath string, conID string, update bool, updateID string) (*rpcpb.TransactionRequest, string, error) {
	fd, err := readFile(codePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read source code file: %v", err)
	}
	code := string(fd)

	fd, err = readFile(abiPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read abi file: %v", err)
	}
	abi := string(fd)

	var info *contract.Info
	err = json.Unmarshal([]byte(abi), &info)
	if err != nil {
		return nil, "", err
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
			return nil, "", err
		}
		contractStr = string(buf)
	} else {
		buf, err := proto.Marshal(c)
		if err != nil {
			return nil, "", err
		}
		contractStr = base64.StdEncoding.EncodeToString(buf)
	}
	arr := []string{contractStr}
	if update {
		arr = append(arr, updateID)
	}
	data, err := json.Marshal(arr)
	if err != nil {
		return nil, "", err
	}
	action := NewAction("system.iost", methodName, string(data))
	trx, err := s.createTx([]*rpcpb.Action{action})
	if err != nil {
		return nil, "", err
	}
	txHash, err := s.SendTx(trx)
	if err != nil {
		return nil, "", err
	}
	return trx, txHash, nil
}

// SendTxFromActions send transaction and check result if sdk.checkResult is set
func (s *SDK) SendTxFromActions(actions []*rpcpb.Action) (txHash string, err error) {
	trx, err := s.createTx(actions)
	if err != nil {
		return "", err
	}
	return s.SendTx(trx)
}

// SendTx send transaction and check result if sdk.checkResult is set
func (s *SDK) SendTx(tx *rpcpb.TransactionRequest) (string, error) {
	signedTx, err := s.signTx(tx)
	if err != nil {
		return "", fmt.Errorf("sign tx error %v", err)
	}
	fmt.Println("Sending transaction...")
	if s.verbose {
		fmt.Println("Transaction:")
		fmt.Println(marshalTextString(signedTx))
	}
	txHash, err := s.sendTx(signedTx)
	if err != nil {
		return "", fmt.Errorf("send tx error %v", err)
	}
	fmt.Println("Transaction has been sent.")
	fmt.Println("The transaction hash is:", txHash)
	if s.checkResult {
		if err = s.checkTransaction(txHash); err != nil {
			return txHash, err
		}
	}
	return txHash, nil
}

func actionToBytes(a *rpcpb.Action) []byte {
	se := common.NewSimpleEncoder()
	se.WriteString(a.Contract)
	se.WriteString(a.ActionName)
	se.WriteString(a.Data)
	return se.Bytes()
}

func amountToBytes(a *rpcpb.AmountLimit) []byte {
	se := common.NewSimpleEncoder()
	se.WriteString(a.Token)
	se.WriteString(a.Value)
	return se.Bytes()
}

func signatureToBytes(s *rpcpb.Signature) []byte {
	se := common.NewSimpleEncoder()
	se.WriteByte(byte(s.Algorithm))
	se.WriteBytes(s.Signature)
	se.WriteBytes(s.PublicKey)
	return se.Bytes()
}

func txToBytes(t *rpcpb.TransactionRequest, withSign bool) []byte {
	se := common.NewSimpleEncoder()
	se.WriteInt64(t.Time)
	se.WriteInt64(t.Expiration)
	se.WriteInt64(int64(t.GasRatio * 100))
	se.WriteInt64(int64(t.GasLimit * 100))
	se.WriteInt64(t.Delay)
	se.WriteInt32(int32(t.ChainId))
	se.WriteBytes(nil)
	se.WriteStringSlice(t.Signers)

	actionBytes := make([][]byte, 0, len(t.Actions))
	for _, a := range t.Actions {
		actionBytes = append(actionBytes, actionToBytes(a))
	}
	se.WriteBytesSlice(actionBytes)

	amountBytes := make([][]byte, 0, len(t.AmountLimit))
	for _, a := range t.AmountLimit {
		amountBytes = append(amountBytes, amountToBytes(a))
	}
	se.WriteBytesSlice(amountBytes)

	if withSign {
		signBytes := make([][]byte, 0, len(t.Signatures))
		for _, sig := range t.Signatures {
			signBytes = append(signBytes, signatureToBytes(sig))
		}
		se.WriteBytesSlice(signBytes)
	}

	return se.Bytes()
}

// NewAction ...
func NewAction(contract string, name string, data string) *rpcpb.Action {
	return &rpcpb.Action{
		Contract:   contract,
		ActionName: name,
		Data:       data,
	}
}
