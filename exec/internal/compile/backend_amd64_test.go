// Copyright 2019 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !appengine amd64

package compile

import (
	"bytes"
	"encoding/binary"
	"math"
	"runtime"
	"testing"
	"unsafe"

	ops "github.com/go-interpreter/wagon/wasm/operators"
	asm "github.com/twitchyliquid64/golang-asm"
	"github.com/twitchyliquid64/golang-asm/obj"
	"github.com/twitchyliquid64/golang-asm/obj/x86"
)

func TestAMD64JitCall(t *testing.T) {
	if !supportedOS(runtime.GOOS) {
		t.SkipNow()
	}
	allocator := &MMapAllocator{}
	defer allocator.Close()
	builder, err := asm.NewBuilder("amd64", 64)
	if err != nil {
		t.Fatal(err)
	}

	ret := builder.NewProg()
	ret.As = obj.ARET
	builder.AddInstruction(ret)
	out := builder.Assemble()

	nativeBlock, err := allocator.AllocateExec(out)
	if err != nil {
		t.Fatal(err)
	}

	fakeStack := make([]uint64, 0, 5)
	fakeLocals := make([]uint64, 0, 0)
	nativeBlock.Invoke(&fakeStack, &fakeLocals, nil, nil)

	if got, want := len(fakeStack), 0; got != want {
		t.Errorf("fakeStack.Len = %d, want %d", got, want)
	}
}

func TestAMD64StackPush(t *testing.T) {
	if !supportedOS(runtime.GOOS) {
		t.SkipNow()
	}
	allocator := &MMapAllocator{}
	defer allocator.Close()
	builder, err := asm.NewBuilder("amd64", 64)
	if err != nil {
		t.Fatal(err)
	}

	b := &AMD64Backend{EmitBoundsChecks: true}
	b.emitPreamble(builder)
	mov := builder.NewProg()
	mov.As = x86.AMOVQ
	mov.From.Type = obj.TYPE_CONST
	mov.From.Offset = 1234
	mov.To.Type = obj.TYPE_REG
	mov.To.Reg = x86.REG_AX
	builder.AddInstruction(mov)
	b.emitSymbolicPushFromReg(builder, currentInstruction{}, x86.REG_AX)
	mov = builder.NewProg()
	mov.As = x86.AMOVQ
	mov.From.Type = obj.TYPE_CONST
	mov.From.Offset = 5678
	mov.To.Type = obj.TYPE_REG
	mov.To.Reg = x86.REG_AX
	builder.AddInstruction(mov)
	b.emitSymbolicPushFromReg(builder, currentInstruction{}, x86.REG_AX)

	b.emitPostamble(builder)
	b.lowerAMD64(builder)
	out := builder.Assemble()

	// debugPrintAsm(out)

	nativeBlock, err := allocator.AllocateExec(out)
	if err != nil {
		t.Fatal(err)
	}

	fakeStack := make([]uint64, 3, 5)
	fakeLocals := make([]uint64, 0, 10)
	if exitSignal := nativeBlock.Invoke(&fakeStack, &fakeLocals, nil, nil); exitSignal.CompletionStatus() != CompletionOK {
		t.Fatalf("native execution returned non-ok completion status: %v", exitSignal.CompletionStatus())
	}

	if got, want := len(fakeStack), 5; got != want {
		t.Errorf("fakeStack.Len = %d, want %d", got, want)
	}
	if got, want := len(fakeLocals), 0; got != want {
		t.Errorf("fakeLocals.Len = %d, want %d", got, want)
	}

	if got, want := fakeStack[3], uint64(1234); got != want {
		t.Errorf("fakeStack[0] = %d, want %d", got, want)
	}
	if got, want := fakeStack[4], uint64(5678); got != want {
		t.Errorf("fakeStack[1] = %d, want %d", got, want)
	}
}

func TestAMD64StackPop(t *testing.T) {
	if !supportedOS(runtime.GOOS) {
		t.SkipNow()
	}
	allocator := &MMapAllocator{}
	defer allocator.Close()
	builder, err := asm.NewBuilder("amd64", 64)
	if err != nil {
		t.Fatal(err)
	}

	b := &AMD64Backend{EmitBoundsChecks: true}
	b.emitPreamble(builder)
	b.emitSymbolicPopToReg(builder, currentInstruction{}, x86.REG_AX)
	b.emitSymbolicPopToReg(builder, currentInstruction{}, x86.REG_BX)
	b.emitSymbolicPushFromReg(builder, currentInstruction{}, x86.REG_AX)
	b.emitPostamble(builder)
	b.lowerAMD64(builder)
	out := builder.Assemble()

	nativeBlock, err := allocator.AllocateExec(out)
	if err != nil {
		t.Fatal(err)
	}

	fakeStack := make([]uint64, 2, 5)
	fakeStack[1] = 1337
	fakeLocals := make([]uint64, 0, 0)
	if exitSignal := nativeBlock.Invoke(&fakeStack, &fakeLocals, nil, nil); exitSignal.CompletionStatus() != CompletionOK {
		t.Fatalf("native execution returned non-ok completion status: %v", exitSignal.CompletionStatus())
	}

	if got, want := len(fakeStack), 1; got != want {
		t.Errorf("fakeStack.Len = %d, want %d", got, want)
	}
	if got, want := fakeStack[0], uint64(1337); got != want {
		t.Errorf("fakeStack[0] = %d, want %d", got, want)
	}
}

func TestAMD64ExitSignal(t *testing.T) {
	if !supportedOS(runtime.GOOS) {
		t.SkipNow()
	}
	allocator := &MMapAllocator{}
	defer allocator.Close()
	builder, err := asm.NewBuilder("amd64", 64)
	if err != nil {
		t.Fatal(err)
	}

	b := &AMD64Backend{EmitBoundsChecks: true}
	fakeStack := make([]uint64, 1, 2)
	fakeLocals := make([]uint64, 2, 2)

	// Test CompletionOK.
	b.emitPreamble(builder)
	b.emitPostamble(builder)
	b.lowerAMD64(builder)
	out := builder.Assemble()
	nativeBlock, err := allocator.AllocateExec(out)
	if err != nil {
		t.Fatal(err)
	}
	result := nativeBlock.Invoke(&fakeStack, &fakeLocals, nil, nil)
	if result.CompletionStatus() != CompletionOK {
		t.Errorf("Execution returned non-OK completion status: %v", result.CompletionStatus())
	}
	if got, want := result.Index(), unknownIndex; got != want {
		t.Errorf("result.Index() = %v, want %v", got, want)
	}

	// Test CompletionBadBounds - stack underrun.
	builder, err = asm.NewBuilder("amd64", 64)
	if err != nil {
		t.Fatal(err)
	}
	b = &AMD64Backend{EmitBoundsChecks: true}
	fakeStack = make([]uint64, 1, 2)
	b.emitPreamble(builder)
	b.emitSymbolicPopToReg(builder, currentInstruction{idx: 1}, x86.REG_BX)
	b.emitSymbolicPopToReg(builder, currentInstruction{idx: 2}, x86.REG_BX)
	b.emitSymbolicPopToReg(builder, currentInstruction{idx: 3}, x86.REG_BX)
	b.emitPostamble(builder)
	b.lowerAMD64(builder)
	out = builder.Assemble()
	if nativeBlock, err = allocator.AllocateExec(out); err != nil {
		t.Fatal(err)
	}
	result = nativeBlock.Invoke(&fakeStack, &fakeLocals, nil, nil)
	if result.CompletionStatus() != CompletionBadBounds {
		t.Errorf("Execution returned non-BadBounds completion status: %v", result.CompletionStatus())
	}
	if got, want := result.Index(), 2; got != want {
		t.Errorf("result.Index() = %v, want %v", got, want)
	}

	// Test CompletionBadBounds - stack overrun.
	builder, err = asm.NewBuilder("amd64", 64)
	if err != nil {
		t.Fatal(err)
	}
	b = &AMD64Backend{EmitBoundsChecks: true}
	fakeStack = make([]uint64, 0, 2)
	b.emitPreamble(builder)
	b.emitSymbolicPushFromReg(builder, currentInstruction{idx: 1}, x86.REG_BX)
	b.emitSymbolicPushFromReg(builder, currentInstruction{idx: 2}, x86.REG_BX)
	b.emitSymbolicPushFromReg(builder, currentInstruction{idx: 3}, x86.REG_BX)
	b.emitPostamble(builder)
	b.lowerAMD64(builder)
	out = builder.Assemble()
	if nativeBlock, err = allocator.AllocateExec(out); err != nil {
		t.Fatal(err)
	}
	result = nativeBlock.Invoke(&fakeStack, &fakeLocals, nil, nil)
	if result.CompletionStatus() != CompletionBadBounds {
		t.Errorf("Execution returned non-BadBounds completion status: %v", result.CompletionStatus())
	}
	if got, want := result.Index(), 3; got != want {
		t.Errorf("result.Index() = %v, want %v", got, want)
	}
}

