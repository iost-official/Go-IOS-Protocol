package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	blockpb "github.com/iost-official/go-iost/v3/core/block/pb"

	"github.com/syndtr/goleveldb/leveldb/util"

	simplejson "github.com/bitly/go-simplejson"
	"github.com/iost-official/go-iost/v3/vm"

	"github.com/iost-official/go-iost/v3/chainbase"
	"github.com/iost-official/go-iost/v3/common"
	"github.com/iost-official/go-iost/v3/consensus/cverifier"
	"github.com/iost-official/go-iost/v3/core/block"
	"github.com/iost-official/go-iost/v3/core/blockcache"
	"github.com/iost-official/go-iost/v3/core/event"
	"github.com/iost-official/go-iost/v3/core/global"
	"github.com/iost-official/go-iost/v3/core/tx"
	"github.com/iost-official/go-iost/v3/core/txpool"
	"github.com/iost-official/go-iost/v3/db"
	"github.com/iost-official/go-iost/v3/ilog"
	"github.com/iost-official/go-iost/v3/p2p"
	rpcpb "github.com/iost-official/go-iost/v3/rpc/pb"
	"github.com/iost-official/go-iost/v3/verifier"
	"github.com/iost-official/go-iost/v3/vm/database"
	"github.com/iost-official/go-iost/v3/vm/host"
)

//go:generate mockgen -destination mock_rpc/mock_api.go -package main github.com/iost-official/go-iost/v3/rpc/pb ApiServiceServer

// APIService implements all rpc APIs.
type APIService struct {
	bc         blockcache.BlockCache
	p2pService p2p.Service
	txpool     txpool.TxPool
	blockchain block.Chain
	stateDB    db.MVCCDB
	config     *common.Config

	quitCh chan struct{}
}

// NewAPIService returns a new APIService instance.
func NewAPIService(tp txpool.TxPool, chainBase *chainbase.ChainBase, config *common.Config, p2pService p2p.Service, quitCh chan struct{}) *APIService {
	return &APIService{
		p2pService: p2pService,
		txpool:     tp,
		blockchain: chainBase.BlockChain(),
		bc:         chainBase.BlockCache(),
		stateDB:    chainBase.StateDB(),
		config:     config,
		quitCh:     quitCh,
	}
}

// GetNodeInfo returns information abount node.
func (as *APIService) GetNodeInfo(context.Context, *rpcpb.EmptyRequest) (*rpcpb.NodeInfoResponse, error) {
	res := &rpcpb.NodeInfoResponse{
		BuildTime:   global.BuildTime,
		GitHash:     global.GitHash,
		CodeVersion: global.CodeVersion,
		Mode:        common.Mode(),
		Network:     &rpcpb.NetworkInfo{},
		ServerTime:  time.Now().UnixNano(),
	}
	p2pNeighbors := as.p2pService.GetAllNeighbors()
	networkInfo := &rpcpb.NetworkInfo{
		Id:        as.p2pService.ID(),
		PeerCount: int32(len(p2pNeighbors)),
	}
	res.Network = networkInfo
	return res, nil
}

// GetRAMInfo returns the chain info.
func (as *APIService) GetRAMInfo(context.Context, *rpcpb.EmptyRequest) (*rpcpb.RAMInfoResponse, error) {
	dbVisitor, _, err := as.getStateDBVisitor(true)
	if err != nil {
		return nil, err
	}
	return &rpcpb.RAMInfoResponse{
		AvailableRam: dbVisitor.LeftRAM(),
		UsedRam:      dbVisitor.UsedRAM(),
		TotalRam:     dbVisitor.TotalRAM(),
		SellPrice:    dbVisitor.SellPrice(),
		BuyPrice:     dbVisitor.BuyPrice(),
	}, nil
}

// GetChainInfo returns the chain info.
func (as *APIService) GetChainInfo(context.Context, *rpcpb.EmptyRequest) (*rpcpb.ChainInfoResponse, error) {
	head := as.bc.Head()
	lib := as.bc.LinkedRoot()
	netName := "unknown"
	version := "unknown"
	if as.config.Version != nil {
		netName = as.config.Version.NetName
		version = as.config.Version.ProtocolVersion
	}
	return &rpcpb.ChainInfoResponse{
		NetName:            netName,
		ProtocolVersion:    version,
		ChainId:            as.config.P2P.ChainID,
		WitnessList:        head.Active(),
		LibWitnessList:     lib.Active(),
		PendingWitnessList: head.Pending(),
		HeadBlock:          head.Head.Number,
		HeadBlockHash:      common.Base58Encode(head.HeadHash()),
		LibBlock:           lib.Head.Number,
		LibBlockHash:       common.Base58Encode(lib.HeadHash()),
		HeadBlockTime:      head.Head.Time,
		LibBlockTime:       lib.Head.Time,
	}, nil
}

