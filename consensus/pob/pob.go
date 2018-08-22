package pob

import (
	"errors"
	"fmt"
	"time"

	"math"

	"github.com/iost-official/Go-IOS-Protocol/account"
	"github.com/iost-official/Go-IOS-Protocol/common"
	"github.com/iost-official/Go-IOS-Protocol/consensus/synchronizer"
	"github.com/iost-official/Go-IOS-Protocol/core/global"
	"github.com/iost-official/Go-IOS-Protocol/core/new_block"
	"github.com/iost-official/Go-IOS-Protocol/core/new_blockcache"
	"github.com/iost-official/Go-IOS-Protocol/core/new_txpool"
	"github.com/iost-official/Go-IOS-Protocol/db"
	"github.com/iost-official/Go-IOS-Protocol/ilog"
	"github.com/iost-official/Go-IOS-Protocol/p2p"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	generatedBlockCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "generated_block_count",
			Help: "Count of generated block by current node",
		},
	)
	receivedBlockCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "received_block_count",
			Help: "Count of received block by current node",
		},
	)
	confirmedBlockchainLength = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "confirmed_blockchain_length",
			Help: "Length of confirmed blockchain on current node",
		},
	)
	txPoolSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "tx_poo_size",
			Help: "size of tx pool on current node",
		},
	)
)

func init() {
	prometheus.MustRegister(generatedBlockCount)
	prometheus.MustRegister(receivedBlockCount)
	prometheus.MustRegister(confirmedBlockchainLength)
	prometheus.MustRegister(txPoolSize)
}

//PoB is a struct that handles the consensus logic.
type PoB struct {
	account      account.Account
	baseVariable global.BaseVariable
	blockChain   block.Chain
	blockCache   blockcache.BlockCache
	txPool       txpool.TxPool
	p2pService   p2p.Service
	synchronizer synchronizer.Synchronizer
	verifyDB     db.MVCCDB
	produceDB    db.MVCCDB
	exitSignal   chan struct{}
	chRecvBlock  chan p2p.IncomingMessage
	chGenBlock   chan *block.Block
}

// NewPoB init a new PoB.
func NewPoB(account account.Account, baseVariable global.BaseVariable, blockCache blockcache.BlockCache, txPool txpool.TxPool, p2pService p2p.Service, synchronizer synchronizer.Synchronizer, witnessList []string) *PoB {
	p := PoB{
		account:      account,
		baseVariable: baseVariable,
		blockChain:   baseVariable.BlockChain(),
		blockCache:   blockCache,
		txPool:       txPool,
		p2pService:   p2pService,
		synchronizer: synchronizer,
		verifyDB:     baseVariable.StateDB(),
		produceDB:    baseVariable.StateDB().Fork(),
		exitSignal:   make(chan struct{}),
		chRecvBlock:  p2pService.Register("consensus channel", p2p.NewBlock, p2p.SyncBlockResponse),
		chGenBlock:   make(chan *block.Block, 10),
	}
	staticProperty = newStaticProperty(p.account, witnessList)
	return &p
}

//Run make the PoB run.
func (p *PoB) Run() {
	p.synchronizer.Start()
	go p.blockLoop()
	go p.scheduleLoop()
}

//Stop make the PoB stop.
func (p *PoB) Stop() {
	close(p.exitSignal)
	close(p.chRecvBlock)
	close(p.chGenBlock)
}

func (p *PoB) blockLoop() {
	ilog.Info("start block")
	for {
		select {
		case incomingMessage, ok := <-p.chRecvBlock:
			if !ok {
				ilog.Info("chRecvBlock has closed")
				return
			}
			var blk block.Block
			err := blk.Decode(incomingMessage.Data())
			if err != nil {
				ilog.Debug(err.Error())
				continue
			}
			err = p.handleRecvBlock(&blk)
			if err != nil {
				ilog.Debug(err.Error())
				continue
			}
			if incomingMessage.Type() == p2p.SyncBlockResponse {
				go p.synchronizer.OnBlockConfirmed(string(blk.HeadHash()), incomingMessage.From())
			}
			if incomingMessage.Type() == p2p.NewBlock {
				go p.p2pService.Broadcast(incomingMessage.Data(), incomingMessage.Type(), p2p.UrgentMessage)
				if ok, start, end := p.synchronizer.NeedSync(blk.Head.Number); ok {
					go p.synchronizer.SyncBlocks(start, end)
				}
			}
		case blk, ok := <-p.chGenBlock:
			if !ok {
				ilog.Info("chGenBlock has closed")
				return
			}
			err := p.handleRecvBlock(blk)
			if err != nil {
				ilog.Debug(err.Error())
			}
		case <-p.exitSignal:
			return
		}
	}
}

func (p *PoB) scheduleLoop() {
	nextSchedule := timeUntilNextSchedule(time.Now().UnixNano())
	ilog.Info("next schedule:%v", math.Round(float64(nextSchedule)/float64(second2nanosecond)))
	for {
		select {
		case <-time.After(time.Duration(nextSchedule)):
			if witnessOfSec(time.Now().Unix()) == p.account.ID {
				blk, err := generateBlock(p.account, p.blockCache.Head().Block, p.txPool, p.produceDB)
				ilog.Info("gen block:%v", blk.Head.Number)
				if err != nil {
					ilog.Debug(err.Error())
					continue
				}
				blkByte, err := blk.Encode()
				if err != nil {
					ilog.Debug(err.Error())
					continue
				}
				p.chGenBlock <- blk
				go p.p2pService.Broadcast(blkByte, p2p.NewBlock, p2p.UrgentMessage)
				time.Sleep(common.SlotLength * time.Second)
			}
			nextSchedule = timeUntilNextSchedule(time.Now().UnixNano())
			ilog.Info("next schedule:%v", math.Round(float64(nextSchedule)/float64(second2nanosecond)))
		case <-p.exitSignal:
			return
		}
	}
}

func (p *PoB) handleRecvBlock(blk *block.Block) error {
	ilog.Info("block number:%v", blk.Head.Number)
	_, err := p.blockCache.Find(blk.HeadHash())
	if err == nil {
		return errors.New("duplicate block")
	}
	err = verifyBasics(blk)
	if err != nil {
		return fmt.Errorf("fail to verify blocks, %v", err)
	}
	parent, err := p.blockCache.Find(blk.Head.ParentHash)
	p.blockCache.Add(blk)
	if err == nil && parent.Type == blockcache.Linked {
		return p.addExistingBlock(blk, parent.Block)
	}
	staticProperty.addSlot(blk.Head.Time)
	return nil
}

func (p *PoB) addExistingBlock(blk *block.Block, parentBlock *block.Block) error {
	node, _ := p.blockCache.Find(blk.HeadHash())
	if blk.Head.Witness != p.account.ID {
		p.verifyDB.Checkout(string(blk.Head.ParentHash))
		err := verifyBlock(blk, parentBlock, p.blockCache.LinkedRoot().Block, p.txPool, p.verifyDB)
		if err != nil {
			p.blockCache.Del(node)
			ilog.Debug(err.Error())
			return err
		}
		p.verifyDB.Tag(string(blk.HeadHash()))
	} else {
		p.verifyDB.Checkout(string(blk.HeadHash()))
	}
	p.blockCache.Link(node)
	p.updateInfo(node)
	for child := range node.Children {
		p.addExistingBlock(child.Block, node.Block)
	}
	return nil
}

func (p *PoB) updateInfo(node *blockcache.BlockCacheNode) {
	updateWaterMark(node)
	updateLib(node, p.blockCache)
	p.txPool.AddLinkedNode(node, p.blockCache.Head())
}