func TestAMD64LocalsGet(t *testing.T) {
	if !supportedOS(runtime.GOOS) {
		t.SkipNow()
	}
	allocator := &MMapAllocator{}
	defer allocator.Close()
	builder, err := asm.NewBuilder("amd64", 64)
	if err != nil {
		t.Fatal(err)
	}

	b := &AMD64Backend{}
	b.emitPreamble(builder)
	b.emitWasmLocalsLoad(builder, currentInstruction{}, x86.REG_AX, 0)
	b.emitSymbolicPushFromReg(builder, currentInstruction{}, x86.REG_AX)
	b.emitWasmLocalsLoad(builder, currentInstruction{}, x86.REG_AX, 1)
	b.emitSymbolicPushFromReg(builder, currentInstruction{}, x86.REG_AX)
	b.emitBinaryI64(builder, currentInstruction{inst: InstructionMetadata{Op: ops.I64Add}})
	b.emitPostamble(builder)
	b.lowerAMD64(builder)
	out := builder.Assemble()

	nativeBlock, err := allocator.AllocateExec(out)
	if err != nil {
		t.Fatal(err)
	}

	fakeStack := make([]uint64, 0, 5)
	fakeLocals := make([]uint64, 2, 2)
	fakeLocals[0] = 1335
	fakeLocals[1] = 2
	nativeBlock.Invoke(&fakeStack, &fakeLocals, nil, nil)

	if got, want := len(fakeStack), 1; got != want {
		t.Errorf("fakeStack.Len = %d, want %d", got, want)
	}
	if got, want := fakeStack[0], uint64(1337); got != want {
		t.Errorf("fakeStack[0] = %d, want %d", got, want)
	}
}

func TestAMD64LocalsSet(t *testing.T) {
	if !supportedOS(runtime.GOOS) {
		t.SkipNow()
	}
	allocator := &MMapAllocator{}
	defer allocator.Close()
	builder, err := asm.NewBuilder("amd64", 64)
	if err != nil {
		t.Fatal(err)
	}

	b := &AMD64Backend{}
	b.emitPreamble(builder)
	b.emitWasmLocalsLoad(builder, currentInstruction{}, x86.REG_AX, 0)
	b.emitWasmLocalsSave(builder, currentInstruction{}, x86.REG_AX, 1)
	b.emitWasmLocalsSave(builder, currentInstruction{}, x86.REG_AX, 2)
	b.emitPushImmediate(builder, currentInstruction{}, 11)
	b.emitSymbolicPopToReg(builder, currentInstruction{}, x86.REG_DX)
	b.emitWasmLocalsSave(builder, currentInstruction{}, x86.REG_DX, 4)
	b.emitPostamble(builder)
	b.lowerAMD64(builder)
	out := builder.Assemble()

	nativeBlock, err := allocator.AllocateExec(out)
	if err != nil {
		t.Fatal(err)
	}

	fakeStack := make([]uint64, 0, 5)
	fakeLocals := make([]uint64, 5, 5)
	fakeLocals[0] = 1335
	fakeLocals[1] = 2
	nativeBlock.Invoke(&fakeStack, &fakeLocals, nil, nil)

	if got, want := len(fakeStack), 0; got != want {
		t.Errorf("fakeStack.Len = %d, want %d", got, want)
	}
	if got, want := fakeLocals[1], uint64(1335); got != want {
		t.Errorf("fakeLocals[1] = %d, want %d", got, want)
	}
	if got, want := fakeLocals[4], uint64(11); got != want {
		t.Errorf("fakeLocals[4] = %d, want %d", got, want)
	}
}

func TestAMD64GlobalsGet(t *testing.T) {
	if !supportedOS(runtime.GOOS) {
		t.SkipNow()
	}
	allocator := &MMapAllocator{}
	defer allocator.Close()
	builder, err := asm.NewBuilder("amd64", 64)
	if err != nil {
		t.Fatal(err)
	}

	b := &AMD64Backend{}
	b.emitPreamble(builder)
	b.emitWasmGlobalsLoad(builder, currentInstruction{}, x86.REG_AX, 0)
	b.emitSymbolicPushFromReg(builder, currentInstruction{}, x86.REG_AX)
	b.emitPostamble(builder)
	b.lowerAMD64(builder)
	out := builder.Assemble()

	nativeBlock, err := allocator.AllocateExec(out)
	if err != nil {
		t.Fatal(err)
	}

	fakeStack := make([]uint64, 0, 5)
	fakeGlobals := make([]uint64, 2, 2)
	fakeGlobals[0] = 1335
	nativeBlock.Invoke(&fakeStack, nil, &fakeGlobals, nil)

	if got, want := len(fakeStack), 1; got != want {
		t.Errorf("fakeStack.Len = %d, want %d", got, want)
	}
	if got, want := fakeStack[0], uint64(1335); got != want {
		t.Errorf("fakeStack[0] = %d, want %d", got, want)
	}
}

func TestAMD64GlobalsSet(t *testing.T) {
	if !supportedOS(runtime.GOOS) {
		t.SkipNow()
	}
	allocator := &MMapAllocator{}
	defer allocator.Close()
	builder, err := asm.NewBuilder("amd64", 64)
	if err != nil {
		t.Fatal(err)
	}

	b := &AMD64Backend{}
	b.emitPreamble(builder)
	b.emitWasmGlobalsLoad(builder, currentInstruction{}, x86.REG_AX, 0)
	b.emitWasmGlobalsSave(builder, currentInstruction{}, x86.REG_AX, 1)
	b.emitWasmGlobalsSave(builder, currentInstruction{}, x86.REG_AX, 2)
	b.emitPushImmediate(builder, currentInstruction{}, 11)
	b.emitSymbolicPopToReg(builder, currentInstruction{}, x86.REG_DX)
	b.emitWasmGlobalsSave(builder, currentInstruction{}, x86.REG_DX, 4)
	b.emitPostamble(builder)
	b.lowerAMD64(builder)
	out := builder.Assemble()

	nativeBlock, err := allocator.AllocateExec(out)
	if err != nil {
		t.Fatal(err)
	}

	fakeStack := make([]uint64, 0, 5)
	fakeGlobals := make([]uint64, 5, 5)
	fakeGlobals[0] = 1335
	fakeGlobals[1] = 2
	nativeBlock.Invoke(&fakeStack, nil, &fakeGlobals, nil)

	if got, want := len(fakeStack), 0; got != want {
		t.Errorf("fakeStack.Len = %d, want %d", got, want)
	}
	if got, want := fakeGlobals[1], uint64(1335); got != want {
		t.Errorf("fakeGlobals[1] = %d, want %d", got, want)
	}
	if got, want := fakeGlobals[2], uint64(1335); got != want {
		t.Errorf("fakeGlobals[2] = %d, want %d", got, want)
	}
	if got, want := fakeGlobals[4], uint64(11); got != want {
		t.Errorf("fakeGlobals[4] = %d, want %d", got, want)
	}
}

func TestAMD64Select(t *testing.T) {
	if !supportedOS(runtime.GOOS) {
		t.SkipNow()
	}
	allocator := &MMapAllocator{}
	defer allocator.Close()
	builder, err := asm.NewBuilder("amd64", 64)
	if err != nil {
		t.Fatal(err)
	}

	b := &AMD64Backend{}
	b.emitPreamble(builder)
	b.emitSelect(builder, currentInstruction{})
	b.emitPostamble(builder)
	b.lowerAMD64(builder)
	out := builder.Assemble()

	nativeBlock, err := allocator.AllocateExec(out)
	if err != nil {
		t.Fatal(err)
	}

	fakeStack := make([]uint64, 3, 5)
	fakeLocals := make([]uint64, 0, 5)
	fakeStack[0] = 11
	fakeStack[1] = 2
	fakeStack[2] = 0
	nativeBlock.Invoke(&fakeStack, &fakeLocals, nil, nil)

	if got, want := len(fakeStack), 1; got != want {
		t.Errorf("fakeStack.Len = %d, want %d", got, want)
	}
	if got, want := fakeStack[0], uint64(2); got != want {
		t.Errorf("fakeStack[0] = %d, want %d", got, want)
	}
}

