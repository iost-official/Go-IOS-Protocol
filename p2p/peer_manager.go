package p2p

import (
	"bufio"
	"context"
	"fmt"
	"github.com/iost-official/go-iost/common/config"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/iost-official/go-iost/ilog"
	p2pb "github.com/iost-official/go-iost/p2p/pb"

	"github.com/libp2p/go-libp2p-core/host"
	libnet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	kbucket "github.com/libp2p/go-libp2p-kbucket"
	multiaddr "github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"
	"go.uber.org/atomic"
)

var (
	dumpRoutingTableInterval = 5 * time.Minute
	syncRoutingTableInterval = 30 * time.Second
	metricsStatInterval      = 3 * time.Second
	findBPInterval           = 2 * time.Second

	dialTimeout        = 10 * time.Second
	deadPeerRetryTimes = 5
)

type connDirection int

const (
	inbound connDirection = iota
	outbound
)

const (
	defaultOutboundConn = 10
	defaultInboundConn  = 20

	bucketSize        = 1000
	peerResponseCount = 20
	maxPeerQuery      = 30
	maxAddrCount      = 10

	incomingMsgChanSize = 4096

	routingTableFile = "routing.table"
)

// PeerManager manages all peers we connect directily.
//
// PeerManager's jobs are:
//   * holding a certain amount of peers.
//   * handling messages according to its type.
//   * discovering peers and maintaining routing table.
type PeerManager struct {
	neighbors     map[peer.ID]*Peer
	neighborCount map[connDirection]int
	neighborMutex sync.RWMutex

	neighborCap map[connDirection]int

	subs    *sync.Map //  map[MessageType]map[string]chan IncomingMessage
	quitCh  chan struct{}
	started atomic.Int32

	host           host.Host
	config         *config.P2PConfig
	routingTable   *kbucket.RoutingTable
	peerStore      peerstore.Peerstore
	lastUpdateTime atomic.Int64

	wg *sync.WaitGroup

	bpIDs   []peer.ID
	bpMutex sync.RWMutex

	blackPIDs  map[string]bool
	blackIPs   map[string]bool
	blackMutex sync.RWMutex

	retryTimes map[string]int
	rtMutex    sync.RWMutex
}

// NewPeerManager returns a new instance of PeerManager struct.
func NewPeerManager(host host.Host, config *config.P2PConfig) *PeerManager {
	routingTable := kbucket.NewRoutingTable(bucketSize, kbucket.ConvertPeerID(host.ID()), time.Second, host.Peerstore())
	pm := &PeerManager{
		neighbors:     make(map[peer.ID]*Peer),
		neighborCount: make(map[connDirection]int),
		neighborCap:   make(map[connDirection]int),
		subs:          new(sync.Map),
		quitCh:        make(chan struct{}),
		routingTable:  routingTable,
		host:          host,
		config:        config,
		peerStore:     host.Peerstore(),
		wg:            new(sync.WaitGroup),
		blackPIDs:     make(map[string]bool),
		blackIPs:      make(map[string]bool),
		retryTimes:    make(map[string]int),
	}
	if config.InboundConn <= 0 {
		pm.neighborCap[inbound] = defaultOutboundConn
	} else {
		pm.neighborCap[inbound] = config.InboundConn
	}

	if config.OutboundConn <= 0 {
		pm.neighborCap[outbound] = defaultInboundConn
	} else {
		pm.neighborCap[outbound] = config.OutboundConn
	}

	for _, blackIP := range config.BlackIP {
		pm.blackIPs[blackIP] = true
	}
	for _, blackPID := range config.BlackPID {
		pm.blackPIDs[blackPID] = true
	}
	return pm
}

// Start starts peer manager's job.
func (pm *PeerManager) Start() {
	if !pm.started.CAS(0, 1) {
		return
	}

	pm.parseSeeds()
	pm.LoadRoutingTable()

	pm.wg.Add(4)
	go pm.dumpRoutingTableLoop()
	go pm.syncRoutingTableLoop()
	go pm.metricsStatLoop()
	go pm.findBPLoop()

}

// Stop stops peer manager's loop.
func (pm *PeerManager) Stop() {
	if !pm.started.CAS(1, 0) {
		return
	}

	close(pm.quitCh)
	pm.wg.Wait()
	pm.CloseAllNeighbors()
}

