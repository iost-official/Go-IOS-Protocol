package p2p

import (
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/iost-official/go-iost/v3/ilog"

	bloom "github.com/bits-and-blooms/bloom/v3"
	libnet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	multiaddr "github.com/multiformats/go-multiaddr"
	"go.uber.org/atomic"
)

// errors
var (
	ErrMessageChannelFull = errors.New("message channel is full")
	ErrDuplicateMessage   = errors.New("reduplicate message")
)

const (
	bloomMaxItemCount = 100000
	bloomErrRate      = 0.001

	msgChanSize          = 1024
	maxDataLength        = 10000000 // 10MB
	routingQueryTimeout  = 10
	maxContinuousTimeout = 10
)

// Peer represents a neighbor which we connect directily.
//
// Peer's jobs are:
//   * managing streams which are responsible for sending and reading messages.
//   * recording messages we have sent and received so as to reduce redundant message in network.
//   * maintaning a priority queue of message to be sending.
type Peer struct {
	id          peer.ID
	addr        multiaddr.Multiaddr
	conn        libnet.Conn
	peerManager *PeerManager

	stream            libnet.Stream
	continuousTimeout int

	recentMsg      *bloom.BloomFilter
	bloomMutex     sync.Mutex
	bloomItemCount int

	urgentMsgCh chan *p2pMessage
	normalMsgCh chan *p2pMessage

	direction connDirection

	quitWriteCh chan struct{}
	once        sync.Once

	lastRoutingQueryTime atomic.Int64
}

// NewPeer returns a new instance of Peer struct.
func NewPeer(stream libnet.Stream, pm *PeerManager, direction connDirection) *Peer {
	peer := &Peer{
		id:          stream.Conn().RemotePeer(),
		addr:        stream.Conn().RemoteMultiaddr(),
		conn:        stream.Conn(),
		stream:      stream,
		peerManager: pm,
		recentMsg:   bloom.NewWithEstimates(bloomMaxItemCount, bloomErrRate),
		urgentMsgCh: make(chan *p2pMessage, msgChanSize),
		normalMsgCh: make(chan *p2pMessage, msgChanSize),
		quitWriteCh: make(chan struct{}),
		direction:   direction,
	}
	peer.lastRoutingQueryTime.Store(time.Now().Unix())
	return peer
}

// ID return the net id.
func (p *Peer) ID() string {
	return p.id.Pretty()
}

// Addr return the address.
func (p *Peer) Addr() string {
	return p.addr.String()
}

// Start starts peer's loop.
func (p *Peer) Start() {
	ilog.Infof("peer is started. id=%s", p.ID())

	go p.readLoop()
	go p.writeLoop()
}

// Stop stops peer's loop and cuts off the TCP connection.
func (p *Peer) Stop() {
	ilog.Infof("peer is stopped. id=%s", p.ID())

	p.once.Do(func() {
		close(p.quitWriteCh)
	})
	p.conn.Close()
}

func (p *Peer) write(m *p2pMessage) error {
	// 5 kB/s
	deadline := time.Now().Add(time.Duration(len(m.content())/1024/5+3) * time.Second)
	if err := p.stream.SetWriteDeadline(deadline); err != nil {
		ilog.Warnf("setting write deadline failed. err=%v, pid=%v", err, p.ID())
		p.peerManager.RemoveNeighbor(p.id)
		return err
	}
	_, err := p.stream.Write(m.content())
	if err != nil {
		ilog.Warnf("writing message failed. err=%v, pid=%v", err, p.ID())
		if strings.Contains(err.Error(), "i/o timeout") {
			p.continuousTimeout++
			if p.continuousTimeout >= maxContinuousTimeout {
				ilog.Warnf("max continuous timeout times, remove peer %v", p.ID())
				p.peerManager.RemoveNeighbor(p.id)
			}
		} else {
			p.peerManager.RemoveNeighbor(p.id)
		}
		return err
	}
	p.continuousTimeout = 0
	tagkv := map[string]string{"mtype": m.messageType().String()}
	byteOutCounter.Add(float64(len(m.content())), tagkv)
	packetOutCounter.Add(1, tagkv)

	return nil
}