func TestAMD64MemoryLoad(t *testing.T) {
	tcs := []struct {
		name   string
		op     byte
		mem    []byte
		stack  []uint64
		expect uint64
		oob    bool
	}{
		{
			name:   "i64 within bounds",
			op:     ops.I64Load,
			mem:    []byte{0, 0, 0, 55, 5, 0, 0, 0, 0, 0, 0},
			stack:  []uint64{3},
			expect: 1335,
		},
		{
			name:  "i64 out of bounds",
			op:    ops.I64Load,
			oob:   true,
			mem:   []byte{0, 0, 0, 55, 5, 0, 0, 0, 0, 0, 0},
			stack: []uint64{4},
		},
		{
			name:   "i32 within bounds",
			op:     ops.I32Load,
			mem:    []byte{43, 55, 5, 0, 0},
			stack:  []uint64{1},
			expect: 1335,
		},
		{
			name:  "i32 out of bounds",
			op:    ops.I32Load,
			mem:   []byte{11, 55, 5, 0, 0},
			stack: []uint64{2},
			oob:   true,
		},
	}
	if !supportedOS(runtime.GOOS) {
		t.SkipNow()
	}
	allocator := &MMapAllocator{}
	defer allocator.Close()

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			builder, err := asm.NewBuilder("amd64", 64)
			if err != nil {
				t.Fatal(err)
			}

			b := &AMD64Backend{}
			b.emitPreamble(builder)
			b.emitWasmMemoryLoad(builder, currentInstruction{inst: InstructionMetadata{Op: tc.op}}, x86.REG_AX, 0)
			b.emitSymbolicPushFromReg(builder, currentInstruction{}, x86.REG_AX)
			b.emitPostamble(builder)
			b.lowerAMD64(builder)
			out := builder.Assemble()
			// debugPrintAsm(out)

			nativeBlock, err := allocator.AllocateExec(out)
			if err != nil {
				t.Fatal(err)
			}

			result := nativeBlock.Invoke(&tc.stack, nil, nil, &tc.mem)
			if !tc.oob {
				if result.CompletionStatus() != CompletionOK {
					t.Fatalf("Execution returned non-ok completion status: %v", result.CompletionStatus())
				}

				if got, want := len(tc.stack), 1; got != want {
					t.Errorf("fakeStack.Len = %d, want %d", got, want)
				}
				if got, want := tc.stack[0], tc.expect; got != want {
					t.Errorf("fakeStack[0] = %d, want %d", got, want)
				}
			} else {
				if result.CompletionStatus() != CompletionBadBounds {
					t.Errorf("Execution returned non-bounds completion status: %v", result.CompletionStatus())
				}
			}
		})
	}
}

func TestAMD64MemoryStore(t *testing.T) {
	tcs := []struct {
		name      string
		op        byte
		mem       []byte
		stack     []uint64
		expectMem []byte
		oob       bool
	}{
		{
			name:      "i64 within bounds",
			op:        ops.I64Store,
			mem:       []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			stack:     []uint64{3, 1335},
			expectMem: []byte{0, 0, 0, 55, 5, 0, 0, 0, 0, 0, 0},
		},
		{
			name:  "i64 out of bounds",
			op:    ops.I64Store,
			mem:   []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			stack: []uint64{3, 1335},
			oob:   true,
		},
		{
			name:      "i32 within bounds",
			op:        ops.I32Store,
			mem:       []byte{0, 0, 0, 0, 0, 0, 0},
			stack:     []uint64{3, 1335},
			expectMem: []byte{0, 0, 0, 55, 5, 0, 0},
		},
		{
			name:  "i32 out of bounds",
			op:    ops.I32Store,
			mem:   []byte{0, 0, 0, 0, 0, 0},
			stack: []uint64{3, 1335},
			oob:   true,
		},
	}
	if !supportedOS(runtime.GOOS) {
		t.SkipNow()
	}
	allocator := &MMapAllocator{}
	defer allocator.Close()

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			builder, err := asm.NewBuilder("amd64", 64)
			if err != nil {
				t.Fatal(err)
			}

			b := &AMD64Backend{}
			b.emitPreamble(builder)
			b.emitSymbolicPopToReg(builder, currentInstruction{}, x86.REG_DX)
			b.emitWasmMemoryStore(builder, currentInstruction{inst: InstructionMetadata{Op: tc.op}}, 0, x86.REG_DX)
			b.emitSymbolicPushFromReg(builder, currentInstruction{}, x86.REG_AX)
			b.emitPostamble(builder)
			b.lowerAMD64(builder)
			out := builder.Assemble()
			// debugPrintAsm(out)

			nativeBlock, err := allocator.AllocateExec(out)
			if err != nil {
				t.Fatal(err)
			}

			result := nativeBlock.Invoke(&tc.stack, nil, nil, &tc.mem)
			if !tc.oob {
				if result.CompletionStatus() != CompletionOK {
					t.Fatalf("Execution returned non-ok completion status: %v", result.CompletionStatus())
				}

				if got, want := tc.mem, tc.expectMem; !bytes.Equal(got, want) {
					t.Errorf("mem] = %v, want %v", got, want)
				}
			} else {
				if result.CompletionStatus() != CompletionBadBounds {
					t.Errorf("Execution returned non-bounds completion status: %v", result.CompletionStatus())
				}
			}
		})
	}
}

func TestAMD64FusedConstStore(t *testing.T) {
	tcs := []struct {
		name         string
		op           byte
		stack        []uint64
		expectLocal  []uint64
		expectGlobal []uint64
		expectMem    []byte
	}{
		{
			name:         "set local",
			op:           ops.SetLocal,
			stack:        nil,
			expectLocal:  []uint64{5},
			expectGlobal: []uint64{0},
			expectMem:    []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:         "set global",
			op:           ops.SetGlobal,
			stack:        nil,
			expectLocal:  []uint64{0},
			expectGlobal: []uint64{5},
			expectMem:    []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:         "store i64",
			op:           ops.I64Store,
			stack:        []uint64{0},
			expectLocal:  []uint64{0},
			expectGlobal: []uint64{0},
			expectMem:    []byte{5, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:         "store f64",
			op:           ops.F64Store,
			stack:        []uint64{0},
			expectLocal:  []uint64{0},
			expectGlobal: []uint64{0},
			expectMem:    []byte{5, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:         "store i64 offset",
			op:           ops.I64Store,
			stack:        []uint64{1},
			expectLocal:  []uint64{0},
			expectGlobal: []uint64{0},
			expectMem:    []byte{0, 5, 0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:         "store i32",
			op:           ops.I32Store,
			stack:        []uint64{0},
			expectLocal:  []uint64{0},
			expectGlobal: []uint64{0},
			expectMem:    []byte{5, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:         "store f32",
			op:           ops.F32Store,
			stack:        []uint64{0},
			expectLocal:  []uint64{0},
			expectGlobal: []uint64{0},
			expectMem:    []byte{5, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:         "store i32 offset",
			op:           ops.I32Store,
			stack:        []uint64{5},
			expectLocal:  []uint64{0},
			expectGlobal: []uint64{0},
			expectMem:    []byte{0, 0, 0, 0, 0, 5, 0, 0, 0, 0},
		},
	}
	if !supportedOS(runtime.GOOS) {
		t.SkipNow()
	}
	allocator := &MMapAllocator{}
	defer allocator.Close()

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			b := &AMD64Backend{}
			out, err := b.Build(CompilationCandidate{
				EndInstruction: 2,
			}, []byte{ops.I64Const, 5, 0, 0, 0, tc.op, 0, 0, 0, 0}, &BytecodeMetadata{
				Instructions: []InstructionMetadata{
					{Op: ops.I64Const, Size: 5},
					{Op: tc.op, Start: 5, Size: 5},
				},
			})
			if err != nil {
				t.Fatalf("b.Build() failed: %v", err)
			}
			// debugPrintAsm(out)

			nativeBlock, err := allocator.AllocateExec(out)
			if err != nil {
				t.Fatal(err)
			}

			fakeLocals := make([]uint64, 1)
			fakeGlobals := make([]uint64, 1)
			fakeMem := make([]byte, 10)
			result := nativeBlock.Invoke(&tc.stack, &fakeLocals, &fakeGlobals, &fakeMem)
			if result.CompletionStatus() != CompletionOK {
				t.Fatalf("Execution returned non-ok completion status: %v", result.CompletionStatus())
			}

			if got, want := fakeMem, tc.expectMem; !bytes.Equal(got, want) {
				t.Errorf("mem = %v, want %v", got, want)
			}
			if got, want := fakeLocals[0], tc.expectLocal[0]; got != want {
				t.Errorf("locals[0] = %v, want %v", got, want)
			}
			if got, want := fakeGlobals[0], tc.expectGlobal[0]; got != want {
				t.Errorf("globals[0] = %v, want %v", got, want)
			}
		})
	}
}

func TestAMD64RHSConstOptimizedBinOp(t *testing.T) {
	tcs := []struct {
		name        string
		op          byte
		constVal    uint64
		stack       []uint64
		expectStack []uint64
	}{
		{
			name:        "add i64",
			op:          ops.I64Add,
			constVal:    11,
			stack:       []uint64{3},
			expectStack: []uint64{14},
		},
		{
			name:        "sub i64",
			op:          ops.I64Sub,
			constVal:    1,
			stack:       []uint64{3},
			expectStack: []uint64{2},
		},
		{
			name:        "add i32",
			op:          ops.I32Add,
			constVal:    11,
			stack:       []uint64{3},
			expectStack: []uint64{14},
		},
		{
			name:        "sub i32",
			op:          ops.I32Sub,
			constVal:    1,
			stack:       []uint64{3},
			expectStack: []uint64{2},
		},
		{
			name:        "shl i64",
			op:          ops.I64Shl,
			constVal:    2,
			stack:       []uint64{1},
			expectStack: []uint64{4},
		},
		{
			name:        "shr i64",
			op:          ops.I64ShrU,
			constVal:    2,
			stack:       []uint64{64},
			expectStack: []uint64{16},
		},
		{
			name:        "and i64",
			op:          ops.I64And,
			constVal:    3,
			stack:       []uint64{1},
			expectStack: []uint64{1},
		},
		{
			name:        "and i32",
			op:          ops.I32And,
			constVal:    3,
			stack:       []uint64{1},
			expectStack: []uint64{1},
		},
		{
			name:        "or i64",
			op:          ops.I64Or,
			constVal:    2,
			stack:       []uint64{1},
			expectStack: []uint64{3},
		},
		{
			name:        "or i32",
			op:          ops.I32Or,
			constVal:    2,
			stack:       []uint64{1},
			expectStack: []uint64{3},
		},
		{
			name:        "xor i64",
			op:          ops.I64Xor,
			constVal:    1,
			stack:       []uint64{1 << 33},
			expectStack: []uint64{1<<33 + 1},
		},
		{
			name:        "xor i32",
			op:          ops.I32Xor,
			constVal:    1,
			stack:       []uint64{1 << 33},
			expectStack: []uint64{1},
		},
	}
	if !supportedOS(runtime.GOOS) {
		t.SkipNow()
	}
	allocator := &MMapAllocator{}
	defer allocator.Close()

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			builder, err := asm.NewBuilder("amd64", 64)
			if err != nil {
				t.Fatal(err)
			}

			b := &AMD64Backend{}
			b.emitPreamble(builder)
			b.emitRHSConstOptimizedInstruction(builder, currentInstruction{inst: InstructionMetadata{Op: tc.op}}, tc.constVal)
			b.emitPostamble(builder)
			b.lowerAMD64(builder)
			out := builder.Assemble()
			// debugPrintAsm(out)

			nativeBlock, err := allocator.AllocateExec(out)
			if err != nil {
				t.Fatal(err)
			}

			result := nativeBlock.Invoke(&tc.stack, nil, nil, nil)
			if result.CompletionStatus() != CompletionOK {
				t.Fatalf("Execution returned non-ok completion status: %v", result.CompletionStatus())
			}
			if tc.stack[0] != tc.expectStack[0] {
				t.Errorf("stack[0] = %v, want %v", tc.stack[0], tc.expectStack[0])
			}
		})
	}
}

