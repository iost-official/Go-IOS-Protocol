package chainbase

import (
	"fmt"
	"github.com/iost-official/go-iost/common/config"

	"github.com/iost-official/go-iost/common"
	"github.com/iost-official/go-iost/consensus/genesis"
	"github.com/iost-official/go-iost/ilog"
)

func (c *ChainBase) checkGenesis(conf *config.Config) error {
	if c.bChain.Length() == int64(0) { //blockchaindb is empty
		// TODO: remove the module of starting iserver from yaml.
		ilog.Infof("Genesis is not exist.")
		hash := c.stateDB.CurrentTag()
		if hash != "" {
			return fmt.Errorf("blockchaindb is empty, but statedb is not")
		}

		blk, err := genesis.GenGenesisByFile(c.stateDB, conf.Genesis)
		if err != nil {
			return fmt.Errorf("new GenGenesis failed: %v", err)
		}
		err = c.bChain.Push(blk)
		if err != nil {
			return fmt.Errorf("push block to blockChain failed: %v", err)
		}

		err = c.stateDB.Flush(string(blk.HeadHash()))
		if err != nil {
			return fmt.Errorf("flush stateDB failed: %v", err)
		}
		ilog.Infof("Created Genesis.")

		// TODO check genesis hash between config and db
		ilog.Infof("GenesisHash: %v", common.Base58Encode(blk.HeadHash()))
	}
	return nil
}

func (c *ChainBase) recoverDB(conf *config.Config) error {
	blockChain := c.BlockChain()

	length := int64(0)
	hash := c.stateDB.CurrentTag()
	ilog.Infoln("current Tag:", common.Base58Encode([]byte(hash)))
	if hash != "" {
		blk, err := c.bChain.GetBlockByHash([]byte(hash))
		if err != nil {
			return fmt.Errorf(
				"The block %v of the stateDB is not found in blockchainDB, err: %v",
				common.Base58Encode([]byte(hash)),
				err,
			)
		}
		length = blk.Head.Number + 1
	}
	blockChain.SetLength(length)
	return nil
}

// recoverBlockCache will recover chainbase data from WAL.
func (c *ChainBase) recoverBlockCache() error {
	err := c.bCache.Recover(c)
	if err != nil {
		return fmt.Errorf("recover blockCache failed: %v", err)
	}
	return nil
}
