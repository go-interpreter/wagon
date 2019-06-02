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

var rhsConstOptimizable = map[byte]bool{
	ops.I64Add:  true,
	ops.I64Sub:  true,
	ops.I64Shl:  true,
	ops.I64ShrU: true,
}

// dirtyRegs tracks registers which hold values that need to
// be flushed.
type dirtyRegs struct {
	R13 dirtyState
	R14 dirtyState
	R15 dirtyState
}

func (regs *dirtyRegs) flush(builder *asm.Builder, reg uint16) {
	var regState *dirtyState
	switch reg {
	case x86.REG_R13:
		regState = &regs.R13
	default:
		panic(fmt.Sprintf("compile: unknown register: %v", reg))
	}

	switch *regState {
	case stateScratch:
		return
	case stateStackFirstElem, stateLocalFirstElem:
		return // Value does not change - no need to write back.
	case stateStackLen:
		prog := builder.NewProg()
		prog.As = x86.AMOVQ
		prog.From.Type = obj.TYPE_REG
		prog.From.Reg = x86.REG_R13
		prog.To.Type = obj.TYPE_MEM
		prog.To.Reg = x86.REG_R10
		prog.To.Offset = 8
		builder.AddInstruction(prog)
	default:
		panic(fmt.Sprintf("compile: unknown regState: %v", regState))
	}

	*regState = stateScratch
}

// Details of the AMD64 backend:
// Reserved registers (for now):
//  - R10 - pointer to stack sliceHeader
//  - R11 - pointer to locals sliceHeader
//  - R12 - reserved for stack handling
//  - R13 - stack size
// Pseudo-scratch registers (can be used for scratch as long as their
// dirtyState is updated):
//  - R14 (cache's pointer to stack backing array)
//  - R15 (cache's pointer to locals backing array)
// Scratch registers:
//  - RAX, RBX, RCX, RDX, R8, R9
// Most emission instructions make few attempts to optimize in order
// to keep things simple, however a planned second pass peephole-optimizer
//  should make a big difference.

// AMD64Backend is the native compiler backend for x86-64 architectures.
type AMD64Backend struct {
	s *scanner

	EmitBoundsChecks bool
}

// currentInstruction describes the instruction currently being emitted.
type currentInstruction struct {
	idx  int
	inst InstructionMetadata
}

// Scanner returns a scanner that can be used for
// emitting compilation candidates.
func (b *AMD64Backend) Scanner() *scanner {
	if b.s == nil {
		b.s = &scanner{
			supportedOpcodes: map[byte]bool{
				ops.Drop:              true,
				ops.Select:            true,
				ops.I64Const:          true,
				ops.I32Const:          true,
				ops.F64Const:          true,
				ops.F32Const:          true,
				ops.I64Add:            true,
				ops.I32Add:            true,
				ops.I64Sub:            true,
				ops.I32Sub:            true,
				ops.I64And:            true,
				ops.I32And:            true,
				ops.I64Or:             true,
				ops.I32Or:             true,
				ops.I64Xor:            true,
				ops.I32Xor:            true,
				ops.I64Mul:            true,
				ops.I32Mul:            true,
				ops.I64DivU:           true,
				ops.I32DivU:           true,
				ops.I64DivS:           true,
				ops.I32DivS:           true,
				ops.I64RemU:           true,
				ops.I32RemU:           true,
				ops.I64RemS:           true,
				ops.I32RemS:           true,
				ops.GetLocal:          true,
				ops.SetLocal:          true,
				ops.I64Shl:            true,
				ops.I64ShrU:           true,
				ops.I64ShrS:           true,
				ops.I64Eq:             true,
				ops.I64Ne:             true,
				ops.I64LtU:            true,
				ops.I64GtU:            true,
				ops.I64LeU:            true,
				ops.I64GeU:            true,
				ops.I64Eqz:            true,
				ops.F64Add:            true,
				ops.F32Add:            true,
				ops.F64Sub:            true,
				ops.F32Sub:            true,
				ops.F64Div:            true,
				ops.F32Div:            true,
				ops.F64Mul:            true,
				ops.F32Mul:            true,
				ops.F64Min:            true,
				ops.F32Min:            true,
				ops.F64Max:            true,
				ops.F32Max:            true,
				ops.F64Eq:             true,
				ops.F32Eq:             true,
				ops.F64Ne:             true,
				ops.F32Ne:             true,
				ops.F64Lt:             true,
				ops.F32Lt:             true,
				ops.F64Gt:             true,
				ops.F32Gt:             true,
				ops.F64Le:             true,
				ops.F32Le:             true,
				ops.F64Ge:             true,
				ops.F32Ge:             true,
				ops.F64ConvertUI64:    true,
				ops.F64ConvertSI64:    true,
				ops.F32ConvertUI64:    true,
				ops.F32ConvertSI64:    true,
				ops.F64ConvertUI32:    true,
				ops.F64ConvertSI32:    true,
				ops.F32ConvertUI32:    true,
				ops.F32ConvertSI32:    true,
				ops.F64ReinterpretI64: true,
				ops.F32ReinterpretI32: true,
				ops.I64ReinterpretF64: true,
				ops.I32ReinterpretF32: true,
			},
		}
	}
	return b.s
}