func TestAMD64OperationsI64(t *testing.T) {
	if !supportedOS(runtime.GOOS) {
		t.SkipNow()
	}
	testCases := []struct {
		Name   string
		Op     byte
		Args   []uint64
		Result uint64
	}{
		{
			Name:   "add",
			Op:     ops.I64Add,
			Args:   []uint64{12, 3},
			Result: 15,
		},
		{
			Name:   "subtract",
			Op:     ops.I64Sub,
			Args:   []uint64{12, 3},
			Result: 9,
		},
		{
			Name:   "and",
			Op:     ops.I64And,
			Args:   []uint64{15, 3},
			Result: 3,
		},
		{
			Name:   "or",
			Op:     ops.I64Or,
			Args:   []uint64{1, 2},
			Result: 3,
		},
		{
			Name:   "xor",
			Op:     ops.I64Xor,
			Args:   []uint64{1, 5},
			Result: 4,
		},
		{
			Name:   "multiply",
			Op:     ops.I64Mul,
			Args:   []uint64{11, 5},
			Result: 55,
		},
		{
			Name:   "shift-left",
			Op:     ops.I64Shl,
			Args:   []uint64{1, 3},
			Result: 8,
		},
		{
			Name:   "shift-right-unsigned",
			Op:     ops.I64ShrU,
			Args:   []uint64{16, 3},
			Result: 2,
		},
		{
			Name:   "shift-right-signed-1",
			Op:     ops.I64ShrS,
			Args:   []uint64{32, 1},
			Result: 16,
		},
		{
			Name:   "shift-right-signed-2",
			Op:     ops.I64ShrS,
			Args:   []uint64{-u64Const(64), 2},
			Result: -u64Const(16),
		},
	}

	allocator := &MMapAllocator{}
	defer allocator.Close()
	b := &AMD64Backend{}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			builder, err := asm.NewBuilder("amd64", 64)
			if err != nil {
				t.Fatal(err)
			}

			b.emitPreamble(builder)
			for _, arg := range tc.Args {
				b.emitPushImmediate(builder, currentInstruction{}, arg)
			}
			switch tc.Op {
			case ops.I64Shl, ops.I64ShrU, ops.I64ShrS:
				b.emitShiftI64(builder, currentInstruction{inst: InstructionMetadata{Op: tc.Op}})
			default:
				b.emitBinaryI64(builder, currentInstruction{inst: InstructionMetadata{Op: tc.Op}})
			}
			b.emitPostamble(builder)
			b.lowerAMD64(builder)
			out := builder.Assemble()

			// debugPrintAsm(out)

			nativeBlock, err := allocator.AllocateExec(out)
			if err != nil {
				t.Fatal(err)
			}

			fakeStack := make([]uint64, 0, 5)
			fakeLocals := make([]uint64, 0, 0)
			nativeBlock.Invoke(&fakeStack, &fakeLocals, nil, nil)

			if got, want := len(fakeStack), 1; got != want {
				t.Fatalf("fakeStack.Len = %d, want %d", got, want)
			}
			if got, want := fakeStack[0], tc.Result; got != want {
				t.Errorf("fakeStack[0] = %d, want %d", got, want)
			}
		})
	}
}

func TestDivOps(t *testing.T) {
	if !supportedOS(runtime.GOOS) {
		t.SkipNow()
	}
	testCases := []struct {
		Name   string
		Op     byte
		Args   []uint64
		Result uint64
	}{
		{
			Name:   "I64-unsigned-divide-1",
			Op:     ops.I64DivU,
			Args:   []uint64{88, 8},
			Result: 11,
		},
		{
			Name:   "I64-unsigned-divide-2",
			Op:     ops.I64DivU,
			Args:   []uint64{7, 2},
			Result: 3,
		},
		{
			Name:   "I64-unsigned-divide-3",
			Op:     ops.I64DivU,
			Args:   []uint64{200, 20},
			Result: 10,
		},
		{
			Name:   "I64-unsigned-remainder-1",
			Op:     ops.I64RemU,
			Args:   []uint64{7, 2},
			Result: 1,
		},
		{
			Name:   "I64-unsigned-remainder-2",
			Op:     ops.I64RemU,
			Args:   []uint64{12345, 12345},
			Result: 0,
		},
		{
			Name:   "I64-signed-divide-1",
			Op:     ops.I64DivS,
			Args:   []uint64{88, 8},
			Result: 11,
		},
		{
			Name:   "I64-signed-divide-2",
			Op:     ops.I64DivS,
			Args:   []uint64{-u64Const(80), 8},
			Result: -u64Const(10),
		},
		{
			Name:   "I64-signed-divide-3",
			Op:     ops.I64DivS,
			Args:   []uint64{-u64Const(80), -u64Const(8)},
			Result: 10,
		},
		{
			Name:   "I64-signed-remainder",
			Op:     ops.I64RemS,
			Args:   []uint64{7, 2},
			Result: 1,
		},
		{
			Name:   "I32-unsigned-divide",
			Op:     ops.I32DivU,
			Args:   []uint64{2<<31 - 1, 2},
			Result: 2<<30 - 1,
		},
		{
			Name:   "I32-signed-divide",
			Op:     ops.I32DivS,
			Args:   []uint64{u32ConstNegated(80), 8},
			Result: u32ConstNegated(10),
		},
		{
			Name:   "I32-unsigned-remainder",
			Op:     ops.I32RemU,
			Args:   []uint64{7, 2},
			Result: 1,
		},
		{
			Name:   "I32-signed-remainder",
			Op:     ops.I32RemS,
			Args:   []uint64{u32ConstNegated(8), u32ConstNegated(6)},
			Result: u32ConstNegated(2),
		},
	}

	allocator := &MMapAllocator{}
	defer allocator.Close()
	b := &AMD64Backend{}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			builder, err := asm.NewBuilder("amd64", 64)
			if err != nil {
				t.Fatal(err)
			}
			b.emitPreamble(builder)

			for _, arg := range tc.Args {
				b.emitPushImmediate(builder, currentInstruction{}, arg)
			}
			b.emitDivide(builder, currentInstruction{inst: InstructionMetadata{Op: tc.Op}})
			b.emitPostamble(builder)
			b.lowerAMD64(builder)
			out := builder.Assemble()

			nativeBlock, err := allocator.AllocateExec(out)
			if err != nil {
				t.Fatal(err)
			}

			fakeStack := make([]uint64, 0, 5)
			fakeLocals := make([]uint64, 0, 0)
			nativeBlock.Invoke(&fakeStack, &fakeLocals, nil, nil)

			if got, want := len(fakeStack), 1; got != want {
				t.Fatalf("fakeStack.Len = %d, want %d", got, want)
			}
			if got, want := fakeStack[0], tc.Result; got != want {
				t.Errorf("fakeStack[0] = %d, want %d", got, want)
			}
		})
	}
}