func (pm *PeerManager) isStopped() bool {
	return pm.started.Load() == 0
}

func (pm *PeerManager) setBPs(ids []string) {
	peerIDs := make([]peer.ID, 0, len(ids))
	for _, id := range ids {
		if len(id) == 0 {
			continue
		}
		peerID, err := peer.Decode(id)
		if err != nil {
			ilog.Warnf("decode peerID failed. err=%v, id=%v", err, id)
			continue
		}
		peerIDs = append(peerIDs, peerID)
	}
	pm.bpMutex.Lock()
	pm.bpIDs = peerIDs
	pm.bpMutex.Unlock()
}

func (pm *PeerManager) getBPs() []peer.ID {
	pm.bpMutex.RLock()
	defer pm.bpMutex.RUnlock()

	return pm.bpIDs
}

func (pm *PeerManager) isBP(id peer.ID) bool {
	for _, bp := range pm.getBPs() {
		if bp == id {
			return true
		}
	}
	return false
}

func (pm *PeerManager) findBPLoop() {
	defer pm.wg.Done()
	for {
		select {
		case <-pm.quitCh:
			return
		case <-time.After(findBPInterval):
			unknownBPs := make([]string, 0)
			for _, id := range pm.getBPs() {
				if len(pm.peerStore.Addrs(id)) == 0 {
					unknownBPs = append(unknownBPs, id.Pretty())
				}
			}
			pm.routingQuery(unknownBPs)
			pm.connectBPs()
		}
	}
}

func (pm *PeerManager) newStream(pid peer.ID) (libnet.Stream, error) {
	ctx, _ := context.WithTimeout(context.Background(), dialTimeout) // nolint
	return pm.host.NewStream(ctx, pid, protocolID)
}

func (pm *PeerManager) connectBPs() {
	for _, bpID := range pm.getBPs() {
		if pm.isStopped() {
			return
		}

		if pm.GetNeighbor(bpID) == nil && bpID != pm.host.ID() && len(pm.peerStore.Addrs(bpID)) > 0 {
			stream, err := pm.newStream(bpID)
			if err != nil {
				ilog.Warnf("create stream to bp failed. pid=%s, err=%v", bpID.Pretty(), err)
				continue
			}
			pm.HandleStream(stream, outbound)
		}
	}
}

// ConnectBPs makes the local host connected to the block producers directly.
func (pm *PeerManager) ConnectBPs(ids []string) {
	pm.setBPs(ids)
}

// HandleStream handles the incoming stream.
//
// It checks whether the remote peer already exists.
// If the peer is new and the neighbor count doesn't reach the threshold, it adds the peer into the neighbor list.
// If peer already exits, just add the stream to the peer.
// In other cases, reset the stream.
func (pm *PeerManager) HandleStream(s libnet.Stream, direction connDirection) {
	remotePID := s.Conn().RemotePeer()
	pm.freshPeer(remotePID)

	if pm.isStreamBlack(s) {
		ilog.Infof("Remote peer is in black list, close connection. pid=%v, addr=%v", remotePID.Pretty(), s.Conn().RemoteMultiaddr())
		s.Conn().Close()
		return
	}
	ilog.Debugf("handle new stream. pid=%s, addr=%v, direction=%v", remotePID.Pretty(), s.Conn().RemoteMultiaddr(), direction)

	peer := pm.GetNeighbor(remotePID)
	if peer != nil {
		s.Reset()
		return
	}

	if pm.NeighborCount(direction) >= pm.neighborCap[direction] {
		if !pm.isBP(remotePID) {
			ilog.Infof("neighbor count exceeds, close connection. remoteID=%v, addr=%v", remotePID.Pretty(), s.Conn().RemoteMultiaddr())
			if direction == inbound {
				pid, _ := randomPID()
				bytes, _ := pm.getRoutingResponse([]string{pid.Pretty()})
				if len(bytes) > 0 {
					msg := newP2PMessage(pm.config.ChainID, RoutingTableResponse, pm.config.Version, defaultReservedFlag, bytes)
					s.Write(msg.content())
				}
				time.AfterFunc(time.Second, func() { s.Conn().Close() })
			} else {
				s.Conn().Close()
			}
			return
		}
		pm.kickNormalNeighbors(direction)
	}
	pm.AddNeighbor(NewPeer(s, pm, direction))
}