func constOp(op byte) bool {
	switch op {
	case ops.I64Const, ops.I32Const, ops.F64Const, ops.F32Const:
		return true
	default:
		return false
	}
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
		ci := currentInstruction{idx: i, inst: inst}

		// Optimization: Const followed by binary instruction: sometimes can be
		// reduced to a single operation.
		if constOp(inst.Op) && (i+1) < candidate.EndInstruction {
			imm := b.readIntImmediate(code, inst)
			nextInst := meta.Instructions[i+1]
			nextCI := currentInstruction{idx: i + 1, inst: nextInst}

			switch _, ok := rhsConstOptimizable[nextInst.Op]; {
			case ok && 0 <= imm && imm < 256:
				if err := b.emitRHSConstOptimizedInstruction(builder, &regs, nextCI, imm); err != nil {
					return nil, fmt.Errorf("compile: amd64.emitRHSConstOptimizedInstruction: %v", err)
				}
				i++
				continue
			case nextInst.Op == ops.SetLocal:
				prog := builder.NewProg()
				prog.As = x86.AMOVQ
				prog.To.Type = obj.TYPE_REG
				prog.To.Reg = x86.REG_AX
				prog.From.Type = obj.TYPE_CONST
				prog.From.Offset = int64(imm)
				builder.AddInstruction(prog)
				b.emitWasmLocalsSave(builder, &regs, nextCI, x86.REG_AX, b.readIntImmediate(code, nextInst))
				i++
				continue
			}
		}

		switch inst.Op {
		case ops.I64Const, ops.I32Const, ops.F64Const, ops.F32Const:
			b.emitPushImmediate(builder, &regs, ci, b.readIntImmediate(code, inst))
		case ops.GetLocal:
			b.emitWasmLocalsLoad(builder, &regs, ci, x86.REG_AX, b.readIntImmediate(code, inst))
			b.emitWasmStackPush(builder, &regs, ci, x86.REG_AX)
		case ops.SetLocal:
			b.emitWasmStackLoad(builder, &regs, ci, x86.REG_AX)
			b.emitWasmLocalsSave(builder, &regs, ci, x86.REG_AX, b.readIntImmediate(code, inst))
		case ops.I64Add, ops.I32Add, ops.I64Sub, ops.I32Sub, ops.I64Mul, ops.I32Mul,
			ops.I64Or, ops.I32Or, ops.I64And, ops.I32And, ops.I64Xor, ops.I32Xor:
			if err := b.emitBinaryI64(builder, &regs, ci); err != nil {
				return nil, fmt.Errorf("compile: amd64.emitBinaryI64: %v", err)
			}
		case ops.I64DivU, ops.I32DivU, ops.I64RemU, ops.I32RemU, ops.I64DivS, ops.I32DivS, ops.I64RemS, ops.I32RemS:
			b.emitDivide(builder, &regs, ci)
		case ops.I64Shl, ops.I64ShrU, ops.I64ShrS:
			if err := b.emitShiftI64(builder, &regs, ci); err != nil {
				return nil, fmt.Errorf("compile: amd64.emitShiftI64: %v", err)
			}
		case ops.I64Eq, ops.I64Ne, ops.I64LtU, ops.I64GtU, ops.I64LeU, ops.I64GeU:
			if err := b.emitComparison(builder, &regs, ci); err != nil {
				return nil, fmt.Errorf("compile: amd64.emitComparison: %v", err)
			}
		case ops.I64Eqz:
			if err := b.emitUnaryComparison(builder, &regs, ci); err != nil {
				return nil, fmt.Errorf("compile: amd64.emitUnaryComparison: %v", err)
			}
		case ops.F64Add, ops.F32Add, ops.F64Sub, ops.F32Sub, ops.F64Div, ops.F32Div, ops.F64Mul, ops.F32Mul,
			ops.F64Min, ops.F32Min, ops.F64Max, ops.F32Max:
			if err := b.emitBinaryFloat(builder, &regs, ci); err != nil {
				return nil, fmt.Errorf("compile: amd64.emitBinaryFloat: %v", err)
			}
		case ops.F64Eq, ops.F64Ne, ops.F64Lt, ops.F64Gt, ops.F64Le, ops.F64Ge,
			ops.F32Eq, ops.F32Ne, ops.F32Lt, ops.F32Gt, ops.F32Le, ops.F32Ge:
			if err := b.emitComparisonFloat(builder, &regs, ci); err != nil {
				return nil, fmt.Errorf("compile: amd64.emitComparisonFloat: %v", err)
			}

		case ops.F64ConvertUI64, ops.F64ConvertSI64, ops.F32ConvertUI64, ops.F32ConvertSI64,
			ops.F64ConvertUI32, ops.F64ConvertSI32, ops.F32ConvertUI32, ops.F32ConvertSI32:
			if err := b.emitConvertIntToFloat(builder, &regs, ci); err != nil {
				return nil, fmt.Errorf("compile: amd64.emitConvertIntToFloat: %v", err)
			}

		case ops.Drop:
			b.emitWasmStackLoad(builder, &regs, ci, x86.REG_AX)
		case ops.Select:
			if err := b.emitSelect(builder, &regs, ci); err != nil {
				return nil, fmt.Errorf("compile: amd64.emitSelect: %v", err)
			}

			// Reinterpret opcodes symbolize type transformations without any actual
			// changes to data on the stack. As such, we treat them as a no-op.
		case ops.F64ReinterpretI64, ops.F32ReinterpretI32, ops.I64ReinterpretF64, ops.I32ReinterpretF32:

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

func (b *AMD64Backend) emitWasmLocalsLoad(builder *asm.Builder, regs *dirtyRegs, ci currentInstruction, reg int16, index uint64) {
	// movq rbx, $(index)
	// movq r15, [r11] (if not cached)
	// leaq r12, [r15 + rbx*8]
	// movq reg, [r12]

	prog := builder.NewProg()
	prog.As = x86.AMOVQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_BX
	prog.From.Type = obj.TYPE_CONST
	prog.From.Offset = int64(index)
	builder.AddInstruction(prog)

	if regs.R15 != stateLocalFirstElem {
		prog = builder.NewProg()
		prog.As = x86.AMOVQ
		prog.To.Type = obj.TYPE_REG
		prog.To.Reg = x86.REG_R15
		prog.From.Type = obj.TYPE_MEM
		prog.From.Reg = x86.REG_R11
		builder.AddInstruction(prog)
		regs.R15 = stateLocalFirstElem
	}

	prog = builder.NewProg()
	prog.As = x86.ALEAQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R12
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_R15
	prog.From.Scale = 8
	prog.From.Index = x86.REG_BX
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.AMOVQ
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_R12
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = reg
	builder.AddInstruction(prog)
}

func (b *AMD64Backend) emitWasmLocalsSave(builder *asm.Builder, regs *dirtyRegs, ci currentInstruction, reg int16, index uint64) {
	// movq rbx, $(index)
	// movq r15, [r11] (if not cached)
	// leaq r12, [r15 + rbx*8]
	// movq [r12], reg

	prog := builder.NewProg()
	prog.As = x86.AMOVQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_BX
	prog.From.Type = obj.TYPE_CONST
	prog.From.Offset = int64(index)
	builder.AddInstruction(prog)

	if regs.R15 != stateLocalFirstElem {
		prog = builder.NewProg()
		prog.As = x86.AMOVQ
		prog.To.Type = obj.TYPE_REG
		prog.To.Reg = x86.REG_R15
		prog.From.Type = obj.TYPE_MEM
		prog.From.Reg = x86.REG_R11
		builder.AddInstruction(prog)
		regs.R15 = stateLocalFirstElem
	}

	prog = builder.NewProg()
	prog.As = x86.ALEAQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R12
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_R15
	prog.From.Scale = 8
	prog.From.Index = x86.REG_BX
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.AMOVQ
	prog.To.Type = obj.TYPE_MEM
	prog.To.Reg = x86.REG_R12
	prog.From.Type = obj.TYPE_REG
	prog.From.Reg = reg
	builder.AddInstruction(prog)
}

func (b *AMD64Backend) emitWasmStackLoad(builder *asm.Builder, regs *dirtyRegs, ci currentInstruction, reg int16) {
	// movq r13,     [r10+8] (if not already loaded)
	// decq r13
	// movq r14,     [r10] (if not already loaded)
	// leaq r12,     [r14 + r13*8]
	// movq reg,     [r12]
	var prog *obj.Prog

	if regs.R13 != stateStackLen {
		prog = builder.NewProg()
		prog.As = x86.AMOVQ
		prog.To.Type = obj.TYPE_REG
		prog.To.Reg = x86.REG_R13
		prog.From.Type = obj.TYPE_MEM
		prog.From.Reg = x86.REG_R10
		prog.From.Offset = 8
		builder.AddInstruction(prog)
		regs.R13 = stateStackLen
	}

	prog = builder.NewProg()
	prog.As = x86.ADECQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R13
	builder.AddInstruction(prog)

	if b.EmitBoundsChecks {
		// movq r12, [r10+16]
		// cmp r12, r13
		// ja endbounds
		// <emitExit() code>
		// endbounds:
		prog = builder.NewProg()
		prog.As = x86.AMOVQ
		prog.To.Type = obj.TYPE_REG
		prog.To.Reg = x86.REG_R12
		prog.From.Type = obj.TYPE_MEM
		prog.From.Reg = x86.REG_R10
		prog.From.Offset = 16
		builder.AddInstruction(prog)

		prog = builder.NewProg()
		prog.As = x86.ACMPQ
		prog.To.Type = obj.TYPE_REG
		prog.To.Reg = x86.REG_R13
		prog.From.Type = obj.TYPE_REG
		prog.From.Reg = x86.REG_R12
		builder.AddInstruction(prog)

		jmp := builder.NewProg()
		jmp.As = x86.AJHI
		jmp.To.Type = obj.TYPE_BRANCH
		builder.AddInstruction(jmp)

		b.emitExit(builder, &dirtyRegs{}, CompletionBadBounds|makeExitIndex(ci.idx))

		prog = builder.NewProg()
		prog.As = obj.ANOP // branch target - assembler will optimize out.
		jmp.Pcond = prog
		builder.AddInstruction(prog)
	}

	if regs.R14 != stateStackFirstElem {
		prog = builder.NewProg()
		prog.As = x86.AMOVQ
		prog.To.Type = obj.TYPE_REG
		prog.To.Reg = x86.REG_R14
		prog.From.Type = obj.TYPE_MEM
		prog.From.Reg = x86.REG_R10
		builder.AddInstruction(prog)
		regs.R14 = stateStackFirstElem
	}

	prog = builder.NewProg()
	prog.As = x86.ALEAQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R12
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_R14
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

func (b *AMD64Backend) emitWasmStackPush(builder *asm.Builder, regs *dirtyRegs, ci currentInstruction, reg int16) {
	// movq r14,     [r10] (if not already loaded)
	// movq r13,     [r10+8] (if not already loaded)
	// leaq r12,     [r14 + r13*8]
	// movq [r12],   reg
	// incq r13

	var prog *obj.Prog
	if regs.R14 != stateStackFirstElem {
		prog = builder.NewProg()
		prog.As = x86.AMOVQ
		prog.To.Type = obj.TYPE_REG
		prog.To.Reg = x86.REG_R14
		prog.From.Type = obj.TYPE_MEM
		prog.From.Reg = x86.REG_R10
		builder.AddInstruction(prog)
		regs.R14 = stateStackFirstElem
	}

	if regs.R13 != stateStackLen {
		prog = builder.NewProg()
		prog.As = x86.AMOVQ
		prog.To.Type = obj.TYPE_REG
		prog.To.Reg = x86.REG_R13
		prog.From.Type = obj.TYPE_MEM
		prog.From.Reg = x86.REG_R10
		prog.From.Offset = 8
		builder.AddInstruction(prog)
		regs.R13 = stateStackLen
	}

	if b.EmitBoundsChecks {
		// movq r12, [r10+16]
		// cmp r12, r13
		// ja endbounds
		// <emitExit() code>
		// endbounds:
		prog = builder.NewProg()
		prog.As = x86.AMOVQ
		prog.To.Type = obj.TYPE_REG
		prog.To.Reg = x86.REG_R12
		prog.From.Type = obj.TYPE_MEM
		prog.From.Reg = x86.REG_R10
		prog.From.Offset = 16
		builder.AddInstruction(prog)

		prog = builder.NewProg()
		prog.As = x86.ACMPQ
		prog.To.Type = obj.TYPE_REG
		prog.To.Reg = x86.REG_R13
		prog.From.Type = obj.TYPE_REG
		prog.From.Reg = x86.REG_R12
		builder.AddInstruction(prog)

		jmp := builder.NewProg()
		jmp.As = x86.AJHI
		jmp.To.Type = obj.TYPE_BRANCH
		builder.AddInstruction(jmp)

		b.emitExit(builder, &dirtyRegs{}, CompletionBadBounds|makeExitIndex(ci.idx))

		prog = builder.NewProg()
		prog.As = obj.ANOP // branch target - assembler will optimize out.
		jmp.Pcond = prog
		builder.AddInstruction(prog)
	}

	prog = builder.NewProg()
	prog.As = x86.ALEAQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R12
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_R14
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
}

func (b *AMD64Backend) emitBinaryI64(builder *asm.Builder, regs *dirtyRegs, ci currentInstruction) error {
	b.emitWasmStackLoad(builder, regs, ci, x86.REG_R9)
	b.emitWasmStackLoad(builder, regs, ci, x86.REG_AX)

	prog := builder.NewProg()
	prog.From.Type = obj.TYPE_REG
	prog.From.Reg = x86.REG_R9
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_AX
	switch ci.inst.Op {
	case ops.I64Add:
		prog.As = x86.AADDQ
	case ops.I32Add:
		prog.As = x86.AADDL
	case ops.I64Sub:
		prog.As = x86.ASUBQ
	case ops.I32Sub:
		prog.As = x86.ASUBL
	case ops.I64And:
		prog.As = x86.AANDQ
	case ops.I32And:
		prog.As = x86.AANDL
	case ops.I64Or:
		prog.As = x86.AORQ
	case ops.I32Or:
		prog.As = x86.AORL
	case ops.I64Xor:
		prog.As = x86.AXORQ
	case ops.I32Xor:
		prog.As = x86.AXORL
	case ops.I64Mul:
		prog.As = x86.AMULQ
		prog.From.Reg = x86.REG_R9
		prog.To.Type = obj.TYPE_NONE
	case ops.I32Mul:
		prog.As = x86.AMULL
		prog.From.Reg = x86.REG_R9
		prog.To.Type = obj.TYPE_NONE
	default:
		return fmt.Errorf("cannot handle op: %x", ci.inst.Op)
	}
	builder.AddInstruction(prog)

	b.emitWasmStackPush(builder, regs, ci, x86.REG_AX)
	return nil
}

func (b *AMD64Backend) emitBinaryFloat(builder *asm.Builder, regs *dirtyRegs, ci currentInstruction) error {
	b.emitWasmStackLoad(builder, regs, ci, x86.REG_X1)
	b.emitWasmStackLoad(builder, regs, ci, x86.REG_X0)

	prog := builder.NewProg()
	prog.From.Type = obj.TYPE_REG
	prog.From.Reg = x86.REG_X1
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_X0
	switch ci.inst.Op {
	case ops.F64Add:
		prog.As = x86.AADDSD
	case ops.F32Add:
		prog.As = x86.AADDSS
	case ops.F64Sub:
		prog.As = x86.ASUBSD
	case ops.F32Sub:
		prog.As = x86.ASUBSS
	case ops.F64Div:
		prog.As = x86.ADIVSD
	case ops.F32Div:
		prog.As = x86.ADIVSS
	case ops.F64Mul:
		prog.As = x86.AMULSD
	case ops.F32Mul:
		prog.As = x86.AMULSS
	case ops.F64Min:
		prog.As = x86.AMINSD
	case ops.F32Min:
		prog.As = x86.AMINSS
	case ops.F64Max:
		prog.As = x86.AMAXSD
	case ops.F32Max:
		prog.As = x86.AMAXSS
	default:
		return fmt.Errorf("cannot handle op: %x", ci.inst.Op)
	}
	builder.AddInstruction(prog)

	b.emitWasmStackPush(builder, regs, ci, x86.REG_X0)
	return nil
}

func (b *AMD64Backend) emitComparisonFloat(builder *asm.Builder, regs *dirtyRegs, ci currentInstruction) error {
	// xor rax, rax
	// XOR is used as that is the fastest way to zero a register,
	// and takes a single cycle on every generation since Pentium.
	prog := builder.NewProg()
	prog.As = x86.AXORQ
	prog.From.Type = obj.TYPE_REG
	prog.From.Reg = x86.REG_AX
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_AX
	builder.AddInstruction(prog)

	b.emitWasmStackLoad(builder, regs, ci, x86.REG_X1)
	b.emitWasmStackLoad(builder, regs, ci, x86.REG_X0)

	// COMISD/COMISS xmm0, xmm1
	prog = builder.NewProg()
	prog.From.Type = obj.TYPE_REG
	prog.From.Reg = x86.REG_X1
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_X0
	switch ci.inst.Op {
	case ops.F64Eq, ops.F64Ne, ops.F64Lt, ops.F64Gt, ops.F64Le, ops.F64Ge:
		prog.As = x86.ACOMISD
	case ops.F32Eq, ops.F32Ne, ops.F32Lt, ops.F32Gt, ops.F32Le, ops.F32Ge:
		prog.As = x86.ACOMISS
	default:
		return fmt.Errorf("cannot handle op: %x", ci.inst.Op)
	}

	builder.AddInstruction(prog)

	// To handle the case where an operand is NaN, we check the parity
	// bit and jump accordingly.
	jmpNaN := builder.NewProg()
	jmpNaN.As = x86.AJPS // jump parity set. Parity is set for NaN computations.
	jmpNaN.To.Type = obj.TYPE_BRANCH
	builder.AddInstruction(jmpNaN)

	// setXX al
	// A set is used instead of conditional moves or branches, as it is the
	// shortest instruction with the least impact on the branch predictor/cache.
	prog = builder.NewProg()
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_AX
	switch ci.inst.Op {
	case ops.F64Eq, ops.F32Eq:
		prog.As = x86.ASETEQ
	case ops.F64Ne, ops.F32Ne:
		prog.As = x86.ASETNE
	case ops.F64Lt, ops.F32Lt:
		prog.As = x86.ASETCS // SETA
	case ops.F64Gt, ops.F32Gt:
		prog.As = x86.ASETHI // SETB
	case ops.F64Le, ops.F32Le:
		prog.As = x86.ASETLS // SETBE
	case ops.F64Ge, ops.F32Ge:
		prog.As = x86.ASETCC // SETAE
	default:
		return fmt.Errorf("cannot handle op: %x", ci.inst.Op)
	}
	builder.AddInstruction(prog)

	// If we got here, the output is not NaN, so
	// skip over code which sets the result for NaN values.
	jmp := builder.NewProg()
	jmp.As = obj.AJMP // jump parity set. Parity is set for NaN computations.
	jmp.To.Type = obj.TYPE_BRANCH
	builder.AddInstruction(jmp)

	// mov rax, $val - should only be jmp'ed to if the value is NaN.
	writeNaNVal := builder.NewProg()
	writeNaNVal.From.Type = obj.TYPE_CONST
	switch ci.inst.Op {
	case ops.F64Ne, ops.F32Ne:
		writeNaNVal.From.Offset = 1 // NaN != dontcare results in True
	default:
		writeNaNVal.From.Offset = 0 // All other ops result in False
	}
	writeNaNVal.To.Type = obj.TYPE_REG
	writeNaNVal.To.Reg = x86.REG_AX
	writeNaNVal.As = x86.AMOVQ
	jmpNaN.Pcond = writeNaNVal
	builder.AddInstruction(writeNaNVal)

	// Symbolic instruction so the not-NaN case can avoid being
	// overwritten. Normal flow (!NaN) results in a jump to here.
	// The assembler will optimize this pseudo-instruction so as
	// to not emit a NOP.
	branchEnd := builder.NewProg()
	branchEnd.As = obj.ANOP
	jmp.Pcond = branchEnd
	builder.AddInstruction(branchEnd)

	b.emitWasmStackPush(builder, regs, ci, x86.REG_AX)
	return nil
}

func (b *AMD64Backend) emitRHSConstOptimizedInstruction(builder *asm.Builder, regs *dirtyRegs, ci currentInstruction, immediate uint64) error {
	b.emitWasmStackLoad(builder, regs, ci, x86.REG_AX)

	prog := builder.NewProg()
	prog.From.Type = obj.TYPE_CONST
	prog.From.Offset = int64(immediate)
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_AX
	switch ci.inst.Op {
	case ops.I64Add:
		prog.As = x86.AADDQ
	case ops.I64Sub:
		prog.As = x86.ASUBQ
	case ops.I64Shl:
		prog.As = x86.ASHLQ
	case ops.I64ShrU:
		prog.As = x86.ASHRQ
	default:
		return fmt.Errorf("cannot handle op: %x", ci.inst.Op)
	}
	builder.AddInstruction(prog)

	b.emitWasmStackPush(builder, regs, ci, x86.REG_AX)
	return nil
}

func (b *AMD64Backend) emitShiftI64(builder *asm.Builder, regs *dirtyRegs, ci currentInstruction) error {
	b.emitWasmStackLoad(builder, regs, ci, x86.REG_CX)
	b.emitWasmStackLoad(builder, regs, ci, x86.REG_AX)

	prog := builder.NewProg()
	prog.From.Type = obj.TYPE_REG
	prog.From.Reg = x86.REG_CX
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_AX
	switch ci.inst.Op {
	case ops.I64Shl:
		prog.As = x86.ASHLQ
	case ops.I64ShrU:
		prog.As = x86.ASHRQ
	case ops.I64ShrS:
		prog.As = x86.ASARQ
	default:
		return fmt.Errorf("cannot handle op: %x", ci.inst.Op)
	}
	builder.AddInstruction(prog)

	b.emitWasmStackPush(builder, regs, ci, x86.REG_AX)
	return nil
}

func (b *AMD64Backend) emitConvertIntToFloat(builder *asm.Builder, regs *dirtyRegs, ci currentInstruction) error {
	b.emitWasmStackLoad(builder, regs, ci, x86.REG_AX)

	prog := builder.NewProg()
	prog.From.Type = obj.TYPE_REG
	prog.From.Reg = x86.REG_AX
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_X0
	switch ci.inst.Op {
	case ops.F64ConvertUI64, ops.F64ConvertSI64:
		prog.As = x86.ACVTSQ2SD
	case ops.F32ConvertUI64, ops.F32ConvertSI64:
		prog.As = x86.ACVTSQ2SS
	case ops.F64ConvertUI32, ops.F64ConvertSI32:
		prog.As = x86.ACVTSL2SD
	case ops.F32ConvertUI32, ops.F32ConvertSI32:
		prog.As = x86.ACVTSL2SS
	default:
		return fmt.Errorf("cannot handle op: %x", ci.inst.Op)
	}
	builder.AddInstruction(prog)

	b.emitWasmStackPush(builder, regs, ci, x86.REG_X0)
	return nil
}

func (b *AMD64Backend) emitPushImmediate(builder *asm.Builder, regs *dirtyRegs, ci currentInstruction, c uint64) {
	prog := builder.NewProg()
	prog.As = x86.AMOVQ
	prog.From.Type = obj.TYPE_CONST
	prog.From.Offset = int64(c)
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_AX
	builder.AddInstruction(prog)
	b.emitWasmStackPush(builder, regs, ci, x86.REG_AX)
}

func (b *AMD64Backend) emitDivide(builder *asm.Builder, regs *dirtyRegs, ci currentInstruction) {
	b.emitWasmStackLoad(builder, regs, ci, x86.REG_R9)
	b.emitWasmStackLoad(builder, regs, ci, x86.REG_AX)

	prog := builder.NewProg()
	prog.As = x86.AXORQ
	prog.From.Type = obj.TYPE_REG
	prog.From.Reg = x86.REG_DX
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_DX
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	switch ci.inst.Op {
	case ops.I64DivU, ops.I64RemU:
		prog.As = x86.ADIVQ
	case ops.I32DivU, ops.I32RemU:
		prog.As = x86.ADIVL
	case ops.I64DivS, ops.I64RemS:
		ext := builder.NewProg()
		ext.As = x86.ACQO
		builder.AddInstruction(ext)
		prog.As = x86.AIDIVQ
	case ops.I32DivS, ops.I32RemS:
		ext := builder.NewProg()
		ext.As = x86.ACDQ
		builder.AddInstruction(ext)
		prog.As = x86.AIDIVL
	default:
		panic(fmt.Sprintf("cannot handle op: %x", ci.inst.Op))
	}
	prog.From.Type = obj.TYPE_REG
	prog.From.Reg = x86.REG_R9
	builder.AddInstruction(prog)

	switch ci.inst.Op {
	case ops.I64DivU, ops.I32DivU, ops.I64DivS, ops.I32DivS:
		b.emitWasmStackPush(builder, regs, ci, x86.REG_AX)
	case ops.I64RemU, ops.I32RemU, ops.I64RemS, ops.I32RemS:
		b.emitWasmStackPush(builder, regs, ci, x86.REG_DX)
	}
}

func (b *AMD64Backend) emitComparison(builder *asm.Builder, regs *dirtyRegs, ci currentInstruction) error {
	b.emitWasmStackLoad(builder, regs, ci, x86.REG_BX)
	b.emitWasmStackLoad(builder, regs, ci, x86.REG_CX)

	// Operands are loaded in BX & CX.
	// Output (1 or 0) is stored in AX, and initialized to 0.
	// A set is used to update the register if the condition
	// is true.

	// xor rax, rax
	// XOR is used as that is the fastest way to zero a register,
	// and takes a single cycle on every generation since Pentium.
	prog := builder.NewProg()
	prog.As = x86.AXORQ
	prog.From.Type = obj.TYPE_REG
	prog.From.Reg = x86.REG_AX
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_AX
	builder.AddInstruction(prog)

	// cmp rbx, rcx
	prog = builder.NewProg()
	prog.As = x86.ACMPQ
	prog.From.Type = obj.TYPE_REG
	prog.From.Reg = x86.REG_CX
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_BX
	builder.AddInstruction(prog)

	// setXX al
	// A set is used instead of conditional moves or branches, as it is the
	// shortest instruction with the least impact on the branch predictor/cache.
	prog = builder.NewProg()
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_AX
	switch ci.inst.Op {
	case ops.I64Eq:
		prog.As = x86.ASETEQ
	case ops.I64Ne:
		prog.As = x86.ASETNE
	case ops.I64LtU:
		prog.As = x86.ASETCS // SETA
	case ops.I64GtU:
		prog.As = x86.ASETHI // SETB
	case ops.I64LeU:
		prog.As = x86.ASETLS // SETBE
	case ops.I64GeU:
		prog.As = x86.ASETCC // SETAE
	default:
		return fmt.Errorf("cannot handle op: %x", ci.inst.Op)
	}
	builder.AddInstruction(prog)

	b.emitWasmStackPush(builder, regs, ci, x86.REG_AX)
	return nil
}

func (b *AMD64Backend) emitUnaryComparison(builder *asm.Builder, regs *dirtyRegs, ci currentInstruction) error {
	b.emitWasmStackLoad(builder, regs, ci, x86.REG_BX)

	prog := builder.NewProg()
	prog.As = x86.AXORQ
	prog.From.Type = obj.TYPE_REG
	prog.From.Reg = x86.REG_AX
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_AX
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.ATESTQ
	prog.From.Type = obj.TYPE_REG
	prog.From.Reg = x86.REG_BX
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_BX
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_AX
	switch ci.inst.Op {
	case ops.I64Eqz:
		prog.As = x86.ASETEQ
	default:
		return fmt.Errorf("cannot handle op: %x", ci.inst.Op)
	}
	builder.AddInstruction(prog)

	b.emitWasmStackPush(builder, regs, ci, x86.REG_AX)
	return nil
}

func (b *AMD64Backend) emitSelect(builder *asm.Builder, regs *dirtyRegs, ci currentInstruction) error {
	b.emitWasmStackLoad(builder, regs, ci, x86.REG_R9)
	b.emitWasmStackLoad(builder, regs, ci, x86.REG_AX)
	b.emitWasmStackLoad(builder, regs, ci, x86.REG_BX)

	prog := builder.NewProg()
	prog.As = x86.ATESTQ
	prog.From.Type = obj.TYPE_REG
	prog.From.Reg = x86.REG_R9
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R9
	builder.AddInstruction(prog)

	cond := builder.NewProg()
	cond.As = x86.AJEQ
	cond.To.Type = obj.TYPE_BRANCH
	builder.AddInstruction(cond)

	b.emitWasmStackPush(builder, regs, ci, x86.REG_BX)
	jmp := builder.NewProg()
	jmp.As = obj.AJMP
	jmp.To.Type = obj.TYPE_BRANCH
	builder.AddInstruction(jmp)

	val2 := builder.NewProg()
	val2.As = obj.ANOP // branch target - assembler will optimize out.
	cond.Pcond = val2
	builder.AddInstruction(val2)
	b.emitWasmStackPush(builder, regs, ci, x86.REG_AX)

	end := builder.NewProg()
	end.As = obj.ANOP // branch target - assembler will optimize out.
	jmp.Pcond = end
	builder.AddInstruction(end)
	return nil
}

// emitPreamble is currently not needed.
func (b *AMD64Backend) emitPreamble(builder *asm.Builder, regs *dirtyRegs) {
}

func (b *AMD64Backend) emitPostamble(builder *asm.Builder, regs *dirtyRegs) {
	b.emitExit(builder, regs, CompletionOK|makeExitIndex(unknownIndex))
}

func (b *AMD64Backend) emitExit(builder *asm.Builder, regs *dirtyRegs, status CompletionStatus) {
	regs.flush(builder, x86.REG_R13)

	retValue := builder.NewProg()
	retValue.As = x86.AMOVQ
	retValue.From.Type = obj.TYPE_CONST
	retValue.From.Offset = int64(status)
	retValue.To.Type = obj.TYPE_MEM
	retValue.To.Reg = x86.REG_SP
	retValue.To.Offset = 32 // Return value - above jitcall()'s arguments
	builder.AddInstruction(retValue)
	ret := builder.NewProg()
	ret.As = obj.ARET
	builder.AddInstruction(ret)
}
