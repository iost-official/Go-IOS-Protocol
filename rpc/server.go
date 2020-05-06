package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/iost-official/go-iost/common/config"
	"net"
	"net/http"
	"time"

	"github.com/iost-official/go-iost/chainbase"
	"github.com/iost-official/go-iost/core/txpool"
	"github.com/iost-official/go-iost/ilog"
	"github.com/iost-official/go-iost/p2p"
	"github.com/iost-official/go-iost/rpc/pb"
	"github.com/rs/cors"
	"golang.org/x/net/netutil"

	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc"
)

const (
	maxConcurrentStreams = 200
	connectionLimit      = 128
)

// Server is the rpc server including grpc server and json gateway server.
type Server struct {
	grpcAddr   string
	grpcServer *grpc.Server

	gatewayAddr   string
	gatewayServer *http.Server
	allowOrigins  []string

	quitCh chan struct{}

	enable bool
}

func p(pp interface{}) error {
	return fmt.Errorf("%v", pp)
}

// New returns a new rpc server instance.
func New(tp txpool.TxPool, chainBase *chainbase.ChainBase, config *config.Config, p2pService p2p.Service) *Server {
	s := &Server{
		grpcAddr:     config.RPC.GRPCAddr,
		gatewayAddr:  config.RPC.GatewayAddr,
		allowOrigins: config.RPC.AllowOrigins,
		quitCh:       make(chan struct{}),
		enable:       config.RPC.Enable,
	}
	s.grpcServer = grpc.NewServer(
		grpc.UnaryInterceptor(
			grpc_middleware.ChainUnaryServer(
				metricsUnaryMiddleware,
				grpc_recovery.UnaryServerInterceptor(grpc_recovery.WithRecoveryHandler(p)),
			),
		),
		grpc.StreamInterceptor(
			grpc_middleware.ChainStreamServer(
				metricsStreamMiddleware,
				grpc_recovery.StreamServerInterceptor(grpc_recovery.WithRecoveryHandler(p)),
			),
		),
		grpc.MaxConcurrentStreams(maxConcurrentStreams))
	apiService := NewAPIService(tp, chainBase, config, p2pService, s.quitCh)
	rpcpb.RegisterApiServiceServer(s.grpcServer, apiService)
	return s
}

// Start starts the rpc server.
func (s *Server) Start() error {
	if !s.enable {
		return nil
	}
	if err := s.startGrpc(); err != nil {
		return err
	}
	return s.startGateway()
}

func (s *Server) startGrpc() error {
	lis, err := net.Listen("tcp", s.grpcAddr)
	if err != nil {
		return err
	}
	lis = netutil.LimitListener(lis, connectionLimit)
	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			ilog.Fatalf("start grpc failed. err=%v", err)
		}
	}()
	return nil
}

func (s *Server) startGateway() error {
	mux := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{OrigName: true, EmitDefaults: true}),
		runtime.WithProtoErrorHandler(errorHandler))
	opts := []grpc.DialOption{grpc.WithInsecure()}
	err := rpcpb.RegisterApiServiceHandlerFromEndpoint(context.Background(), mux, s.grpcAddr, opts)
	if err != nil {
		return err
	}
	c := cors.New(cors.Options{
		AllowedHeaders: []string{"Content-Type", "Accept"},
		AllowedMethods: []string{"GET", "HEAD", "POST", "PUT", "DELETE"},
		AllowedOrigins: s.allowOrigins,
	})
	s.gatewayServer = &http.Server{
		Addr:    s.gatewayAddr,
		Handler: c.Handler(mux),
	}
	go func() {
		if err := s.gatewayServer.ListenAndServe(); err != http.ErrServerClosed {
			ilog.Fatalf("start gateway failed. err=%v", err)
		}
	}()
	return nil
}

func errorHandler(_ context.Context, _ *runtime.ServeMux, _ runtime.Marshaler, w http.ResponseWriter, _ *http.Request, err error) {
	w.WriteHeader(400)
	bytes, e := json.Marshal(err)
	if e != nil {
		bytes = []byte(fmt.Sprint(err))
	}
	w.Write(bytes)
}

// Stop stops the rpc server.
func (s *Server) Stop() {
	if !s.enable {
		return
	}
	close(s.quitCh)
	ctx, _ := context.WithTimeout(context.Background(), time.Second) // nolint
	s.gatewayServer.Shutdown(ctx)
	s.grpcServer.GracefulStop()
}
