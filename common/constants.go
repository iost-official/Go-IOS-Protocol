package common

import "time"

// Constant of limit
var (
	MaxBlockGasLimit  = int64(800000000)
	MaxTxTimeLimit    = 200 * time.Millisecond
	MaxBlockTimeLimit = 400 * time.Millisecond
)
