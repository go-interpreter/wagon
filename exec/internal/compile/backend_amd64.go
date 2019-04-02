// Copyright 2019 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compile

import (
	"encoding/binary"
	"fmt"

	ops "github.com/go-interpreter/wagon/wasm/operators"
	asm "github.com/twitchyliquid64/golang-asm"
	"github.com/twitchyliquid64/golang-asm/obj"
	"github.com/twitchyliquid64/golang-asm/obj/x86"
)

// dirtyRegs hold booleans that are true when the register stores
// a reserved value that needs to be flushed to memory.
type dirtyRegs struct {
	R12 bool
	R13 bool
}

// Details of the AMD64 backend:
// Reserved registers (for now):
//  - R10 - pointer to stack sliceHeader
//  - R11 - pointer to locals sliceHeader
//  - R12 - pointer for stack item
//  - R13 - stack size
// Scratch registers:
//  - RAX, RBX, RCX, RDX, R8, R9, R15
// Most emission instructions make few attempts to optimize in order
// to keep things simple, however a planned second pass peephole-optimizer
//  should make a big difference.

// AMD64Backend is the native compiler backend for x86-64 architectures.
type AMD64Backend struct {
	s *scanner
}

// Scanner returns a scanner that can be used for
// emitting compilation candidates.
func (b *AMD64Backend) Scanner() *scanner {
	if b.s == nil {
		b.s = &scanner{
			supportedOpcodes: map[byte]bool{
				ops.I64Const: true,
				ops.I64Add:   true,
				ops.I64Sub:   true,
				ops.I64And:   true,
				ops.I64Or:    true,
				ops.I64Mul:   true,
				ops.GetLocal: true,
			},
		}
	}
	return b.s
}

// Build implements exec.instructionBuilder.
func (b *AMD64Backend) Build(candidate CompilationCandidate, code []byte, meta *BytecodeMetadata) ([]byte, error) {
	// Pre-allocate 128 instruction objects. This number is arbitrarily chosen,
	// and can be tuned if profiling indicates a bottleneck allocating
	// *obj.Prog objects.
	builder, err := asm.NewBuilder("amd64", 128)
	if err != nil {
		return nil, err
	}
	var regs dirtyRegs
	b.emitPreamble(builder, &regs)

	for i := candidate.StartInstruction; i < candidate.EndInstruction; i++ {
		//fmt.Printf("i=%d, meta=%+v, len=%d\n", i, meta.Instructions[i], len(code))
		inst := meta.Instructions[i]
		switch inst.Op {
		case ops.I64Const:
			b.emitPushI64(builder, &regs, b.readIntImmediate(code, inst))
		case ops.GetLocal:
			b.emitWasmLocalsLoad(builder, &regs, x86.REG_AX, b.readIntImmediate(code, inst))
			b.emitWasmStackPush(builder, &regs, x86.REG_AX)
		case ops.I64Add, ops.I64Sub, ops.I64Mul, ops.I64Or, ops.I64And:
			if err := b.emitBinaryI64(builder, &regs, inst.Op); err != nil {
				return nil, fmt.Errorf("compile: amd64.emitBinaryI64: %v", err)
			}
		default:
			return nil, fmt.Errorf("compile: amd64 backend cannot handle inst[%d].Op 0x%x", i, inst.Op)
		}
	}
	b.emitPostamble(builder, &regs)

	out := builder.Assemble()
	// debugPrintAsm(out)
	return out, nil
}

func (b *AMD64Backend) readIntImmediate(code []byte, meta InstructionMetadata) uint64 {
	if meta.Size == 5 {
		return uint64(binary.LittleEndian.Uint32(code[meta.Start+1 : meta.Start+meta.Size]))
	}
	return binary.LittleEndian.Uint64(code[meta.Start+1 : meta.Start+meta.Size])
}

func (b *AMD64Backend) emitWasmLocalsLoad(builder *asm.Builder, regs *dirtyRegs, reg int16, index uint64) {
	// movq r13, $(index)
	// movq r12, [r11]
	// leaq r12, [r12 + r13*8]
	// movq reg, [r12]

	prog := builder.NewProg()
	prog.As = x86.AMOVQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R13
	prog.From.Type = obj.TYPE_CONST
	prog.From.Offset = int64(index)
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.AMOVQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R12
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_R11
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.ALEAQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R12
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_R12
	prog.From.Scale = 8
	prog.From.Index = x86.REG_R13
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.AMOVQ
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_R12
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = reg
	builder.AddInstruction(prog)
}

