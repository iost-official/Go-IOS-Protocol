package iserver

import (
	"context"
	"encoding/json"
	"github.com/iost-official/go-iost/common/config"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/iost-official/go-iost/core/block"
	"github.com/iost-official/go-iost/core/blockcache"
	"github.com/iost-official/go-iost/ilog"
	"github.com/iost-official/go-iost/p2p"
)

// DebugServer is a http server for debug
type DebugServer struct {
	srv      *http.Server
	conf     *config.DebugConfig
	p2p      *p2p.NetService
	blkCache blockcache.BlockCache
	blkChain block.Chain
}

// NewDebugServer returns new debug server
func NewDebugServer(conf *config.DebugConfig, p2p *p2p.NetService, blkCache blockcache.BlockCache, blkChain block.Chain) *DebugServer {
	return &DebugServer{
		srv:      &http.Server{Addr: conf.ListenAddr},
		conf:     conf,
		p2p:      p2p,
		blkCache: blkCache,
		blkChain: blkChain,
	}
}

// Start starts debug server
func (d *DebugServer) Start() error {
	http.HandleFunc(
		"/debug/blockcache/",
		func(rw http.ResponseWriter, r *http.Request) {
			rw.Write([]byte(d.blkCache.Draw()))
		})

	http.HandleFunc(
		"/debug/blockchain/",
		func(rw http.ResponseWriter, r *http.Request) {
			rg := r.URL.Query()
			sp := strings.Split(rg["range"][0], "-")
			start, err := strconv.Atoi(sp[0])
			if err != nil {
				return
			}
			end, err := strconv.Atoi(sp[1])
			if err != nil {
				return
			}
			rw.Write([]byte(d.blkChain.Draw(int64(start), int64(end))))
		})

	http.HandleFunc(
		"/debug/p2p/neighbors/",
		func(rw http.ResponseWriter, r *http.Request) {
			neighbors := d.p2p.NeighborStat()
			bytes, _ := json.MarshalIndent(neighbors, "", "    ")
			rw.Write(bytes)
		})

	http.HandleFunc(
		"/debug/setloglevel/",
		func(rw http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			lvl := q["level"]
			if len(lvl) > 0 {
				ilog.SetLevel(ilog.NewLevel(lvl[0]))
				rw.Write([]byte("ok"))
				return
			}
			rw.Write([]byte("param error"))
		})

	go func() {
		if err := d.srv.ListenAndServe(); err != http.ErrServerClosed {
			ilog.Errorf("Debug server listen failed. err=%v", err)
		}
	}()

	return nil
}

// Stop stops debug server
func (d *DebugServer) Stop() {
	ilog.Infof("Stopping debug server...")

	ctx, _ := context.WithTimeout(context.Background(), time.Second) // nolint
	if err := d.srv.Shutdown(ctx); err != nil {
		ilog.Errorf("Stop debug server failed: %v", err)
	} else {
		ilog.Infof("Stopped debug server.")
	}
}
