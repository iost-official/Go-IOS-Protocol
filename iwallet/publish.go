package iwallet

import (
	"fmt"

	"github.com/iost-official/go-iost/ilog"
	"github.com/iost-official/go-iost/sdk"

	"github.com/spf13/cobra"
)

var update bool
var postCheck string

// publishCmd represents the publish command.
var publishCmd = &cobra.Command{
	Use:     "publish codePath abiPath [contractID [updateID]]",
	Aliases: []string{"pub"},
	Short:   "Publish a contract",
	Long:    `Publish a contract by a contract and an abi file`,
	Example: `  iwallet publish ./example.js ./example.js.abi --account test0
  iwallet publish -u ./example.js ./example.js.abi ContractXXX --account test0`,
	Args: func(cmd *cobra.Command, args []string) error {
		var err error
		if update {
			err = checkArgsNumber(cmd, args, "codePath", "abiPath", "contractID")
		} else {
			err = checkArgsNumber(cmd, args, "codePath", "abiPath")
		}
		if err != nil {
			return err
		}
		if outputTxFile == "" {
			return checkAccount(cmd)
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		codePath := args[0]
		abiPath := args[1]

		conID := ""
		if update {
			conID = args[2]
		}
		updateID := ""
		if update && len(args) >= 4 {
			updateID = args[3]
		}

		actions, err := iwalletSDK.PublishContractActions(codePath, abiPath, conID, update, updateID)
		if err != nil {
			return err
		}
		if postCheck != "" {
			if !update {
				ilog.Warn("Contract is not being updated, skip --post_check")
			} else {
				actions = append(actions, sdk.NewAction(conID, postCheck, string(`[]`)))
			}
		}
		tx, err := initTxFromActions(actions)
		if err != nil {
			return err
		}
		if outputTxFile != "" {
			return saveTx(tx)
		}
		txHash, err := sendTxGetHash(tx)
		if err != nil {
			return err
		}
		if !update {
			fmt.Println("The contract id is: Contract" + txHash)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(publishCmd)
	publishCmd.Flags().BoolVarP(&update, "update", "u", false, "update contract")
	publishCmd.Flags().StringVarP(&postCheck, "post_check", "", "", "method to call after the publish."+
		" It this method fails, the whole tx will fail. For example, you can check `can_update` in a method as post publish hook, "+
		"to prevent disabling `can_update` accidentally")
}