func (pm *PeerManager) dumpRoutingTableLoop() {
	defer pm.wg.Done()
	var lastSaveTime int64
	for {
		select {
		case <-pm.quitCh:
			return
		case <-time.After(dumpRoutingTableInterval):
			if lastSaveTime < pm.lastUpdateTime.Load() {
				pm.DumpRoutingTable()
				lastSaveTime = time.Now().Unix()
			}
		}
	}
}

func (pm *PeerManager) syncRoutingTableLoop() {
	pm.routingQuery([]string{pm.host.ID().Pretty()})

	defer pm.wg.Done()
	for {
		select {
		case <-pm.quitCh:
			return
		case <-time.After(syncRoutingTableInterval):
			//ilog.Debugf("start sync routing table.")
			pid, _ := randomPID()
			pm.routingQuery([]string{pid.Pretty()})
		}
	}
}

func (pm *PeerManager) metricsStatLoop() {
	defer pm.wg.Done()
	for {
		select {
		case <-pm.quitCh:
			return
		case <-time.After(metricsStatInterval):
			neighborCountGauge.Set(float64(pm.AllNeighborCount()), nil)
			routingCountGauge.Set(float64(pm.routingTable.Size()), nil)
		}
	}

}

// storePeerInfo stores peer information in peerStore and routingTable. It doesn't need lock since the
// peerStore.AddAddrs and routingTable.Update function are already thread safe.
func (pm *PeerManager) storePeerInfo(peerID peer.ID, addrs []multiaddr.Multiaddr) {
	// SetAddrs won't overwrite old value. clear and add.
	pm.peerStore.ClearAddrs(peerID)
	pm.peerStore.AddAddrs(peerID, addrs, peerstore.PermanentAddrTTL)
	pm.routingTable.Update(peerID)
	pm.lastUpdateTime.Store(time.Now().Unix())
}

// deletePeerInfo deletes peer information in peerStore and routingTable. It doesn't need lock since the
// peerStore.ClearAddrs and routingTable.Update function are thread safe.
func (pm *PeerManager) deletePeerInfo(peerID peer.ID) {
	pm.peerStore.ClearAddrs(peerID)
	pm.routingTable.Remove(peerID)
	pm.lastUpdateTime.Store(time.Now().Unix())
}

// AddNeighbor starts a peer and adds it to the neighbor list.
func (pm *PeerManager) AddNeighbor(p *Peer) {

	pm.neighborMutex.Lock()
	defer pm.neighborMutex.Unlock()

	if pm.neighbors[p.id] == nil {
		p.Start()
		// pm.storePeerInfo(p.id, []multiaddr.Multiaddr{p.addr})
		pm.neighbors[p.id] = p
		pm.neighborCount[p.direction]++
	}
}

// RemoveNeighbor stops a peer and removes it from the neighbor list.
func (pm *PeerManager) RemoveNeighbor(peerID peer.ID) {

	pm.neighborMutex.Lock()
	defer pm.neighborMutex.Unlock()

	p := pm.neighbors[peerID]
	if p != nil {
		p.Stop()
		delete(pm.neighbors, peerID)
		pm.neighborCount[p.direction]--
	}
}

// GetNeighbor returns the peer of the given peerID from the neighbor list.
func (pm *PeerManager) GetNeighbor(peerID peer.ID) *Peer {
	pm.neighborMutex.RLock()
	defer pm.neighborMutex.RUnlock()

	return pm.neighbors[peerID]
}

// GetAllNeighbors returns the peer of the given peerID from the neighbor list.
func (pm *PeerManager) GetAllNeighbors() []*Peer {
	pm.neighborMutex.RLock()
	defer pm.neighborMutex.RUnlock()

	peers := make([]*Peer, 0, len(pm.neighbors))
	for _, p := range pm.neighbors {
		peers = append(peers, p)
	}
	return peers
}

// CloseAllNeighbors close all connections.
func (pm *PeerManager) CloseAllNeighbors() {
	for _, p := range pm.GetAllNeighbors() {
		p.Stop()
	}
}

// AllNeighborCount returns the total neighbor amount.
func (pm *PeerManager) AllNeighborCount() int {
	pm.neighborMutex.RLock()
	defer pm.neighborMutex.RUnlock()

	return len(pm.neighbors)
}

// NeighborCount returns the neighbor amount of the given direction.
func (pm *PeerManager) NeighborCount(direction connDirection) int {
	pm.neighborMutex.RLock()
	defer pm.neighborMutex.RUnlock()

	return pm.neighborCount[direction]
}

