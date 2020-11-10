package iwallet

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/iost-official/go-iost/account"
	"github.com/iost-official/go-iost/common"
	"github.com/iost-official/go-iost/sdk"
	"github.com/spf13/cobra"
)

var (
	ownerKey         string
	activeKey        string
	initialRAM       int64
	initialBalance   int64
	initialGasPledge int64
)

type acc struct {
	Name    string
	KeyPair *key
}

type accounts struct {
	Dir     string
	Account []*acc
}

// accountCmd represents the account command.
var accountCmd = &cobra.Command{
	Use:     "account",
	Aliases: []string{"acc"},
	Short:   "KeyPair manager",
	Long:    `Manage account in local storage`,
}

var createCmd = &cobra.Command{
	Use:   "create accountName",
	Short: "Create an account on blockchain",
	Long:  `Create an account on blockchain`,
	Example: `  iwallet account create test1 --account test0
  iwallet account create test2 --account test0 --initial_balance 0 --initial_gas_pledge 10 --initial_ram 0
  iwallet account create test3 --account test0 --owner 7Z9US64vfcyopQpyEwV1FF52HTB8maEacjU4SYeAUrt1 --active 7Z9US64vfcyopQpyEwV1FF52HTB8maEacjU4SYeAUrt1`,
	Args: func(cmd *cobra.Command, args []string) error {
		if err := checkArgsNumber(cmd, args, "accountName"); err != nil {
			return err
		}
		return checkAccount(cmd)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		var (
			autoKey    bool
			okey, akey string
			newKp      *account.KeyPair
			err        error
		)

		newName := args[0]
		if strings.ContainsAny(newName, `?*:|/\"`) || len(newName) < 5 || len(newName) > 11 {
			return fmt.Errorf("invalid account name")
		}

		if ownerKey == "" && activeKey == "" {
			autoKey = true
			newKp, err = account.NewKeyPair(nil, sdk.GetSignAlgoByName(signAlgo))
			if err != nil {
				return fmt.Errorf("failed to create key pair: %v", err)
			}
			okey = newKp.ReadablePubkey()
			akey = okey
		} else if sdk.CheckPubKey(ownerKey) && sdk.CheckPubKey(activeKey) {
			okey, akey = ownerKey, activeKey
		} else {
			return fmt.Errorf("key provided but not valid")
		}

		// Set account since making actions needs accountName.
		iwalletSDK.SetAccount(accountName, nil)
		actions, err := iwalletSDK.CreateNewAccountActions(newName, okey, akey, initialGasPledge, initialRAM, initialBalance)
		if err != nil {
			return err
		}
		tx, err := initTxFromActions(actions)
		if err != nil {
			return err
		}
		if outputTxFile != "" {
			return saveTx(tx)
		}

		if err := iwalletSDK.Connect(); err != nil {
			return err
		}
		defer iwalletSDK.CloseConn()

		if err := sendTx(tx); err != nil {
			return err
		}
		if checkResult {
			info, err := iwalletSDK.GetAccountInfo(newName)
			if err != nil {
				return fmt.Errorf("failed to get account info: %v", err)
			}
			fmt.Println("Account info of <", newName, ">:")
			fmt.Println(sdk.MarshalTextString(info))
		}

		fmt.Println("The IOST account ID is:", newName)
		fmt.Println("Owner permission key:", okey)
		fmt.Println("Active permission key:", akey)

		if autoKey {
			accInfo := NewAccountInfo()
			accInfo.Name = newName
			kp := &KeyPairInfo{RawKey: common.Base58Encode(newKp.Seckey), PubKey: common.Base58Encode(newKp.Pubkey), KeyType: signAlgo}
			accInfo.Keypairs["active"] = kp
			accInfo.Keypairs["owner"] = kp
			err = accInfo.Save(encrypt)
			if err != nil {
				return fmt.Errorf("failed to save account: %v", err)
			}
		}
		return nil
	},
}

var viewCmd = &cobra.Command{
	Use:   "view [accountName]",
	Short: "View account by name or omit to show all accounts",
	Long:  `View account by name or omit to show all accounts`,
	Example: `  iwallet account view test0
  iwallet account view`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := getAccountDir()
		if err != nil {
			return fmt.Errorf("failed to get account dir: %v", err)
		}
		a := accounts{}
		a.Dir = dir
		addAcc := func(ac *AccountInfo) {
			var k key
			k.Algorithm = ac.Keypairs["active"].KeyType
			k.Pubkey = ac.Keypairs["active"].PubKey
			if ac.isEncrypted() {
				k.Seckey = "---encrypted secret key---"
			} else {
				k.Seckey = ac.Keypairs["active"].RawKey
			}
			a.Account = append(a.Account, &acc{ac.Name, &k})
		}
		if len(args) < 1 {
			files, err := ioutil.ReadDir(dir)
			if err != nil {
				return err
			}
			for _, f := range files {
				ac, err := loadAccountFromFile(dir+"/"+f.Name(), false)
				if err != nil {
					continue
				}
				addAcc(ac)
			}
		} else {
			name := args[0]
			ac, err := loadAccountByName(name, false)
			if err != nil {
				return err
			}
			addAcc(ac)
		}
		info, err := json.MarshalIndent(a, "", "    ")
		if err != nil {
			return err
		}
		fmt.Println(string(info))
		return nil
	},
}

