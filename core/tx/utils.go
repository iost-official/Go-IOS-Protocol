package tx

import (
	"fmt"
	"math"
	"regexp"
	"strconv"

	"github.com/bitly/go-simplejson"
	"github.com/iost-official/go-iost/v3/common"
)

func checkAmount(amount string, token string) error {
	matched, err := regexp.MatchString("^([0-9]+[.])?[0-9]+$", amount)
	if err != nil || !matched {
		return fmt.Errorf("invalid amount: %v", amount)
	}
	f1, err := common.NewDecimalFromString(amount, -1)
	if err != nil {
		return fmt.Errorf("invalid amount: %v, %v", err, amount)
	}
	f2, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return fmt.Errorf("invalid amount: %v, %v", err, amount)
	}
	if math.Abs(f1.Float64()-f2) > 1e-4 {
		return fmt.Errorf("invalid amount: %v, %v", err, amount)
	}
	if token == "iost" && f1.Scale > 8 {
		return fmt.Errorf("invalid decimal: %v", amount)
	}
	return nil
}

func checkBadAction(action *Action) error { // nolint:gocyclo
	data := action.Data
	js, err := simplejson.NewJson([]byte(data))
	if err != nil {
		return fmt.Errorf("invalid json array: %v, %v", err, data)
	}
	arr, err := js.Array()
	if err != nil {
		return fmt.Errorf("invalid json array: %v, %v", err, data)
	}
	if action.Contract == "token.iost" && action.ActionName == "transfer" {
		if len(arr) != 5 {
			return fmt.Errorf("wrong args num: %v", data)
		}
		token, err := js.GetIndex(0).String()
		if err != nil {
			return fmt.Errorf("invalid token: %v, %v", err, data)
		}
		amount, err := js.GetIndex(3).String()
		if err != nil {
			return fmt.Errorf("invalid amount: %v, %v", err, data)
		}
		err = checkAmount(amount, token)
		if err != nil {
			return err
		}
		return nil
	}
	if action.Contract == "gas.iost" && (action.ActionName == "pledge" || action.ActionName == "unpledge") {
		if len(arr) != 3 {
			return fmt.Errorf("wrong args num: %v", data)
		}
		amount, err := js.GetIndex(2).String()
		if err != nil {
			return fmt.Errorf("invalid amount: %v, %v", err, data)
		}
		err = checkAmount(amount, "iost")
		if err != nil {
			return fmt.Errorf("invalid amount: %v, %v", err, data)
		}
		f, err := common.NewDecimalFromString(amount, -1)
		if err != nil {
			return fmt.Errorf("invalid amount: %v, %v", err, data)
		}
		if f.ShrinkScale().Scale > 2 {
			return fmt.Errorf("decimal should not exceed 2: %v", data)
		}
	}
	return nil
}

// CheckBadTx ...
func CheckBadTx(tx *Tx) error {
	for _, a := range tx.Actions {
		err := checkBadAction(a)
		if err != nil {
			return err
		}
	}
	return nil
}