// kickNormalNeighbors removes neighbors that are not block producers.
func (pm *PeerManager) kickNormalNeighbors(direction connDirection) {
	pm.neighborMutex.Lock()
	defer pm.neighborMutex.Unlock()

	for _, p := range pm.neighbors {
		if pm.neighborCount[direction] < pm.neighborCap[direction] {
			return
		}
		if direction == p.direction && !pm.isBP(p.id) {
			p.Stop()
			delete(pm.neighbors, p.id)
			pm.neighborCount[direction]--
		}
	}
}

// DumpRoutingTable saves routing table in file.
func (pm *PeerManager) DumpRoutingTable() {
	file, err := os.Create(filepath.Join(pm.config.DataPath, routingTableFile))
	if err != nil {
		ilog.Errorf("create routing file failed. err=%v, path=%v", err, pm.config.DataPath)
		return
	}
	defer file.Close()
	file.WriteString(fmt.Sprintf("# %s\n", time.Now().String()))
	for _, peerID := range pm.routingTable.ListPeers() {
		for _, addr := range pm.peerStore.Addrs(peerID) {
			if isPublicMaddr(addr.String()) {
				line := fmt.Sprintf("%s/ipfs/%s\n", addr.String(), peerID.Pretty())
				file.WriteString(line)
			}
		}
	}
}

// LoadRoutingTable reads routing table file and parses it.
func (pm *PeerManager) LoadRoutingTable() {
	routingFile := filepath.Join(pm.config.DataPath + routingTableFile)
	if _, err := os.Stat(routingFile); err != nil {
		if os.IsNotExist(err) {
			ilog.Infof("no routing file. file=%v", routingFile)
			return
		}
	}

	file, err := os.Open(routingFile)
	if err != nil {
		ilog.Errorf("open routing file failed. err=%v, file=%v", err, routingFile)
		return
	}
	defer file.Close()
	br := bufio.NewReader(file)
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			break
		}
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		if !isPublicMaddr(line) {
			ilog.Debugf("Ignoring private addr %v", line)
			continue
		}
		peerID, addr, err := parseMultiaddr(line[:len(line)-1])
		if err != nil {
			ilog.Warnf("parse multiaddr failed. err=%v, str=%v", err, line)
			continue
		}
		if peerID == pm.host.ID() {
			continue
		}
		pm.storePeerInfo(peerID, []multiaddr.Multiaddr{addr})
	}
}

// routingQuery broadcasts a routing query message. If the neighbor count is less than the threshold,
// it will pick the rest amount of peers from the routing table and sends query to them.
func (pm *PeerManager) routingQuery(ids []string) {
	if len(ids) == 0 {
		return
	}
	query := &p2pb.RoutingQuery{
		Ids: ids,
	}
	bytes, err := proto.Marshal(query)
	if err != nil {
		ilog.Errorf("pb encode failed. err=%v, obj=%+v", err, query)
		return
	}

	pm.Broadcast(bytes, RoutingTableQuery, UrgentMessage)
	outboundNeighborCount := pm.NeighborCount(outbound)
	if outboundNeighborCount >= pm.neighborCap[outbound] {
		return
	}
	allPeerIDs := pm.routingTable.ListPeers()
	r := rand.New(rand.NewSource(time.Now().Unix()))
	perm := r.Perm(len(allPeerIDs))

	for i, t := 0, 0; i < len(perm) && t < pm.neighborCap[outbound]-outboundNeighborCount; i++ {
		if pm.isStopped() {
			return
		}

		peerID := allPeerIDs[perm[i]]
		if peerID == pm.host.ID() {
			continue
		}
		if pm.GetNeighbor(peerID) != nil {
			continue
		}
		ilog.Debugf("dial peer: pid=%v", peerID.Pretty())
		stream, err := pm.newStream(peerID)
		if err != nil {
			ilog.Warnf("create stream failed. pid=%s, err=%v", peerID.Pretty(), err)

			if strings.Contains(err.Error(), "connected to wrong peer") {
				pm.deletePeerInfo(peerID)
				continue
			}

			pm.recordDialFail(peerID)
			if pm.isDead(peerID) {
				pm.deletePeerInfo(peerID)
			}
			continue
		}
		pm.HandleStream(stream, outbound)
		pm.SendToPeer(peerID, bytes, RoutingTableQuery, UrgentMessage)
		t++
	}
}

