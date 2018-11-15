package iwallet

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/iost-official/go-iost/account"
	"github.com/iost-official/go-iost/core/contract"
	"github.com/iost-official/go-iost/core/tx"
	"github.com/iost-official/go-iost/core/tx/pb"
	"github.com/iost-official/go-iost/crypto"
	"github.com/iost-official/go-iost/rpc"
	"github.com/mitchellh/go-homedir"
	"google.golang.org/grpc"
	"os"
	"path/filepath"
	"time"
)

// SDK ...
type SDK struct {
	isLocal             bool
	server              string
	kpPath              string
	signAlgo            string
	gasLimit            int64
	gasPrice            int64
	expiration          int64
	delaySecond         int64
	accountDir          string
	accountName         string
	checkResult         bool
	checkResultDelay    float32
	checkResultMaxRetry int32
	useLongestChain     bool
	keyPair             *account.KeyPair
}

var sdk = &SDK{
	signAlgo: "ed25519",
}

// SetAccount ...
func (s *SDK) SetAccount(name string, kp *account.KeyPair) {
	s.accountName = name
	s.keyPair = kp
}

// SetTxInfo ...
func (s *SDK) SetTxInfo(gasLimit int64, gasPrice int64, expiration int64, delaySecond int64) {
	s.gasLimit = gasLimit
	s.gasPrice = gasPrice
	s.expiration = expiration
	s.delaySecond = delaySecond
}

// SetServer ...
func (s *SDK) SetServer(server string) {
	s.server = server
}

// CreateNewAccount ...
func (s *SDK) CreateNewAccount(newID string, newKp *account.KeyPair, initialGasPledge int64, initialRAM int64, initialCoins int64) error {
	var acts []*tx.Action
	acts = append(acts, tx.NewAction("iost.auth", "SignUp", fmt.Sprintf(`["%v", "%v", "%v"]`, newID, newKp.ID, newKp.ID)))
	acts = append(acts, tx.NewAction("iost.ram", "buy", fmt.Sprintf(`["%v", "%v", %v]`, s.accountName, newID, initialRAM)))
	acts = append(acts, tx.NewAction("iost.gas", "pledge", fmt.Sprintf(`["%v", "%v", "%v"]`, s.accountName, newID, initialGasPledge)))
	if initialCoins != 0 {
		acts = append(acts, tx.NewAction("iost.token", "transfer", fmt.Sprintf(`["iost", "%v", "%v", "%v", ""]`, s.accountName, newID, initialCoins)))
	}
	trx := s.createTx(acts)
	stx, err := s.signTx(trx)
	if err != nil {
		return err
	}
	var txHash string
	txHash, err = s.sendTx(stx)
	if err != nil {
		return err
	}
	fmt.Printf("send tx done\n")
	fmt.Println("the create user transaction hash is:", txHash)
	if s.checkResult {
		s.checkTransaction(txHash)
	}
	fmt.Printf("\nbalance of %v\n", newID)
	info, err := s.GetAccountInfo(newID)
	if err != nil {
		return err
	}
	fmt.Println(info)
	return nil
}

