package blockcache

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/iost-official/go-iost/common/config"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/iost-official/go-iost/common"
	"github.com/iost-official/go-iost/core/block"
	"github.com/iost-official/go-iost/db"
	"github.com/iost-official/go-iost/db/wal"
	"github.com/iost-official/go-iost/ilog"
)

//go:generate mockgen -destination mock/mock_blockcache.go -package mock github.com/iost-official/go-iost/core/blockcache BlockCache

// CacheStatus ...
type CacheStatus int

// ConAlgo ...
type ConAlgo interface {
	Add(*block.Block, bool, bool) error
}

const (
	// DelSingleBlockTime ...
	DelSingleBlockTime int64 = 10
)

// BCNType type of BlockCacheNode
type BCNType int

// The types of BlockCacheNode
const (
	Linked BCNType = iota
	Single
)

// The directory of block cache wal.
var (
	BlockCacheWALDir = "./BlockCacheWAL"
)

// BlockCacheNode is the implementation of BlockCacheNode
type BlockCacheNode struct { //nolint:golint
	*block.Block
	WitnessList
	rw           sync.RWMutex
	parent       *BlockCacheNode
	Children     map[*BlockCacheNode]bool
	Type         BCNType
	walIndex     uint64
	ValidWitness []string
	SerialNum    int64
}

// GetParent returns the node's parent node.
func (bcn *BlockCacheNode) GetParent() *BlockCacheNode {
	bcn.rw.RLock()
	defer bcn.rw.RUnlock()
	return bcn.parent
}

// SetParent sets the node's parent.
func (bcn *BlockCacheNode) SetParent(p *BlockCacheNode) {
	bcn.rw.Lock()
	bcn.parent = p
	bcn.rw.Unlock()
}
func (bcn *BlockCacheNode) addChild(child *BlockCacheNode) {
	if child != nil {
		bcn.Children[child] = true
	}
}

func encodeUpdateLinkedRootWitness(bc *BlockCacheImpl) (b []byte, err error) {
	uwRaw := &UpdateLinkedRootWitnessRaw{
		BlockHashBytes:    bc.LinkedRoot().HeadHash(),
		LinkedRootWitness: bc.linkedRootWitness,
	}
	b, err = proto.Marshal(uwRaw)
	return
}

func decodeUpdateLinkedRootWitness(b []byte) (blockHeadHash []byte, wt []string, err error) {
	var uwRaw UpdateLinkedRootWitnessRaw
	err = proto.Unmarshal(b, &uwRaw)
	if err != nil {
		return
	}
	blockHeadHash = uwRaw.BlockHashBytes
	wt = uwRaw.LinkedRootWitness
	return
}

func encodeUpdateActive(bcn *BlockCacheNode) (b []byte, err error) {
	// First add block
	uaRaw := &UpdateActiveRaw{
		BlockHashBytes: bcn.HeadHash(),
		WitnessList:    &bcn.WitnessList,
	}
	b, err = proto.Marshal(uaRaw)
	return
}

func decodeUpdateActive(b []byte) (blockHeadHash []byte, wt WitnessList, err error) {
	var uaRaw UpdateActiveRaw
	err = proto.Unmarshal(b, &uaRaw)
	if err != nil {
		return
	}
	blockHeadHash = uaRaw.BlockHashBytes
	wt = *(uaRaw.WitnessList)
	return
}

func encodeBCN(bcn *BlockCacheNode) (b []byte, err error) {
	// First add block
	blockByte, err := bcn.Block.Encode()
	if err != nil {
		return
	}
	bcRaw := &BlockCacheRaw{
		BlockBytes:  blockByte,
		WitnessList: &bcn.WitnessList,
		SerialNum:   bcn.SerialNum,
	}
	b, err = proto.Marshal(bcRaw)
	return
}

func decodeBCN(b []byte) (block block.Block, wt WitnessList, serialNum int64, err error) {
	var bcRaw BlockCacheRaw
	err = proto.Unmarshal(b, &bcRaw)
	if err != nil {
		return
	}
	err = block.Decode(bcRaw.BlockBytes)
	if err != nil {
		return
	}
	wt = *(bcRaw.WitnessList)
	serialNum = bcRaw.SerialNum
	return
}