func (pm *PeerManager) parseSeeds() {
	for _, seed := range pm.config.SeedNodes {
		peerID, addr, err := parseMultiaddr(seed)
		if err != nil {
			ilog.Errorf("parse seed nodes error. seed=%s, err=%v", seed, err)
			continue
		}

		if madns.Matches(addr) {
			err = pm.dnsResolve(peerID, addr)
			if err != nil {
				time.AfterFunc(60*time.Second, func() {
					ilog.Info("retry resolve dns")
					pm.dnsResolve(peerID, addr)
				})
			}
		} else {
			pm.storePeerInfo(peerID, []multiaddr.Multiaddr{addr})
		}
	}

}

func (pm *PeerManager) dnsResolve(peerID peer.ID, addr multiaddr.Multiaddr) error {
	resAddrs, err := madns.Resolve(context.Background(), addr)
	if err != nil {
		ilog.Errorf("resolve multiaddr failed. err=%v, addr=%v", err, addr)
		return err
	}
	pm.storePeerInfo(peerID, resAddrs)
	return nil
}

// Broadcast sends message to all the neighbors.
func (pm *PeerManager) Broadcast(data []byte, typ MessageType, mp MessagePriority) {
	msg := newP2PMessage(pm.config.ChainID, typ, pm.config.Version, defaultReservedFlag, data)

	wg := new(sync.WaitGroup)
	for _, p := range pm.GetAllNeighbors() {
		wg.Add(1)
		go func(p *Peer) {
			p.SendMessage(msg, mp, true)
			wg.Done()
		}(p)
	}
	wg.Wait()
}

// SendToPeer sends message to the specified peer.
func (pm *PeerManager) SendToPeer(peerID peer.ID, data []byte, typ MessageType, mp MessagePriority) {
	msg := newP2PMessage(pm.config.ChainID, typ, pm.config.Version, defaultReservedFlag, data)

	peer := pm.GetNeighbor(peerID)
	if peer != nil {
		peer.SendMessage(msg, mp, false)
	}
}

// Register registers a message channel of the given types.
func (pm *PeerManager) Register(id string, mTyps ...MessageType) chan IncomingMessage {
	if len(mTyps) == 0 {
		return nil
	}
	c := make(chan IncomingMessage, incomingMsgChanSize)
	for _, typ := range mTyps {
		m, _ := pm.subs.LoadOrStore(typ, new(sync.Map))
		m.(*sync.Map).Store(id, c)
	}
	return c
}

// Deregister deregisters a message channel of the given types.
func (pm *PeerManager) Deregister(id string, mTyps ...MessageType) {
	for _, typ := range mTyps {
		if m, exist := pm.subs.Load(typ); exist {
			m.(*sync.Map).Delete(id)
		}
	}
}

func (pm *PeerManager) getRoutingResponse(peerIDs []string) ([]byte, error) {
	queryIDs := peerIDs
	if len(queryIDs) > maxPeerQuery {
		queryIDs = queryIDs[:maxPeerQuery]
	}

	pidSet := make(map[peer.ID]struct{})
	for _, queryID := range queryIDs {
		pid, err := peer.Decode(queryID)
		if err != nil {
			ilog.Warnf("decode peerID failed. err=%v, id=%v", err, queryID)
			continue
		}
		peerIDs := pm.routingTable.NearestPeers(kbucket.ConvertPeerID(pid), peerResponseCount)
		for _, id := range peerIDs {
			if !pm.isDead(id) {
				pidSet[id] = struct{}{}
			}
		}
	}

	resp := &p2pb.RoutingResponse{}
	for pid := range pidSet {
		info := pm.peerStore.PeerInfo(pid)
		if len(info.Addrs) > 0 {
			peerInfo := &p2pb.PeerInfo{
				Id: info.ID.Pretty(),
			}
			for _, addr := range info.Addrs {
				if isPublicMaddr(addr.String()) {
					peerInfo.Addrs = append(peerInfo.Addrs, addr.String())
				}
			}
			if len(peerInfo.Addrs) > maxAddrCount {
				peerInfo.Addrs = peerInfo.Addrs[:maxAddrCount]
			}
			if len(peerInfo.Addrs) > 0 {
				resp.Peers = append(resp.Peers, peerInfo)
			}
		}
	}
	selfInfo := &p2pb.PeerInfo{Id: pm.host.ID().Pretty()}
	for _, addr := range pm.host.Addrs() {
		selfInfo.Addrs = append(selfInfo.Addrs, addr.String())
	}
	resp.Peers = append(resp.Peers, selfInfo)

	bytes, err := proto.Marshal(resp)
	if err != nil {
		ilog.Errorf("pb encode failed. err=%v, obj=%+v", err, resp)
		return nil, err
	}
	return bytes, nil
}

