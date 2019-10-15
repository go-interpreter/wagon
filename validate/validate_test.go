// Copyright 2019 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package validate

import (
	"testing"

	"github.com/go-interpreter/wagon/wasm"
	"github.com/go-interpreter/wagon/wasm/operators"
)

func TestValidateAlignment(t *testing.T) {
	tcs := []struct {
		name string
		code []byte
		err  error
	}{
		{
			name: "i32.load8s alignment",
			code: []byte{
				// (i32.load8_s align=2 (i32.const 0))
				operators.I32Const, 0,
				operators.I32Load8s, 2, 0,
			},
			err: InvalidImmediateError{OpName: "i32.load8_s", ImmType: "naturally aligned"},
		},
		{
			name: "i32.load8u alignment",
			code: []byte{
				// (i32.load8_u align=2 (i32.const 0))
				operators.I32Const, 0,
				operators.I32Load8u, 2, 0,
			},
			err: InvalidImmediateError{OpName: "i32.load8_u", ImmType: "naturally aligned"},
		},
		{
			name: "i32.load16s alignment",
			code: []byte{
				// (i32.load16_s align=4 (i32.const 0))
				operators.I32Const, 0,
				operators.I32Load16s, 4, 0,
			},
			err: InvalidImmediateError{OpName: "i32.load16_s", ImmType: "naturally aligned"},
		},
		{
			name: "i32.load16u alignment",
			code: []byte{
				// (i32.load16_u align=4 (i32.const 0))
				operators.I32Const, 0,
				operators.I32Load16u, 4, 0,
			},
			err: InvalidImmediateError{OpName: "i32.load16_u", ImmType: "naturally aligned"},
		},
		{
			name: "i32.load alignment",
			code: []byte{
				// (i32.load align=8 (i32.const 0))
				operators.I32Const, 0,
				operators.I32Load, 8, 0,
			},
			err: InvalidImmediateError{OpName: "i32.load", ImmType: "naturally aligned"},
		},
		{
			name: "i64.load8s alignment",
			code: []byte{
				// (i64.load8_s align=2 (i32.const 0))
				operators.I32Const, 0,
				operators.I64Load8s, 2, 0,
			},
			err: InvalidImmediateError{OpName: "i64.load8_s", ImmType: "naturally aligned"},
		},
		{
			name: "i64.load8u alignment",
			code: []byte{
				// (i64.load8_u align=2 (i32.const 0))
				operators.I32Const, 0,
				operators.I64Load8u, 2, 0,
			},
			err: InvalidImmediateError{OpName: "i64.load8_u", ImmType: "naturally aligned"},
		},
		{
			name: "i64.load16s alignment",
			code: []byte{
				// (i64.load16_s align=4 (i32.const 0))
				operators.I32Const, 0,
				operators.I64Load16s, 4, 0,
			},
			err: InvalidImmediateError{OpName: "i64.load16_s", ImmType: "naturally aligned"},
		},
		{
			name: "i64.load16u alignment",
			code: []byte{
				// (i64.load16_u align=4 (i32.const 0))
				operators.I32Const, 0,
				operators.I64Load16u, 4, 0,
			},
			err: InvalidImmediateError{OpName: "i64.load16_u", ImmType: "naturally aligned"},
		},
		{
			name: "i64.load32s alignment",
			code: []byte{
				// (i64.load32_s align=8 (i32.const 0))
				operators.I32Const, 0,
				operators.I64Load32s, 8, 0,
			},
			err: InvalidImmediateError{OpName: "i64.load32_s", ImmType: "naturally aligned"},
		},
		{
			name: "i64.load32u alignment",
			code: []byte{
				// (i64.load32_u align=8 (i32.const 0))
				operators.I32Const, 0,
				operators.I64Load32u, 8, 0,
			},
			err: InvalidImmediateError{OpName: "i64.load32_u", ImmType: "naturally aligned"},
		},
		{
			name: "i64.load alignment",
			code: []byte{
				// (i64.load align=16 (i32.const 0))
				operators.I32Const, 0,
				operators.I64Load, 16, 0,
			},
			err: InvalidImmediateError{OpName: "i64.load", ImmType: "naturally aligned"},
		},
		{
			name: "f32.load alignment",
			code: []byte{
				// (f32.load align=8 (i32.const 0))
				operators.I32Const, 0,
				operators.F32Load, 8, 0,
			},
			err: InvalidImmediateError{OpName: "f32.load", ImmType: "naturally aligned"},
		},
		{
			name: "f64.load alignment",
			code: []byte{
				// (f64.load align=16 (i32.const 0))
				operators.I32Const, 0,
				operators.F64Load, 16, 0,
			},
			err: InvalidImmediateError{OpName: "f64.load", ImmType: "naturally aligned"},
		},
	}

	for i := range tcs {
		tc := tcs[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mod := wasm.Module{}
			sig := wasm.FunctionSig{Form: 0x60 /* Must always be 0x60 */}
			fn := wasm.FunctionBody{Module: &mod, Code: tc.code}

			_, err := verifyBody(&sig, &fn, &mod)
			if err != tc.err {
				t.Fatalf("verify returned '%v', want '%v'", err, tc.err)
			}
		})
	}
}

