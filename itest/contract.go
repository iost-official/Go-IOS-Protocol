package itest

import (
	"io/ioutil"

	"github.com/iost-official/go-iost/core/contract"
)

// Constant of Contract
const (
	DefaultCode = `
	class Test {
		init() {
			//Execute once when contract is packed into a block
		}
		transfer(from, to, amount, memo) {
			//Function called by other
			blockchain.transfer(from, to, amount, memo)
		}

	};
	module.exports = Test;
	`
	DefaultABI = `
	{
		"lang": "javascript",
		"version": "1.0.0",
		"abi": [
			{
				"name": "transfer",
				"args": [
					"string",
					"string",
					"string",
					"string"
				],
				"amountLimit": [
					{
						"token": "iost",
						"val": "100"
					}
				]
			}
		]
	}
	`
)

// Contract is the contract object
type Contract struct {
	*contract.Contract
}

// NewContract will return a new contract
func NewContract(code, abi string) (*Contract, error) {
	c, err := (&contract.Compiler{}).Parse("", code, abi)
	if err != nil {
		return nil, err
	}

	contract := &Contract{
		Contract: c,
	}
	return contract, nil
}

// LoadContract will load a contract from file
func LoadContract(codepath, abipath string) (*Contract, error) {
	code := DefaultCode
	abi := DefaultABI

	if codepath != "" {
		data, err := ioutil.ReadFile(codepath)
		if err != nil {
			return nil, err
		}
		code = string(data)
	}

	if abipath != "" {
		data, err := ioutil.ReadFile(abipath)
		if err != nil {
			return nil, err
		}
		abi = string(data)
	}

	contract, err := NewContract(code, abi)
	if err != nil {
		return nil, err
	}

	return contract, nil
}

// String will return the string of contract
func (c *Contract) String() string {
	return c.B64Encode()
}