func TestDivideByZero(t *testing.T) {
	if !supportedOS(runtime.GOOS) {
		t.SkipNow()
	}
	testCases := []struct {
		Name string
		Op   byte
		Args []uint64
	}{
		{
			Name: "I64-unsigned-divide",
			Op:   ops.I64DivU,
			Args: []uint64{88, 0},
		},
		{
			Name: "I64-signed-divide",
			Op:   ops.I64DivS,
			Args: []uint64{88, 0},
		},
		{
			Name: "I32-unsigned-divide",
			Op:   ops.I32DivU,
			Args: []uint64{88, 0},
		},
		{
			Name: "I32-signed-divide",
			Op:   ops.I32DivS,
			Args: []uint64{88, 0},
		},
		{
			Name: "I64-unsigned-rem",
			Op:   ops.I64RemU,
			Args: []uint64{88, 0},
		},
		{
			Name: "I64-signed-rem",
			Op:   ops.I64RemS,
			Args: []uint64{88, 0},
		},
		{
			Name: "I32-unsigned-rem",
			Op:   ops.I32RemU,
			Args: []uint64{88, 0},
		},
		{
			Name: "I32-signed-rem",
			Op:   ops.I32RemS,
			Args: []uint64{88, 0},
		},
	}

	allocator := &MMapAllocator{}
	defer allocator.Close()
	b := &AMD64Backend{}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			builder, err := asm.NewBuilder("amd64", 64)
			if err != nil {
				t.Fatal(err)
			}
			b.emitPreamble(builder)

			for _, arg := range tc.Args {
				b.emitPushImmediate(builder, currentInstruction{}, arg)
			}
			b.emitDivide(builder, currentInstruction{inst: InstructionMetadata{Op: tc.Op}})
			b.emitPostamble(builder)
			b.lowerAMD64(builder)
			out := builder.Assemble()

			nativeBlock, err := allocator.AllocateExec(out)
			if err != nil {
				t.Fatal(err)
			}

			fakeStack := make([]uint64, 0, 5)
			fakeLocals := make([]uint64, 0, 0)
			exit := nativeBlock.Invoke(&fakeStack, &fakeLocals, nil, nil)

			if exit.CompletionStatus() != CompletionDivideZero {
				t.Fatalf("completion status = %v, want CompletionDivideZero", exit.CompletionStatus())
			}
		})
	}
}

func TestComparisonOps64(t *testing.T) {
	if !supportedOS(runtime.GOOS) {
		t.SkipNow()
	}
	testCases := []struct {
		Name   string
		Op     byte
		Args   []uint64
		Result uint64
	}{
		{
			Name:   "equal-1",
			Op:     ops.I64Eq,
			Args:   []uint64{88, 8},
			Result: 0,
		},
		{
			Name:   "equal-2",
			Op:     ops.I64Eq,
			Args:   []uint64{88, 88},
			Result: 1,
		},
		{
			Name:   "not-equal-1",
			Op:     ops.I64Ne,
			Args:   []uint64{88, 88},
			Result: 0,
		},
		{
			Name:   "not-equal-2",
			Op:     ops.I64Ne,
			Args:   []uint64{-u64Const(2), -u64Const(3)},
			Result: 1,
		},
		{
			Name:   "less-than-1",
			Op:     ops.I64LtU,
			Args:   []uint64{2, 2},
			Result: 0,
		},
		{
			Name:   "less-than-2",
			Op:     ops.I64LtU,
			Args:   []uint64{2, 12145},
			Result: 1,
		},
		{
			Name:   "greater-than-1",
			Op:     ops.I64GtU,
			Args:   []uint64{2, 2},
			Result: 0,
		},
		{
			Name:   "greater-than-2",
			Op:     ops.I64GtU,
			Args:   []uint64{4, 2},
			Result: 1,
		},
		{
			Name:   "greater-equal-1",
			Op:     ops.I64GeU,
			Args:   []uint64{2, 2},
			Result: 1,
		},
		{
			Name:   "greater-equal-2",
			Op:     ops.I64GeU,
			Args:   []uint64{2, 4},
			Result: 0,
		},
		{
			Name:   "greater-equal-3",
			Op:     ops.I64GeU,
			Args:   []uint64{2, 1},
			Result: 1,
		},
		{
			Name:   "less-equal-1",
			Op:     ops.I64LeU,
			Args:   []uint64{2, 2},
			Result: 1,
		},
		{
			Name:   "less-equal-2",
			Op:     ops.I64LeU,
			Args:   []uint64{2, 4},
			Result: 1,
		},
		{
			Name:   "less-equal-3",
			Op:     ops.I64LeU,
			Args:   []uint64{2, 1},
			Result: 0,
		},
		{
			Name:   "equal-zero-1",
			Op:     ops.I64Eqz,
			Args:   []uint64{2},
			Result: 0,
		},
		{
			Name:   "equal-zero-2",
			Op:     ops.I64Eqz,
			Args:   []uint64{0},
			Result: 1,
		},
	}

	allocator := &MMapAllocator{}
	defer allocator.Close()
	b := &AMD64Backend{}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			builder, err := asm.NewBuilder("amd64", 64)
			if err != nil {
				t.Fatal(err)
			}
			b.emitPreamble(builder)

			for _, arg := range tc.Args {
				b.emitPushImmediate(builder, currentInstruction{}, arg)
			}
			switch tc.Op {
			case ops.I64Eqz:
				b.emitUnaryComparison(builder, currentInstruction{inst: InstructionMetadata{Op: tc.Op}})
			default:
				b.emitComparison(builder, currentInstruction{inst: InstructionMetadata{Op: tc.Op}})
			}
			b.emitPostamble(builder)
			b.lowerAMD64(builder)
			out := builder.Assemble()
			//debugPrintAsm(out)

			nativeBlock, err := allocator.AllocateExec(out)
			if err != nil {
				t.Fatal(err)
			}

			fakeStack := make([]uint64, 0, 5)
			fakeLocals := make([]uint64, 0, 0)
			nativeBlock.Invoke(&fakeStack, &fakeLocals, nil, nil)

			if got, want := len(fakeStack), 1; got != want {
				t.Fatalf("fakeStack.Len = %d, want %d", got, want)
			}
			if got, want := fakeStack[0], tc.Result; got != want {
				t.Errorf("fakeStack[0] = %d, want %d", got, want)
			}
		})
	}
}

