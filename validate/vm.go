// Copyright 2017 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package validate

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/go-interpreter/wagon/wasm"
	"github.com/go-interpreter/wagon/wasm/leb128"
	ops "github.com/go-interpreter/wagon/wasm/operators"
)

// mockVM is a minimal implementation of a virtual machine to
// validate WebAssembly code
type mockVM struct {
	stack      []operand
	stackTop   int // the top of the operand stack
	origLength int // the original length of the bytecode stream

	code *bytes.Reader

	blocks []block // a stack of encountered blocks
}

// a block reprsents an instruction sequence preceeded by a control flow operator
// it is used to verify that the block signature set by the operator is the correct
// one when the block ends
type block struct {
	pc        int            // the pc where the control flow operator starting the block is located
	stackTop  int            // stack top when the block started
	blockType wasm.BlockType // block_type signature of the control operator
	op        byte           // opcode for the operator starting the new block
}

func (vm *mockVM) fetchVarUint() (uint32, error) {
	return leb128.ReadVarUint32(vm.code)
}

func (vm *mockVM) fetchVarInt() (int32, error) {
	return leb128.ReadVarint32(vm.code)
}

func (vm *mockVM) fetchVarInt64() (int64, error) {
	return leb128.ReadVarint64(vm.code)
}

func (vm *mockVM) fetchUint32() (uint32, error) {
	var buf [4]byte
	_, err := io.ReadFull(vm.code, buf[:])
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(buf[:]), nil
}

func (vm *mockVM) fetchUint64() (uint64, error) {
	var buf [8]byte
	_, err := io.ReadFull(vm.code, buf[:])
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(buf[:]), nil
}

func (vm *mockVM) pushBlock(op byte, blockType wasm.BlockType) {
	logger.Printf("Pushing block %v", blockType)
	vm.blocks = append(vm.blocks, block{
		pc:        vm.pc(),
		stackTop:  vm.stackTop,
		blockType: blockType,
		op:        op})
}

// Get a block from it's relative nesting depth
func (vm *mockVM) getBlockFromDepth(depth int) *block {
	if depth >= len(vm.blocks) {
		return nil
	}

	return &vm.blocks[len(vm.blocks)-1-depth]
}

// Returns nil if depth is a valid nesting depth value that can be
// branched to
func (vm *mockVM) canBranch(depth int) error {
	block := vm.getBlockFromDepth(depth)
	if block == nil {
		return InvalidLabelError(uint32(depth))
	}
	if block.blockType != wasm.BlockTypeEmpty {
		top, under := vm.topOperand()
		if under || top.Type != wasm.ValueType(block.blockType) {
			return InvalidTypeError{wasm.ValueType(block.blockType), top.Type}
		}
	}

	return nil
}

// returns nil in case of an underflow
func (vm *mockVM) popBlock() *block {
	if len(vm.blocks) == 0 {
		return nil
	}

	stackTop := len(vm.blocks) - 1
	block := vm.blocks[stackTop]
	vm.blocks = append(vm.blocks[:stackTop], vm.blocks[stackTop+1:]...)

	return &block
}

func (vm *mockVM) topBlock() *block {
	if len(vm.blocks) == 0 {
		return nil
	}

	return &vm.blocks[len(vm.blocks)-1]
}

func (vm *mockVM) topOperand() (o operand, under bool) {
	stackTop := vm.stackTop - 1
	if stackTop == -1 {
		under = true
		return
	}
	o = vm.stack[stackTop]
	return
}

func (vm *mockVM) popOperand() (operand, bool) {
	var o operand
	stackTop := vm.stackTop - 1
	if stackTop == -1 {
		return o, true
	}
	o = vm.stack[stackTop]
	vm.stackTop--

	logger.Printf("Stack after pop is %v. Popped %v", vm.stack[:vm.stackTop], o)
	return o, false
}

func (vm *mockVM) pushOperand(t wasm.ValueType) {
	o := operand{t}
	logger.Printf("Stack top: %d, Len of stack :%d", vm.stackTop, len(vm.stack))
	if vm.stackTop == len(vm.stack) {
		vm.stack = append(vm.stack, o)
	} else {
		vm.stack[vm.stackTop] = o
	}
	vm.stackTop++

	logger.Printf("Stack after push is %v. Pushed %v", vm.stack[:vm.stackTop], o)
}

func (vm *mockVM) adjustStack(op ops.Op) error {
	for _, t := range op.Args {
		op, under := vm.popOperand()
		if under || op.Type != t {
			return InvalidTypeError{t, op.Type}
		}
	}

	if op.Returns != wasm.ValueType(wasm.BlockTypeEmpty) {
		vm.pushOperand(op.Returns)
	}

	return nil
}

func (vm *mockVM) pc() int {
	return vm.origLength - vm.code.Len()
}
