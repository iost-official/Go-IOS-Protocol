package txpool

import (
	"github.com/iost-official/go-iost/core/block"
	"github.com/iost-official/go-iost/core/blockcache"
	"github.com/iost-official/go-iost/core/tx"
)

//go:generate mockgen -destination mock/mock_txpool.go -package txpool_mock github.com/iost-official/go-iost/core/txpool TxPool

// TxPool defines all the API of txpool package.
type TxPool interface {
	Start() error
	Stop()
	AddLinkedNode(linkedNode *blockcache.BlockCacheNode, headNode *blockcache.BlockCacheNode) error
	AddTx(tx *tx.Tx) TAddTx
	DelTx(hash []byte) error
	DelTxList(delList []*tx.Tx)
	TxIterator() (*Iterator, *blockcache.BlockCacheNode)
	ExistTxs(hash []byte, chainBlock *block.Block) FRet
	Lock()
	Release()
	TxTimeOut(tx *tx.Tx) bool
}