// NewBCN return a new block cache node instance
func NewBCN(parent *BlockCacheNode, blk *block.Block) *BlockCacheNode {
	bcn := &BlockCacheNode{
		Block:        blk,
		Type:         Single,
		parent:       parent,
		Children:     make(map[*BlockCacheNode]bool),
		ValidWitness: make([]string, 0),
		WitnessList: WitnessList{
			WitnessInfo: make([]string, 0),
		},
	}
	// TODO: Move this outside.
	if parent != nil {
		parent.addChild(bcn)
	}
	return bcn
}

func (bcn *BlockCacheNode) updateValidWitness() {
	witness := bcn.Head.Witness
	parent := bcn.GetParent()
	for _, w := range parent.ValidWitness {
		bcn.ValidWitness = append(bcn.ValidWitness, w)
		if w == witness {
			witness = ""
		}
	}
	if witness != "" {
		bcn.ValidWitness = append(bcn.ValidWitness, witness)
	}
}

func (bcn *BlockCacheNode) removeValidWitness(root *BlockCacheNode) {
	if bcn != root && bcn.Head.Witness == root.Head.Witness {
		return
	}
	newValidWitness := make([]string, 0, len(bcn.ValidWitness))
	for _, w := range bcn.ValidWitness {
		if w != root.Head.Witness {
			newValidWitness = append(newValidWitness, w)
		}
	}
	bcn.ValidWitness = newValidWitness
	for child := range bcn.Children {
		child.removeValidWitness(root)
	}
}

// BlockCache defines BlockCache's API
type BlockCache interface {
	Add(*block.Block) *BlockCacheNode
	AddGenesis(*block.Block)
	Link(*BlockCacheNode)
	UpdateLib(*BlockCacheNode)
	Del(*BlockCacheNode)
	GetBlockByNumber(int64) (*block.Block, error)
	GetBlockByHash([]byte) (*block.Block, error)
	LinkedRoot() *BlockCacheNode
	Head() *BlockCacheNode
	Draw() string
	Recover(p ConAlgo) (err error)
	AddNodeToWAL(bcn *BlockCacheNode)
}

// BlockCacheImpl is the implementation of BlockCache
type BlockCacheImpl struct { //nolint:golint
	linkRW            sync.RWMutex
	linkedRoot        *BlockCacheNode
	singleRoot        map[string]*BlockCacheNode
	linkedRootWitness []string
	headRW            sync.RWMutex
	head              *BlockCacheNode
	hash2node         *sync.Map // map[string]*BlockCacheNode
	numberMutex       sync.Mutex
	number2node       *sync.Map // map[int64]*BlockCacheNode
	leaf              map[*BlockCacheNode]int64
	witnessNum        int64
	blockChain        block.Chain
	stateDB           db.MVCCDB
	wal               *wal.WAL
}

func (bc *BlockCacheImpl) hmget(hash []byte) (*BlockCacheNode, bool) {
	rtnI, ok := bc.hash2node.Load(string(hash))
	if !ok {
		return nil, false
	}
	bcn, okn := rtnI.(*BlockCacheNode)
	if !okn {
		bc.hash2node.Delete(string(hash))
		return nil, false
	}
	return bcn, true
}

func (bc *BlockCacheImpl) hmset(hash []byte, bcn *BlockCacheNode) {
	bc.hash2node.Store(string(hash), bcn)
}

func (bc *BlockCacheImpl) hmdel(hash []byte) {
	bc.hash2node.Delete(string(hash))
}

func (bc *BlockCacheImpl) nmget(num int64) (*BlockCacheNode, bool) {
	rtnI, ok := bc.number2node.Load(num)
	if !ok {
		return nil, false
	}
	bcn, okn := rtnI.(*BlockCacheNode)
	if !okn {
		bc.number2node.Delete(num)
		return nil, false
	}
	return bcn, true
}

func (bc *BlockCacheImpl) nmset(num int64, bcn *BlockCacheNode) {
	bc.number2node.Store(num, bcn)
}

func (bc *BlockCacheImpl) nmdel(num int64) {
	bc.number2node.Delete(num)
}