func TestValidateMemory(t *testing.T) {
	tcs := []struct {
		name string
		code []byte
		err  error
	}{
		{
			name: "memory.grow",
			code: []byte{
				operators.I32Const, 3,
				operators.GrowMemory, 0,
				operators.Drop,
			},
			err: nil,
		},
		{
			name: "memory.grow invalid index",
			code: []byte{
				operators.I32Const, 1,
				operators.GrowMemory, 1,
				operators.Drop,
			},
			err: InvalidTableIndexError{"memory", 1},
		},
		{
			name: "memory.size",
			code: []byte{
				operators.CurrentMemory, 0,
				operators.Drop,
			},
			err: nil,
		},
		{
			name: "memory.size invalid index",
			code: []byte{
				operators.I32Const, 1,
				operators.CurrentMemory, 1,
				operators.Drop,
			},
			err: InvalidTableIndexError{"memory", 1},
		},
	}

	for i := range tcs {
		tc := tcs[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mod := wasm.Module{}
			sig := wasm.FunctionSig{Form: 0x60 /* Must always be 0x60 */}
			fn := wasm.FunctionBody{Module: &mod, Code: tc.code}

			_, err := verifyBody(&sig, &fn, &mod)
			if err != tc.err {
				t.Fatalf("verify returned '%v', want '%v'", err, tc.err)
			}
		})
	}
}

func TestValidateFuncTypecheck(t *testing.T) {
	tcs := []struct {
		name     string
		code     []byte
		fnReturn wasm.ValueType
		err      error
	}{
		{
			name:     "voidfunc-i32",
			fnReturn: noReturn,
			code: []byte{
				operators.I32Const, 0,
			},
			err: InvalidTypeError{noReturn, wasm.ValueTypeI32},
		},
		{
			name:     "i32func-void",
			fnReturn: wasm.ValueTypeI32,
			code: []byte{
				operators.Nop,
			},
			err: ErrStackUnderflow,
		},
		{
			name:     "i32func-i32",
			fnReturn: wasm.ValueTypeI32,
			code: []byte{
				operators.I32Const, 0,
			},
		},
		{
			name:     "voidfunc-i64",
			fnReturn: noReturn,
			code: []byte{
				operators.I64Const, 0,
			},
			err: InvalidTypeError{noReturn, wasm.ValueTypeI64},
		},
		{
			name:     "i64func-i64",
			fnReturn: wasm.ValueTypeI64,
			code: []byte{
				operators.I64Const, 0,
			},
		},
		{
			name:     "i64func-void",
			fnReturn: wasm.ValueTypeI64,
			code: []byte{
				operators.Nop,
			},
			err: ErrStackUnderflow,
		},
		{
			name:     "voidfunc-f32",
			fnReturn: noReturn,
			code: []byte{
				operators.F32Const, 0, 0, 0, 0,
			},
			err: InvalidTypeError{noReturn, wasm.ValueTypeF32},
		},
		{
			name:     "f32func-f32",
			fnReturn: wasm.ValueTypeF32,
			code: []byte{
				operators.F32Const, 0, 0, 0, 0,
			},
		},
		{
			name:     "f32func-void",
			fnReturn: wasm.ValueTypeF32,
			code: []byte{
				operators.Nop,
			},
			err: ErrStackUnderflow,
		},
		{
			name:     "voidfunc-f32",
			fnReturn: noReturn,
			code: []byte{
				operators.F32Const, 0, 0, 0, 0,
			},
			err: InvalidTypeError{noReturn, wasm.ValueTypeF32},
		},
		{
			name:     "f64func-f64",
			fnReturn: wasm.ValueTypeF64,
			code: []byte{
				operators.F64Const, 0, 0, 0, 0, 0, 0, 0, 0,
			},
		},
		{
			name:     "f64func-void",
			fnReturn: wasm.ValueTypeF64,
			code: []byte{
				operators.Nop,
			},
			err: ErrStackUnderflow,
		},
		{
			name:     "resolve unreachable",
			fnReturn: wasm.ValueTypeI64,
			// (block (result i32) (select (unreachable) (unreachable) (unreachable)))
			code: []byte{
				operators.Block, byte(wasm.ValueTypeI32),
				operators.Unreachable, operators.Unreachable, operators.Unreachable, operators.Select,
				operators.End,
			},
			err: InvalidTypeError{wasm.ValueTypeI64, wasm.ValueTypeI32},
		},
		{
			name:     "return i32func-i64",
			fnReturn: wasm.ValueTypeI32,
			// (i64.const 0) (return) (i32.const 0)
			code: []byte{
				operators.I64Const, 0,
				operators.Return,
				operators.I32Const, 0,
			},
			err: InvalidTypeError{wasm.ValueTypeI32, wasm.ValueTypeI64},
		},
		{
			name:     "return i64func-i64",
			fnReturn: wasm.ValueTypeI64,
			// (i64.const 0) (return) (i64.const 0)
			code: []byte{
				operators.I64Const, 0,
				operators.Return,
				operators.I64Const, 0,
			},
			err: nil,
		},
		{
			name:     "local funci32-i32",
			fnReturn: wasm.ValueTypeI32,
			// (getLocal 0)
			code: []byte{
				operators.GetLocal, 0,
			},
			err: nil,
		},
		{
			name:     "local funci64-i32",
			fnReturn: wasm.ValueTypeI64,
			// (getLocal 0) (return) (i64.const 0) (drop)
			code: []byte{
				operators.GetLocal, 0,
				operators.Return,
				operators.I64Const, 0,
				operators.Drop,
			},
			err: InvalidTypeError{wasm.ValueTypeI64, wasm.ValueTypeI32},
		},
	}

	for i := range tcs {
		tc := tcs[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mod := wasm.Module{}
			sig := wasm.FunctionSig{Form: 0x60 /* Must always be 0x60 */, ReturnTypes: []wasm.ValueType{tc.fnReturn}}
			fn := wasm.FunctionBody{
				Module: &mod,
				Code:   tc.code,
				Locals: []wasm.LocalEntry{
					{Count: 1, Type: wasm.ValueTypeI32},
				},
			}

			_, err := verifyBody(&sig, &fn, &mod)
			if err != tc.err {
				t.Fatalf("verify returned '%v', want '%v'", err, tc.err)
			}
		})
	}
}

