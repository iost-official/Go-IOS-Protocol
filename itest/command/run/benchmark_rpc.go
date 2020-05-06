package run

import (
	"context"
	"fmt"
	"github.com/iost-official/go-iost/core/global"
	"github.com/iost-official/go-iost/rpc/pb"
	"math/rand"
	"time"

	"github.com/iost-official/go-iost/ilog"
	"github.com/iost-official/go-iost/itest"
	"github.com/urfave/cli"
)

// BenchmarkRPCCommand is the subcommand for benchmark.
var BenchmarkRPCCommand = cli.Command{
	Name:      "benchmark_rpc",
	ShortName: "bench_rpc",
	Usage:     "Run benchmark rpc by given tps",
	Flags:     BenchmarkRPCFlags,
	Action:    BenchmarkRPCAction,
}

// BenchmarkRPCFlags is the list of flags for benchmark.
var BenchmarkRPCFlags = []cli.Flag{
	cli.Float64Flag{
		Name:  "interval",
		Value: 0.2,
		Usage: "The interval (in second) between every call",
	},
}

var rpcCount = 0

func loopCreateAccount(it *itest.ITest, interval float64) {
	for {
		time.Sleep(time.Duration(interval*1000) * time.Millisecond)
		randName := fmt.Sprintf("acc%08d", rand.Int63n(100000000))
		_, err := it.CreateAccount(it.GetDefaultAccount(), randName, false)
		if err != nil {
			ilog.Errorf("create account err %v", err)
		}
	}
}

func loopGetNodeInfo(it *itest.ITest, interval float64) {
	for {
		time.Sleep(time.Duration(interval*1000) * time.Millisecond)
		c, err := it.GetRandClient().GetGRPC()
		if err != nil {
			ilog.Errorf("cannot get grpc client %v", err)
			continue
		}
		_, err = c.GetNodeInfo(context.Background(), &rpcpb.EmptyRequest{})
		if err != nil {
			ilog.Errorf("cannot get node info %v", err)
		}
		rpcCount++
	}
}

func loopGetChainInfo(it *itest.ITest, interval float64) {
	for {
		time.Sleep(time.Duration(interval*1000) * time.Millisecond)
		c, err := it.GetRandClient().GetGRPC()
		if err != nil {
			ilog.Errorf("cannot get grpc client %v", err)
			continue
		}
		_, err = c.GetChainInfo(context.Background(), &rpcpb.EmptyRequest{})
		if err != nil {
			ilog.Errorf("cannot get chain info %v", err)
		}
		rpcCount++
	}
}

func loopGetStorage(it *itest.ITest, interval float64) {
	for {
		time.Sleep(time.Duration(interval*1000) * time.Millisecond)
		c := it.GetRandClient()
		r := rand.Intn(5)
		var err error
		if r == 0 {
			_, _, _, err = c.GetContractStorage("bonus.iost", "blockContrib", "")
		} else if r == 1 {
			_, _, _, err = c.GetContractStorage("ram.iost", "leftSpace", "")
		} else if r == 2 {
			_, _, _, err = c.GetContractStorage("token.iost", "TIiost", "totalSupply")
		} else if r == 3 {
			_, _, _, err = c.GetContractStorage("token.iost", "TBadmin", global.Token)
		} else if r == 4 {
			_, _, _, err = c.GetContractStorage("vote_producer.iost", "producerTable", "admin")
		}
		if err != nil {
			ilog.Errorf("cannot get contract storage %v", err)
		}
		rpcCount++
	}
}

func loopGetContract(it *itest.ITest, interval float64) {
	for {
		time.Sleep(time.Duration(interval*1000) * time.Millisecond)
		c, err := it.GetRandClient().GetGRPC()
		if err != nil {
			ilog.Errorf("cannot get grpc client %v", err)
			continue
		}
		_, err = c.GetContract(context.Background(), &rpcpb.GetContractRequest{
			Id:             "vote.iost",
			ByLongestChain: rand.Int()%2 == 0,
		})
		if err != nil {
			ilog.Errorf("cannot get contract info %v", err)
		}
		rpcCount++
	}
}

func loopGetGasRatio(it *itest.ITest, interval float64) {
	for {
		time.Sleep(time.Duration(interval*1000) * time.Millisecond)
		c, err := it.GetRandClient().GetGRPC()
		if err != nil {
			ilog.Errorf("cannot get grpc client %v", err)
			continue
		}
		_, err = c.GetGasRatio(context.Background(), &rpcpb.EmptyRequest{})
		if err != nil {
			ilog.Errorf("cannot get gas ratio %v", err)
		}
		rpcCount++
	}
}

func loopGetRAMInfo(it *itest.ITest, interval float64) {
	for {
		time.Sleep(time.Duration(interval*1000) * time.Millisecond)
		c, err := it.GetRandClient().GetGRPC()
		if err != nil {
			ilog.Errorf("cannot get grpc client %v", err)
			continue
		}
		_, err = c.GetRAMInfo(context.Background(), &rpcpb.EmptyRequest{})
		if err != nil {
			ilog.Errorf("cannot get ram info %v", err)
		}
		rpcCount++
	}
}

func loopGetAccount(it *itest.ITest, interval float64) {
	for {
		time.Sleep(time.Duration(interval*1000) * time.Millisecond)
		c, err := it.GetRandClient().GetGRPC()
		if err != nil {
			ilog.Errorf("cannot get grpc client %v", err)
			continue
		}
		_, err = c.GetAccount(context.Background(), &rpcpb.GetAccountRequest{
			Name:           "admin",
			ByLongestChain: rand.Int()%2 == 0,
		})
		if err != nil {
			ilog.Errorf("cannot get ram info %v", err)
		}
		rpcCount++
	}
}

// BenchmarkRPCAction is the action of benchmark.
var BenchmarkRPCAction = func(c *cli.Context) error {
	it, err := itest.Load(c.GlobalString("keys"), c.GlobalString("config"))
	if err != nil {
		return err
	}

	interval := c.Float64("interval")
	go loopCreateAccount(it, interval)
	go loopGetAccount(it, interval)
	go loopGetChainInfo(it, interval)
	go loopGetContract(it, interval)
	go loopGetGasRatio(it, interval)
	go loopGetNodeInfo(it, interval)
	go loopGetRAMInfo(it, interval)
	go loopGetStorage(it, interval)

	lastCount := 0
	lastTime := time.Now()
	for {
		time.Sleep(time.Second)
		currentTps := float64(rpcCount-lastCount) / time.Since(lastTime).Seconds()
		ilog.Infof("Current tps %v", currentTps)
		lastCount = rpcCount
		lastTime = time.Now()
	}
}