// NewBlockCache return a new BlockCache instance
func NewBlockCache(conf *config.Config, bChain block.Chain, stateDB db.MVCCDB) (*BlockCacheImpl, error) {
	w, err := wal.Create(conf.DB.LdbPath+BlockCacheWALDir, []byte("block_cache_wal"))
	if err != nil {
		return nil, err
	}
	bc := BlockCacheImpl{
		linkedRoot:        nil,
		singleRoot:        make(map[string]*BlockCacheNode),
		linkedRootWitness: make([]string, 0),
		hash2node:         new(sync.Map),
		number2node:       new(sync.Map),
		leaf:              make(map[*BlockCacheNode]int64),
		blockChain:        bChain,
		stateDB:           stateDB.Fork(),
		wal:               w,
	}

	lib, err := bc.blockChain.Top()
	if err != nil {
		ilog.Errorf("lib not found. err:%v", err)
	}

	ilog.Info("Got LIB: ", lib.Head.Number)
	bc.linkedRoot = NewBCN(nil, lib)
	bc.linkedRoot.Type = Linked
	bc.hmset(bc.linkedRoot.HeadHash(), bc.linkedRoot)
	bc.leaf[bc.linkedRoot] = bc.linkedRoot.Head.Number

	if err := bc.updatePending(bc.linkedRoot); err != nil {
		return nil, err
	}
	bc.LinkedRoot().SetActive(bc.LinkedRoot().Pending()) // For genesis case
	ilog.Info("Witness Block Num:", bc.LinkedRoot().Head.Number)
	for _, v := range bc.linkedRoot.Active() {
		ilog.Info("ActiveWitness:", v)
	}
	for _, v := range bc.linkedRoot.Pending() {
		ilog.Info("PendingWitness:", v)
	}
	bc.head = bc.linkedRoot
	bc.witnessNum = int64(len(bc.LinkedRoot().Pending()))

	return &bc, nil
}

// Recover recover previews block cache
func (bc *BlockCacheImpl) Recover(p ConAlgo) (err error) {
	if bc.wal.HasDecoder() {
		//Get All entries
		_, entries, err := bc.wal.ReadAll()
		if err != nil {
			return err
		}
		ilog.Info("Recover block start")
		for i, entry := range entries {
			if i%2000 == 0 {
				ilog.Infof("Recover block progress:%v/%v", i, len(entries))
			}
			err := bc.apply(entry, p)
			if err != nil {
				return err
			}
		}
	}
	return
}

func (bc *BlockCacheImpl) apply(entry wal.Entry, p ConAlgo) (err error) {
	var bcMessage BcMessage
	proto.Unmarshal(entry.Data, &bcMessage)
	switch bcMessage.Type {
	case BcMessageType_LinkType:
		err = bc.applyLink(bcMessage.Data, p)
		if err != nil {
			return
		}
	case BcMessageType_UpdateActiveType:
		err = bc.applyUpdateActive(bcMessage.Data)
		ilog.Info("Finish ApplySetRoot!")
		if err != nil {
			return
		}
	case BcMessageType_UpdateLinkedRootWitnessType:
		err = bc.applyUpdateLinkedRootWitness(bcMessage.Data)
		if err != nil {
			return
		}
	}
	return
}

func (bc *BlockCacheImpl) applyLink(b []byte, p ConAlgo) (err error) {
	block, witnessList, serialNum, err := decodeBCN(b)
	if string(bc.LinkedRoot().HeadHash()) == string(block.HeadHash()) {
		bc.LinkedRoot().WitnessList = witnessList
		bc.LinkedRoot().SerialNum = serialNum
	}
	p.Add(&block, true, false)
	return err
}

func (bc *BlockCacheImpl) applyUpdateActive(b []byte) (err error) {
	blockHeadHash, witnessList, err := decodeUpdateActive(b)
	if bytes.Equal(blockHeadHash, bc.LinkedRoot().HeadHash()) {
		bc.LinkedRoot().SetActive(witnessList.Active())
		ilog.Infof("Set Root active to :%v", bc.LinkedRoot().Active())
	} else {
		block, ok := bc.hmget(blockHeadHash)
		if ok {
			block.SetActive(witnessList.Active())
			ilog.Infof("Set node %d active to :%v", block.Head.Number, block.Active())
		}
	}
	return
}

func (bc *BlockCacheImpl) applyUpdateLinkedRootWitness(b []byte) (err error) {
	blockHeadHash, wl, err := decodeUpdateLinkedRootWitness(b)
	if bytes.Equal(blockHeadHash, bc.LinkedRoot().HeadHash()) {
		bc.linkedRootWitness = wl
		ilog.Infof("Set linkedRootWitness to :%v", bc.linkedRootWitness)
	}
	return
}