// GetTxByHash returns the transaction corresponding to the given hash.
func (as *APIService) GetTxByHash(ctx context.Context, req *rpcpb.TxHashRequest) (*rpcpb.TransactionResponse, error) {
	err := checkHashValid(req.GetHash())
	if err != nil {
		return nil, err
	}
	txHashBytes := common.Base58Decode(req.GetHash())
	status := rpcpb.TransactionResponse_IRREVERSIBLE
	var (
		t         *tx.Tx
		txReceipt *tx.TxReceipt
	)
	blockNumber := int64(-1)

	t, err = as.blockchain.GetTx(txHashBytes)
	if err != nil {
		status = rpcpb.TransactionResponse_PACKED
		t, txReceipt, err = as.txpool.GetFromChain(txHashBytes)
		if err != nil {
			status = rpcpb.TransactionResponse_PENDING
			t, err = as.txpool.GetFromPending(txHashBytes)
			if err != nil {
				return nil, errors.New("tx not found")
			}
		}
	} else {
		txReceipt, err = as.blockchain.GetReceiptByTxHash(txHashBytes)
		if err != nil {
			return nil, errors.New("txreceipt not found")
		}
		blockNumber, err = as.blockchain.GetBlockNumberByTxHash(txHashBytes)
		if err != nil {
			return nil, errors.New("number of block not found")
		}
	}

	return &rpcpb.TransactionResponse{
		Status:      status,
		Transaction: toPbTx(t, txReceipt),
		BlockNumber: blockNumber,
	}, nil
}

// GetTxReceiptByTxHash returns transaction receipts corresponding to the given tx hash.
func (as *APIService) GetTxReceiptByTxHash(ctx context.Context, req *rpcpb.TxHashRequest) (*rpcpb.TxReceipt, error) {
	err := checkHashValid(req.GetHash())
	if err != nil {
		return nil, err
	}
	txHashBytes := common.Base58Decode(req.GetHash())
	receipt, err := as.blockchain.GetReceiptByTxHash(txHashBytes)
	if err != nil {
		return nil, err
	}
	return toPbTxReceipt(receipt), nil
}

// GetBlockByHash returns block corresponding to the given hash.
func (as *APIService) GetBlockByHash(ctx context.Context, req *rpcpb.GetBlockByHashRequest) (*rpcpb.BlockResponse, error) {
	err := checkHashValid(req.GetHash())
	if err != nil {
		return nil, err
	}
	hashBytes := common.Base58Decode(req.GetHash())
	var (
		blk *block.Block
	)
	status := rpcpb.BlockResponse_IRREVERSIBLE
	blk, err = as.blockchain.GetBlockByHash(hashBytes)
	if err != nil {
		status = rpcpb.BlockResponse_PENDING
		blk, err = as.bc.GetBlockByHash(hashBytes)
		if err != nil {
			return nil, err
		}
	}
	return &rpcpb.BlockResponse{
		Status: status,
		Block:  toPbBlock(blk, req.GetComplete()),
	}, nil
}

// GetBlockByNumber returns block corresponding to the given number.
func (as *APIService) GetBlockByNumber(ctx context.Context, req *rpcpb.GetBlockByNumberRequest) (*rpcpb.BlockResponse, error) {
	number := req.GetNumber()
	var (
		blk *block.Block
		err error
	)
	status := rpcpb.BlockResponse_IRREVERSIBLE
	blk, err = as.blockchain.GetBlockByNumber(number)
	if err != nil {
		status = rpcpb.BlockResponse_PENDING
		blk, err = as.bc.GetBlockByNumber(number)
		if err != nil {
			return nil, err
		}
	}
	return &rpcpb.BlockResponse{
		Status: status,
		Block:  toPbBlock(blk, req.GetComplete()),
	}, nil
}

// GetBlockByNumber returns block corresponding to the given number.
func (as *APIService) GetRawBlockByNumber(ctx context.Context, req *rpcpb.GetBlockByNumberRequest) (*rpcpb.RawBlockResponse, error) {
	number := req.GetNumber()
	var (
		blk *block.Block
		err error
	)
	status := rpcpb.RawBlockResponse_IRREVERSIBLE
	blk, err = as.blockchain.GetBlockByNumber(number)
	if err != nil {
		status = rpcpb.RawBlockResponse_PENDING
		blk, err = as.bc.GetBlockByNumber(number)
		if err != nil {
			return nil, err
		}
	}
	var mode blockpb.BlockType
	if req.Complete {
		mode = blockpb.BlockType_NORMAL
	} else {
		mode = blockpb.BlockType_ONLYHASH
	}
	return &rpcpb.RawBlockResponse{
		Status: status,
		Block:  blk.ToPb(mode),
	}, nil
}