// GetAccountInfo return account info
func (s *SDK) GetAccountInfo(id string) (*rpc.GetAccountRes, error) {
	conn, err := grpc.Dial(s.server, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	client := rpc.NewApisClient(conn)
	req := rpc.GetAccountReq{ID: id}
	if s.useLongestChain {
		req.UseLongestChain = true
	}
	value, err := client.GetAccountInfo(context.Background(), &req)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (s *SDK) getGetBlockByNum(num int64, complete bool) (*rpc.BlockInfo, error) {
	conn, err := grpc.Dial(s.server, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	client := rpc.NewApisClient(conn)
	return client.GetBlockByNum(context.Background(), &rpc.BlockByNumReq{Num: num, Complete: complete})
}

func (s *SDK) getGetBlockByHash(hash string, complete bool) (*rpc.BlockInfo, error) {
	conn, err := grpc.Dial(s.server, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	client := rpc.NewApisClient(conn)
	return client.GetBlockByHash(context.Background(), &rpc.BlockByHashReq{Hash: hash, Complete: complete})
}

func (s *SDK) getTxByHash(hash string) (*rpc.TxRes, error) {
	conn, err := grpc.Dial(s.server, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	client := rpc.NewApisClient(conn)
	return client.GetTxByHash(context.Background(), &rpc.HashReq{Hash: hash})
}

func (s *SDK) createTx(actions []*tx.Action) *tx.Tx {
	trx := tx.NewTx(actions, []string{}, s.gasLimit, s.gasPrice, time.Now().Add(time.Second*time.Duration(s.expiration)).UnixNano(), s.delaySecond*1e9)
	return trx
}

func (s *SDK) signTx(t *tx.Tx) (*tx.Tx, error) {
	return tx.SignTx(t, s.accountName, []*account.KeyPair{s.keyPair})
}

func (s *SDK) sendTx(stx *tx.Tx) (string, error) {
	conn, err := grpc.Dial(s.server, grpc.WithInsecure())
	if err != nil {
		return "", err
	}
	defer conn.Close()
	client := rpc.NewApisClient(conn)
	resp, err := client.SendTx(context.Background(), &rpc.TxReq{Tx: stx.ToPb()})
	if err != nil {
		return "", err
	}
	return resp.Hash, nil
}

func (s *SDK) checkTransaction(txHash string) bool {
	// It may be better to to create a grpc client and reuse it. TODO later
	for i := int32(0); i < s.checkResultMaxRetry; i++ {
		time.Sleep(time.Duration(s.checkResultDelay*1000) * time.Millisecond)
		txReceipt, err := s.getTxReceiptByTxHash(txHash)
		if err != nil {
			fmt.Println("result not ready, please wait. Details: ", err)
			continue
		}
		if txReceipt == nil {
			fmt.Println("result not ready, please wait.")
			continue
		}
		if tx.StatusCode(txReceipt.Status.Code) != tx.Success {
			fmt.Println("exec tx failed: ", txReceipt.Status.Message)
			fmt.Println("full error information: ", txReceipt)
		} else {
			fmt.Println("exec tx done. ", txReceipt.String())
			return true
		}
		break
	}
	return false
}

func (s *SDK) getNodeInfo() (*rpc.NodeInfoRes, error) {
	conn, err := grpc.Dial(s.server, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	client := rpc.NewApisClient(conn)
	value, err := client.GetNodeInfo(context.Background(), &empty.Empty{})
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (s *SDK) getTxReceiptByTxHash(txHashStr string) (*txpb.TxReceipt, error) {
	conn, err := grpc.Dial(s.server, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	client := rpc.NewApisClient(conn)
	resp, err := client.GetTxReceiptByTxHash(context.Background(), &rpc.HashReq{Hash: txHashStr})
	if err != nil {
		return nil, err
	}
	//ilog.Debugf("getTxReceiptByTxHash(%v): %v", txHashStr, resp.TxReceiptRaw)
	return resp.TxReceipt, nil
}

func (s *SDK) getSignAlgoName() string {
	return s.signAlgo
}

func (s *SDK) getSignAlgo() crypto.Algorithm {
	switch s.getSignAlgoName() {
	case "secp256k1":
		return crypto.Secp256k1
	case "ed25519":
		return crypto.Ed25519
	default:
		return crypto.Ed25519
	}
}

func (s *SDK) loadAccount() error {
	dir, err := s.getAccountDir()
	if err != nil {
		return err
	}
	if s.accountName == "" {
		return fmt.Errorf("you must provide account name")
	}
	kpPath := fmt.Sprintf("%s/%s_%s", dir, s.accountName, s.getSignAlgoName())
	fsk, err := readFile(kpPath)
	if err != nil {
		return fmt.Errorf("read file failed: %v", err)
	}
	s.keyPair, err = account.NewKeyPair(loadBytes(string(fsk)), s.getSignAlgo())
	if err != nil {
		return err
	}
	return nil
}

func (s *SDK) getAccountDir() (string, error) {
	if s.accountDir != "" {
		// TODO move this code to load condig
		if !filepath.IsAbs(s.accountDir) {
			dir, err := filepath.Abs(s.accountDir)
			if err != nil {
				return "", err
			}
			return dir, nil
		}
		return s.accountDir, nil
	}
	home, err := homedir.Dir()
	if err != nil {
		return "", err
	}
	return home + "/.iwallet", nil
}

func (s *SDK) saveAccount(name string, kp *account.KeyPair) error {
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

	_, err = pubfile.WriteString(saveBytes(kp.Pubkey))
	if err != nil {
		return err
	}

	secFile, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer secFile.Close()

	_, err = secFile.WriteString(saveBytes(kp.Seckey))
	if err != nil {
		return err
	}

	idFileName := fileName + ".id"
	idFile, err := os.Create(idFileName)
	if err != nil {
		return err
	}
	defer idFile.Close()
	id := account.GetIDByPubkey(kp.Pubkey)
	_, err = idFile.WriteString(id)
	if err != nil {
		return err
	}

	fmt.Println("create account done")
	fmt.Println("the iost account ID is:")
	fmt.Println(name)
	//fmt.Println("your account id is saved at:")
	//fmt.Println(idFileName)
	fmt.Println("your account private key is saved at:")
	fmt.Println(fileName)
	return nil
}

// PublishContract converts contract js code to transaction. If 'send', also send it to chain.
func (s *SDK) PublishContract(codePath string, abiPath string, conID string, update bool, updateID string) (stx *tx.Tx, txHash string, err error) {
	fd, err := readFile(codePath)
	if err != nil {
		fmt.Println("Read source code file failed: ", err.Error())
		return nil, "", err
	}
	code := string(fd)

	fd, err = readFile(abiPath)
	if err != nil {
		fmt.Println("Read abi file failed: ", err.Error())
		return nil, "", err
	}
	abi := string(fd)

	compiler := new(contract.Compiler)
	if compiler == nil {
		fmt.Println("gen compiler instance failed")
		return nil, "", err
	}
	c, err := compiler.Parse(conID, code, abi)
	if err != nil {
		fmt.Printf("gen contract error:%v\n", err)
		return nil, "", err
	}

	methodName := "SetCode"
	data := `["` + c.B64Encode() + `"]`
	if update {
		methodName = "UpdateCode"
		data = `["` + c.B64Encode() + `", "` + updateID + `"]`
	}

	action := tx.NewAction("iost.system", methodName, data)
	trx := s.createTx([]*tx.Action{action})
	stx, err = s.signTx(trx)
	if err != nil {
		return nil, "", fmt.Errorf("sign tx error %v", err)
	}
	var hash string
	hash, err = s.sendTx(stx)
	if err != nil {
		fmt.Println(err.Error())
		return nil, "", err
	}
	fmt.Println("Sending tx to rpc server finished. The transaction hash is:", hash)
	return trx, hash, nil
}