// UpdateLib will update last inreversible block
func (bc *BlockCacheImpl) UpdateLib(node *BlockCacheNode) {
	confirmLimit := int(bc.witnessNum*2/3 + 1)

	updateActive := false
	if len(node.ValidWitness) >= confirmLimit {
		bc.updateLib(node, confirmLimit)

		if !common.StringSliceEqual(node.Active(), bc.LinkedRoot().Pending()) {
			updateActive = true
		}
	} else if len(node.ValidWitness)+len(bc.linkedRootWitness) >= confirmLimit &&
		!common.StringSliceEqual(node.Active(), bc.LinkedRoot().Pending()) {
		updateActive = bc.checkUpdateActive(node, confirmLimit)
	}

	if updateActive {
		bc.updateActive(node)
	}
}

func (bc *BlockCacheImpl) updateLib(node *BlockCacheNode, confirmLimit int) {
	if !common.StringSliceEqual(node.Active(), bc.LinkedRoot().Pending()) {
		return
	}
	root := bc.LinkedRoot()
	blockList := make(map[int64]*BlockCacheNode, node.Head.Number-root.Head.Number)
	blockList[node.Head.Number] = node
	loopNode := node.GetParent()
	for loopNode != root {
		blockList[loopNode.Head.Number] = loopNode
		loopNode = loopNode.GetParent()
	}

	for len(node.ValidWitness) >= confirmLimit &&
		common.StringSliceEqual(node.Active(), bc.LinkedRoot().Pending()) &&
		blockList[bc.LinkedRoot().Head.Number+1] != nil {
		// bc.updateLinkedRoot() will change node.ValidWitness, bc.LinkedRoot() and bc.linkedRootWitness
		bc.updateLinkedRoot(blockList[bc.LinkedRoot().Head.Number+1])
		bc.flush()
	}
}

func (bc *BlockCacheImpl) checkUpdateActive(node *BlockCacheNode, confirmLimit int) bool {
	cnt := len(bc.linkedRootWitness)
	for _, w := range node.ValidWitness {
		inc := true
		for _, w1 := range bc.linkedRootWitness {
			if w == w1 {
				inc = false
				break
			}
		}
		if inc {
			cnt++
		}
		if cnt >= confirmLimit {
			return true
		}
	}
	return false
}

func (bc *BlockCacheImpl) updateActive(node *BlockCacheNode) {
	newValidWitness := make([]string, 0)
	for _, witness := range node.ValidWitness {
		for _, w := range bc.LinkedRoot().Pending() {
			if witness == w {
				newValidWitness = append(newValidWitness, witness)
				break
			}
		}
	}
	node.ValidWitness = newValidWitness
	node.SetActive(bc.LinkedRoot().Pending())
	ilog.Infof("update node:%d activelist to %v", node.Head.Number, node.Active())
	err := bc.writeUpdateActiveWAL(node)
	if err != nil {
		ilog.Errorf("Write updateactive to wal failed. err=%v, activelist=%v", err, bc.LinkedRoot().Pending())
	}
}

// Link call this when you run the block verify after Add() to ensure add single bcn to linkedRoot
func (bc *BlockCacheImpl) Link(bcn *BlockCacheNode) {
	bcn.Type = Linked
	delete(bc.leaf, bcn.GetParent())
	bc.leaf[bcn] = bcn.Head.Number
	bcn.updateValidWitness()

	// Update WitnessList of bcn
	bcn.CopyWitness(bcn.GetParent())
	if bcn.Head.Number%common.VoteInterval == 0 {
		if err := bc.updatePending(bcn); err != nil {
			// TODO: Should handle err
			ilog.Errorf("Update block %v pending failed: %v", common.Base58Encode(bcn.HeadHash()), err)
		}
	}

	if bcn.Head.Number > bc.Head().Head.Number || (bcn.Head.Number == bc.Head().Head.Number && bcn.Head.Time < bc.Head().Head.Time) {
		bc.SetHead(bcn)
	}
}

// AddNodeToWAL add write node message to WAL
func (bc *BlockCacheImpl) AddNodeToWAL(bcn *BlockCacheNode) {
	index, err := bc.writeAddNodeWAL(bcn)
	if err != nil {
		ilog.Error("Failed to write add node WAL!", err)
	}
	bcn.walIndex = index
}

func (bc *BlockCacheImpl) updateLinkedRootWitness(parent, bcn *BlockCacheNode) {
	if !common.StringSliceEqual(parent.Pending(), bcn.Pending()) {
		bc.linkedRootWitness = make([]string, 0)
	}
	witness := bcn.Head.Witness
	for _, w := range bc.linkedRootWitness {
		if w == witness {
			witness = ""
			break
		}
	}
	if witness != "" {
		bc.linkedRootWitness = append(bc.linkedRootWitness, witness)
	}
}