func (b *AMD64Backend) emitWasmStackLoad(builder *asm.Builder, regs *dirtyRegs, reg int16) {
	// movq r13,     [r10+8]
	// decq r13
	// movq [r10+8], r13
	// movq r12,     [r10]
	// leaq r12,     [r12 + r13*8]
	// movq reg,     [r12]

	prog := builder.NewProg()
	prog.As = x86.AMOVQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R13
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_R10
	prog.From.Offset = 8
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.ADECQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R13
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.AMOVQ
	prog.From.Type = obj.TYPE_REG
	prog.From.Reg = x86.REG_R13
	prog.To.Type = obj.TYPE_MEM
	prog.To.Reg = x86.REG_R10
	prog.To.Offset = 8
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.AMOVQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R12
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_R10
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.ALEAQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R12
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_R12
	prog.From.Scale = 8
	prog.From.Index = x86.REG_R13
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.AMOVQ
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_R12
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = reg
	builder.AddInstruction(prog)
}

func (b *AMD64Backend) emitWasmStackPush(builder *asm.Builder, regs *dirtyRegs, reg int16) {
	// movq r12,     [r10]
	// movq r13,     [r10+8]
	// leaq r12,     [r12 + r13*8]
	// movq [r12],   reg
	// incq r13
	// movq [r10+8], r13
	prog := builder.NewProg()
	prog.As = x86.AMOVQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R12
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_R10
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.AMOVQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R13
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_R10
	prog.From.Offset = 8
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.ALEAQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R12
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_R12
	prog.From.Scale = 8
	prog.From.Index = x86.REG_R13
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.AMOVQ
	prog.To.Type = obj.TYPE_MEM
	prog.To.Reg = x86.REG_R12
	prog.From.Type = obj.TYPE_REG
	prog.From.Reg = reg
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.AINCQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R13
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.AMOVQ
	prog.From.Type = obj.TYPE_REG
	prog.From.Reg = x86.REG_R13
	prog.To.Type = obj.TYPE_MEM
	prog.To.Reg = x86.REG_R10
	prog.To.Offset = 8
	builder.AddInstruction(prog)
}

func (b *AMD64Backend) emitBinaryI64(builder *asm.Builder, regs *dirtyRegs, op byte) error {
	b.emitWasmStackLoad(builder, regs, x86.REG_R9)
	b.emitWasmStackLoad(builder, regs, x86.REG_AX)

	prog := builder.NewProg()
	prog.From.Type = obj.TYPE_REG
	prog.From.Reg = x86.REG_R9
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_AX
	switch op {
	case ops.I64Add:
		prog.As = x86.AADDQ
	case ops.I64Sub:
		prog.As = x86.ASUBQ
	case ops.I64And:
		prog.As = x86.AANDQ
	case ops.I64Or:
		prog.As = x86.AORQ
	case ops.I64Mul:
		prog.As = x86.AMULQ
		prog.From.Reg = x86.REG_R9
		prog.To.Type = obj.TYPE_NONE
	default:
		return fmt.Errorf("cannot handle op: %x", op)
	}
	builder.AddInstruction(prog)

	b.emitWasmStackPush(builder, regs, x86.REG_AX)
	return nil
}

func (b *AMD64Backend) emitPushI64(builder *asm.Builder, regs *dirtyRegs, c uint64) {
	prog := builder.NewProg()
	prog.As = x86.AMOVQ
	prog.From.Type = obj.TYPE_CONST
	prog.From.Offset = int64(c)
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_AX
	builder.AddInstruction(prog)
	b.emitWasmStackPush(builder, regs, x86.REG_AX)
}

// emitPreamble is currently not needed.
func (b *AMD64Backend) emitPreamble(builder *asm.Builder, regs *dirtyRegs) {
}

func (b *AMD64Backend) emitPostamble(builder *asm.Builder, regs *dirtyRegs) {
	ret := builder.NewProg()
	ret.As = obj.ARET
	builder.AddInstruction(ret)
}