func TestAMD64RHSOptimizations(t *testing.T) {
	if !supportedOS(runtime.GOOS) {
		t.SkipNow()
	}
	tcs := []struct {
		name        string
		candidate   CompilationCandidate
		code        []byte
		meta        *BytecodeMetadata
		checkOffset int
		expected    []byte
	}{
		{
			name: "add",
			code: []byte{ops.I64Const, 3, 0, 0, 0, ops.I64Add},
			candidate: CompilationCandidate{
				EndInstruction: 2,
				End:            6,
			},
			meta: &BytecodeMetadata{
				Instructions: []InstructionMetadata{
					{Op: ops.I64Const, Size: 5},
					{Op: ops.I64Add, Start: 5, Size: 1},
				},
			},
			checkOffset: 0x12,
			expected:    []byte{0x48, 0x83, 0xC0, 0x03}, // add rax, 0x3
		},
		{
			name: "sub",
			code: []byte{ops.I64Const, 6, 0, 0, 0, ops.I64Sub},
			candidate: CompilationCandidate{
				EndInstruction: 2,
				End:            6,
			},
			meta: &BytecodeMetadata{
				Instructions: []InstructionMetadata{
					{Op: ops.I64Const, Size: 5},
					{Op: ops.I64Sub, Start: 5, Size: 1},
				},
			},
			checkOffset: 0x12,
			expected:    []byte{0x48, 0x83, 0xE8, 0x06}, // sub rax, 0x6
		},
		{
			name: "shl",
			code: []byte{ops.I64Const, 6, 0, 0, 0, ops.I64Shl},
			candidate: CompilationCandidate{
				EndInstruction: 2,
				End:            6,
			},
			meta: &BytecodeMetadata{
				Instructions: []InstructionMetadata{
					{Op: ops.I64Const, Size: 5},
					{Op: ops.I64Shl, Start: 5, Size: 1},
				},
			},
			checkOffset: 0x12,
			expected:    []byte{0x48, 0xC1, 0xE0, 0x06}, // shl rax, 0x6
		},
		{
			name: "shrU",
			code: []byte{ops.I64Const, 6, 0, 0, 0, ops.I64ShrU},
			candidate: CompilationCandidate{
				EndInstruction: 2,
				End:            6,
			},
			meta: &BytecodeMetadata{
				Instructions: []InstructionMetadata{
					{Op: ops.I64Const, Size: 5},
					{Op: ops.I64ShrU, Start: 5, Size: 1},
				},
			},
			checkOffset: 0x12,
			expected:    []byte{0x48, 0xC1, 0xE8, 0x06}, // shr rax, 0x6
		},
	}

	allocator := &MMapAllocator{}
	defer allocator.Close()
	b := &AMD64Backend{}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			out, err := b.Build(tc.candidate, tc.code, tc.meta)
			if err != nil {
				t.Fatal(err)
			}

			if len(out) < tc.checkOffset+len(tc.expected) {
				t.Fatalf("len(out) < %d", tc.checkOffset+len(tc.expected))
			}
			if !bytes.Equal(out[tc.checkOffset:tc.checkOffset+len(tc.expected)], tc.expected) {
				t.Errorf("got %v, want %v", out[tc.checkOffset:tc.checkOffset+len(tc.expected)], tc.expected)
			}
		})
	}
}

func TestAMD64OperationsI32(t *testing.T) {
	if !supportedOS(runtime.GOOS) {
		t.SkipNow()
	}
	testCases := []struct {
		Name   string
		Op     byte
		Args   []uint64
		Result uint64
	}{
		{
			Name:   "add",
			Op:     ops.I32Add,
			Args:   []uint64{12, 3},
			Result: 15,
		},
		{
			Name:   "add-overflow",
			Op:     ops.I32Add,
			Args:   []uint64{2<<31 - 1, 2},
			Result: 1,
		},
		{
			Name:   "subtract",
			Op:     ops.I32Sub,
			Args:   []uint64{12, 3},
			Result: 9,
		},
		{
			Name:   "subtract-overflow",
			Op:     ops.I32Sub,
			Args:   []uint64{0, 3},
			Result: 2<<31 - 3,
		},
		{
			Name:   "and",
			Op:     ops.I32And,
			Args:   []uint64{15, 3},
			Result: 3,
		},
		{
			Name:   "or",
			Op:     ops.I32Or,
			Args:   []uint64{1, 2},
			Result: 3,
		},
		{
			Name:   "xor",
			Op:     ops.I32Xor,
			Args:   []uint64{1, 5},
			Result: 4,
		},
		{
			Name:   "multiply",
			Op:     ops.I32Mul,
			Args:   []uint64{5, 5},
			Result: 25,
		},
		{
			Name:   "multiply-overflow",
			Op:     ops.I32Mul,
			Args:   []uint64{2 << 30, 3},
			Result: 2 << 30,
		},
	}

	allocator := &MMapAllocator{}
	defer allocator.Close()
	b := &AMD64Backend{}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			builder, err := asm.NewBuilder("amd64", 64)
			if err != nil {
				t.Fatal(err)
			}

			b.emitPreamble(builder)
			for _, arg := range tc.Args {
				b.emitPushImmediate(builder, currentInstruction{}, arg)
			}

			b.emitBinaryI64(builder, currentInstruction{inst: InstructionMetadata{Op: tc.Op}})
			b.emitPostamble(builder)
			b.lowerAMD64(builder)
			out := builder.Assemble()
			// debugPrintAsm(out)

			nativeBlock, err := allocator.AllocateExec(out)
			if err != nil {
				t.Fatal(err)
			}

			fakeStack := make([]uint64, 0, 5)
			fakeLocals := make([]uint64, 0, 0)
			nativeBlock.Invoke(&fakeStack, &fakeLocals, nil, nil)

			if got, want := len(fakeStack), 1; got != want {
				t.Fatalf("fakeStack.Len = %d, want %d", got, want)
			}
			if got, want := fakeStack[0], tc.Result; got != want {
				t.Errorf("fakeStack[0] = %d, want %d", got, want)
			}
		})
	}
}