// handleRoutingTableQuery picks the nearest peers of the given peerIDs and sends the result to inquirer.
func (pm *PeerManager) handleRoutingTableQuery(msg *p2pMessage, from peer.ID) {
	data, _ := msg.data()

	query := &p2pb.RoutingQuery{}
	err := proto.Unmarshal(data, query)
	if err != nil {
		ilog.Errorf("pb decode failed. err=%v, bytes=%v", err, data)
		return
	}

	queryIDs := query.GetIds()
	ilog.Debugf("handling routing table query. %v", queryIDs)
	bytes, _ := pm.getRoutingResponse(queryIDs)
	if len(bytes) > 0 {
		pm.SendToPeer(from, bytes, RoutingTableResponse, UrgentMessage)
	}
}

// handleRoutingTableResponse stores the peer information received.
func (pm *PeerManager) handleRoutingTableResponse(msg *p2pMessage, from peer.ID) { // nolint

	data, _ := msg.data()
	resp := &p2pb.RoutingResponse{}
	err := proto.Unmarshal(data, resp)
	if err != nil {
		ilog.Errorf("Decoding pb failed. err=%v, bytes=%v", err, data)
		return
	}
	ilog.Debugf("Receiving peer infos: %v, from=%v", resp, from.Pretty())
	for _, peerInfo := range resp.Peers {
		if len(peerInfo.Addrs) > 0 {
			pid, err := peer.Decode(peerInfo.Id)
			if err != nil {
				ilog.Warnf("Decoding peerID failed. err=%v, id=%v", err, peerInfo.Id)
				continue
			}

			if pm.isDead(pid) || pm.isPIDBlack(pid) { // ignore bad node's addr
				continue
			}

			if pid == pm.host.ID() { // ignore self's addr
				continue
			}

			// TODO: Take more reasonable measures to avoid routing pollution
			if pm.GetNeighbor(pid) != nil && from != pid { // ignore neighbor's addr
				continue
			}

			addrs := make([]string, 0, len(peerInfo.Addrs))
			if from != pid {
				for _, addr := range peerInfo.Addrs {
					if isPublicMaddr(addr) {
						addrs = append(addrs, addr)
					}
				}
			} else {
				// choose public multiaddr if exists, else take out port and combine it with remoteAddr
				var port string
				var hasPublicMaddr bool
				for _, addr := range peerInfo.Addrs {
					if isPublicMaddr(addr) {
						addrs = append(addrs, addr)
						hasPublicMaddr = true
					} else {
						port = addr[strings.LastIndex(addr, "/")+1:]
					}
				}
				if !hasPublicMaddr {
					neighbor := pm.GetNeighbor(pid)
					if neighbor != nil {
						remoteAddr := neighbor.addr.String()
						remoteListenAddr := remoteAddr[:strings.LastIndex(remoteAddr, "/")+1] + port
						addrs = append(addrs, remoteListenAddr)
					}
				}
			}
			maddrs := make([]multiaddr.Multiaddr, 0, len(addrs))
			for _, addr := range addrs {
				ma, err := multiaddr.NewMultiaddr(addr)
				if err != nil {
					ilog.Warnf("Parsing multiaddr failed. err=%v, addr=%v", err, addr)
					continue
				}
				maddrs = append(maddrs, ma)
			}
			if len(maddrs) > maxAddrCount {
				maddrs = maddrs[:maxAddrCount]
			}
			if len(maddrs) > 0 {
				pm.storePeerInfo(pid, maddrs)
			}
		}
	}
}