// GetBlockHeaderByRange returns block headers of a range
func (as *APIService) GetBlockHeaderByRange(ctx context.Context, req *rpcpb.GetBlockHeaderByRangeRequest) (*rpcpb.BlockHeaderByRangeResponse, error) {
	start := req.Start
	end := req.End
	var rangeLimit int64 = 110
	if end <= start || end > start+rangeLimit {
		return nil, fmt.Errorf("invalid range %v to %v", start, end)
	}
	res := &rpcpb.BlockHeaderByRangeResponse{
		BlockList: make([]*blockpb.Block, 0, end-start),
	}
	for bn := start; bn < end; bn++ {
		b, err := as.blockchain.GetBlockByNumber(bn)
		if err != nil {
			break
		}
		blk := b.ToPb(blockpb.BlockType_ONLYHASH)
		blk.TxHashes = nil
		blk.Receipts = nil
		blk.ReceiptHashes = nil
		blk.Txs = nil
		res.BlockList = append(res.BlockList, blk)
	}
	return res, nil
}

// GetAccount returns account information corresponding to the given account name.
func (as *APIService) GetAccount(ctx context.Context, req *rpcpb.GetAccountRequest) (*rpcpb.Account, error) {
	err := checkIDValid(req.GetName())
	if err != nil {
		return nil, err
	}

	dbVisitor, _, err := as.getStateDBVisitor(req.ByLongestChain)
	if err != nil {
		return nil, err
	}
	// pack basic account information
	acc, _ := host.ReadAuth(dbVisitor, req.GetName())
	if acc == nil {
		return nil, errors.New("account not found")
	}
	ret := toPbAccount(acc)

	// pack balance and ram information
	balance := dbVisitor.TokenBalanceFixed("iost", req.GetName()).Float64()
	ret.Balance = balance
	ramInfo := dbVisitor.RAMHandler.GetAccountRAMInfo(req.GetName())
	ret.RamInfo = &rpcpb.Account_RAMInfo{
		Available: ramInfo.Available,
		Used:      ramInfo.Used,
		Total:     ramInfo.Total,
	}

	// pack gas information
	var blkTime int64
	if req.GetByLongestChain() {
		blkTime = as.bc.Head().Head.Time
	} else {
		blkTime = as.bc.LinkedRoot().Head.Time
	}
	totalGas := dbVisitor.PGasAtTime(req.GetName(), blkTime)
	gasLimit := dbVisitor.GasLimit(req.GetName())
	gasRate := dbVisitor.GasPledgeTotal(req.GetName()).Mul(database.GasIncreaseRate)
	pledgedInfo := dbVisitor.PledgerInfo(req.GetName())
	ret.GasInfo = &rpcpb.Account_GasInfo{
		CurrentTotal:    totalGas.Float64(),
		PledgeGas:       totalGas.Float64(),
		TransferableGas: 0,
		Limit:           gasLimit.Float64(),
		IncreaseSpeed:   gasRate.Float64(),
	}
	for _, p := range pledgedInfo {
		ret.GasInfo.PledgedInfo = append(ret.GasInfo.PledgedInfo, &rpcpb.Account_PledgeInfo{
			Amount:  p.Amount.Float64(),
			Pledger: p.Pledger,
		})
	}

	// pack frozen balance information
	frozen := dbVisitor.AllFreezedTokenBalanceFixed("iost", req.GetName())
	unfrozen, stillFrozen := as.getUnfrozenToken(frozen, req.ByLongestChain)
	ret.FrozenBalances = stillFrozen
	ret.Balance += unfrozen

	voteInfo := dbVisitor.GetAccountVoteInfo(req.GetName())
	for _, v := range voteInfo {
		ret.VoteInfos = append(ret.VoteInfos, &rpcpb.VoteInfo{
			Option:       v.Option,
			Votes:        v.Votes.Float64(),
			ClearedVotes: v.ClearedVotes.Float64(),
		})
	}

	return ret, nil
}

// GetTokenBalance returns contract information corresponding to the given contract ID.
func (as *APIService) GetTokenBalance(ctx context.Context, req *rpcpb.GetTokenBalanceRequest) (*rpcpb.GetTokenBalanceResponse, error) {
	err := checkIDValid(req.GetAccount())
	if err != nil {
		return nil, err
	}

	dbVisitor, _, err := as.getStateDBVisitor(req.ByLongestChain)
	if err != nil {
		return nil, err
	}
	//acc, _ := host.ReadAuth(dbVisitor, req.GetAccount())
	//if acc == nil {
	//	return nil, errors.New("account not found")
	//}
	balance := dbVisitor.TokenBalanceFixed(req.GetToken(), req.GetAccount()).Float64()
	// pack frozen balance information
	frozen := dbVisitor.AllFreezedTokenBalanceFixed(req.GetToken(), req.GetAccount())
	unfrozen, stillFrozen := as.getUnfrozenToken(frozen, req.ByLongestChain)
	return &rpcpb.GetTokenBalanceResponse{
		Balance:        balance + unfrozen,
		FrozenBalances: stillFrozen,
	}, nil
}

