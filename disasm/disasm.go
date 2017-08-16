// Copyright 2017 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package disasm provides functions for disassembling WebAssembly bytecode.
package disasm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"

	"github.com/go-interpreter/wagon/internal/stack"
	"github.com/go-interpreter/wagon/wasm"
	"github.com/go-interpreter/wagon/wasm/leb128"
	ops "github.com/go-interpreter/wagon/wasm/operators"
)

// Instr describes an instruction, consisting of an operator, with its
// appropriate immediate value(s).
type Instr struct {
	Op ops.Op

	// Immediates are arguments to an operator in the bytecode stream itself.
	// Valid value types are:
	// - (u)(int/float)(32/64)
	// - wasm.BlockType
	Immediates []interface{}
	NewStack   *StackInfo // non-nil if the instruction requires the current stack to be unwound.
	Block      *BlockInfo // non-nil if the instruction starts a new block.
}

// StackInfo stores details about a new stack ended by an instruction.
type StackInfo struct {
	StackTopDiff int64 // The difference between the stack depths at the end of the block
	PreserveTop  bool  // Whether the value on the top of the stack should be preserved while unwinding
}

// BlockInfo stores details about a block created or ended by an instruction.
type BlockInfo struct {
	Start     bool           // If true, this instruction starts a block. Else this instruction ends it.
	Signature wasm.BlockType // The block signature
	// The index to the accompanying control operator.
	// For 'if', this is an index to the 'else' operator
	// For else/loop/block, the index is to the 'end' operator
	PairIndex int
}

// Disassembly is the result of disassembling a WebAssembly function.
type Disassembly struct {
	Code     []Instr
	MaxDepth int // The maximum stack depth that can be reached while executing this function
}

func (d *Disassembly) checkMaxDepth(depth int) {
	if depth > d.MaxDepth {
		d.MaxDepth = depth
	}
}

var ErrStackUnderflow = errors.New("disasm: stack underflow")

