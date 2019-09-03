// Copyright 2017 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package validate

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/go-interpreter/wagon/wasm"
	"github.com/go-interpreter/wagon/wasm/leb128"
	ops "github.com/go-interpreter/wagon/wasm/operators"
)

// mockVM is a minimal implementation of a virtual machine to
// validate WebAssembly code
type mockVM struct {
	origLength int // the original length of the bytecode stream
	code       *bytes.Reader

	stack      []operand
	ctrlFrames []frame // a stack of encountered blocks

	curFunc *wasm.FunctionSig
}

// a frame represents a structured control instruction & any corresponding
// blocks.
type frame struct {
	pc          int              // the pc of the instruction declaring the frame
	labelTypes  []wasm.ValueType // types signatures of associated labels
	endTypes    []wasm.ValueType // type signatures of frame return values
	stackHeight int              // height of the stack when the frame was started

	op          byte // opcode for the operator starting the new block
	unreachable bool // whether the frame has been marked unreachable
}

func (f *frame) matchingLabelTypes(in *frame) error {
	if len(f.labelTypes) != len(in.labelTypes) {
		return fmt.Errorf("label type len mismatch: %d != %d", len(f.labelTypes), len(in.labelTypes))
	}
	for i := range f.labelTypes {
		if (!operand{f.labelTypes[i]}.Equal(in.labelTypes[i])) {
			return InvalidTypeError{f.labelTypes[i], in.labelTypes[i]}
		}
	}
	return nil
}

func (vm *mockVM) fetchVarUint() (uint32, error) {
	return leb128.ReadVarUint32(vm.code)
}

func (vm *mockVM) fetchVarInt() (int32, error) {
	return leb128.ReadVarint32(vm.code)
}

func (vm *mockVM) fetchByte() (byte, error) {
	return vm.code.ReadByte()
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

func (vm *mockVM) pushFrame(op byte, labelTypes, returnTypes []wasm.ValueType) {
	vm.ctrlFrames = append(vm.ctrlFrames, frame{
		pc:          vm.pc(),
		stackHeight: len(vm.stack),
		labelTypes:  labelTypes,
		endTypes:    returnTypes,
		op:          op,
	})
	logger.Printf("Pushed frame %+v", vm.topFrame())
}

// Get a frame from it's relative nesting depth
func (vm *mockVM) getFrameFromDepth(depth int) *frame {
	if depth >= len(vm.ctrlFrames) {
		return nil
	}

	return &vm.ctrlFrames[len(vm.ctrlFrames)-1-depth]
}

func (vm *mockVM) popFrame() (*frame, error) {
	top := vm.topFrame()
	if top == nil {
		return nil, errors.New("missing frame")
	}

	for i := len(top.endTypes) - 1; i >= 0; i-- {
		ret := top.endTypes[i]
		op, err := vm.popOperand()
		if err != nil {
			return nil, err
		}
		if !op.Equal(ret) {
			return nil, InvalidTypeError{ret, op.Type}
		}
	}
	if len(vm.stack) != top.stackHeight {
		return nil, errors.New("unbalanced stack")
	}
	logger.Printf("Removing frame: %+v", top)
	vm.ctrlFrames = vm.ctrlFrames[:len(vm.ctrlFrames)-1]
	logger.Printf("ctrlFrames = %+v", vm.ctrlFrames)

	return top, nil
}

func (vm *mockVM) topFrame() *frame {
	if len(vm.ctrlFrames) == 0 {
		return nil
	}
	return &vm.ctrlFrames[len(vm.ctrlFrames)-1]
}

func (vm *mockVM) topFrameUnreachable() bool {
	return vm.topFrame().unreachable
}

// popOperand returns details about a potential operand on the stack.
// If there are no values on the stack and the current frame is unreachable,
// an operand of unknown type is returned.
func (vm *mockVM) popOperand() (op operand, err error) {
	logger.Printf("Stack before pop: %+v", vm.stack)
	logger.Printf("Frame before pop: %+v", vm.topFrame())
	if len(vm.stack) == vm.topFrame().stackHeight {
		if vm.topFrameUnreachable() {
			return operand{unknownType}, nil
		}
		return op, ErrStackUnderflow
	}
	nl := len(vm.stack) - 1
	op = vm.stack[nl]
	vm.stack = vm.stack[:nl]

	logger.Printf("Stack after pop is %v. Popped %v", vm.stack, op)
	return op, nil
}

func (vm *mockVM) pushOperand(t wasm.ValueType) {
	o := operand{t}
	// logger.Printf("Stack top: %d, Len of stack :%d", vm.stack[len(vm.stack)-1], len(vm.stack))
	vm.stack = append(vm.stack, o)

	logger.Printf("Stack after push is %v. Pushed %v", vm.stack, o)
}

func (vm *mockVM) adjustStack(op ops.Op) error {
	for _, t := range op.Args {
		op, err := vm.popOperand()
		if err != nil {
			return err
		}
		if !op.Equal(t) {
			return InvalidTypeError{t, op.Type}
		}
	}

	if op.Returns != noReturn {
		vm.pushOperand(op.Returns)
	}
	return nil
}

func (vm *mockVM) setUnreachable() {
	frame := vm.topFrame()
	frame.unreachable = true
	vm.stack = vm.stack[:frame.stackHeight]
}

func (vm *mockVM) pc() int {
	return vm.origLength - vm.code.Len()
}