// GetToken721Balance returns balance of account of an specific token721 token.
func (as *APIService) GetToken721Balance(ctx context.Context, req *rpcpb.GetTokenBalanceRequest) (*rpcpb.GetToken721BalanceResponse, error) {
	err := checkIDValid(req.GetAccount())
	if err != nil {
		return nil, err
	}

	dbVisitor, _, err := as.getStateDBVisitor(req.ByLongestChain)
	if err != nil {
		return nil, err
	}
	acc, _ := host.ReadAuth(dbVisitor, req.GetAccount())
	if acc == nil {
		return nil, errors.New("account not found")
	}
	balance := dbVisitor.Token721Balance(req.GetToken(), req.GetAccount())
	ids := dbVisitor.Token721IDList(req.GetToken(), req.GetAccount())
	return &rpcpb.GetToken721BalanceResponse{
		Balance:  balance,
		TokenIDs: ids,
	}, nil
}

// GetToken721Metadata returns metadata of an specific token721 token.
func (as *APIService) GetToken721Metadata(ctx context.Context, req *rpcpb.GetToken721InfoRequest) (*rpcpb.GetToken721MetadataResponse, error) {
	dbVisitor, _, err := as.getStateDBVisitor(req.ByLongestChain)
	if err != nil {
		return nil, err
	}
	metadata, err := dbVisitor.Token721Metadata(req.GetToken(), req.GetTokenId())
	return &rpcpb.GetToken721MetadataResponse{
		Metadata: metadata,
	}, err
}

// GetToken721Owner returns owner of an specific token721 token.
func (as *APIService) GetToken721Owner(ctx context.Context, req *rpcpb.GetToken721InfoRequest) (*rpcpb.GetToken721OwnerResponse, error) {
	dbVisitor, _, err := as.getStateDBVisitor(req.ByLongestChain)
	if err != nil {
		return nil, err
	}
	owner, err := dbVisitor.Token721Owner(req.GetToken(), req.GetTokenId())
	return &rpcpb.GetToken721OwnerResponse{
		Owner: owner,
	}, err
}

// GetContract returns contract information corresponding to the given contract ID.
func (as *APIService) GetContract(ctx context.Context, req *rpcpb.GetContractRequest) (*rpcpb.Contract, error) {
	err := checkIDValid(req.GetId())
	if err != nil {
		return nil, err
	}
	dbVisitor, _, err := as.getStateDBVisitor(req.ByLongestChain)
	if err != nil {
		return nil, err
	}
	contract := dbVisitor.Contract(req.GetId())
	if contract == nil {
		return nil, errors.New("contract not found")
	}
	return toPbContract(contract), nil
}

// GetContractVote returns contract vote information by contract ID.
func (as *APIService) GetContractVote(ctx context.Context, req *rpcpb.GetContractRequest) (*rpcpb.ContractVote, error) {
	err := checkIDValid(req.GetId())
	if err != nil {
		return nil, err
	}
	dbVisitor, _, err := as.getStateDBVisitor(req.ByLongestChain)
	if err != nil {
		return nil, err
	}

	ret := &rpcpb.ContractVote{}
	voteInfo := dbVisitor.GetAccountVoteInfo(req.GetId())
	for _, v := range voteInfo {
		ret.VoteInfos = append(ret.VoteInfos, &rpcpb.VoteInfo{
			Option:       v.Option,
			Votes:        v.Votes.Float64(),
			ClearedVotes: v.ClearedVotes.Float64(),
		})
	}
	return ret, nil
}

// GetGasRatio returns gas ratio information in head block
func (as *APIService) GetGasRatio(ctx context.Context, req *rpcpb.EmptyRequest) (*rpcpb.GasRatioResponse, error) {
	ratios := make([]float64, 0)
	for _, tx := range as.bc.Head().Block.Txs {
		if tx.Publisher != "base.iost" {
			ratios = append(ratios, float64(tx.GasRatio)/100.0)
		}
	}
	if len(ratios) == 0 {
		return &rpcpb.GasRatioResponse{
			LowestGasRatio: 1.0,
			MedianGasRatio: 1.0,
		}, nil
	}
	sort.Float64s(ratios)
	lowest := ratios[0]
	mid := ratios[len(ratios)/2]
	return &rpcpb.GasRatioResponse{
		LowestGasRatio: lowest,
		MedianGasRatio: mid,
	}, nil
}

// GetProducerVoteInfo returns producers's vote info
func (as *APIService) GetProducerVoteInfo(ctx context.Context, req *rpcpb.GetProducerVoteInfoRequest) (*rpcpb.GetProducerVoteInfoResponse, error) {
	err := checkIDValid(req.Account)
	if err != nil {
		return nil, err
	}
	dbVisitor, _, err := as.getStateDBVisitor(req.ByLongestChain)
	if err != nil {
		return nil, err
	}
	votes, err := dbVisitor.GetProducerVotes(req.Account)
	if err != nil {
		return nil, err
	}
	info, err := dbVisitor.GetProducerVoteInfo(req.Account)
	if err != nil {
		return nil, err
	}
	return &rpcpb.GetProducerVoteInfoResponse{
		Pubkey:     info.Pubkey,
		Loc:        info.Loc,
		Url:        info.URL,
		NetId:      info.NetID,
		IsProducer: info.IsProducer,
		Status:     info.Status,
		Online:     info.Online,
		Votes:      votes.Float64(),
	}, nil
}