// Disassemble disassembles the given function. It also takes the function's
// parent module as an argument for locating any other functions referenced by
// fn.
func Disassemble(fn wasm.Function, module *wasm.Module) (*Disassembly, error) {
	code := fn.Body.Code
	reader := bytes.NewReader(code)
	disas := &Disassembly{}

	// a stack of current execution stack depth values, so that the depth for each
	// stack is maintained indepepdently for calculating discard values
	stackDepths := &stack.Stack{}
	stackDepths.Push(0)
	blockIndices := &stack.Stack{} // a stack of indices to operators which start new blocks
	curIndex := 0
	var lastOpReturn bool

	for {
		op, err := reader.ReadByte()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		logger.Printf("stack top is %d", stackDepths.Top())

		opStr, err := ops.New(op)
		if err != nil {
			return nil, err
		}
		instr := Instr{
			Op:         opStr,
			Immediates: [](interface{}){},
		}

		logger.Printf("Name is %s", opStr.Name)
		if !opStr.Polymorphic {
			top := int(stackDepths.Top())
			stackDepths.SetTop(uint64(top - len(opStr.Args)))
			if top < -1 {
				return nil, ErrStackUnderflow
			}
			if opStr.Returns != wasm.ValueType(wasm.BlockTypeEmpty) {
				stackDepths.SetTop(uint64(top) + 1)
			}
			if top > disas.MaxDepth {
				disas.MaxDepth = top
			}
		}

		switch op {
		case ops.Drop:
			stackDepths.SetTop(stackDepths.Top() - 1)
		case ops.Select:
			stackDepths.SetTop(stackDepths.Top() - 3)
		case ops.Return:
			stackDepths.SetTop(stackDepths.Top() - uint64(len(fn.Sig.ReturnTypes)))
			lastOpReturn = true
		case ops.End, ops.Else:
			// The max depth reached while execing the current block
			curDepth := stackDepths.Top()
			blockStartIndex := blockIndices.Pop()
			blockSig := disas.Code[blockStartIndex].Block.Signature
			instr.Block = &BlockInfo{
				Start:     false,
				Signature: blockSig,
				PairIndex: int(blockStartIndex),
			}

			// The max depth reached while execing the last block
			// If the signature of the current block is not empty,
			// this will be incremented
			prevDepthIndex := stackDepths.Len() - 2
			prevDepth := stackDepths.Get(prevDepthIndex)

			if blockSig != wasm.BlockTypeEmpty {
				stackDepths.Set(prevDepthIndex, prevDepth+1)
				disas.checkMaxDepth(int(stackDepths.Get(prevDepthIndex)))
			}

			if !lastOpReturn {
				elemsDiscard := int(curDepth) - int(prevDepth)
				if elemsDiscard < -1 {
					return nil, ErrStackUnderflow
				}
				instr.NewStack = &StackInfo{
					StackTopDiff: int64(elemsDiscard),
					PreserveTop:  blockSig != wasm.BlockTypeEmpty,
				}
			} else {
				instr.NewStack = &StackInfo{}
			}

			stackDepths.Pop()
			if op == ops.Else {
				stackDepths.Push(0)
				blockIndices.Push(uint64(curIndex))
			}
		case ops.Block, ops.Loop, ops.If:
			sig, err := leb128.ReadVarint32(reader)
			if err != nil {
				return nil, err
			}
			logger.Printf("if, depth is %d", stackDepths.Top())
			stackDepths.Push(stackDepths.Top())
			instr.Block = &BlockInfo{
				Start:     true,
				Signature: wasm.BlockType(sig),
			}

			blockIndices.Push(uint64(curIndex))
			instr.Immediates = append(instr.Immediates, wasm.BlockType(sig))
		case ops.Br, ops.BrIf:
			depth, err := leb128.ReadVarUint32(reader)
			if err != nil {
				return nil, err
			}
			instr.Immediates = append(instr.Immediates, depth)

			curDepth := stackDepths.Top()
			elemsDiscard := int64(int(curDepth) - int(stackDepths.Get(int(depth))))
			if elemsDiscard < 0 {
				return nil, ErrStackUnderflow
			}

			// we use getBlockDepth because a getBlockIndex would
			// have the same code
			index := blockIndices.Get(int(depth))
			instr.NewStack = &StackInfo{
				StackTopDiff: elemsDiscard,
				PreserveTop:  disas.Code[index].Block.Signature != wasm.BlockTypeEmpty,
			}

		case ops.BrTable:
			targetCount, err := leb128.ReadVarUint32(reader)
			if err != nil {
				return nil, err
			}
			instr.Immediates = append(instr.Immediates, targetCount)
			for i := uint32(0); i < targetCount; i++ {
				entry, err := leb128.ReadVarUint32(reader)
				if err != nil {
					return nil, err
				}
				instr.Immediates = append(instr.Immediates, entry)
			}

			defaultTarget, err := leb128.ReadVarUint32(reader)
			if err != nil {
				return nil, err
			}
			instr.Immediates = append(instr.Immediates, defaultTarget)
			stackDepths.SetTop(stackDepths.Top() - 1)
		case ops.Call, ops.CallIndirect:
			index, err := leb128.ReadVarUint32(reader)
			if err != nil {
				return nil, err
			}
			instr.Immediates = append(instr.Immediates, index)
			if op == ops.CallIndirect {
				reserved, err := leb128.ReadVarUint32(reader)
				if err != nil {
					return nil, err
				}
				instr.Immediates = append(instr.Immediates, reserved)
			}
			fn := module.GetFunction(int(index))
			top := int(stackDepths.Top())
			top -= len(fn.Sig.ParamTypes)
			top += len(fn.Sig.ReturnTypes)
			stackDepths.SetTop(uint64(top))
			disas.checkMaxDepth(top)
		case ops.GetLocal, ops.SetLocal, ops.TeeLocal, ops.GetGlobal, ops.SetGlobal:
			index, err := leb128.ReadVarUint32(reader)
			if err != nil {
				return nil, err
			}
			instr.Immediates = append(instr.Immediates, index)

			top := stackDepths.Top()
			switch op {
			case ops.GetLocal, ops.GetGlobal:
				stackDepths.SetTop(top + 1)
				disas.checkMaxDepth(int(stackDepths.Top()))
			case ops.SetLocal, ops.SetGlobal:
				stackDepths.SetTop(top - 1)
			case ops.TeeLocal:
				// stack remains unchanged for tee_local
			}
		case ops.I32Const:
			i, err := leb128.ReadVarint32(reader)
			if err != nil {
				return nil, err
			}
			instr.Immediates = append(instr.Immediates, i)
		case ops.I64Const:
			i, err := leb128.ReadVarint64(reader)
			if err != nil {
				return nil, err
			}
			instr.Immediates = append(instr.Immediates, i)
		case ops.F32Const:
			var i uint32
			// TODO(vibhavp): Switch to a reflect-free method in the future
			// for reading off the bytestream.
			err := binary.Read(reader, binary.LittleEndian, &i)
			if err != nil {
				return nil, err
			}
			instr.Immediates = append(instr.Immediates, math.Float32frombits(i))
		case ops.F64Const:
			var i uint64
			// TODO(vibhavp): Switch to a reflect-free method in the future
			// for reading off the bytestream.
			err := binary.Read(reader, binary.LittleEndian, &i)
			if err != nil {
				return nil, err
			}
			instr.Immediates = append(instr.Immediates, math.Float64frombits(i))
		case ops.I32Load, ops.I64Load, ops.F32Load, ops.F64Load, ops.I32Load8s, ops.I32Load8u, ops.I32Load16s, ops.I32Load16u, ops.I64Load8s, ops.I64Load8u, ops.I64Load16s, ops.I64Load16u, ops.I64Load32s, ops.I64Load32u, ops.I32Store, ops.I64Store, ops.F32Store, ops.F64Store, ops.I32Store8, ops.I32Store16, ops.I64Store8, ops.I64Store16, ops.I64Store32:
			// read memory_immediate
			flags, err := leb128.ReadVarUint32(reader)
			if err != nil {
				return nil, err
			}
			instr.Immediates = append(instr.Immediates, flags)

			offset, err := leb128.ReadVarUint32(reader)
			if err != nil {
				return nil, err
			}
			instr.Immediates = append(instr.Immediates, offset)
		case ops.CurrentMemory, ops.GrowMemory:
			res, err := leb128.ReadVarUint32(reader)
			if err != nil {
				return nil, err
			}
			instr.Immediates = append(instr.Immediates, res)

			curDepth := stackDepths.Top()
			switch op {
			case ops.CurrentMemory:
				stackDepths.SetTop(curDepth + 1)
				disas.checkMaxDepth(int(stackDepths.Top()))
			case ops.GrowMemory:
				stackDepths.SetTop(curDepth - 1)
			}
		}

		if op != ops.Return {
			lastOpReturn = false
		}

		disas.Code = append(disas.Code, instr)
		curIndex++
	}

	return disas, nil
}