func TestValidateLocalsGlobals(t *testing.T) {
	tcs := []struct {
		name string
		code []byte
		err  error
	}{
		{
			name: "get_local",
			code: []byte{
				operators.GetLocal, 0,
				operators.Drop,
			},
			err: nil,
		},
		{
			name: "get_local invalid index",
			code: []byte{
				operators.GetLocal, 100,
				operators.Drop,
			},
			err: InvalidLocalIndexError(100),
		},
		{
			name: "get_local overflow",
			code: []byte{
				operators.GetLocal, 0,
			},
			err: UnbalancedStackErr(wasm.ValueTypeI32),
		},
		{
			name: "get_local type mismatch",
			code: []byte{
				operators.I32Const, 1,
				operators.GetLocal, 1,
				operators.I32Add,
			},
			err: InvalidTypeError{wasm.ValueTypeI32, wasm.ValueTypeI64},
		},
		{
			name: "set_local",
			code: []byte{
				operators.I32Const, 12,
				operators.SetLocal, 0,
			},
			err: nil,
		},
		{
			name: "set_local underflow",
			code: []byte{
				operators.SetLocal, 0,
			},
			err: ErrStackUnderflow,
		},
		{
			name: "set_local type mismatch",
			code: []byte{
				operators.I32Const, 1,
				operators.SetLocal, 1,
			},
			err: InvalidTypeError{wasm.ValueTypeI64, wasm.ValueTypeI32},
		},
		{
			name: "tee_local",
			code: []byte{
				operators.I64Const, 1,
				operators.TeeLocal, 1,
				operators.Drop,
			},
			err: nil,
		},
		{
			name: "get_global",
			code: []byte{
				operators.GetGlobal, 0,
				operators.Drop,
			},
			err: nil,
		},
		{
			name: "get_global overflow",
			code: []byte{
				operators.GetGlobal, 2,
			},
			err: UnbalancedStackErr(wasm.ValueTypeF64),
		},
		{
			name: "get_global type mismatch",
			code: []byte{
				operators.GetGlobal, 0,
				operators.I64Const, 1,
				operators.I64Add,
				operators.Drop,
			},
			err: InvalidTypeError{wasm.ValueTypeI64, wasm.ValueTypeI32},
		},
		{
			name: "get_global invalid index",
			code: []byte{
				operators.GetGlobal, 100,
				operators.Drop,
			},
			err: wasm.InvalidGlobalIndexError(100),
		},
		{
			name: "set_global",
			code: []byte{
				operators.I64Const, 42,
				operators.SetGlobal, 1,
			},
			err: nil,
		},
		{
			name: "set_global underflow",
			code: []byte{
				operators.SetGlobal, 1,
			},
			err: ErrStackUnderflow,
		},
		{
			name: "set_global type mismatch",
			code: []byte{
				operators.F32Const, 0, 0, 0, 0,
				operators.SetGlobal, 1,
			},
			err: InvalidTypeError{wasm.ValueTypeI64, wasm.ValueTypeF32},
		},
	}

	for i := range tcs {
		tc := tcs[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mod := wasm.Module{
				GlobalIndexSpace: []wasm.GlobalEntry{
					{Type: wasm.GlobalVar{Type: wasm.ValueTypeI32}},
					{Type: wasm.GlobalVar{Type: wasm.ValueTypeI64}},
					{Type: wasm.GlobalVar{Type: wasm.ValueTypeF64}},
				},
			}
			sig := wasm.FunctionSig{Form: 0x60 /* Must always be 0x60 */}
			fn := wasm.FunctionBody{
				Module: &mod,
				Code:   tc.code,
				Locals: []wasm.LocalEntry{
					{Count: 1, Type: wasm.ValueTypeI32},
					{Count: 1, Type: wasm.ValueTypeI64},
					{Count: 1, Type: wasm.ValueTypeF64},
				},
			}

			_, err := verifyBody(&sig, &fn, &mod)
			if err != tc.err {
				t.Fatalf("verify returned '%v', want '%v'", err, tc.err)
			}
		})
	}
}