func (p *Peer) writeLoop() {
	for {
		select {
		case <-p.quitWriteCh:
			ilog.Infof("peer is stopped. pid=%v, addr=%v", p.ID(), p.addr)
			return
		case um := <-p.urgentMsgCh:
			p.write(um)
		case nm := <-p.normalMsgCh:
			for done := false; !done; {
				select {
				case <-p.quitWriteCh:
					ilog.Infof("peer is stopped. pid=%v, addr=%v", p.ID(), p.addr)
					return
				case um := <-p.urgentMsgCh:
					p.write(um)
				default:
					done = true
				}
			}
			p.write(nm)
		}
	}
}

func (p *Peer) readLoop() {
	header := make([]byte, dataBegin)
	for {
		_, err := io.ReadFull(p.stream, header)
		if err != nil {
			ilog.Warnf("read header failed. err=%v", err)
			break
		}
		chainID := binary.BigEndian.Uint32(header[chainIDBegin:chainIDEnd])
		if chainID != p.peerManager.config.ChainID {
			ilog.Warnf("Mismatched chainID, put peer to blacklist. remotePeer=%v, chainID=%d", p.ID(), chainID)
			p.peerManager.PutPeerToBlack(p.ID())
			return
		}
		length := binary.BigEndian.Uint32(header[dataLengthBegin:dataLengthEnd])
		if length > maxDataLength {
			ilog.Warnf("data length too large: %d", length)
			break
		}
		data := make([]byte, dataBegin+length)
		_, err = io.ReadFull(p.stream, data[dataBegin:])
		if err != nil {
			ilog.Warnf("read message failed. err=%v", err)
			break
		}
		copy(data[0:dataBegin], header)
		msg, err := parseP2PMessage(data)
		if err != nil {
			ilog.Errorf("parse p2pmessage failed. err=%v", err)
			break
		}
		tagkv := map[string]string{"mtype": msg.messageType().String()}
		byteInCounter.Add(float64(len(msg.content())), tagkv)
		packetInCounter.Add(1, tagkv)
		p.handleMessage(msg)
	}

	p.peerManager.RemoveNeighbor(p.id)
}

// SendMessage puts message into the corresponding channel.
func (p *Peer) SendMessage(msg *p2pMessage, mp MessagePriority, deduplicate bool) error {
	if deduplicate && msg.needDedup() {
		if p.hasMessage(msg) {
			// ilog.Debug("ignore reduplicate message")
			return ErrDuplicateMessage
		}
	}

	ch := p.urgentMsgCh
	if mp == NormalMessage {
		ch = p.normalMsgCh
	}
	select {
	case ch <- msg:
	default:
		ilog.Errorf("sending message failed. channel is full. messagePriority=%d", mp)
		return ErrMessageChannelFull
	}
	if msg.needDedup() {
		p.recordMessage(msg)
	}
	if msg.messageType() == RoutingTableQuery {
		p.routingQueryNow()
	}
	return nil
}

func (p *Peer) handleMessage(msg *p2pMessage) error {
	if msg.needDedup() {
		p.recordMessage(msg)
	}
	if msg.messageType() == RoutingTableResponse {
		if p.isRoutingQueryTimeout() {
			ilog.Debugf("receive timeout routing response. pid=%v", p.ID())
			return nil
		}
		p.resetRoutingQueryTime()
	}
	p.peerManager.HandleMessage(msg, p.id)
	return nil
}

func (p *Peer) recordMessage(msg *p2pMessage) {
	p.bloomMutex.Lock()
	defer p.bloomMutex.Unlock()

	if p.bloomItemCount >= bloomMaxItemCount {
		p.recentMsg = bloom.NewWithEstimates(bloomMaxItemCount, bloomErrRate)
		p.bloomItemCount = 0
	}

	p.recentMsg.Add(msg.content())
	p.bloomItemCount++
}

func (p *Peer) hasMessage(msg *p2pMessage) bool {
	p.bloomMutex.Lock()
	defer p.bloomMutex.Unlock()

	return p.recentMsg.Test(msg.content())
}

// resetRoutingQueryTime resets last routing query time.
func (p *Peer) resetRoutingQueryTime() {
	p.lastRoutingQueryTime.Store(-1)
}

// isRoutingQueryTimeout returns whether the last routing query time is too old.
func (p *Peer) isRoutingQueryTimeout() bool {
	return time.Now().Unix()-p.lastRoutingQueryTime.Load() > routingQueryTimeout
}

// routingQueryNow sets the routing query time to the current timestamp.
func (p *Peer) routingQueryNow() {
	p.lastRoutingQueryTime.Store(time.Now().Unix())
}