func (bc *BlockCacheImpl) updatePending(h *BlockCacheNode) error {
	ok := bc.stateDB.Checkout(string(h.HeadHash()))
	if ok {
		rules := h.Head.Rules()
		if err := h.UpdatePending(bc.stateDB, rules); err != nil {
			ilog.Error("failed to update pending, err:", err)
			return err
		}
		if err := h.UpdateInfo(bc.stateDB, rules); err != nil {
			ilog.Error("failed to update witness info, err:", err)
			return err
		}
	} else {
		return errors.New("failed to checkout state db")
	}
	return nil
}

// Add is add a block
func (bc *BlockCacheImpl) Add(blk *block.Block) *BlockCacheNode {
	newNode, nok := bc.hmget(blk.HeadHash())
	if nok {
		return newNode
	}
	parent, ok := bc.hmget(blk.Head.ParentHash)
	if !ok {
		parent, ok = bc.singleRoot[string(blk.Head.ParentHash)]
		if !ok {
			parent = NewBCN(nil, nil)
			bc.singleRoot[string(blk.Head.ParentHash)] = parent
		}
	}
	newNode, ok = bc.singleRoot[string(blk.HeadHash())]
	if ok {
		delete(bc.singleRoot, string(blk.HeadHash()))
		// updateVirtualBCN
		newNode.Block = blk
		newNode.SetParent(parent)
		parent.addChild(newNode)
	} else {
		newNode = NewBCN(parent, blk)
	}
	bc.hmset(blk.HeadHash(), newNode)
	return newNode
}

// AddGenesis is add genesis block
func (bc *BlockCacheImpl) AddGenesis(blk *block.Block) {
	l := NewBCN(nil, blk)
	l.Type = Linked
	bc.SetLinkedRoot(l)

	bc.SetHead(bc.LinkedRoot())
	bc.hmset(bc.LinkedRoot().HeadHash(), bc.LinkedRoot())
	bc.leaf[bc.LinkedRoot()] = bc.LinkedRoot().Head.Number
}

// Del is delete a block
func (bc *BlockCacheImpl) Del(bcn *BlockCacheNode) {
	bc.del(bcn)
}

func (bc *BlockCacheImpl) del(bcn *BlockCacheNode) {
	for ch := range bcn.Children {
		bc.del(ch)
	}
	delete(bcn.GetParent().Children, bcn)
	bcn.SetParent(nil)
	bc.hmdel(bcn.HeadHash())
	delete(bc.leaf, bcn)
}

func (bc *BlockCacheImpl) delSingle() {
	// TODO: process with other goroutine
	height := bc.LinkedRoot().Head.Number
	if height%DelSingleBlockTime != 0 {
		return
	}

	for hash, vbcn := range bc.singleRoot {
		ok := false
		for bcn := range vbcn.Children {
			if bcn.Head.Number <= height {
				bc.del(bcn)
				ok = true
			}
		}
		if ok && (len(vbcn.Children) == 0) {
			delete(bc.singleRoot, hash)
		}
	}
}

func (bc *BlockCacheImpl) updateLinkedRoot(bcn *BlockCacheNode) {
	parent := bcn.GetParent()
	if parent != bc.LinkedRoot() {
		ilog.Errorf("block isn't blockcache root's child")
	}
	for child := range parent.Children {
		if child != bcn {
			bc.del(child)
		}
	}

	bc.updateLinkedRootWitness(parent, bcn)
	bcn.removeValidWitness(bcn)

	bc.nmdel(parent.Head.Number)
	bc.hmdel(parent.HeadHash())

	bcn.SetParent(nil)
	bc.SetLinkedRoot(bcn)
	bc.delSingle()

	// Update Longest
	_, ok := bc.hmget(bc.Head().HeadHash())
	if ok {
		return
	}
	for bcn := range bc.leaf {
		if bcn.Head.Number > bc.Head().Head.Number || (bcn.Head.Number == bc.Head().Head.Number && bcn.Head.Time < bc.Head().Head.Time) {
			bc.SetHead(bcn)
		}
	}
}