func TestValidateBlockTypecheck(t *testing.T) {
	tcs := []struct {
		name string
		code []byte
		err  error
	}{
		{
			name: "dangling block",
			// (block) (block (nop))
			code: []byte{
				operators.Block, byte(wasm.BlockTypeEmpty),
				operators.Block, byte(wasm.BlockTypeEmpty), operators.Nop, operators.End,
			},
			err: UnmatchedOpError(operators.Block),
		},
		{
			name: "dangling loop",
			// (loop) (block (nop))
			code: []byte{
				operators.Loop, byte(wasm.BlockTypeEmpty),
				operators.Block, byte(wasm.BlockTypeEmpty), operators.Nop, operators.End,
			},
			err: UnmatchedOpError(operators.Loop),
		},
		{
			name: "dangling end",
			// (block (nop)) (nop) (end)
			code: []byte{
				operators.Block, byte(wasm.BlockTypeEmpty), operators.Nop, operators.End,
				operators.Nop, operators.End,
			},
			err: UnmatchedOpError(operators.End),
		},
		{
			name: "underflow",
			// (block (result i32) (nop))
			code: []byte{
				operators.Block, byte(wasm.ValueTypeI32), operators.Nop, operators.End,
			},
			err: ErrStackUnderflow,
		},
		{
			name: "i32-void",
			// (block (block (result i32) (i32.const 0)))
			code: []byte{
				operators.Block, byte(wasm.BlockTypeEmpty),
				operators.Block, byte(wasm.ValueTypeI32), operators.I32Const, 0, operators.End,
				operators.End,
			},
			err: UnbalancedStackErr(wasm.ValueTypeI32),
		},
		{
			name: "void-i32",
			// (block (result i32) (block (nop)))
			code: []byte{
				operators.Block, byte(wasm.ValueTypeI32),
				operators.Block, byte(wasm.BlockTypeEmpty), operators.Nop, operators.End,
				operators.End,
			},
			err: ErrStackUnderflow,
		},
		{
			name: "i64-void",
			// (block (block (result i64) (i64.const 0)))
			code: []byte{
				operators.Block, byte(wasm.BlockTypeEmpty),
				operators.Block, byte(wasm.ValueTypeI64), operators.I64Const, 0, operators.End,
				operators.End,
			},
			err: UnbalancedStackErr(wasm.ValueTypeI64),
		},
		{
			name: "void-i64",
			// (block (result i64) (block (nop)))
			code: []byte{
				operators.Block, byte(wasm.ValueTypeI64),
				operators.Block, byte(wasm.BlockTypeEmpty), operators.Nop, operators.End,
				operators.End,
			},
			err: ErrStackUnderflow,
		},
		{
			name: "f64-void",
			// (block (block (result f64) (f64.const 0)))
			code: []byte{
				operators.Block, byte(wasm.BlockTypeEmpty),
				operators.Block, byte(wasm.ValueTypeF64), operators.F64Const, 0, 0, 0, 0, 0, 0, 0, 0, operators.End,
				operators.End,
			},
			err: UnbalancedStackErr(wasm.ValueTypeF64),
		},
		{
			name: "void-f64",
			// (block (result f64) (block (nop)))
			code: []byte{
				operators.Block, byte(wasm.ValueTypeF64),
				operators.Block, byte(wasm.BlockTypeEmpty), operators.Nop, operators.End,
				operators.End,
			},
			err: ErrStackUnderflow,
		},
		{
			name: "f32-void",
			// (block (block (result f32) (f32.const 0)))
			code: []byte{
				operators.Block, byte(wasm.BlockTypeEmpty),
				operators.Block, byte(wasm.ValueTypeF32), operators.F32Const, 0, 0, 0, 0, operators.End,
				operators.End,
			},
			err: UnbalancedStackErr(wasm.ValueTypeF32),
		},
		{
			name: "void-f32",
			// (block (result f32) (block (nop)))
			code: []byte{
				operators.Block, byte(wasm.ValueTypeF32),
				operators.Block, byte(wasm.BlockTypeEmpty), operators.Nop, operators.End,
				operators.End,
			},
			err: ErrStackUnderflow,
		},
		{
			name: "bad stack balance",
			// (i32.const 0) (block (drop))
			code: []byte{
				operators.I32Const, 0,
				operators.Block, byte(wasm.BlockTypeEmpty), operators.Drop, operators.End,
			},
			err: ErrStackUnderflow,
		},
		{
			name: "break void-i32",
			// (block (result i32) (br 0))
			code: []byte{
				operators.Block, byte(wasm.ValueTypeI32),
				operators.Br, 0,
				operators.End,
			},
			err: ErrStackUnderflow,
		},
		{
			name: "break i32-void",
			// (block (result i32) (br 0) (i32.const 0))
			code: []byte{
				operators.Block, byte(wasm.ValueTypeI32),
				operators.Br, 0, operators.I32Const, 0,
				operators.End,
			},
			err: ErrStackUnderflow,
		},
		{
			name: "break void-i64",
			// (block (result i64) (br 0))
			code: []byte{
				operators.Block, byte(wasm.ValueTypeI64),
				operators.Br, 0,
				operators.End,
			},
			err: ErrStackUnderflow,
		},
		{
			name: "break i64-void",
			// (block (result i64) (br 0) (i64.const 0))
			code: []byte{
				operators.Block, byte(wasm.ValueTypeI64),
				operators.Br, 0, operators.I64Const, 0,
				operators.End,
			},
			err: ErrStackUnderflow,
		},
		{
			name: "break nested i32",
			// (block (result i32) (block (result i32) (br 1 (i32.const 1))) (br 0))
			code: []byte{
				operators.Block, byte(wasm.ValueTypeI32),
				operators.Block, byte(wasm.ValueTypeI32),
				operators.Br, 1,
				operators.I32Const, 1,
				operators.End,
				operators.Br, 0,
				operators.End,
			},
			err: ErrStackUnderflow,
		},
		{
			name: "break nested void-i32",
			// (block (result i32) (block (br 1)) (br 0 (i32.const 0)))
			code: []byte{
				operators.Block, byte(wasm.ValueTypeI32),
				operators.Block, byte(wasm.BlockTypeEmpty),
				operators.Br, 1,
				operators.End,
				operators.Br, 0,
				operators.I32Const, 0,
				operators.End,
			},
			err: ErrStackUnderflow,
		},
		{
			name: "brif void-void",
			// (block (br_if 0 (i32.const 0)))
			code: []byte{
				operators.Block, byte(wasm.BlockTypeEmpty),
				operators.I32Const, 0,
				operators.BrIf, 0,
				operators.End,
			},
			err: nil,
		},
		{
			name: "brif i32-i32",
			// (block (result i32.const) (br_if 0 (i32.const 0))) (drop)
			code: []byte{
				operators.Block, byte(wasm.ValueTypeI32),
				operators.I32Const, 0,
				operators.I32Const, 0,
				operators.BrIf, 0,
				operators.End,
				operators.Drop,
			},
			err: nil,
		},
		{
			name: "loop brif i32-i32",
			// (loop (result i32.const) (br_if 0 (i32.const 0))) (drop)
			code: []byte{
				operators.Loop, byte(wasm.ValueTypeI32),
				operators.I32Const, 0,
				operators.I32Const, 0,
				operators.BrIf, 0,
				operators.End,
				operators.Drop,
			},
			err: nil,
		},
		{
			name: "brif i32-void",
			// (block (br_if 0 (i32.const 0)))
			code: []byte{
				operators.Block, byte(wasm.BlockTypeEmpty),
				operators.I32Const, 0,
				operators.I32Const, 0,
				operators.BrIf, 0,
				operators.End,
			},
			err: UnbalancedStackErr(wasm.ValueTypeI32),
		},
		{
			name: "brif void-i32",
			// (block (return i32.const) (br_if 0 (i32.const 0)))
			code: []byte{
				operators.Block, byte(wasm.ValueTypeI32),
				operators.I32Const, 0,
				operators.BrIf, 0,
				operators.End,
			},
			err: ErrStackUnderflow,
		},
		{
			name: "brif invalid branch",
			// (block (i32.const 0) (br_if 2))
			code: []byte{
				operators.Block, byte(wasm.BlockTypeEmpty),
				operators.I32Const, 0,
				operators.BrIf, 2,
				operators.End,
			},
			err: InvalidLabelError(2),
		},
		{
			name: "loop invalid branch",
			// (loop (br 2))
			code: []byte{
				operators.Loop, byte(wasm.BlockTypeEmpty),
				operators.Br, 2,
				operators.End,
			},
			err: InvalidLabelError(2),
		},
		{
			name: "loop i32-void",
			// (loop (result i32) (nop))
			code: []byte{
				operators.Loop, byte(wasm.BlockTypeEmpty),
				operators.I32Const, 0, operators.End,
				operators.End,
			},
			err: UnbalancedStackErr(wasm.ValueTypeI32),
		},
		{
			name: "loop i32-i32",
			// (loop (result i32) (i32.const 0)) (drop)
			code: []byte{
				operators.Loop, byte(wasm.ValueTypeI32),
				operators.I32Const, 0,
				operators.End,
				operators.Drop,
			},
			err: nil,
		},
		{
			name: "brtable void-void",
			// (block (block (i32.const 0) (brtable 0 0 1)))
			code: []byte{
				operators.Block, byte(wasm.BlockTypeEmpty),
				operators.Block, byte(wasm.BlockTypeEmpty),
				operators.I32Const, 0,
				operators.BrTable,
				/* target count */ 2,
				0, 0, 1,
				operators.End,
				operators.End,
			},
			err: nil,
		},
		{
			name: "brtable i64-i32",
			// (block (return i64) (i64.const 0) (block (return i64) (i32.const 0) (brtable 0 0 1)))
			code: []byte{
				operators.Block, byte(wasm.ValueTypeI64),
				operators.I64Const, 0,
				operators.Block, byte(wasm.ValueTypeI32),
				operators.I32Const, 0,
				operators.BrTable,
				/* target count */ 2,
				0, 0, 1,
				operators.End,
				operators.I32Const, 0,
				operators.End,
			},
			err: InvalidTypeError{wasm.ValueTypeI64, wasm.ValueTypeI32},
		},
		{
			name: "brtable invalid default branch",
			// (i32.const 0) (br_table 1)
			code: []byte{
				operators.I32Const, 0,
				operators.BrTable,
				/* target count */ 0,
				1,
			},
			err: InvalidLabelError(1),
		},
		{
			name: "brtable invalid entry branch",
			// (i32.const 0) (br_table 3 0)
			code: []byte{
				operators.I32Const, 0,
				operators.BrTable,
				/* target count */ 1,
				3, 0,
			},
			err: InvalidLabelError(3),
		},
		{
			name: "brtable default i32-i32",
			// (block (return i32) (i32.const 0) (i32.const 0) (br_table 0 0))
			code: []byte{
				operators.Block, byte(wasm.ValueTypeI32),
				operators.I32Const, 0,
				operators.I32Const, 0,
				operators.BrTable,
				/* target count */ 0,
				0,
				operators.End,
				operators.Drop,
			},
			err: nil,
		},
	}

	for i := range tcs {
		tc := tcs[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mod := wasm.Module{}
			sig := wasm.FunctionSig{Form: 0x60 /* Must always be 0x60 */}
			fn := wasm.FunctionBody{Module: &mod, Code: tc.code}

			_, err := verifyBody(&sig, &fn, &mod)
			if err != tc.err {
				t.Fatalf("verify returned '%v', want '%v'", err, tc.err)
			}
		})
	}
}

