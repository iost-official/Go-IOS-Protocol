package global

import (
	"github.com/iost-official/go-iost/common/config"
)

// BuildTime build time
var BuildTime string

// GitHash git hash
var GitHash string

// CodeVersion is the version string of code
var CodeVersion string

var globalConf *config.Config

// Token is the blockchain native token name
var Token = "iost"

// SetGlobalConf ...
func SetGlobalConf(conf *config.Config) {
	globalConf = conf
	if conf.NativeToken != "" {
		Token = conf.NativeToken
	}
}

// GetGlobalConf ...
func GetGlobalConf() *config.Config {
	return globalConf
}
