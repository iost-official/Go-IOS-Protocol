// Package block 是区块和区块链的结构体定义和操作方法
package block

import "github.com/iost-official/go-iost/core/tx"

//go:generate mockgen -destination ../mocks/mock_blockchain.go -package core_mock github.com/iost-official/go-iost/core/block Chain

// Chain defines Chain's API.
type Chain interface {
	Push(block *Block) error
	Length() int64
	CheckLength()
	Top() (*Block, error)
	GetHashByNumber(number int64) ([]byte, error)
	GetBlockByNumber(number int64) (*Block, error)
	GetBlockByHash(blockHash []byte) (*Block, error)
	GetBlockByteByHash(blockHash []byte) ([]byte, error)
	GetTx(hash []byte) (*tx.Tx, error)
	HasTx(hash []byte) (bool, error)
	GetReceipt(Hash []byte) (*tx.TxReceipt, error)
	GetReceiptByTxHash(Hash []byte) (*tx.TxReceipt, error)
	HasReceipt(hash []byte) (bool, error)
	Close()
	Draw(int64, int64) string
}