// GetContractStorage returns contract storage corresponding to the given key and field.
func (as *APIService) GetContractStorage(ctx context.Context, req *rpcpb.GetContractStorageRequest) (*rpcpb.GetContractStorageResponse, error) {
	dbVisitor, bcn, err := as.getStateDBVisitor(req.ByLongestChain)
	if err != nil {
		return nil, err
	}
	h := host.NewHost(host.NewContext(nil), dbVisitor, bcn.Head.Rules(), nil, nil)
	var value interface{}
	switch {
	case req.GetField() == "":
		value, _ = h.GlobalGet(req.GetId(), req.GetKey())
	default:
		value, _ = h.GlobalMapGet(req.GetId(), req.GetKey(), req.GetField())
	}
	data, err := formatInternalValue(value)
	if err != nil {
		return nil, err
	}
	return &rpcpb.GetContractStorageResponse{
		Data:        data,
		BlockHash:   common.Base58Encode(bcn.HeadHash()),
		BlockNumber: bcn.Head.Number,
	}, nil
}

// GetBatchContractStorage returns contract storage corresponding to the given keys and fields.
func (as *APIService) GetBatchContractStorage(ctx context.Context, req *rpcpb.GetBatchContractStorageRequest) (*rpcpb.GetBatchContractStorageResponse, error) {
	dbVisitor, bcn, err := as.getStateDBVisitor(req.ByLongestChain)
	if err != nil {
		return nil, err
	}
	h := host.NewHost(host.NewContext(nil), dbVisitor, bcn.Head.Rules(), nil, nil)
	var datas []string

	keyFields := req.GetKeyFields()
	if len(keyFields) > 50 {
		keyFields = keyFields[:50]
	}

	for _, keyField := range keyFields {
		var data string
		var value interface{}
		switch {
		case keyField.Field == "":
			value, _ = h.GlobalGet(req.GetId(), keyField.Key)
		default:
			value, _ = h.GlobalMapGet(req.GetId(), keyField.Key, keyField.Field)
		}
		data, err := formatInternalValue(value)
		if err != nil {
			return nil, err
		}
		datas = append(datas, data)
	}

	return &rpcpb.GetBatchContractStorageResponse{
		Datas:       datas,
		BlockHash:   common.Base58Encode(bcn.HeadHash()),
		BlockNumber: bcn.Head.Number,
	}, nil
}

// GetContractStorageFields returns contract storage corresponding to the given fields.
func (as *APIService) GetContractStorageFields(ctx context.Context, req *rpcpb.GetContractStorageFieldsRequest) (*rpcpb.GetContractStorageFieldsResponse, error) {
	dbVisitor, bcn, err := as.getStateDBVisitor(req.ByLongestChain)
	if err != nil {
		return nil, err
	}
	h := host.NewHost(host.NewContext(nil), dbVisitor, bcn.Head.Rules(), nil, nil)

	value, _ := h.GlobalMapKeys(req.GetId(), req.GetKey())

	return &rpcpb.GetContractStorageFieldsResponse{
		Fields:      value,
		BlockHash:   common.Base58Encode(bcn.HeadHash()),
		BlockNumber: bcn.Head.Number,
	}, nil
}

// nolint: gocyclo
func (as *APIService) ListContractStorage(ctx context.Context, req *rpcpb.ListContractStorageRequest) (*rpcpb.ListContractStorageResponse, error) {
	if req.Id == "" {
		return nil, fmt.Errorf("invalid contract id")
	}
	if req.Limit > 100 {
		return nil, fmt.Errorf("invalid limit in request. Max: 500")
	}
	limit := req.Limit
	if limit == 0 {
		limit = 100
	}
	var bcn *blockcache.BlockCacheNode
	if req.ByLongestChain {
		bcn = as.bc.Head()
	} else {
		bcn = as.bc.LinkedRoot()
	}
	mvcc, err := as.getStateDBByBlock(bcn)
	if err != nil {
		return nil, err
	}
	var prefix string
	if req.StorageType == rpcpb.ListContractStorageRequest_KV {
		prefix = database.BasicPrefix + req.Id + database.Separator
	} else {
		prefix = database.MapPrefix + req.Id + database.Separator
	}
	queryRange := util.BytesPrefix([]byte(prefix + req.Prefix))
	if req.From != "" && (prefix+req.From) > string(queryRange.Start) {
		queryRange.Start = []byte(prefix + req.From)
	}
	if req.To != "" && (prefix+req.To) < string(queryRange.Limit) {
		queryRange.Limit = []byte(prefix + req.To)
	}
	keys, err := mvcc.KeysByRange(database.StateTable, string(queryRange.Start),
		string(queryRange.Limit), int(limit))
	if err != nil {
		return nil, err
	}
	results := &rpcpb.ListContractStorageResponse{
		BlockHash:   common.Base58Encode(bcn.HeadHash()),
		BlockNumber: bcn.Head.Number,
	}
	for _, k := range keys {
		value, err := mvcc.Get(database.StateTable, k)
		if err != nil {
			return nil, err
		}
		var res interface{}
		if (req.StorageType == rpcpb.ListContractStorageRequest_MAP) && strings.HasPrefix(value, database.MapKeysSeparator) {
			// map keys
			res = strings.Split(value, database.MapKeysSeparator)[1:]
		} else {
			res = database.Unmarshal(value)
		}
		value, err = formatInternalValue(res)
		if err != nil {
			return nil, err
		}
		results.Datas = append(results.Datas, &rpcpb.ListContractStorageResponse_Data{
			Key:   strings.TrimPrefix(k, prefix),
			Value: value,
		})
	}
	return results, nil
}

