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
	"context"
	"fmt"

	"github.com/iost-official/Go-IOS-Protocol/rpc"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

// transactionCmd represents the transaction command
var transactionCmd = &cobra.Command{
	Use:   "transaction",
	Short: "find transactions",
	Long:  `find transaction by transaction hash`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			fmt.Println(`Error: transaction hash not given`)
			return
		}

		conn, err := grpc.Dial(server, grpc.WithInsecure())
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		defer conn.Close()
		client := rpc.NewApisClient(conn)
		txRaw, err := client.GetTxByHash(context.Background(), &rpc.HashReq{Hash: loadBytes(args[0])})
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		//fmt.Println("tx raw:", txRaw.TxRaw)
		if err != nil {
			fmt.Println(err.Error())
		}
		fmt.Println(txRaw)
	},
}

func init() {
	rootCmd.AddCommand(transactionCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// transactionCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// transactionCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
