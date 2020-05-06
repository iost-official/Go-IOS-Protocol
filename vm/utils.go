package vm

import (
	"fmt"
	"github.com/iost-official/go-iost/common"
	"github.com/iost-official/go-iost/core/global"
	"github.com/iost-official/go-iost/core/tx"
	"github.com/iost-official/go-iost/vm/database"
	"github.com/iost-official/go-iost/vm/native"
)

// CheckTxGasLimitValid ...
func CheckTxGasLimitValid(t *tx.Tx, currentGas *common.Fixed, dbVisitor *database.Visitor) (err error) {
	gasLimit := &common.Fixed{Value: t.GasLimit, Decimal: 2}
	if !currentGas.LessThan(gasLimit) {
		return nil
	}
	defaultErr := fmt.Errorf("gas not enough: user %v has %v < %v", t.Publisher, currentGas.ToString(), gasLimit.ToString())
	if !(len(t.Actions) == 1 && t.Actions[0].Contract == native.GasContractName && t.Actions[0].ActionName == "pledge") {
		return defaultErr
	}
	// user is trying to pledge for gas without initial gas
	args, err := UnmarshalArgs(dbVisitor.Contract(native.GasContractName).ABI("pledge"), t.Actions[0].Data)
	if err != nil {
		return fmt.Errorf("invalid gas pledge args %v %v", err, t.Actions[0].Data)
	}
	if !(args[0] == t.Publisher && args[1] == t.Publisher) {
		return defaultErr
	}
	balance := dbVisitor.TokenBalanceFixed(global.Token, t.Publisher)
	pledgeAmount, err := common.NewFixed(args[2].(string), 8)
	if err != nil {
		return fmt.Errorf("invalid gas pledge amount %v %v", err, args[2].(string))
	}
	if pledgeAmount.LessThan(database.GasMinPledgePerAction) {
		return fmt.Errorf("invalid gas pledge amount %v %v", err, args[2].(string))
	}
	if balance.LessThan(pledgeAmount) {
		return fmt.Errorf("iost token amount not enough for pledgement %v < %v", balance.ToString(), pledgeAmount.ToString())
	}
	if currentGas.Add(pledgeAmount.Multiply(database.GasImmediateReward)).LessThan(gasLimit) {
		return fmt.Errorf("gas not enough even if considering the new gas pledgement %v + %v < %v",
			currentGas.ToString(), pledgeAmount.Multiply(database.GasImmediateReward).ToString(), gasLimit.ToString())
	}
	return nil
}