func (as *APIService) tryTransaction(t *tx.Tx) (*tx.TxReceipt, error) {
	topBlock := as.bc.Head()
	blkHead := &block.BlockHead{
		Version:    block.V1,
		ParentHash: topBlock.HeadHash(),
		Number:     topBlock.Head.Number + 1,
		Time:       time.Now().UnixNano(),
	}
	v := verifier.Executor{}
	stateDB := as.stateDB.Fork()
	ok := stateDB.Checkout(string(topBlock.HeadHash()))
	if !ok {
		return nil, fmt.Errorf("failed to checkout blockhash: %s", common.Base58Encode(topBlock.HeadHash()))
	}
	return v.Try(blkHead, stateDB, t, cverifier.TxExecTimeLimit)
}

// SendTransaction sends a transaction to iserver.
func (as *APIService) SendTransaction(ctx context.Context, req *rpcpb.TransactionRequest) (*rpcpb.SendTransactionResponse, error) {
	t := toCoreTx(req)
	err := tx.CheckBadTx(t)
	if err != nil {
		return nil, err
	}
	ret := &rpcpb.SendTransactionResponse{
		Hash: common.Base58Encode(t.Hash()),
	}
	if as.config.RPC.TryTx {
		tr, err := as.tryTransaction(t)
		if err != nil {
			return nil, fmt.Errorf("try transaction failed: %v", err)
		}
		ret.PreTxReceipt = toPbTxReceipt(tr)
	}
	headBlock := as.bc.Head()
	dbVisitor, err := as.getStateDBVisitorByBlock(headBlock)
	if err != nil {
		ilog.Errorf("[internal error] SendTransaction error: %v", err)
		return nil, err
	}
	currentGas := dbVisitor.TotalGasAtTime(t.Publisher, headBlock.Head.Time)
	err = vm.CheckTxGasLimitValid(t, currentGas, dbVisitor)
	if err != nil {
		return nil, err
	}
	err = as.txpool.AddTx(t, "rpc")
	if err != nil {
		return nil, err
	}
	if t.Time < time.Now().UnixNano() {
		as.p2pService.Broadcast(t.Encode(), p2p.PublishTx, p2p.NormalMessage)
	} else {
		waitTime := time.Until(time.Unix(0, t.Time))
		time.AfterFunc(waitTime, func() {
			as.p2pService.Broadcast(t.Encode(), p2p.PublishTx, p2p.NormalMessage)
		})
	}
	return ret, nil
}

// ExecTransaction executes a transaction by the node and returns the receipt.
func (as *APIService) ExecTransaction(ctx context.Context, req *rpcpb.TransactionRequest) (*rpcpb.TxReceipt, error) {
	if !as.config.RPC.ExecTx {
		return nil, errors.New("The node has't enabled this method")
	}
	t := toCoreTx(req)
	receipt, err := as.tryTransaction(t)
	if err != nil {
		return nil, err
	}
	return toPbTxReceipt(receipt), nil
}

// Subscribe used for event.
func (as *APIService) Subscribe(req *rpcpb.SubscribeRequest, res rpcpb.ApiService_SubscribeServer) error {
	topics := make([]event.Topic, 0)
	for _, t := range req.Topics {
		topics = append(topics, event.Topic(t))
	}
	var filter *event.Meta
	if req.GetFilter() != nil {
		filter = &event.Meta{
			ContractID: req.GetFilter().GetContractId(),
		}
	}

	ec := event.GetCollector()
	id := time.Now().UnixNano()
	ch := ec.Subscribe(id, topics, filter)
	defer ec.Unsubscribe(id, topics)

	timeup := time.NewTimer(time.Hour)
	for {
		select {
		case <-timeup.C:
			return nil
		case <-as.quitCh:
			return nil
		case <-res.Context().Done():
			return res.Context().Err()
		case ev := <-ch:
			e := &rpcpb.Event{
				Topic: rpcpb.Event_Topic(ev.Topic),
				Data:  ev.Data,
				Time:  ev.Time,
			}
			err := res.Send(&rpcpb.SubscribeResponse{Event: e})
			if err != nil {
				ilog.Errorf("stream send failed. err=%v", err)
				return err
			}
		}
	}
}

