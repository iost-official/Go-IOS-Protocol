package v8

/*
#include <stdlib.h>
#include "v8/vm.h"
#cgo darwin LDFLAGS: -lvm
#cgo linux LDFLAGS: -L${SRCDIR}/v8/libv8/_linux_amd64 -lvm -lv8 -Wl,-rpath,${SRCDIR}/v8/libv8/_linux_amd64
*/
import "C"
import (
	"sync"

	"github.com/iost-official/Go-IOS-Protocol/core/contract"
	"github.com/iost-official/Go-IOS-Protocol/vm/host"
)

var CVMInitOnce = sync.Once{}
var customStartupData C.CustomStartupData
var customCompileStartupData C.CustomStartupData

// VM contains isolate instance, which is a v8 VM with its own heap.
type VM struct {
	isolate              C.IsolatePtr
	sandbox              *Sandbox
	releaseChannel       chan *VM
	vmType               vmPoolType
	jsPath               string
	limitsOfInstructions int64
	limitsOfMemorySize   int64
}

// NewVM return new vm with isolate and sandbox
func NewVM(vmType vmPoolType, jsPath string) *VM {
	CVMInitOnce.Do(func() {
		C.init()
		customStartupData = C.createStartupData()
		customCompileStartupData = C.createCompileStartupData()
	})
	var isolate C.IsolatePtr
	if vmType == CompileVMPool {
		isolate = C.newIsolate(customCompileStartupData)
	} else {
		isolate = C.newIsolate(customStartupData)
	}
	e := &VM{
		isolate: isolate,
		vmType:  vmType,
		jsPath:  jsPath,
	}
	e.sandbox = NewSandbox(e)
	return e
}

func NewVMWithChannel(vmType vmPoolType, jsPath string, releaseChannel chan *VM) *VM {
	e := NewVM(vmType, jsPath)
	e.releaseChannel = releaseChannel
	return e
}

func (e *VM) init() error {
	return nil
}

// Run load contract from code and invoke api function
func (e *VM) Run(code, api string, args ...interface{}) (interface{}, error) {
	contr := &contract.Contract{
		ID:   "run_id",
		Code: code,
	}

	preparedCode, err := e.sandbox.Prepare(contr, api, args)
	if err != nil {
		return "", err
	}

	rs, _, err := e.sandbox.Execute(preparedCode)
	return rs, err
}

func (e *VM) compile(contract *contract.Contract) (string, error) {
	return e.sandbox.Compile(contract)
}

func (e *VM) setHost(host *host.Host) {
	e.sandbox.SetHost(host)
}

func (e *VM) setContract(contract *contract.Contract, api string, args ...interface{}) (string, error) {
	return e.sandbox.Prepare(contract, api, args)
}

func (e *VM) execute(code string) (rtn []interface{}, cost *contract.Cost, err error) {
	rs, gasUsed, err := e.sandbox.Execute(code)
	gasCost := contract.NewCost(gasUsed, 0, 0)
	return []interface{}{rs}, gasCost, err
}

func (e *VM) setJSPath(path string) {
	e.sandbox.SetJSPath(path, e.vmType)
}

func (e *VM) setReleaseChannel(releaseChannel chan *VM) {
	e.releaseChannel = releaseChannel
}

func (e *VM) recycle() {
	// first release sandbox
	if e.sandbox != nil {
		e.sandbox.Release()
	}
	// then gen new sandbox
	e.sandbox = NewSandbox(e)
	if e.releaseChannel != nil {
		e.releaseChannel <- e
	}
}

// Release release all engine associate resource
func (e *VM) release() {
	// first release sandbox
	if e.sandbox != nil {
		e.sandbox.Release()
	}
	e.sandbox = nil

	// then release isolate
	if e.isolate != nil {
		C.releaseIsolate(e.isolate)
	}
	e.isolate = nil
}