var encrypt bool
var importCmd = &cobra.Command{
	Use:   "import accountName accountPrivateKey",
	Short: "Import an account by name and private key",
	Long:  `Import an account by name and private key`,
	Example: `  iwallet account import test0 XXXXXXXXXXXXXXXXXXXXX
  iwallet account import test0 active:XXXXXXXXXXXXXXXXXXXXX,owner:YYYYYYYYYYYYYYYYYYYYYYYY`,
	Args: func(cmd *cobra.Command, args []string) error {
		if err := checkArgsNumber(cmd, args, "accountName", "accountPrivateKey"); err != nil {
			return err
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		acc := AccountInfo{Name: name, Keypairs: make(map[string]*KeyPairInfo)}
		keys := strings.Split(args[1], ",")
		if len(keys) == 1 {
			key := keys[0]
			if len(strings.Split(key, ":")) != 1 {
				return fmt.Errorf("importing one key need not specifying permission")
			}
			kp, err := NewKeyPairInfo(key, signAlgo)
			if err != nil {
				return err
			}
			acc.Keypairs["active"] = kp
			acc.Keypairs["owner"] = kp
		} else {
			for _, permAndKey := range keys {
				splits := strings.Split(permAndKey, ":")
				if len(splits) != 2 {
					return fmt.Errorf("importing more than one keys need specifying permissions")
				}
				kp, err := NewKeyPairInfo(splits[1], signAlgo)
				if err != nil {
					return err
				}
				acc.Keypairs[splits[0]] = kp
			}
		}
		err := acc.Save(encrypt)
		if err != nil {
			return fmt.Errorf("failed to save account: %v", err)
		}
		fmt.Printf("import account %v done\n", name)
		return nil
	},
}

var dumpKeyCmd = &cobra.Command{
	Use:     "dumpkey accountName",
	Short:   "Print private key of the account to stdout",
	Long:    "Print private key of the account to stdout",
	Example: `  iwallet account dumpkey test0`,
	Args: func(cmd *cobra.Command, args []string) error {
		if err := checkArgsNumber(cmd, args, "accountName"); err != nil {
			return err
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		acc, err := loadAccountByName(args[0], true)
		if err != nil {
			return err
		}
		for k, v := range acc.Keypairs {
			fmt.Printf("%v:%v\n", k, v.RawKey)
		}
		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use:     "delete accountName",
	Aliases: []string{"del"},
	Short:   "Delete an account by name",
	Long:    `Delete an account by name`,
	Example: `  iwallet account delete test0`,
	Args: func(cmd *cobra.Command, args []string) error {
		if err := checkArgsNumber(cmd, args, "accountName"); err != nil {
			return err
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		dir, err := getAccountDir()
		if err != nil {
			return fmt.Errorf("failed to get account dir: %v", err)
		}
		found := false
		sufs := []string{".json"}
		for _, algo := range ValidSignAlgos {
			sufs = append(sufs, "_"+algo)
		}
		for _, suf := range sufs {
			f := fmt.Sprintf("%s/%s%s", dir, name, suf)
			err = os.Remove(f)
			if err == nil {
				found = true
				fmt.Println("File", f, "has been removed.")
			}
			err = os.Remove(f + ".id")
			if err == nil {
				fmt.Println("File", f+".id", "has been removed.")
			}
			err = os.Remove(f + ".pub")
			if err == nil {
				fmt.Println("File", f+".pub", "has been removed.")
			}
		}
		if found {
			fmt.Println("Successfully deleted <", name, ">.")
		} else {
			fmt.Println("Account <", name, "> does not exist.")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(accountCmd)
	accountCmd.PersistentFlags().BoolVarP(&encrypt, "encrypt", "", false, "whether to encrypt local key file")

	accountCmd.AddCommand(createCmd)
	createCmd.Flags().StringVarP(&ownerKey, "owner", "", "", "owner key")
	createCmd.Flags().StringVarP(&activeKey, "active", "", "", "active key")
	createCmd.Flags().Int64VarP(&initialRAM, "initial_ram", "", 1024, "buy $initial_ram bytes ram for the new account")
	createCmd.Flags().Int64VarP(&initialGasPledge, "initial_gas_pledge", "", 10, "pledge $initial_gas_pledge IOSTs for the new account")
	createCmd.Flags().Int64VarP(&initialBalance, "initial_balance", "", 0, "transfer $initial_balance IOSTs to the new account")

	accountCmd.AddCommand(importCmd)
	accountCmd.AddCommand(viewCmd)
	accountCmd.AddCommand(deleteCmd)
	accountCmd.AddCommand(dumpKeyCmd)
}