func TestAMD64OperationsF64(t *testing.T) {
	if !supportedOS(runtime.GOOS) {
		t.SkipNow()
	}
	testCases := []struct {
		Name   string
		Op     byte
		Args   []uint64
		Result uint64
	}{
		{
			Name:   "f64-add",
			Op:     ops.F64Add,
			Args:   []uint64{math.Float64bits(3), math.Float64bits(5)},
			Result: math.Float64bits(8),
		},
		{
			Name:   "f64-add-negative",
			Op:     ops.F64Add,
			Args:   []uint64{math.Float64bits(3), math.Float64bits(-5)},
			Result: math.Float64bits(-2),
		},
		{
			Name:   "f32-add",
			Op:     ops.F32Add,
			Args:   []uint64{uint64(math.Float32bits(3)), uint64(math.Float32bits(5))},
			Result: uint64(math.Float32bits(8)),
		},
		{
			Name:   "f32-add-negative",
			Op:     ops.F32Add,
			Args:   []uint64{uint64(math.Float32bits(3)), uint64(math.Float32bits(-5))},
			Result: uint64(math.Float32bits(-2)),
		},
		{
			Name:   "f64-sub",
			Op:     ops.F64Sub,
			Args:   []uint64{math.Float64bits(12), math.Float64bits(3)},
			Result: math.Float64bits(9),
		},
		{
			Name:   "f64-sub-negative",
			Op:     ops.F64Sub,
			Args:   []uint64{math.Float64bits(-12.5), math.Float64bits(3)},
			Result: math.Float64bits(-15.5),
		},
		{
			Name:   "f64-sub-negative-2",
			Op:     ops.F64Sub,
			Args:   []uint64{math.Float64bits(12), math.Float64bits(-3)},
			Result: math.Float64bits(15),
		},
		{
			Name:   "f32-sub",
			Op:     ops.F32Sub,
			Args:   []uint64{uint64(math.Float32bits(12)), uint64(math.Float32bits(3))},
			Result: uint64(math.Float32bits(9)),
		},
		{
			Name:   "f32-sub-negative",
			Op:     ops.F32Sub,
			Args:   []uint64{uint64(math.Float32bits(-12.5)), uint64(math.Float32bits(3))},
			Result: uint64(math.Float32bits(-15.5)),
		},
		{
			Name:   "f32-sub-negative-2",
			Op:     ops.F32Sub,
			Args:   []uint64{uint64(math.Float32bits(12)), uint64(math.Float32bits(-3))},
			Result: uint64(math.Float32bits(15)),
		},
		{
			Name:   "f64-divide-1",
			Op:     ops.F64Div,
			Args:   []uint64{math.Float64bits(12), math.Float64bits(3)},
			Result: math.Float64bits(4),
		},
		{
			Name:   "f64-divide-2",
			Op:     ops.F64Div,
			Args:   []uint64{math.Float64bits(1), math.Float64bits(5)},
			Result: math.Float64bits(0.2),
		},
		{
			Name:   "f64-divide-3",
			Op:     ops.F64Div,
			Args:   []uint64{math.Float64bits(-8), math.Float64bits(-2)},
			Result: math.Float64bits(4),
		},
		{
			Name:   "f32-divide-1",
			Op:     ops.F32Div,
			Args:   []uint64{uint64(math.Float32bits(12)), uint64(math.Float32bits(3))},
			Result: uint64(math.Float32bits(4)),
		},
		{
			Name:   "f32-divide-2",
			Op:     ops.F32Div,
			Args:   []uint64{uint64(math.Float32bits(1)), uint64(math.Float32bits(5))},
			Result: uint64(math.Float32bits(0.2)),
		},
		{
			Name:   "f64-multiply-1",
			Op:     ops.F64Mul,
			Args:   []uint64{math.Float64bits(12), math.Float64bits(3)},
			Result: math.Float64bits(36),
		},
		{
			Name:   "f64-multiply-2",
			Op:     ops.F64Mul,
			Args:   []uint64{math.Float64bits(-0.25), math.Float64bits(5)},
			Result: math.Float64bits(-1.25),
		},
		{
			Name:   "f64-multiply-3",
			Op:     ops.F64Mul,
			Args:   []uint64{math.Float64bits(5000), math.Float64bits(2.5)},
			Result: math.Float64bits(12500),
		},
		{
			Name:   "f32-multiply-1",
			Op:     ops.F32Mul,
			Args:   []uint64{uint64(math.Float32bits(12)), uint64(math.Float32bits(3))},
			Result: uint64(math.Float32bits(36)),
		},
		{
			Name:   "f32-multiply-2",
			Op:     ops.F32Mul,
			Args:   []uint64{uint64(math.Float32bits(-0.25)), uint64(math.Float32bits(5))},
			Result: uint64(math.Float32bits(-1.25)),
		},
		{
			Name:   "f64-min-1",
			Op:     ops.F64Min,
			Args:   []uint64{math.Float64bits(5000), math.Float64bits(2.5)},
			Result: math.Float64bits(2.5),
		},
		{
			Name:   "f64-min-2",
			Op:     ops.F64Min,
			Args:   []uint64{math.Float64bits(2.33), math.Float64bits(2.44)},
			Result: math.Float64bits(2.33),
		},
		{
			Name:   "f32-min-1",
			Op:     ops.F32Min,
			Args:   []uint64{uint64(math.Float32bits(5000)), uint64(math.Float32bits(2.5))},
			Result: uint64(math.Float32bits(2.5)),
		},
		{
			Name:   "f32-min-2",
			Op:     ops.F32Min,
			Args:   []uint64{uint64(math.Float32bits(2.33)), uint64(math.Float32bits(2.44))},
			Result: uint64(math.Float32bits(2.33)),
		},
		{
			Name:   "f64-max-1",
			Op:     ops.F64Max,
			Args:   []uint64{math.Float64bits(5000), math.Float64bits(2.5)},
			Result: math.Float64bits(5000),
		},
		{
			Name:   "f64-max-2",
			Op:     ops.F64Max,
			Args:   []uint64{math.Float64bits(2.33), math.Float64bits(2.44)},
			Result: math.Float64bits(2.44),
		},
		{
			Name:   "f32-max-1",
			Op:     ops.F32Max,
			Args:   []uint64{uint64(math.Float32bits(5000)), uint64(math.Float32bits(2.5))},
			Result: uint64(math.Float32bits(5000)),
		},
		{
			Name:   "f32-max-2",
			Op:     ops.F32Max,
			Args:   []uint64{uint64(math.Float32bits(2.33)), uint64(math.Float32bits(2.44))},
			Result: uint64(math.Float32bits(2.44)),
		},
	}

	allocator := &MMapAllocator{}
	defer allocator.Close()
	b := &AMD64Backend{}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			builder, err := asm.NewBuilder("amd64", 64)
			if err != nil {
				t.Fatal(err)
			}

			b.emitPreamble(builder)
			for _, arg := range tc.Args {
				b.emitPushImmediate(builder, currentInstruction{}, arg)
			}

			b.emitBinaryFloat(builder, currentInstruction{inst: InstructionMetadata{Op: tc.Op}})
			b.emitPostamble(builder)
			b.lowerAMD64(builder)
			out := builder.Assemble()
			// debugPrintAsm(out)

			nativeBlock, err := allocator.AllocateExec(out)
			if err != nil {
				t.Fatal(err)
			}

			fakeStack := make([]uint64, 0, 5)
			fakeLocals := make([]uint64, 0, 0)
			nativeBlock.Invoke(&fakeStack, &fakeLocals, nil, nil)

			if got, want := len(fakeStack), 1; got != want {
				t.Fatalf("fakeStack.Len = %d, want %d", got, want)
			}
			if got, want := fakeStack[0], tc.Result; got != want {
				t.Errorf("fakeStack[0] = %d (%v), want %d", got, math.Float64frombits(got), want)
			}
		})
	}
}