// HandleMessage handles messages according to its type.
func (pm *PeerManager) HandleMessage(msg *p2pMessage, peerID peer.ID) {
	data, err := msg.data()
	if err != nil {
		ilog.Errorf("get message data failed. err=%v", err)
		return
	}
	switch msg.messageType() {
	case RoutingTableQuery:
		go pm.handleRoutingTableQuery(msg, peerID)
	case RoutingTableResponse:
		go pm.handleRoutingTableResponse(msg, peerID)
	default:
		inMsg := NewIncomingMessage(peerID, data, msg.messageType())
		if m, exist := pm.subs.Load(msg.messageType()); exist {
			m.(*sync.Map).Range(func(k, v interface{}) bool {
				select {
				case v.(chan IncomingMessage) <- *inMsg:
				default:
					ilog.Warnf("sending incoming message failed. type=%s", msg.messageType())
				}
				return true
			})
		}
	}
}

// NeighborStat dumps neighbors' status for debug.
func (pm *PeerManager) NeighborStat() map[string]interface{} {
	ret := make(map[string]interface{})

	blackIPs := make([]string, 0)
	blackPIDs := make([]string, 0)
	pm.blackMutex.RLock()
	for ip := range pm.blackIPs {
		blackIPs = append(blackIPs, ip)
	}
	for id := range pm.blackPIDs {
		blackPIDs = append(blackPIDs, id)
	}
	pm.blackMutex.RUnlock()
	ret["black_ips"] = blackIPs
	ret["black_pids"] = blackPIDs

	in := make([]string, 0)
	out := make([]string, 0)
	for _, p := range pm.GetAllNeighbors() {
		addr := p.addr.String() + "/ipfs/" + p.ID()
		if p.direction == inbound {
			in = append(in, addr)
		} else {
			out = append(out, addr)
		}
	}
	ret["neighbors"] = map[string]interface{}{
		"outbound": out,
		"inbound":  in,
	}

	ret["neighbor_count"] = map[string]interface{}{
		"outbound": pm.NeighborCount(outbound),
		"inbound":  pm.NeighborCount(inbound),
	}

	return ret
}

// PutPeerToBlack puts the peer's PID and IP to black list and close the connection.
func (pm *PeerManager) PutPeerToBlack(id string) {
	pid, err := peer.Decode(id)
	if err != nil {
		ilog.Warnf("decode peerID failed. err=%v, id=%v", err, id)
		return
	}
	pm.RemoveNeighbor(pid)
	pm.PutPIDToBlack(pid)
	pm.deletePeerInfo(pid)
}

// PutPIDToBlack puts the PID and corresponding ip to black list.
func (pm *PeerManager) PutPIDToBlack(pid peer.ID) {
	pm.blackMutex.Lock()
	pm.blackPIDs[pid.Pretty()] = true
	pm.blackMutex.Unlock()

	for _, ma := range pm.peerStore.Addrs(pid) {
		ip := getIPFromMaddr(ma.String())
		if len(ip) > 0 {
			pm.PutIPToBlack(ip)
		}
	}
}

// PutIPToBlack puts the ip to black list.
func (pm *PeerManager) PutIPToBlack(ip string) {
	pm.blackMutex.Lock()
	pm.blackIPs[ip] = true
	pm.blackMutex.Unlock()
}

func (pm *PeerManager) isStreamBlack(s libnet.Stream) bool {
	pid := s.Conn().RemotePeer()
	pm.blackMutex.RLock()
	defer pm.blackMutex.RUnlock()

	if pm.blackPIDs[pid.Pretty()] {
		return true
	}
	ma := s.Conn().RemoteMultiaddr().String()
	ip := getIPFromMaddr(ma)
	return pm.blackIPs[ip]
}

func (pm *PeerManager) isPIDBlack(pid peer.ID) bool {
	pm.blackMutex.RLock()
	defer pm.blackMutex.RUnlock()
	return pm.blackPIDs[pid.Pretty()]
}

func (pm *PeerManager) recordDialFail(pid peer.ID) {
	pm.rtMutex.Lock()
	defer pm.rtMutex.Unlock()

	pm.retryTimes[pid.Pretty()]++
}

func (pm *PeerManager) freshPeer(pid peer.ID) {
	pm.rtMutex.Lock()
	defer pm.rtMutex.Unlock()

	delete(pm.retryTimes, pid.Pretty())
}

func (pm *PeerManager) isDead(pid peer.ID) bool {
	pm.rtMutex.RLock()
	defer pm.rtMutex.RUnlock()

	return pm.retryTimes[pid.Pretty()] > deadPeerRetryTimes
}
