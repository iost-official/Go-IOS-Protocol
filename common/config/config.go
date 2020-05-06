package config

import (
	"github.com/iost-official/go-iost/ilog"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
	"os"
)

// ACCConfig account of the system
type ACCConfig struct {
	ID        string
	SecKey    string
	Algorithm string
}

// Witness config of the genesis block
type Witness struct {
	ID             string
	Owner          string
	Active         string
	SignatureBlock string
	Balance        int64
}

// TokenInfo config of the genesis block
type TokenInfo struct {
	FoundationAccount string
	IOSTTotalSupply   int64
	IOSTDecimal       int64
}

// GenesisConfig config of the genesis bloc
type GenesisConfig struct {
	CreateGenesis    bool
	InitialTimestamp string
	TokenInfo        *TokenInfo
	WitnessInfo      []*Witness
	ContractPath     string
	AdminInfo        *Witness
	FoundationInfo   *Witness
}

// DBConfig config of the database
type DBConfig struct {
	LdbPath string
}

// VMConfig config of the v8vm
type VMConfig struct {
	JsPath   string
	LogLevel string
}

// P2PConfig is the config for p2p network.
type P2PConfig struct {
	ListenAddr   string
	SeedNodes    []string
	ChainID      uint32
	Version      uint16
	DataPath     string
	InboundConn  int
	OutboundConn int
	BlackPID     []string
	BlackIP      []string
	AdminPort    string
}

//RPCConfig is the config for RPC Server.
type RPCConfig struct {
	Enable       bool
	GatewayAddr  string
	GRPCAddr     string
	AllowOrigins []string
	TryTx        bool
	ExecTx       bool
}

// FileLogConfig is the config for filewriter of ilog.
type FileLogConfig struct {
	Path   string
	Level  string
	Enable bool
}

// ConsoleLogConfig is the config for consolewriter of ilog.
type ConsoleLogConfig struct {
	Level  string
	Enable bool
}

// LogConfig is the config of ilog.
type LogConfig struct {
	FileLog           *FileLogConfig
	ConsoleLog        *ConsoleLogConfig
	AsyncWrite        bool
	EnableContractLog bool
}

// MetricsConfig is the config of metrics.
type MetricsConfig struct {
	PushAddr string
	Username string
	Password string
	Enable   bool
	ID       string
}

// SnapshotConfig is the config of snapshot
type SnapshotConfig struct {
	Enable   bool
	FilePath string
}

// DebugConfig is the config of debug.
type DebugConfig struct {
	ListenAddr string
}

// VersionConfig contrains netname(mainnet / testnet etc) and protocol info
type VersionConfig struct {
	NetName         string
	ProtocolVersion string
}

// Config provide all configuration for the application
type Config struct {
	ACC         *ACCConfig
	Genesis     string
	VM          *VMConfig
	DB          *DBConfig
	Snapshot    *SnapshotConfig
	P2P         *P2PConfig
	RPC         *RPCConfig
	Log         *LogConfig
	Metrics     *MetricsConfig
	Debug       *DebugConfig
	Version     *VersionConfig
	NativeToken string
}

// LoadYamlAsViper load yaml file as viper object
func LoadYamlAsViper(configfile string) *viper.Viper {
	v := viper.GetViper()
	v.SetConfigType("yaml")

	f, err := os.Open(configfile)
	if err != nil {
		ilog.Fatalf("Failed to open config file '%v', %v", configfile, err)
	}

	if err := v.ReadConfig(f); err != nil {
		ilog.Fatalf("Failed to read config file: %v", err)
	}

	return v
}

// NewConfig returns a new instance of Config
func NewConfig(configfile string) *Config {
	v := LoadYamlAsViper(configfile)
	c := &Config{}
	if err := v.Unmarshal(c); err != nil {
		ilog.Fatalf("Unable to decode into struct, %v", err)
	}
	return c
}

// YamlString config to string
func (c *Config) YamlString() string {
	bs, err := yaml.Marshal(c)
	if err != nil {
		ilog.Fatalf("Unable to marshal config to YAML: %v", err)
	}
	return string(bs)
}
