package iserver

import (
	"github.com/iost-official/go-iost/chainbase"
	"github.com/iost-official/go-iost/common/config"
	"github.com/iost-official/go-iost/consensus"
	"github.com/iost-official/go-iost/core/tx"
	"github.com/iost-official/go-iost/ilog"
	"github.com/iost-official/go-iost/metrics/exporter"
	"github.com/iost-official/go-iost/p2p"
	"github.com/iost-official/go-iost/rpc"
)

// Service defines APIs of resident goroutines.
type Service interface {
	Start() error
	Stop()
}

// IServer is application for IOST.
type IServer struct {
	config    *config.Config
	cBase     *chainbase.ChainBase
	p2p       *p2p.NetService
	rpcServer *rpc.Server
	consensus consensus.Consensus
	debug     *DebugServer
	exporter  *exporter.Exporter
}

// New returns a iserver application
func New(conf *config.Config) *IServer {

	tx.ChainID = conf.P2P.ChainID

	cBase, err := chainbase.New(conf)
	if err != nil {
		ilog.Fatalf("New chainbase failed: %v.", err)
	}

	exporter := exporter.New()

	p2pService, err := p2p.NewNetService(conf.P2P)
	if err != nil {
		ilog.Fatalf("network initialization failed, stop the program! err:%v", err)
	}

	consensus := consensus.New(consensus.Pob, conf, cBase, p2pService)

	rpcServer := rpc.New(cBase.TxPool(), cBase, conf, p2pService)

	debug := NewDebugServer(conf.Debug, p2pService, cBase.BlockCache(), cBase.BlockChain())

	return &IServer{
		config:    conf,
		cBase:     cBase,
		p2p:       p2pService,
		rpcServer: rpcServer,
		consensus: consensus,
		debug:     debug,
		exporter:  exporter,
	}
}

// Start starts iserver application.
func (s *IServer) Start() error {
	Services := []Service{
		s.p2p,
		s.consensus,
		s.rpcServer,
	}
	for _, s := range Services {
		if err := s.Start(); err != nil {
			return err
		}
	}
	conf := s.config
	if conf.Debug != nil {
		if err := s.debug.Start(); err != nil {
			return err
		}
	}
	return nil
}

// Stop stops iserver application.
func (s *IServer) Stop() {
	conf := s.config
	if conf.Debug != nil {
		s.debug.Stop()
	}
	Services := []Service{
		s.rpcServer,
		s.consensus,
		s.p2p,
	}
	for _, s := range Services {
		s.Stop()
	}
	s.exporter.Close()
	s.cBase.Close()
}