// GetVoterBonus returns the bonus a voter can claim.
func (as *APIService) GetVoterBonus(ctx context.Context, req *rpcpb.GetAccountRequest) (*rpcpb.VoterBonus, error) {
	err := checkIDValid(req.GetName())
	if err != nil {
		return nil, err
	}
	ret := &rpcpb.VoterBonus{
		Detail: make(map[string]float64),
	}
	dbVisitor, bcn, err := as.getStateDBVisitor(req.ByLongestChain)
	if err != nil {
		return nil, err
	}
	h := host.NewHost(host.NewContext(nil), dbVisitor, bcn.Head.Rules(), nil, nil)

	voter := req.GetName()
	value, _ := h.GlobalMapGet("vote.iost", "u_1", voter)
	if value == nil {
		return ret, nil
	}
	var userVotes map[string][]interface{}
	err = json.Unmarshal([]byte(value.(string)), &userVotes)
	if err != nil {
		ilog.Errorf("JSON decoding failed. str=%v, err=%v", value, err)
		return nil, err
	}

	for k, v := range userVotes {
		votes, err := strconv.ParseFloat(v[0].(string), 64)
		if err != nil {
			ilog.Errorf("Parsing str %v to float64 failed. err=%v", v[0], err)
			continue
		}
		voterCoef := float64(0)
		value, _ := h.GlobalMapGet("vote_producer.iost", "voterCoef", k)
		if value != nil {
			vc := value.(string)
			if len(vc) > 1 {
				vc = vc[1 : len(vc)-1]
			}
			voterCoef, err = strconv.ParseFloat(vc, 64)
			if err != nil {
				ilog.Errorf("Parsing str %v to float64 failed. err=%v", vc, err)
				return nil, err
			}
		}
		voterMask := float64(0)
		value, _ = h.GlobalMapGet("vote_producer.iost", "v_"+k, voter)
		if value != nil {
			vm := value.(string)
			if len(vm) > 1 {
				vm = vm[1 : len(vm)-1]
			}
			voterMask, err = strconv.ParseFloat(vm, 64)
			if err != nil {
				ilog.Errorf("Parsing str %v to float64 failed. err=%v", vm, err)
				return nil, err
			}
		}
		earning := voterCoef*votes - voterMask
		earning = math.Trunc(earning*1e8) / 1e8
		if earning > 0 {
			ret.Detail[k] = earning
			ret.Bonus += earning
		}
	}
	return ret, nil
}

// GetCandidateBonus returns the bonus a candidate can claim.
func (as *APIService) GetCandidateBonus(ctx context.Context, req *rpcpb.GetAccountRequest) (*rpcpb.CandidateBonus, error) {
	err := checkIDValid(req.GetName())
	if err != nil {
		return nil, err
	}
	ret := &rpcpb.CandidateBonus{}
	dbVisitor, bcn, err := as.getStateDBVisitor(req.ByLongestChain)
	if err != nil {
		return nil, err
	}
	h := host.NewHost(host.NewContext(nil), dbVisitor, bcn.Head.Rules(), nil, nil)

	candidate := req.GetName()
	candCoef := float64(0)
	value, _ := h.GlobalGet("vote_producer.iost", "candCoef")
	if value != nil {
		cc := value.(string)
		if len(cc) > 1 {
			cc = cc[1 : len(cc)-1]
		}
		candCoef, err = strconv.ParseFloat(cc, 64)
		if err != nil {
			ilog.Errorf("Parsing str %v to float64 failed. err=%v", cc, err)
			return nil, err
		}
	}
	candMask := float64(0)
	value, _ = h.GlobalMapGet("vote_producer.iost", "candMask", candidate)
	if value != nil {
		cm := value.(string)
		if len(cm) > 1 {
			cm = cm[1 : len(cm)-1]
		}
		candMask, err = strconv.ParseFloat(cm, 64)
		if err != nil {
			ilog.Errorf("Parsing str %v to float64 failed. err=%v", cm, err)
			return nil, err
		}
	}
	value, _ = h.GlobalMapGet("vote.iost", "v_1", candidate)
	if value == nil {
		return ret, nil
	}
	v := value.(string)
	j, err := simplejson.NewJson([]byte(v))
	if err != nil {
		ilog.Errorf("JSON decoding %v failed. err=%v", v, err)
		return nil, err
	}
	v, err = j.Get("votes").String()
	if err != nil {
		ilog.Errorf("Getting votes from json failed. err=%v", err)
		return nil, err
	}
	votes, err := strconv.ParseFloat(v, 64)
	if err != nil {
		ilog.Errorf("Parsing str %v to float64 failed. err=%v", v, err)
		return nil, err
	}
	if votes < 2100000 {
		votes = 0
	}
	ret.Bonus = candCoef*votes - candMask
	ret.Bonus = math.Trunc(ret.Bonus*1e8) / 1e8
	return ret, nil
}

