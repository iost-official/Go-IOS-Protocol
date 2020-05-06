package consensus

import (
	"github.com/iost-official/go-iost/chainbase"
	"github.com/iost-official/go-iost/common/config"
	"github.com/iost-official/go-iost/consensus/pob"
	"github.com/iost-official/go-iost/p2p"
)

// Type is the type of consensus
type Type uint8

// The types of consensus
const (
	_ Type = iota
	Pob
)

// Consensus is a consensus server.
type Consensus interface {
	Start() error
	Stop()
}

// New returns the different consensus strategy.
func New(cType Type, conf *config.Config, chainBase *chainbase.ChainBase, service p2p.Service) Consensus {
	switch cType {
	case Pob:
		return pob.New(conf, chainBase, service)
	default:
		return pob.New(conf, chainBase, service)
	}
}
