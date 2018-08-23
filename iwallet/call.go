// Copyright © 2018 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package iwallet

import (
	"fmt"

	"github.com/iost-official/Go-IOS-Protocol/core/new_tx"
	"github.com/spf13/cobra"
)

// callCmd represents the compile command
var callCmd = &cobra.Command{
	Use:   "call",
	Short: "Call a method in some contract",
	Long: `Call a method in some contract
			the format of this command is:iwallet call contract_name0 function_name0 parameters0 contract_name1 function_name1 parameters1 ...
			(you can call more than one function in this command)
			the parameters is a string whose format is: ["arg0","arg1",...]
	`,
	Run: func(cmd *cobra.Command, args []string) {
		argc := len(args)
		if argc%3 != 0 {
			fmt.Println(`Error: number of args should be a multiplier of 3`)
			return
		}
		var actions []tx.Action = make([]tx.Action, argc/3)
		for i := 0; i < len(args); i += 3 {
			actions[i] = tx.NewAction(args[i], args[i+1], args[i+2]) //check sth here
		}
		pubkeys := make([][]byte, len(signers))
		for i, pubkey := range signers {
			pubkeys[i] = loadBytes(string(pubkey))
		}
		trx := tx.NewTx(actions, pubkeys, gasLimit, gasPrice, expiration)

		bytes := trx.Encode()
		if dest == "default" {
			dest = changeSuffix(args[0], ".sc")
		}

		err := saveTo(dest, bytes)
		if err != nil {
			fmt.Println(err.Error())
		}
	},
}

func init() {
	rootCmd.AddCommand(callCmd)

	callCmd.Flags().Int64VarP(&gasLimit, "gaslimit", "l", 1000, "gasLimit for a transaction")
	callCmd.Flags().Int64VarP(&gasPrice, "gasprice", "p", 1, "gasPrice for a transaction")
	callCmd.Flags().Int64VarP(&expiration, "expiration", "", 0, "expiration timestamp for a transaction")
	callCmd.Flags().StringSliceVarP(&signers, "signers", "", []string{}, "signers who should sign this transaction")

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// callCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// compi leCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