func TestValidateIfBlock(t *testing.T) {
	tcs := []struct {
		name string
		code []byte
		err  error
	}{
		{
			name: "if nominal",
			// (i32.const 0) (if (nop)
			code: []byte{
				operators.I32Const, 0,
				operators.If, byte(wasm.BlockTypeEmpty),
				operators.Nop,
				operators.End,
			},
			err: nil,
		},
		{
			name: "if else nominal",
			// (i32.const 0) (if (nop) (else (nop))
			code: []byte{
				operators.I32Const, 0,
				operators.If, byte(wasm.BlockTypeEmpty),
				operators.Nop,
				operators.Else,
				operators.Nop,
				operators.End,
			},
			err: nil,
		},
		{
			name: "if else with value nominal",
			// (i32.const 0) (if (result i64) (i64.const 1) (else (i64.const 2)) (drop)
			code: []byte{
				operators.I32Const, 0,
				operators.If, byte(wasm.ValueTypeI64),
				operators.I64Const, 1,
				operators.Else,
				operators.I64Const, 2,
				operators.End,
				operators.Drop,
			},
			err: nil,
		},
		{
			name: "dangling if",
			// (i32.const 0) (if) (block (nop))
			code: []byte{
				operators.I32Const, 0,
				operators.If, byte(wasm.BlockTypeEmpty),
				operators.Block, byte(wasm.BlockTypeEmpty), operators.Nop, operators.End,
			},
			err: UnmatchedOpError(operators.If),
		},
		{
			name: "dangling else",
			// (block (nop)) (else (nop))
			code: []byte{
				operators.Block, byte(wasm.BlockTypeEmpty), operators.Nop, operators.End,
				operators.Else, operators.Nop, operators.End,
			},
			err: UnmatchedOpError(operators.Else),
		},
		{
			name: "if else i32-i32-i64",
			// (i32.const 0) (if (result i32) (i32.const 0) (else (i64.const 1))) (drop)
			code: []byte{
				operators.I32Const, 0,
				operators.If, byte(wasm.ValueTypeI32),
				operators.I32Const, 0,
				operators.Else,
				operators.I64Const, 1,
				operators.End,
				operators.Drop,
			},
			err: InvalidTypeError{wasm.ValueTypeI32, wasm.ValueTypeI64},
		},
		{
			name: "if else i64-i32-i64",
			// (i32.const 0) (if (result i64) (i32.const 0) (else (i64.const 1))) (drop)
			code: []byte{
				operators.I32Const, 0,
				operators.If, byte(wasm.ValueTypeI64),
				operators.I32Const, 0,
				operators.Else,
				operators.I64Const, 1,
				operators.End,
				operators.Drop,
			},
			err: InvalidTypeError{wasm.ValueTypeI64, wasm.ValueTypeI32},
		},
		{
			name: "if else i64-i32-i32",
			// (i32.const 0) (if (result i64) (i32.const 0) (else (i64.const 1))) (drop)
			code: []byte{
				operators.I32Const, 0,
				operators.If, byte(wasm.ValueTypeI64),
				operators.I32Const, 0,
				operators.Else,
				operators.I32Const, 1,
				operators.End,
				operators.Drop,
			},
			err: InvalidTypeError{wasm.ValueTypeI64, wasm.ValueTypeI32},
		},
		{
			name: "if void-i32",
			// (i32.const 0) (if (result i32) (i32.const 0)) (drop)
			code: []byte{
				operators.I32Const, 0,
				operators.If, byte(wasm.BlockTypeEmpty),
				operators.I32Const, 0,
				operators.End,
				operators.Drop,
			},
			err: UnbalancedStackErr(wasm.ValueTypeI32),
		},
		{
			name: "if i32-void",
			// (i32.const 0) (if (result i32) (nop))
			code: []byte{
				operators.I32Const, 0,
				operators.If, byte(wasm.ValueTypeI32),
				operators.Nop,
				operators.End,
			},
			err: ErrStackUnderflow,
		},
		{
			name: "if i32 missing else",
			// (i32.const 0) (if (nop))
			code: []byte{
				operators.I32Const, 0,
				operators.If, byte(wasm.ValueTypeI32),
				operators.I32Const, 0,
				operators.End,
				operators.Drop,
			},
			err: UnmatchedIfValueErr(wasm.ValueTypeI32),
		},
		{
			name: "if with else missing main block result",
			// (i32.const 0) (if (result i32) (nop) else (i32.const 0))
			code: []byte{
				operators.I32Const, 0,
				operators.If, byte(wasm.ValueTypeI32),
				operators.Nop,
				operators.Else,
				operators.I32Const, 0,
				operators.End,
				operators.Drop,
			},
			err: ErrStackUnderflow,
		},
		{
			name: "if with else missing else block result",
			// (i32.const 0) (if (result i32) (i32.const 0) else (nop))
			code: []byte{
				operators.I32Const, 0,
				operators.If, byte(wasm.ValueTypeI32),
				operators.I32Const, 0,
				operators.Else,
				operators.Nop,
				operators.End,
				operators.Drop,
			},
			err: ErrStackUnderflow,
		},
	}

	for i := range tcs {
		tc := tcs[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mod := wasm.Module{}
			sig := wasm.FunctionSig{Form: 0x60 /* Must always be 0x60 */}
			fn := wasm.FunctionBody{Module: &mod, Code: tc.code}

			_, err := verifyBody(&sig, &fn, &mod)
			if err != tc.err {
				t.Fatalf("verify returned '%v', want '%v'", err, tc.err)
			}
		})
	}
}