// GetTokenInfo returns the information of a given token.
func (as *APIService) GetTokenInfo(ctx context.Context, req *rpcpb.GetTokenInfoRequest) (*rpcpb.TokenInfo, error) {
	var token404 = errors.New("token not found")
	dbVisitor, bcn, err := as.getStateDBVisitor(req.ByLongestChain)
	if err != nil {
		return nil, err
	}
	h := host.NewHost(host.NewContext(nil), dbVisitor, bcn.Head.Rules(), nil, nil)

	symbol := req.GetSymbol()
	ret := &rpcpb.TokenInfo{Symbol: symbol}

	value, _ := h.GlobalMapGet("token.iost", "TI"+symbol, "fullName")
	if value == nil {
		return nil, token404
	}
	ret.FullName = value.(string)

	value, _ = h.GlobalMapGet("token.iost", "TI"+symbol, "issuer")
	if value == nil {
		return nil, token404
	}
	ret.Issuer = value.(string)

	value, _ = h.GlobalMapGet("token.iost", "TI"+symbol, "supply")
	if value == nil {
		return nil, token404
	}
	ret.CurrentSupply = value.(int64)

	value, _ = h.GlobalMapGet("token.iost", "TI"+symbol, "totalSupply")
	if value == nil {
		return nil, token404
	}
	ret.TotalSupply = value.(int64)

	value, _ = h.GlobalMapGet("token.iost", "TI"+symbol, "decimal")
	if value == nil {
		return nil, token404
	}
	ret.Decimal = int32(value.(int64))

	ret.TotalSupplyFloat = float64(ret.TotalSupply) / math.Pow10(int(ret.Decimal))
	ret.CurrentSupplyFloat = float64(ret.CurrentSupply) / math.Pow10(int(ret.Decimal))

	value, _ = h.GlobalMapGet("token.iost", "TI"+symbol, "canTransfer")
	if value == nil {
		return nil, token404
	}
	ret.CanTransfer = value.(bool)

	value, _ = h.GlobalMapGet("token.iost", "TI"+symbol, "onlyIssuerCanTransfer")
	if value == nil {
		return nil, token404
	}
	ret.OnlyIssuerCanTransfer = value.(bool)

	return ret, nil
}

func (as *APIService) getStateDBByBlock(bcn *blockcache.BlockCacheNode) (db db.MVCCDB, err error) {
	db = as.stateDB.Fork()
	ok := db.Checkout(string(bcn.HeadHash()))
	if !ok {
		b2s := func(x *blockcache.BlockCacheNode) string {
			return fmt.Sprintf("b58 hash %v time %v height %v witness %v", common.Base58Encode(x.HeadHash()), x.Head.Time,
				x.Head.Number, x.Head.Witness)
		}
		err = fmt.Errorf("db checkout failed. b58 hash %v, head block %v, li block %v", common.Base58Encode(bcn.HeadHash()),
			b2s(as.bc.Head()), b2s(as.bc.LinkedRoot()))
	}
	return
}

func (as *APIService) getStateDBVisitorByBlock(bcn *blockcache.BlockCacheNode) (dbVisitor *database.Visitor, err error) {
	stateDB, err := as.getStateDBByBlock(bcn)
	dbVisitor = database.NewVisitor(0, stateDB, bcn.Head.Rules())
	return
}

func (as *APIService) getStateDBVisitor(longestChain bool) (*database.Visitor, *blockcache.BlockCacheNode, error) {
	var err error
	var db *database.Visitor
	// retry 3 times as block may be flushed
	for i := 0; i < 3; i++ {
		var b *blockcache.BlockCacheNode
		if longestChain {
			b = as.bc.Head()
		} else {
			b = as.bc.LinkedRoot()
		}
		db, err = as.getStateDBVisitorByBlock(b)
		if err != nil {
			ilog.Errorf("getStateDBVisitor err: %v", err)
			continue
		}
		return db, b, nil
	}
	return nil, nil, err
}

func (as *APIService) getUnfrozenToken(frozens []database.FreezeItemFixed, longestChain bool) (float64, []*rpcpb.FrozenBalance) {
	var blockTime int64
	if longestChain {
		blockTime = as.bc.Head().Head.Time
	} else {
		blockTime = as.bc.LinkedRoot().Head.Time
	}
	var unfrozen float64
	var stillFrozen []*rpcpb.FrozenBalance
	for _, f := range frozens {
		if f.Ftime <= blockTime {
			unfrozen += f.Amount.Float64()
		} else {
			stillFrozen = append(stillFrozen, &rpcpb.FrozenBalance{
				Amount: f.Amount.Float64(),
				Time:   f.Ftime,
			})
		}
	}
	return unfrozen, stillFrozen
}

func formatInternalValue(value interface{}) (data string, err error) {
	if value != nil && reflect.TypeOf(value).Kind() == reflect.String {
		data = value.(string)
	} else {
		var bytes []byte
		bytes, err = json.Marshal(value)
		if err != nil {
			return "", fmt.Errorf("cannot unmarshal %v", value)
		}
		data = string(bytes)
	}
	return
}