func TestComparisonOpsFloat(t *testing.T) {
	if !supportedOS(runtime.GOOS) {
		t.SkipNow()
	}
	testCases := []struct {
		Name   string
		Op     byte
		Args   []uint64
		Result uint64
	}{
		{
			Name:   "f64-equal-1",
			Op:     ops.F64Eq,
			Args:   []uint64{math.Float64bits(2), math.Float64bits(2)},
			Result: 1,
		},
		{
			Name:   "f64-equal-2",
			Op:     ops.F64Eq,
			Args:   []uint64{math.Float64bits(2), math.Float64bits(2.1)},
			Result: 0,
		},
		{
			Name:   "f64-equal-nan-1",
			Op:     ops.F64Eq,
			Args:   []uint64{math.Float64bits(2), math.Float64bits(math.NaN())},
			Result: 0,
		},
		{
			Name:   "f64-equal-nan-2",
			Op:     ops.F64Eq,
			Args:   []uint64{math.Float64bits(math.NaN()), math.Float64bits(math.NaN())},
			Result: 0,
		},
		{
			Name:   "f32-equal-1",
			Op:     ops.F32Eq,
			Args:   []uint64{uint64(math.Float32bits(2)), uint64(math.Float32bits(2))},
			Result: 1,
		},
		{
			Name:   "f32-equal-nan-1",
			Op:     ops.F32Eq,
			Args:   []uint64{uint64(math.Float32bits(2)), uint64(math.Float32bits(float32(math.NaN())))},
			Result: 0,
		},
		{
			Name:   "f64-not-equal-1",
			Op:     ops.F64Ne,
			Args:   []uint64{math.Float64bits(2), math.Float64bits(2)},
			Result: 0,
		},
		{
			Name:   "f64-not-equal-2",
			Op:     ops.F64Ne,
			Args:   []uint64{math.Float64bits(2), math.Float64bits(2.1)},
			Result: 1,
		},
		{
			Name:   "f64-not-equal-nan-1",
			Op:     ops.F64Ne,
			Args:   []uint64{math.Float64bits(math.NaN()), math.Float64bits(2.1)},
			Result: 1,
		},
		{
			Name:   "f64-not-equal-nan-2",
			Op:     ops.F64Ne,
			Args:   []uint64{math.Float64bits(2.1), math.Float64bits(math.NaN())},
			Result: 1,
		},
		{
			Name:   "f32-not-equal-1",
			Op:     ops.F32Ne,
			Args:   []uint64{uint64(math.Float32bits(2)), uint64(math.Float32bits(2))},
			Result: 0,
		},
		{
			Name:   "f32-not-equal-nan-1",
			Op:     ops.F32Ne,
			Args:   []uint64{uint64(math.Float32bits(float32(math.NaN()))), uint64(math.Float32bits(2.1))},
			Result: 1,
		},
		{
			Name:   "f64-less-than-1",
			Op:     ops.F64Lt,
			Args:   []uint64{math.Float64bits(1), math.Float64bits(2)},
			Result: 1,
		},
		{
			Name:   "f64-less-than-2",
			Op:     ops.F64Lt,
			Args:   []uint64{math.Float64bits(-1.1), math.Float64bits(-1.2)},
			Result: 0,
		},
		{
			Name:   "f64-less-than-3",
			Op:     ops.F64Lt,
			Args:   []uint64{math.Float64bits(-1.2), math.Float64bits(-1.2)},
			Result: 0,
		},
		{
			Name:   "f64-less-than-nan",
			Op:     ops.F64Lt,
			Args:   []uint64{math.Float64bits(-1.2), math.Float64bits(math.NaN())},
			Result: 0,
		},
		{
			Name:   "f32-less-than-1",
			Op:     ops.F32Lt,
			Args:   []uint64{uint64(math.Float32bits(1)), uint64(math.Float32bits(2))},
			Result: 1,
		},
		{
			Name:   "f32-less-than-nan",
			Op:     ops.F32Lt,
			Args:   []uint64{uint64(math.Float32bits(-1.2)), uint64(math.Float32bits(float32(math.NaN())))},
			Result: 0,
		},
		{
			Name:   "f64-greater-than-1",
			Op:     ops.F64Gt,
			Args:   []uint64{math.Float64bits(1), math.Float64bits(2)},
			Result: 0,
		},
		{
			Name:   "f64-greater-than-2",
			Op:     ops.F64Gt,
			Args:   []uint64{math.Float64bits(-1.1), math.Float64bits(-1.2)},
			Result: 1,
		},
		{
			Name:   "f64-greater-than-3",
			Op:     ops.F64Gt,
			Args:   []uint64{math.Float64bits(-1.2), math.Float64bits(-1.2)},
			Result: 0,
		},
		{
			Name:   "f32-greater-than-1",
			Op:     ops.F32Gt,
			Args:   []uint64{uint64(math.Float32bits(3)), uint64(math.Float32bits(2))},
			Result: 1,
		},
		{
			Name:   "f64-less-than-equal-1",
			Op:     ops.F64Le,
			Args:   []uint64{math.Float64bits(1), math.Float64bits(2)},
			Result: 1,
		},
		{
			Name:   "f64-less-than-equal-2",
			Op:     ops.F64Le,
			Args:   []uint64{math.Float64bits(-1.1), math.Float64bits(-1.2)},
			Result: 0,
		},
		{
			Name:   "f64-less-than-equal-3",
			Op:     ops.F64Le,
			Args:   []uint64{math.Float64bits(-1.2), math.Float64bits(-1.2)},
			Result: 1,
		},
		{
			Name:   "f32-less-than-equal-1",
			Op:     ops.F32Le,
			Args:   []uint64{uint64(math.Float32bits(1)), uint64(math.Float32bits(2))},
			Result: 1,
		},
		{
			Name:   "f32-less-than-equal-2",
			Op:     ops.F32Le,
			Args:   []uint64{uint64(math.Float32bits(-1.1)), uint64(math.Float32bits(-1.2))},
			Result: 0,
		},
		{
			Name:   "f64-greater-than-equal-1",
			Op:     ops.F64Ge,
			Args:   []uint64{math.Float64bits(1), math.Float64bits(2)},
			Result: 0,
		},
		{
			Name:   "f64-greater-than-equal-2",
			Op:     ops.F64Ge,
			Args:   []uint64{math.Float64bits(-1.1), math.Float64bits(-1.2)},
			Result: 1,
		},
		{
			Name:   "f64-greater-than-equal-3",
			Op:     ops.F64Ge,
			Args:   []uint64{math.Float64bits(-1.2), math.Float64bits(-1.2)},
			Result: 1,
		},
		{
			Name:   "f64-greater-than-equal-4",
			Op:     ops.F64Ge,
			Args:   []uint64{math.Float64bits(2), math.Float64bits(-1)},
			Result: 1,
		},
		{
			Name:   "f32-greater-than-equal-1",
			Op:     ops.F32Ge,
			Args:   []uint64{uint64(math.Float32bits(3)), uint64(math.Float32bits(2))},
			Result: 1,
		},
		{
			Name:   "f32-greater-than-equal-2",
			Op:     ops.F32Ge,
			Args:   []uint64{uint64(math.Float32bits(-1.3)), uint64(math.Float32bits(-1.2))},
			Result: 0,
		},
	}

	allocator := &MMapAllocator{}
	defer allocator.Close()
	b := &AMD64Backend{}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			builder, err := asm.NewBuilder("amd64", 64)
			if err != nil {
				t.Fatal(err)
			}
			b.emitPreamble(builder)

			for _, arg := range tc.Args {
				b.emitPushImmediate(builder, currentInstruction{}, arg)
			}

			b.emitComparisonFloat(builder, currentInstruction{inst: InstructionMetadata{Op: tc.Op}})
			b.emitPostamble(builder)
			b.lowerAMD64(builder)
			out := builder.Assemble()
			// debugPrintAsm(out)

			nativeBlock, err := allocator.AllocateExec(out)
			if err != nil {
				t.Fatal(err)
			}

			fakeStack := make([]uint64, 0, 5)
			fakeLocals := make([]uint64, 0, 0)
			nativeBlock.Invoke(&fakeStack, &fakeLocals, nil, nil)

			if got, want := len(fakeStack), 1; got != want {
				t.Fatalf("fakeStack.Len = %d, want %d", got, want)
			}
			if got, want := fakeStack[0], tc.Result; got != want {
				t.Errorf("fakeStack[0] = %d, want %d", got, want)
			}
		})
	}
}

func TestAMD64IntToFloat(t *testing.T) {
	if !supportedOS(runtime.GOOS) {
		t.SkipNow()
	}
	testCases := []struct {
		Name   string
		Op     byte
		Args   []uint64
		Result uint64
	}{
		{
			Name:   "u64-to-f64",
			Op:     ops.F64ConvertUI64,
			Args:   []uint64{22},
			Result: math.Float64bits(22),
		},
		{
			Name:   "s64-to-f64",
			Op:     ops.F64ConvertUI64,
			Args:   []uint64{-u64Const(80)},
			Result: math.Float64bits(-80),
		},
		{
			Name:   "u32-to-f64",
			Op:     ops.F64ConvertUI32,
			Args:   []uint64{22},
			Result: math.Float64bits(22),
		},
		{
			Name:   "s32-to-f64",
			Op:     ops.F64ConvertUI32,
			Args:   []uint64{u32ConstNegated(80)},
			Result: math.Float64bits(-80),
		},
		{
			Name:   "u64-to-f32",
			Op:     ops.F32ConvertUI64,
			Args:   []uint64{22},
			Result: uint64(math.Float32bits(22)),
		},
		{
			Name:   "s64-to-f32",
			Op:     ops.F32ConvertSI64,
			Args:   []uint64{-u64Const(80)},
			Result: uint64(math.Float32bits(-80)),
		},
		{
			Name:   "u32-to-f32",
			Op:     ops.F32ConvertUI32,
			Args:   []uint64{22},
			Result: uint64(math.Float32bits(22)),
		},
		{
			Name:   "s32-to-f32",
			Op:     ops.F32ConvertSI32,
			Args:   []uint64{u32ConstNegated(80)},
			Result: uint64(math.Float32bits(-80)),
		},
	}

	allocator := &MMapAllocator{}
	defer allocator.Close()
	b := &AMD64Backend{}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			builder, err := asm.NewBuilder("amd64", 64)
			if err != nil {
				t.Fatal(err)
			}

			b.emitPreamble(builder)
			for _, arg := range tc.Args {
				b.emitPushImmediate(builder, currentInstruction{}, arg)
			}

			b.emitConvertIntToFloat(builder, currentInstruction{inst: InstructionMetadata{Op: tc.Op}})
			b.emitPostamble(builder)
			b.lowerAMD64(builder)
			out := builder.Assemble()
			// debugPrintAsm(out)

			nativeBlock, err := allocator.AllocateExec(out)
			if err != nil {
				t.Fatal(err)
			}

			fakeStack := make([]uint64, 0, 5)
			fakeLocals := make([]uint64, 0, 0)
			nativeBlock.Invoke(&fakeStack, &fakeLocals, nil, nil)

			if got, want := len(fakeStack), 1; got != want {
				t.Fatalf("fakeStack.Len = %d, want %d", got, want)
			}
			if got, want := fakeStack[0], tc.Result; got != want {
				t.Errorf("fakeStack[0] = %d (%v), want %d", got, math.Float64frombits(got), want)
			}
		})
	}
}

// TestSliceMemoryLayoutAMD64 tests assumptions about the memory layout
// of slices have not changed. These are not specified in the Go
// spec.
// Specifically, we expect the Go compiler lays out slice headers
// like this:
//    0000: pointer to first element
//    0008: uint64 length of the slice
//    0010: uint64 capacity of the slice.
//
// This test should fail if this ever changes. In that case, stack handling
// instructions that are emitted (emitWasmStackLoad/emitWasmStackPush) will
// need to be revised to match the new memory layout.
func TestSliceMemoryLayoutAMD64(t *testing.T) {
	slice := make([]uint64, 2, 5)
	mem := (*[24]byte)(unsafe.Pointer(&slice))
	if got, want := binary.LittleEndian.Uint64(mem[8:16]), uint64(2); got != want {
		t.Errorf("Got len = %d, want %d", got, want)
	}
	if got, want := binary.LittleEndian.Uint64(mem[16:24]), uint64(5); got != want {
		t.Errorf("Got cap = %d, want %d", got, want)
	}
}

func u64Const(i uint64) uint64 {
	return i
}

func u32ConstNegated(i uint32) uint64 {
	tmp := -i
	return uint64(tmp)
}

func supportedOS(os string) bool {
	if os == "linux" || os == "windows" || os == "darwin" {
		return true
	}
	return false
}