func TestValidateStackTypechecking(t *testing.T) {
	tcs := []struct {
		name string
		code []byte
		err  error
	}{
		{
			name: "call i32",
			code: []byte{
				operators.Call, 0,
				operators.Drop,
			},
			err: nil,
		},
		{
			name: "call bad index",
			code: []byte{
				operators.Call, 100,
			},
			err: wasm.InvalidFunctionIndexError(100),
		},
		{
			name: "call i32 unbalanced",
			code: []byte{
				operators.Call, 0,
			},
			err: UnbalancedStackErr(wasm.ValueTypeI32),
		},
		{
			name: "parameterized call",
			code: []byte{
				operators.I32Const, 1,
				operators.I32Const, 2,
				operators.Call, 1,
				operators.Drop,
			},
			err: nil,
		},
		{
			name: "parameterized call type mismatch",
			code: []byte{
				operators.F32Const, 0, 0, 0, 0,
				operators.I32Const, 2,
				operators.Call, 1,
				operators.Drop,
			},
			err: InvalidTypeError{wasm.ValueTypeI32, wasm.ValueTypeF32},
		},
		{
			name: "call indirect",
			code: []byte{
				operators.I32Const, 0,
				operators.CallIndirect, 0, 0,
				operators.Drop,
			},
			err: nil,
		},
		{
			name: "call indirect invalid selector type",
			code: []byte{
				operators.F32Const, 0, 0, 0, 0,
				operators.CallIndirect, 0, 0,
				operators.Drop,
			},
			err: InvalidTypeError{wasm.ValueTypeI32, wasm.ValueTypeF32},
		},
		{
			name: "call indirect non-zero table index",
			code: []byte{
				operators.I32Const, 0,
				operators.CallIndirect, 0, 1,
				operators.Drop,
			},
			err: InvalidTableIndexError{"table", 1},
		},
		{
			name: "call indirect i32 unbalanced",
			code: []byte{
				operators.I32Const, 0,
				operators.CallIndirect, 0, 0,
			},
			err: UnbalancedStackErr(wasm.ValueTypeI32),
		},
		{
			name: "call indirect parameters",
			code: []byte{
				operators.I32Const, 1,
				operators.I32Const, 2,
				operators.I32Const, 0,
				operators.CallIndirect, 1, 0,
				operators.Drop,
			},
			err: nil,
		},
		{
			name: "call indirect parameters underflow",
			code: []byte{
				operators.I32Const, 1,
				operators.I32Const, 0,
				operators.CallIndirect, 1, 0,
				operators.Drop,
			},
			err: ErrStackUnderflow,
		},
		{
			name: "call indirect parameters mismatch",
			code: []byte{
				operators.I32Const, 1,
				operators.I64Const, 2,
				operators.I32Const, 0,
				operators.CallIndirect, 1, 0,
				operators.Drop,
			},
			err: InvalidTypeError{wasm.ValueTypeI32, wasm.ValueTypeI64},
		},
		{
			name: "call indirect parameters return mismatch",
			code: []byte{
				operators.I32Const, 8,
				operators.I32Const, 1,
				operators.I32Const, 2,
				operators.I32Const, 0,
				operators.CallIndirect, 1, 0,
				operators.I32Add,
				operators.Drop,
			},
			err: InvalidTypeError{wasm.ValueTypeI32, wasm.ValueTypeF32},
		},
	}

	for i := range tcs {
		tc := tcs[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mod := wasm.Module{
				FunctionIndexSpace: []wasm.Function{
					{ // Function at index 0 returns an i32.
						Sig: &wasm.FunctionSig{
							Form:        0x60,
							ParamTypes:  nil, // No parameters
							ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
						},
					},
					{ // Function at index 1 returns an f32, consuming 2 i32's.
						Sig: &wasm.FunctionSig{
							Form:        0x60,
							ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
							ReturnTypes: []wasm.ValueType{wasm.ValueTypeF32},
						},
					},
				},
				Table: &wasm.SectionTables{
					Entries: []wasm.Table{
						{ElementType: wasm.ElemTypeAnyFunc},
						{ElementType: wasm.ElemTypeAnyFunc},
					},
				},
			}
			mod.Types = &wasm.SectionTypes{
				Entries: []wasm.FunctionSig{
					*mod.FunctionIndexSpace[0].Sig,
					*mod.FunctionIndexSpace[1].Sig,
				},
			}

			sig := wasm.FunctionSig{Form: 0x60 /* Must always be 0x60 */}
			fn := wasm.FunctionBody{Module: &mod, Code: tc.code}

			_, err := verifyBody(&sig, &fn, &mod)
			if err != tc.err {
				t.Fatalf("verify returned '%v', want '%v'", err, tc.err)
			}
		})
	}
}
