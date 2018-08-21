package pob

import (
	"github.com/iost-official/Go-IOS-Protocol/account"

	"encoding/binary"
	"errors"
	"time"

	"github.com/iost-official/Go-IOS-Protocol/common"
	"github.com/iost-official/Go-IOS-Protocol/consensus/common"
	"github.com/iost-official/Go-IOS-Protocol/core/new_block"
	"github.com/iost-official/Go-IOS-Protocol/core/new_blockcache"
	"github.com/iost-official/Go-IOS-Protocol/core/new_tx"
	"github.com/iost-official/Go-IOS-Protocol/core/new_txpool"
	"github.com/iost-official/Go-IOS-Protocol/db"
	"github.com/iost-official/Go-IOS-Protocol/new_vm"
)

var (
	ErrWitness     = errors.New("wrong witness")
	ErrPubkey      = errors.New("wrong pubkey")
	ErrSignature   = errors.New("wrong signature")
	ErrSlot        = errors.New("witness slot duplicate")
	ErrTxTooOld    = errors.New("tx too old")
	ErrTxDup       = errors.New("duplicate tx")
	ErrTxSignature = errors.New("tx wrong signature")
)

func generateBlock(account account.Account, topBlock *block.Block, txPool txpool.TxPool, db db.MVCCDB) (*block.Block, error) {
	var err error
	blk := block.Block{
		Head: block.BlockHead{
			Version:    0,
			ParentHash: topBlock.HeadHash(),
			Number:     topBlock.Head.Number + 1,
			Witness:    account.ID,
			Time:       time.Now().Unix() / common.SlotLength,
		},
		Txs:      []*tx.Tx{},
		Receipts: []*tx.TxReceipt{},
	}
	txCnt := 1000
	limitTime := time.NewTicker((common.SlotLength / 3 * time.Second))
	txsList, _ := txPool.PendingTxs(txCnt)
	db.Checkout(string(topBlock.HeadHash()))
	engine := new_vm.NewEngine(&topBlock.Head, db)
	for _, t := range txsList {
		select {
		case <-limitTime.C:
			break
		default:
			if receipt, err := engine.Exec(t); err == nil {
				blk.Txs = append(blk.Txs, t)
				blk.Receipts = append(blk.Receipts, receipt)
			}
		}
	}
	db.Tag(string(blk.HeadHash()))

	blk.Head.TxsHash = blk.CalculateTxsHash()
	blk.Head.MerkleHash = blk.CalculateMerkleHash()
	headInfo := generateHeadInfo(blk.Head)
	sig := common.Sign(common.Secp256k1, headInfo, account.Seckey)
	blk.Head.Signature, err = sig.Encode()
	if err != nil {
		return nil, err
	}
	err = blk.CalculateHeadHash()
	if err != nil {
		return nil, err
	}

	generatedBlockCount.Inc()
	txPoolSize.Set(float64(len(blk.Txs)))
	return &blk, nil
}

func generateHeadInfo(head block.BlockHead) []byte {
	var info, numberInfo, versionInfo []byte
	info = make([]byte, 8)
	versionInfo = make([]byte, 4)
	numberInfo = make([]byte, 4)
	binary.BigEndian.PutUint64(info, uint64(head.Time))
	binary.BigEndian.PutUint32(versionInfo, uint32(head.Version))
	binary.BigEndian.PutUint32(numberInfo, uint32(head.Number))
	info = append(info, versionInfo...)
	info = append(info, numberInfo...)
	info = append(info, head.ParentHash...)
	info = append(info, head.TxsHash...)
	info = append(info, head.MerkleHash...)
	info = append(info, head.Info...)
	return common.Sha256(info)
}

func verifyBasics(blk *block.Block) error {
	if witnessOfSlot(blk.Head.Time) != blk.Head.Witness {
		return ErrWitness
	}
	var signature common.Signature
	signature.Decode(blk.Head.Signature)
	if blk.Head.Witness != account.GetIDByPubkey(signature.Pubkey) {
		return ErrPubkey
	}
	headInfo := generateHeadInfo(blk.Head)
	if !common.VerifySignature(headInfo, signature) {
		return ErrSignature
	}
	if staticProperty.hasSlot(blk.Head.Time) {
		return ErrSlot
	}
	return nil
}

func verifyBlock(blk *block.Block, parent *block.Block, lib *block.Block, txPool txpool.TxPool, db db.MVCCDB) error {
	err := consensus_common.VerifyBlockHead(blk, parent, lib)
	if err != nil {
		return err
	}
	for _, tx := range blk.Txs {
		exist, _ := txPool.ExistTxs(tx.Hash(), parent)
		if exist == txpool.FoundChain {
			return ErrTxDup
		} else if exist != txpool.FoundPending {
			if err := tx.VerifySelf(); err != nil {
				return ErrTxSignature
			}
		}
		if blk.Head.Time*common.SlotLength-tx.Time/1e9 > 60 {
			return ErrTxTooOld
		}
	}
	return consensus_common.VerifyBlockWithVM(blk, db)
}

func updateStaticProperty(node *blockcache.BlockCacheNode) {
	staticProperty.addSlot(node.Block.Head.Time)
	node.ConfirmUntil = staticProperty.Watermark[node.Witness]
	if node.Number >= staticProperty.Watermark[node.Witness] {
		staticProperty.Watermark[node.Witness] = node.Number + 1
	}
}

func updatePendingWitness(node *blockcache.BlockCacheNode, db db.MVCCDB) {
	// TODO how to decode witness list from db?
	//newList, err := db.Get("state", "witnessList"), "id1"
	var err error
	if err == nil {
		//node.PendingWitnessList = newList
	} else {
		node.PendingWitnessList = node.Parent.PendingWitnessList
	}
}

func updateLib(node *blockcache.BlockCacheNode, bc blockcache.BlockCache) {
	confirmedNode := calculateConfirm(node, bc.LinkedRoot())
	if confirmedNode != nil {
		bc.Flush(confirmedNode)
		staticProperty.updateWitnessList(confirmedNode.PendingWitnessList)
	}
}

func calculateConfirm(node *blockcache.BlockCacheNode, root *blockcache.BlockCacheNode) *blockcache.BlockCacheNode {
	confirmLimit := staticProperty.NumberOfWitnesses*2/3 + 1
	startNumber := node.Number
	var confirmNum int64 = 0
	confirmUntilMap := make(map[int64]int64, startNumber-root.Number)
	var index int64 = 0
	for node != root {
		if node.ConfirmUntil <= node.Number {
			confirmNum++
			confirmUntilMap[startNumber-node.ConfirmUntil]++
		}
		if confirmNum >= confirmLimit {
			return node
		}
		confirmNum -= confirmUntilMap[index]
		node = node.Parent
		index++
	}
	return nil
}
