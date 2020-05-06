package main

import (
	"github.com/iost-official/go-iost/common/config"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/iost-official/go-iost/core/global"
	"github.com/iost-official/go-iost/core/version"
	"github.com/iost-official/go-iost/ilog"
	"github.com/iost-official/go-iost/iserver"
	"github.com/iost-official/go-iost/metrics"
	flag "github.com/spf13/pflag"
)

var (
	configFile = flag.StringP("config", "f", "", "Configuration `file`")
	help       = flag.BoolP("help", "h", false, "Display available options")
)

func initMetrics(metricsConfig *config.MetricsConfig) error {
	if metricsConfig == nil || !metricsConfig.Enable {
		return nil
	}
	err := metrics.SetPusher(metricsConfig.PushAddr, metricsConfig.Username, metricsConfig.Password)
	if err != nil {
		return err
	}
	metrics.SetID(metricsConfig.ID)
	return metrics.Start()
}

func initLogger(logConfig *config.LogConfig) {
	if logConfig == nil {
		return
	}
	logger := ilog.New()
	if logConfig.AsyncWrite {
		logger.AsyncWrite()
	}
	if logConfig.ConsoleLog != nil && logConfig.ConsoleLog.Enable {
		consoleWriter := ilog.NewConsoleWriter()
		consoleWriter.SetLevel(ilog.NewLevel(logConfig.ConsoleLog.Level))
		logger.AddWriter(consoleWriter)
	}
	if logConfig.FileLog != nil && logConfig.FileLog.Enable {
		fileWriter := ilog.NewFileWriter(logConfig.FileLog.Path)
		fileWriter.SetLevel(ilog.NewLevel(logConfig.FileLog.Level))
		logger.AddWriter(fileWriter)
	}
	ilog.InitLogger(logger)
}

func main() {
	flag.Parse()
	if *help {
		flag.Usage()
	}

	if *configFile == "" {
		*configFile = os.Getenv("GOPATH") + "/src/github.com/iost-official/go-iost/config/iserver.yml"
	}

	conf := config.NewConfig(*configFile)

	global.SetGlobalConf(conf)
	version.InitChainConf(conf)

	initLogger(conf.Log)

	ilog.Infof("Config Information:\n%v", strings.Replace(conf.YamlString(), conf.ACC.SecKey, conf.ACC.SecKey[:3]+"******", -1))

	ilog.Infof("build time:%v", global.BuildTime)
	ilog.Infof("git hash:%v", global.GitHash)
	ilog.Infof("code version:%v", global.CodeVersion)

	err := initMetrics(conf.Metrics)
	if err != nil {
		ilog.Errorf("init metrics failed. err=%v", err)
	}

	server := iserver.New(conf)
	if err := server.Start(); err != nil {
		ilog.Fatalf("start iserver failed. err=%v", err)
	}

	waitExit()

	server.Stop()
	ilog.Stop()
}

func waitExit() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	i := <-c
	ilog.Infof("IOST server received interrupt[%v], shutting down...", i)
}
