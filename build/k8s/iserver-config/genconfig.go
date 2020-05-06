package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/iost-official/go-iost/common/config"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"strings"
)

var (
	cluster = flag.String("c", "default", "The name of cluster")
	master  = flag.Int("m", 3, "The number of master node")
	slave   = flag.Int("s", 1, "The number of slave node")
)

func main() {
	//	num := flag.String("n", "47", "number")
	//	flag.Parse()
	//
	//	n, err := strconv.Atoi(*num)
	//	if err != nil {
	//		n = 47
	//	}
	//
	//	file, err := os.Create("keypairs2")
	//	if err != nil {
	//		panic(err)
	//	}
	//	defer file.Close()
	//
	//	for i := 0; i < n; i++ {
	//		ac, _ := account.NewAccount(nil, crypto.Ed25519)
	//		file.WriteString(fmt.Sprintf("%s,%s\n", account.GetIDByPubkey(ac.Pubkey), common.Base58Encode(ac.Seckey)))
	//	}

	flag.Parse()
	genconfig()
}

func genconfig() {
	pushAddr := os.Getenv("PROMETHEUS_HOSTNAME")
	username := os.Getenv("PROMETHEUS_USERNAME")
	password := os.Getenv("PROMETHEUS_PASSWORD")
	seedNodes := []string{fmt.Sprintf("/dns4/iserver-0.iserver.%s.svc.cluster.local/tcp/30000/ipfs/12D3KooWA2QZHXCLsVL9rxrtKPRqBSkQj7mCdHEhRoW8eJtn24ht", *cluster)}

	file, err := os.Open("keypairs")
	if err != nil {
		log.Fatal(err)
	}
	scanner := bufio.NewScanner(file)
	ids := make([]string, 0)
	seckeys := make([]string, 0)
	//initialCoins := make([]string, 0)
	for scanner.Scan() {
		s := strings.Split(scanner.Text(), ",")
		ids = append(ids, s[0])
		seckeys = append(seckeys, s[1])
		//initialCoins = append(initialCoins, s[2])
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	file.Close()

	WitnessInfo := make([]*config.Witness, 0)
	for i := 0; i < *master; i++ {
		witness := &config.Witness{
			ID:             fmt.Sprintf("producer%03d", i),
			Owner:          ids[i],
			Active:         ids[i],
			SignatureBlock: ids[i],
			Balance:        int64(0),
		}
		WitnessInfo = append(WitnessInfo, witness)
	}

	adminInfo := &config.Witness{
		ID:      "admin",
		Owner:   "Gcv8c2tH8qZrUYnKdEEdTtASsxivic2834MQW6mgxqto",
		Active:  "Gcv8c2tH8qZrUYnKdEEdTtASsxivic2834MQW6mgxqto",
		Balance: int64(21000000000),
	}

	tokenInfo := &config.TokenInfo{
		FoundationAccount: "foundation",
		IOSTTotalSupply:   90000000000,
		IOSTDecimal:       8,
	}

	foundationInfo := &config.Witness{
		ID:      "foundation",
		Owner:   "Gcv8c2tH8qZrUYnKdEEdTtASsxivic2834MQW6mgxqto",
		Active:  "Gcv8c2tH8qZrUYnKdEEdTtASsxivic2834MQW6mgxqto",
		Balance: 0,
	}

	Genesis := &config.GenesisConfig{
		CreateGenesis:    true,
		TokenInfo:        tokenInfo,
		WitnessInfo:      WitnessInfo,
		AdminInfo:        adminInfo,
		FoundationInfo:   foundationInfo,
		InitialTimestamp: "2018-01-02T15:04:03Z",
	}

	genesisfile, err := os.Create("genesis.yml")
	if err != nil {
		log.Fatal(err)
	}
	bs, err := yaml.Marshal(Genesis)
	if err != nil {
		log.Fatalf("Unable to marshal config to YAML: %v", err)
	}
	genesisfile.WriteString(string(bs))
	genesisfile.Close()

	VM := &config.VMConfig{
		JsPath:   "vm/v8vm/v8/libjs/",
		LogLevel: "",
	}
	DB := &config.DBConfig{
		LdbPath: "/data/storage/",
	}
	Snapshot := &config.SnapshotConfig{
		Enable:   false,
		FilePath: "",
	}
	P2P := &config.P2PConfig{
		ListenAddr:   "0.0.0.0:30000",
		SeedNodes:    seedNodes,
		ChainID:      1020,
		Version:      1,
		DataPath:     "/data/p2p/",
		InboundConn:  18,
		OutboundConn: 12,
		AdminPort:    "30005",
	}
	RPC := &config.RPCConfig{
		Enable:       true,
		GatewayAddr:  "0.0.0.0:30001",
		GRPCAddr:     "0.0.0.0:30002",
		TryTx:        false,
		AllowOrigins: []string{"*"},
	}
	Log := &config.LogConfig{
		FileLog: &config.FileLogConfig{
			Path:   "/data/logs/",
			Level:  "info",
			Enable: false,
		},
		ConsoleLog: &config.ConsoleLogConfig{
			Level:  "info",
			Enable: true,
		},
		AsyncWrite:        true,
		EnableContractLog: false,
	}
	Debug := &config.DebugConfig{
		ListenAddr: "0.0.0.0:30003",
	}

	for i := 0; i < *master+*slave; i++ {
		ACC := &config.ACCConfig{
			ID:        fmt.Sprintf("producer%03d", i),
			SecKey:    seckeys[i],
			Algorithm: "ed25519",
		}
		Metrics := &config.MetricsConfig{
			PushAddr: pushAddr,
			Enable:   true,
			ID:       *cluster + ":node-" + fmt.Sprintf("%d", i),
			Username: username,
			Password: password,
		}
		c := &config.Config{
			ACC:      ACC,
			Genesis:  "/var/lib/iserver/",
			VM:       VM,
			DB:       DB,
			Snapshot: Snapshot,
			P2P:      P2P,
			RPC:      RPC,
			Log:      Log,
			Metrics:  Metrics,
			Debug:    Debug,
		}

		if i == 0 {
			c.P2P.DataPath = "/var/lib/iserver/"
		} else {
			c.P2P.DataPath = "/data/p2p/"
		}

		file, err := os.Create(fmt.Sprintf("iserver-%d.yml", i))
		if err != nil {
			log.Fatal(err)
		}
		file.WriteString(c.YamlString())
		file.Close()
	}
}