func (bc *BlockCacheImpl) flush() {
	//confirm linked root to db
	bcn := bc.LinkedRoot()

	err := bc.blockChain.Push(bcn.Block)
	if err != nil {
		ilog.Errorf("Push blockchain error: %v %v", common.Base58Encode(bcn.HeadHash()), err)
	}

	err = bc.writeUpdateLinkedRootWitnessWAL()
	if err != nil {
		ilog.Errorf("Write linked root witness wal error: %v %v", common.Base58Encode(bcn.HeadHash()), err)
	}

	err = bc.stateDB.Flush(string(bcn.HeadHash()))
	if err != nil {
		ilog.Errorf("Flush state db error: %v %v", common.Base58Encode(bcn.HeadHash()), err)
	}

	err = bc.wal.RemoveFilesBefore(bcn.walIndex)
	if err != nil {
		ilog.Errorf("Cut wal files error: %v %v", common.Base58Encode(bcn.HeadHash()), err)
	}
}

func (bc *BlockCacheImpl) writeUpdateLinkedRootWitnessWAL() (err error) {
	hb, err := encodeUpdateLinkedRootWitness(bc)
	if err != nil {
		return err
	}
	bcMessage := &BcMessage{
		Data: hb,
		Type: BcMessageType_UpdateLinkedRootWitnessType,
	}
	data, err := proto.Marshal(bcMessage)
	if err != nil {
		return err
	}
	ent := wal.Entry{
		Data: data,
	}
	_, err = bc.wal.SaveSingle(ent)
	return
}

func (bc *BlockCacheImpl) writeUpdateActiveWAL(h *BlockCacheNode) (err error) {
	hb, err := encodeUpdateActive(h)
	if err != nil {
		return err
	}
	bcMessage := &BcMessage{
		Data: hb,
		Type: BcMessageType_UpdateActiveType,
	}
	data, err := proto.Marshal(bcMessage)
	if err != nil {
		return
	}
	ent := wal.Entry{
		Data: data,
	}
	_, err = bc.wal.SaveSingle(ent)
	return
}

func (bc *BlockCacheImpl) writeAddNodeWAL(h *BlockCacheNode) (uint64, error) {
	hb, err := encodeBCN(h)
	if err != nil {
		return 0, err
	}
	bcMessage := &BcMessage{
		Data: hb,
		Type: BcMessageType_LinkType,
	}
	data, err := proto.Marshal(bcMessage)
	if err != nil {
		return 0, err
	}
	ent := wal.Entry{
		Data: data,
	}
	return bc.wal.SaveSingle(ent)
}

// GetBlockByNumber get a block by number
func (bc *BlockCacheImpl) GetBlockByNumber(num int64) (*block.Block, error) {
	it := bc.Head()
	if num < bc.LinkedRoot().Head.Number || num > it.Head.Number {
		return nil, fmt.Errorf("block not found")
	}
	bc.numberMutex.Lock()
	for it != nil {
		bcn, ok := bc.nmget(it.Head.Number)
		if ok && bcn == it {
			break
		}
		bc.nmset(it.Head.Number, it)
		it = it.GetParent()
	}
	bc.numberMutex.Unlock()
	bcn, ok := bc.nmget(num)
	if ok {
		return bcn.Block, nil
	}
	return nil, fmt.Errorf("block not found")
}

// GetBlockByHash get a block by hash
func (bc *BlockCacheImpl) GetBlockByHash(hash []byte) (*block.Block, error) {
	bcn, ok := bc.hmget(hash)
	if !ok {
		return nil, errors.New("block not found")
	}
	return bcn.Block, nil
}

// LinkedRoot return the root node
func (bc *BlockCacheImpl) LinkedRoot() *BlockCacheNode {
	bc.linkRW.RLock()
	defer bc.linkRW.RUnlock()
	return bc.linkedRoot
}

// SetLinkedRoot sets linked blockcache node.
func (bc *BlockCacheImpl) SetLinkedRoot(n *BlockCacheNode) {
	bc.linkRW.Lock()
	bc.linkedRoot = n
	bc.linkRW.Unlock()
}

// Head return head of block cache
func (bc *BlockCacheImpl) Head() *BlockCacheNode {
	bc.headRW.RLock()
	defer bc.headRW.RUnlock()
	return bc.head
}

// SetHead sets head blockcache node.
func (bc *BlockCacheImpl) SetHead(n *BlockCacheNode) {
	bc.headRW.Lock()
	bc.head = n
	bc.headRW.Unlock()
}
