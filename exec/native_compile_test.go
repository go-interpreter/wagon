// Copyright 2019 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !appengine

package exec

import (
	"bytes"
	"runtime"
	"testing"

	"github.com/go-interpreter/wagon/disasm"
	"github.com/go-interpreter/wagon/exec/internal/compile"
	ops "github.com/go-interpreter/wagon/wasm/operators"
)

func fakeNativeCompiler(t *testing.T) *nativeCompiler {
	t.Helper()
	return &nativeCompiler{
		Builder:   &mockInstructionBuilder{},
		Scanner:   &mockSequenceScanner{},
		allocator: &mockPageAllocator{},
	}
}

type mockSequenceScanner struct {
	emit []compile.CompilationCandidate
}

func (s *mockSequenceScanner) ScanFunc(bc []byte, meta *compile.BytecodeMetadata) ([]compile.CompilationCandidate, error) {
	return s.emit, nil
}

type mockPageAllocator struct{}

func (a *mockPageAllocator) AllocateExec(asm []byte) (compile.NativeCodeUnit, error) {
	return nil, nil
}

func (a *mockPageAllocator) Close() error {
	return nil
}

type mockInstructionBuilder struct{}

func (b *mockInstructionBuilder) Build(candidate compile.CompilationCandidate, code []byte, meta *compile.BytecodeMetadata) ([]byte, error) {
	return []byte{byte(candidate.Start), byte(candidate.End)}, nil
}

func TestNativeAsmStructureSetup(t *testing.T) {
	nc := fakeNativeCompiler(t)

	constInst, _ := ops.New(ops.I32Const)
	addInst, _ := ops.New(ops.I32Add)
	subInst, _ := ops.New(ops.I32Sub)
	setGlobalInst, _ := ops.New(ops.SetGlobal)

	wasm, err := disasm.Assemble([]disasm.Instr{
		{Op: constInst, Immediates: []interface{}{int32(1)}},
		{Op: constInst, Immediates: []interface{}{int32(1)}},
		{Op: addInst},
		{Op: setGlobalInst, Immediates: []interface{}{uint32(0)}},

		{Op: constInst, Immediates: []interface{}{int32(8)}},
		{Op: constInst, Immediates: []interface{}{int32(16)}},
		{Op: constInst, Immediates: []interface{}{int32(4)}},
		{Op: addInst},
		{Op: subInst},
	})
	if err != nil {
		t.Fatal(err)
	}

	vm := &VM{
		funcs: []function{
			compiledFunction{
				code: wasm,
			},
		},
		nativeBackend: nc,
	}
	vm.newFuncTable()

	// setup mocks.
	nc.Scanner.(*mockSequenceScanner).emit = []compile.CompilationCandidate{
		// Sequence with single integer op - should not compiled.
		{Start: 0, End: 7, EndInstruction: 4, Metrics: compile.Metrics{IntegerOps: 1}},
		// Sequence with two integer ops - should be emitted.
		{Start: 7, End: 15, StartInstruction: 4, EndInstruction: 10, Metrics: compile.Metrics{IntegerOps: 2}},
	}

	if err := vm.tryNativeCompile(); err != nil {
		t.Fatalf("tryNativeCompile() failed: %v", err)
	}

	// Our scanner emitted two sequences. The first should not have resulted in
	// compilation, but the second should have. Lets check thats the case.
	fn := vm.funcs[0].(compiledFunction)
	if got, want := len(fn.asm), 1; got != want {
		t.Fatalf("len(fn.asm) = %d, want %d", got, want)
	}
	if got, want := int(fn.asm[0].resumePC), 15; got != want {
		t.Errorf("fn.asm[0].resumePC = %v, want %v", got, want)
	}

	// The function bytecode should have been modified to call wagon.nativeExec,
	// with the index of the block (0) following, and remaining bytes set to the
	// unreachable opcode.
	if got, want := fn.code[7], ops.WagonNativeExec; got != want {
		t.Errorf("fn.code[7] = %v, want %v", got, want)
	}
	if got, want := fn.code[8:12], []byte{0, 0, 0, 0}; !bytes.Equal(got, want) {
		t.Errorf("fn.code[8:12] = %v, want %v", got, want)
	}
	for i := 13; i < len(fn.code)-2; i++ {
		if fn.code[i] != ops.Unreachable {
			t.Errorf("fn.code[%d] = %v, want ops.Unreachable", i, fn.code[i])
		}
	}
}

func TestBasicAMD64(t *testing.T) {
	if runtime.GOARCH != "amd64" || runtime.GOOS != "linux" {
		t.SkipNow()
	}

	constInst, _ := ops.New(ops.I64Const)
	addInst, _ := ops.New(ops.I64Add)

	code, meta := compile.Compile([]disasm.Instr{
		{Op: constInst, Immediates: []interface{}{int32(100)}},
		{Op: constInst, Immediates: []interface{}{int32(16)}},
		{Op: constInst, Immediates: []interface{}{int32(4)}},
		{Op: addInst},
		{Op: addInst},
	})
	vm := &VM{
		funcs: []function{
			compiledFunction{
				returns:      true,
				maxDepth:     6,
				code:         code,
				branchTables: meta.BranchTables,
				codeMeta:     meta,
			},
		},
		ctx: context{
			stack: make([]uint64, 0, 6),
		},
	}
	vm.newFuncTable()

	_, be := nativeBackend()
	vm.nativeBackend = be
	originalLen := len(code)
	if err := vm.tryNativeCompile(); err != nil {
		t.Fatalf("tryNativeCompile() failed: %v", err)
	}

	fn := vm.funcs[0].(compiledFunction)
	if want := 1; len(fn.asm) != want {
		t.Fatalf("len(fn.asm) = %d, want %d", len(vm.funcs[0].(compiledFunction).asm), want)
	}
	if want := originalLen - 1; int(fn.asm[0].resumePC) != want {
		t.Errorf("fn.asm[0].stride = %v, want %v", fn.asm[0].resumePC, want)
	}

	// The function bytecode should have been modified to call wagon.nativeExec,
	// with the index of the block (0) following, and remaining bytes set to the
	// unreachable opcode.
	if want := ops.WagonNativeExec; fn.code[0] != want {
		t.Errorf("fn.code[0] = %v, want %v", fn.code[0], want)
	}
	if want := []byte{0, 0, 0, 0}; !bytes.Equal(fn.code[1:5], want) {
		t.Errorf("fn.code[1:5] = %v, want %v", fn.code[1:5], want)
	}
	for i := 6; i < 15; i++ {
		if fn.code[i] != ops.Unreachable {
			t.Errorf("fn.code[%d] = %v, want ops.Unreachable", i, fn.code[i])
		}
	}

	fn.call(vm, 0)
	if len(vm.ctx.stack) != 1 || vm.ctx.stack[0] != 120 {
		t.Errorf("stack = %+v, want [120]", vm.ctx.stack)
	}
}
