// Copyright 2017 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package validate provides functions for validating WebAssembly modules.
package validate

import (
	"bytes"
	"errors"
	"io"

	"github.com/go-interpreter/wagon/wasm"
	"github.com/go-interpreter/wagon/wasm/operators"
	ops "github.com/go-interpreter/wagon/wasm/operators"
)

// vibhavp: TODO: We do not verify whether blocks don't access for the parent block, do that.
func verifyBody(fn *wasm.FunctionSig, body *wasm.FunctionBody, module *wasm.Module) (*mockVM, error) {
	vm := &mockVM{
		stack:      make([]operand, 0, 6),
		code:       bytes.NewReader(body.Code),
		origLength: len(body.Code),

		ctrlFrames: []frame{
			// The outermost frame is the function itself.
			// endTypes is not populated as function return types
			// are validated separately.
			{op: operators.Call},
		},
		curFunc: fn,
	}

	localVariables := []operand{}

	// Parameters count as local variables too
	// This comment explains how local variables work: https://github.com/WebAssembly/design/issues/1037#issuecomment-293505798
	for _, entry := range fn.ParamTypes {
		localVariables = append(localVariables, operand{entry})
	}

	for _, entry := range body.Locals {
		vars := make([]operand, entry.Count)
		for i := uint32(0); i < entry.Count; i++ {
			vars[i].Type = entry.Type
			logger.Printf("Var %v", entry.Type)
		}
		localVariables = append(localVariables, vars...)
	}

	for {
		op, err := vm.code.ReadByte()
		if err == io.EOF {
			break
		} else if err != nil {
			return vm, err
		}

		opStruct, err := ops.New(op)
		if err != nil {
			return vm, err
		}

		logger.Printf("PC: %d OP: %s unreachable: %v", vm.pc(), opStruct.Name, vm.topFrameUnreachable())

		if !opStruct.Polymorphic {
			if err := vm.adjustStack(opStruct); err != nil {
				return vm, err
			}
		}

		switch op {

		case ops.Block, ops.If: // If operand is handled in adjustStack()
			sig, err := vm.fetchByte()
			if err != nil {
				return vm, err
			}

			switch retVal := wasm.ValueType(sig); retVal {
			case wasm.ValueType(wasm.BlockTypeEmpty):
				vm.pushFrame(op, nil, nil)
			case wasm.ValueTypeI32, wasm.ValueTypeI64, wasm.ValueTypeF32, wasm.ValueTypeF64:
				vm.pushFrame(op, []wasm.ValueType{retVal}, []wasm.ValueType{retVal})
			default:
				return vm, InvalidImmediateError{"block_type", opStruct.Name}
			}

		case ops.Loop:
			sig, err := vm.fetchByte()
			if err != nil {
				return vm, err
			}

			switch retVal := wasm.ValueType(sig); retVal {
			case wasm.ValueType(wasm.BlockTypeEmpty):
				vm.pushFrame(op, nil, nil)
			case wasm.ValueTypeI32, wasm.ValueTypeI64, wasm.ValueTypeF32, wasm.ValueTypeF64:
				vm.pushFrame(op, nil, []wasm.ValueType{retVal})
			default:
				return vm, InvalidImmediateError{"block_type", opStruct.Name}
			}

		case ops.Else:
			frame, err := vm.popFrame()
			if err != nil {
				return vm, err
			}
			if frame == nil || frame.op != ops.If {
				return vm, UnmatchedOpError(op)
			}
			vm.pushFrame(op, frame.endTypes, frame.endTypes)

		case ops.End:
			// Block 'return' type is validated in popFrame().
			frame, err := vm.popFrame()
			if err != nil {
				return vm, err
			}
			switch {
			// END should match with a IF/BLOCK/LOOP frame.
			case frame == nil || frame.op == operators.Call:
				return vm, UnmatchedOpError(op)
			// IF block with no else cannot have a result.
			case frame.op == operators.If && len(frame.endTypes) > 0:
				return vm, UnmatchedIfValueErr(frame.endTypes[0])
			}
			for _, t := range frame.endTypes {
				vm.pushOperand(t)
			}

		case ops.Br:
			depth, err := vm.fetchVarUint()
			if err != nil {
				return vm, err
			}
			if int(depth) >= len(vm.ctrlFrames) {
				return vm, InvalidLabelError(depth)
			}
			frame := vm.ctrlFrames[len(vm.ctrlFrames)-1-int(depth)]
			logger.Printf("Branch is targeting frame: %+v which is depth %d", frame, depth)
			for i := len(frame.labelTypes) - 1; i >= 0; i-- {
				t := frame.labelTypes[i]
				op, err := vm.popOperand()
				if err != nil {
					return vm, err
				}
				if !op.Equal(t) {
					return vm, InvalidTypeError{t, op.Type}
				}
			}
			vm.setUnreachable()

		case ops.BrIf:
			depth, err := vm.fetchVarUint()
			if err != nil {
				return vm, err
			}
			if int(depth) >= len(vm.ctrlFrames) {
				return vm, InvalidLabelError(depth)
			}
			frame := vm.ctrlFrames[len(vm.ctrlFrames)-1-int(depth)]
			for i := len(frame.labelTypes) - 1; i >= 0; i-- {
				t := frame.labelTypes[i]
				op, err := vm.popOperand()
				if err != nil {
					return vm, err
				}
				if !op.Equal(t) {
					return vm, InvalidTypeError{t, op.Type}
				}
			}
			for _, t := range frame.labelTypes {
				vm.pushOperand(t)
			}

		case ops.BrTable:
			// Read & typecheck branching operand from stack.
			op, err := vm.popOperand()
			if err != nil {
				return vm, err
			}
			if !op.Equal(wasm.ValueTypeI32) {
				return vm, InvalidTypeError{wasm.ValueTypeI32, op.Type}
			}

			// Read each branch table item, and validate they refer to an existant
			// frame.
			targetCount, err := vm.fetchVarUint()
			if err != nil {
				return vm, err
			}
			targets := make([]uint32, int(targetCount))
			for i := uint32(0); i < targetCount; i++ {
				targetDepth, err := vm.fetchVarUint()
				if err != nil {
					return vm, err
				}
				if int(targetDepth) >= len(vm.ctrlFrames) {
					return vm, InvalidLabelError(targetDepth)
				}
				targets[i] = targetDepth
			}

			// Read the default branch target, and validate it refers to an existant
			// frame.
			defaultTarget, err := vm.fetchVarUint()
			if err != nil {
				return vm, err
			}
			if int(defaultTarget) >= len(vm.ctrlFrames) {
				return vm, InvalidLabelError(defaultTarget)
			}

			// Validate the type signature of each branch matches that of
			// the default branch.
			defaultBranch := vm.getFrameFromDepth(int(defaultTarget))
			for _, target := range targets {
				frame := vm.getFrameFromDepth(int(target))
				if err := defaultBranch.matchingLabelTypes(frame); err != nil {
					return vm, err
				}
			}

			// Pop operands based on the type signature of the default branch.
			for i := len(defaultBranch.labelTypes) - 1; i >= 0; i-- {
				t := defaultBranch.labelTypes[i]
				op, err := vm.popOperand()
				if err != nil {
					return vm, err
				}
				if !op.Equal(t) {
					return vm, InvalidTypeError{t, op.Type}
				}
			}
			vm.setUnreachable()

		case ops.Return:
			if len(fn.ReturnTypes) > 1 {
				panic("not implemented")
			}
			if len(fn.ReturnTypes) != 0 {
				// only single returns supported for now
				op, err := vm.popOperand()
				if err != nil {
					return vm, err
				}
				if !op.Equal(fn.ReturnTypes[0]) {
					return vm, InvalidTypeError{fn.ReturnTypes[0], op.Type}
				}
			}
			vm.setUnreachable()

		case ops.Unreachable:
			vm.setUnreachable()

		case ops.I32Const:
			_, err := vm.fetchVarInt()
			if err != nil {
				return vm, err
			}

		case ops.I64Const:
			_, err := vm.fetchVarInt64()
			if err != nil {
				return vm, err
			}

		case ops.F32Const:
			_, err := vm.fetchUint32()
			if err != nil {
				return vm, err
			}

		case ops.F64Const:
			_, err := vm.fetchUint64()
			if err != nil {
				return vm, err
			}

		case ops.GetLocal, ops.SetLocal, ops.TeeLocal:
			i, err := vm.fetchVarUint()
			if err != nil {
				return vm, err
			}
			if int(i) >= len(localVariables) {
				return vm, InvalidLocalIndexError(i)
			}

			v := localVariables[i]

			if op == ops.GetLocal {
				vm.pushOperand(v.Type)
			} else { // == set_local or tee_local
				operand, err := vm.popOperand()
				if err != nil {
					return vm, err
				}
				if !operand.Equal(v.Type) {
					return vm, InvalidTypeError{v.Type, operand.Type}
				}
				if op == ops.TeeLocal {
					vm.pushOperand(v.Type)
				}
			}

		case ops.GetGlobal, ops.SetGlobal:
			index, err := vm.fetchVarUint()
			if err != nil {
				return vm, err
			}

			gv := module.GetGlobal(int(index))
			if gv == nil {
				return vm, wasm.InvalidGlobalIndexError(index)
			}
			if op == ops.GetGlobal {
				vm.pushOperand(gv.Type.Type)
			} else {
				op, err := vm.popOperand()
				if err != nil {
					return vm, err
				}
				if !op.Equal(gv.Type.Type) {
					return vm, InvalidTypeError{gv.Type.Type, op.Type}
				}
			}

		case ops.I32Load, ops.I64Load, ops.F32Load, ops.F64Load, ops.I32Load8s, ops.I32Load8u, ops.I32Load16s, ops.I32Load16u, ops.I64Load8s, ops.I64Load8u, ops.I64Load16s, ops.I64Load16u, ops.I64Load32s, ops.I64Load32u, ops.I32Store, ops.I64Store, ops.F32Store, ops.F64Store, ops.I32Store8, ops.I32Store16, ops.I64Store8, ops.I64Store16, ops.I64Store32:
			// read memory_immediate
			// flags
			align, err := vm.fetchVarUint()
			if err != nil {
				return vm, err
			}
			// offset
			_, err = vm.fetchVarUint()
			if err != nil {
				return vm, err
			}

			switch op {
			case ops.I32Load8s, ops.I32Load8u, ops.I64Load8s, ops.I64Load8u:
				if align > 1 {
					return vm, InvalidImmediateError{OpName: opStruct.Name, ImmType: "naturally aligned"}
				}
			case ops.I32Load16s, ops.I32Load16u, ops.I64Load16s, ops.I64Load16u:
				if align > 2 {
					return vm, InvalidImmediateError{OpName: opStruct.Name, ImmType: "naturally aligned"}
				}
			case ops.I32Load, ops.I64Load32s, ops.I64Load32u, ops.F32Load:
				if align > 4 {
					return vm, InvalidImmediateError{OpName: opStruct.Name, ImmType: "naturally aligned"}
				}
			case ops.I64Load, ops.F64Load:
				if align > 8 {
					return vm, InvalidImmediateError{OpName: opStruct.Name, ImmType: "naturally aligned"}
				}
			}

		case ops.CurrentMemory, ops.GrowMemory:
			memIndex, err := vm.fetchByte()
			if err != nil {
				return vm, err
			}
			if memIndex != 0x00 {
				return vm, InvalidTableIndexError{"memory", uint32(memIndex)}
			}

		case ops.Call:
			index, err := vm.fetchVarUint()
			if err != nil {
				return vm, err
			}

			fn := module.GetFunction(int(index))
			if fn == nil {
				return vm, wasm.InvalidFunctionIndexError(index)
			}

			logger.Printf("Function being called: %v", fn)
			for index := range fn.Sig.ParamTypes {
				argType := fn.Sig.ParamTypes[len(fn.Sig.ParamTypes)-index-1]
				op, err := vm.popOperand()
				if err != nil {
					return vm, err
				}
				if !op.Equal(argType) {
					return vm, InvalidTypeError{argType, op.Type}
				}
			}

			for _, t := range fn.Sig.ReturnTypes {
				vm.pushOperand(t)
			}

		case ops.CallIndirect:
			if module.Table == nil || len(module.Table.Entries) == 0 {
				return vm, NoSectionError(wasm.SectionIDTable)
			}
			// The call_indirect process consists of getting two i32 values
			// off (first from the bytecode stream, and the second from
			//  the stack) and using first as an index into the "Types" section
			// of the module, while the the second one into the function index
			// space. The signature of the two elements are then compared
			// to see if they match, and the call proceeds as normal if they
			// do.
			// Since this is possible only during program execution, we only
			// perform the static check for the function index mentioned
			// in the bytecode stream here.

			// type index
			index, err := vm.fetchVarUint()
			if err != nil {
				return vm, err
			}
			tableIndex, err := vm.fetchByte()
			if err != nil {
				return vm, err
			}
			if tableIndex != 0x00 {
				return vm, InvalidTableIndexError{"table", uint32(tableIndex)}
			}

			if index >= uint32(len(module.Types.Entries)) {
				return vm, errors.New("validate: type index out of range in call_indirect")
			}

			fnExpectSig := module.Types.Entries[index]

			op, err := vm.popOperand()
			if err != nil {
				return vm, err
			}
			if !op.Equal(wasm.ValueTypeI32) {
				return vm, InvalidTypeError{wasm.ValueTypeI32, op.Type}
			}

			for index := range fnExpectSig.ParamTypes {
				argType := fnExpectSig.ParamTypes[len(fnExpectSig.ParamTypes)-index-1]
				op, err := vm.popOperand()
				if err != nil {
					return vm, err
				}
				if !op.Equal(argType) {
					return vm, InvalidTypeError{argType, op.Type}
				}
			}
			for _, t := range fnExpectSig.ReturnTypes {
				vm.pushOperand(t)
			}

		case ops.Drop:
			if _, err := vm.popOperand(); err != nil {
				return vm, err
			}

		case ops.Select:
			op, err := vm.popOperand()
			if err != nil {
				return vm, err
			}
			if !op.Equal(wasm.ValueTypeI32) {
				return vm, InvalidTypeError{wasm.ValueTypeI32, op.Type}
			}

			operands := make([]operand, 2)
			for i := 0; i < 2; i++ {
				operand, err := vm.popOperand()
				if err != nil {
					return vm, err
				}
				operands[i] = operand
			}

			// last 2 popped values should be of the same type
			if !operands[0].Equal(operands[1].Type) {
				return vm, InvalidTypeError{operands[1].Type, operands[2].Type}
			}

			vm.pushOperand(operands[1].Type)
		}
	}

	switch len(fn.ReturnTypes) {
	case 0:
		op, err := vm.popOperand()
		switch {
		case !vm.topFrameUnreachable() && err == nil:
			return vm, UnbalancedStackErr(op.Type)
		case err == ErrStackUnderflow:
		default:
			return vm, err
		}
	case 1:
		op, err := vm.popOperand()
		if err != nil {
			return vm, err
		}
		if !op.Equal(fn.ReturnTypes[0]) {
			return vm, InvalidTypeError{fn.ReturnTypes[0], op.Type}
		}
	}

	f, err := vm.popFrame()
	if err != nil {
		return vm, err
	}
	if f.op != operators.Call {
		return vm, UnmatchedOpError(f.op)
	}

	return vm, nil
}

// VerifyModule verifies the given module according to WebAssembly verification
// specs.
func VerifyModule(module *wasm.Module) error {
	if module.Function == nil || module.Types == nil || len(module.Types.Entries) == 0 {
		return nil
	}
	if module.Code == nil {
		return NoSectionError(wasm.SectionIDCode)
	}

	logger.Printf("There are %d functions", len(module.Function.Types))
	for i, fn := range module.FunctionIndexSpace {
		logger.Printf("Validating function: %q", fn.Name)
		if vm, err := verifyBody(fn.Sig, fn.Body, module); err != nil {
			return Error{vm.pc(), i, err}
		}
		logger.Printf("No errors in function %d (%q)", i, fn.Name)
	}

	return nil
}
