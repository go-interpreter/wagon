// Copyright 2017 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package compile is used internally by wagon to convert standard structured
// WebAssembly bytecode into an unstructured form suitable for execution by
// it's VM.
// The conversion process consists of translating block instruction sequences
// and branch operators (br, br_if, br_table) to absolute jumps to PC values.
// For instance, an instruction sequence like:
//     loop
//       i32.const 1
//       get_local 0
//       i32.add
//       set_local 0
//       get_local 1
//       i32.const 1
//       i32.add
//       tee_local 1
//       get_local 2
//       i32.eq
//       br_if 0
//     end
// Is "compiled" to:
//     i32.const 1
//     i32.add
//     set_local 0
//     get_local 1
//     i32.const 1
//     i32.add
//     tee_local 1
//     get_local 2
//     i32.eq
//     jmpnz <addr> <preserve> <discard>
// Where jmpnz is a jump-if-not-zero operator that takes certain arguments
// plus the jump address as immediates.
// This is in contrast with original WebAssembly bytecode, where the target
// of branch operators are relative block depths instead.
package compile

import (
	"bytes"
	"encoding/binary"

	"github.com/go-interpreter/wagon/disasm"
	ops "github.com/go-interpreter/wagon/wasm/operators"
)

// A small note on the usage of discard instructions:
// A control operator sequence isn't allowed to access nor modify (pop) operands
// that were pushed outside it. Therefore, each sequence has its own stack
// that may or may not push a value to the original stack, depending on the
// block's signature.
// Instead of creating a new stack every time we enter a control structure,
// we record the current stack height on encountering a control operator.
// After we leave the sequence, the stack height is restored using the discard
// operator. A block with a signature will push a value of that type on the parent
// stack (that is, the stack of the parent block where this block started). The
// OpDiscardPreserveTop operator allows us to preserve this value while
// discarding the remaining ones.

// Branches are rewritten as
//     <jmp> <addr>
// Where the address is an 8 byte address, initially set to zero. It is
// later "patched" by patchOffset.

var (
	// OpJmp unconditionally jumps to the provided address.
	OpJmp byte = 0x0c
	// OpJmpZ jumps to the given address if the value at the top of the stack is zero.
	OpJmpZ byte = 0x03
	// OpJmpNz jumps to the given address if the value at the top of the
	// stack is not zero. It also discards elements and optionally preserves
	// the topmost value on the stack
	OpJmpNz byte = 0x0d
	// OpDiscard discards a given number of elements from the execution stack.
	OpDiscard byte = 0x0b
	// OpDiscardPreserveTop discards a given number of elements from the
	// execution stack, while preserving the value on the top of the stack.
	OpDiscardPreserveTop byte = 0x05
)

// Target is the "target" of a br_table instruction.
// Unlike other control instructions, br_table does jumps and discarding all
// by itself.
type Target struct {
	Addr        int64 // The absolute address of the target
	Discard     int64 // The number of elements to discard
	PreserveTop bool  // Whether the top of the stack is to be preserved
}

// BranchTable is the structure pointed to by a rewritten br_table instruction.
// A rewritten br_table instruction is of the format:
//     br_table <table_index>
// where <table_index> is the index to an array of
// BranchTable objects stored by the VM.
type BranchTable struct {
	Targets       []Target // A list of targets, br_table pops an int value, and jumps to Targets[val]
	DefaultTarget Target   // If val > len(Targets), the VM will jump here
	patchedAddrs  []int64  // A list of already patched addresses
	blocksLen     int      // The length of the blocks map in Compile when this table was initialized
}

// block stores the information relevant for a block created by a control operator
// sequence (if...else...end, loop...end, and block...end)
type block struct {
	// the byte offset to which the continuation of the label
	// created by the block operator is located
	// for 'loop', this is the offset of the loop operator itself
	// for 'if', 'else', 'block', this is the 'end' operator
	offset int64

	// Whether this block is created by an 'if' operator
	// in that case, the byte offset to which the address of the
	// else branch is written. This value (0) is overwritten with the correct address
	// when the accompanying 'else' branch/operator is reached
	ifBlock        bool
	elseAddrOffset int64

	// Whether this block is created by a 'loop' operator
	// in that case, the 'offset' field is set at the end of the block
	loopBlock bool

	patchOffsets []int64 // A list of offsets in the bytecode stream that need to be patched with the correct jump addresses

	discard      disasm.StackInfo // Information about the stack created in this block, used while creating Discard instructions
	branchTables []*BranchTable   // All branch tables that were defined in this block.
}

// Compile rewrites WebAssembly bytecode from its disassembly.
// TODO(vibhavp): Add options for optimizing code. Operators like i32.reinterpret/f32
// are no-ops, and can be safely removed.
func Compile(disassembly []disasm.Instr) ([]byte, []*BranchTable) {
	buffer := new(bytes.Buffer)
	branchTables := []*BranchTable{}

	curBlockDepth := -1
	blocks := make(map[int]*block) // maps nesting depths (labels) to blocks
	for _, instr := range disassembly {
		switch instr.Op.Code {
		case ops.I32Load, ops.I64Load, ops.F32Load, ops.F64Load, ops.I32Load8s, ops.I32Load8u, ops.I32Load16s, ops.I32Load16u, ops.I64Load8s, ops.I64Load8u, ops.I64Load16s, ops.I64Load16u, ops.I64Load32s, ops.I64Load32u, ops.I32Store, ops.I64Store, ops.F32Store, ops.F64Store, ops.I32Store8, ops.I32Store16, ops.I64Store8, ops.I64Store16, ops.I64Store32:
			// memory_immediate has two fields, the alignment and the offset.
			// The former is simply an optimization hint and can be safely
			// discarded.
			instr.Immediates = []interface{}{instr.Immediates[1].(uint32)}
		case ops.If:
			curBlockDepth++
			buffer.WriteByte(OpJmpZ)
			blocks[curBlockDepth] = &block{
				ifBlock:        true,
				elseAddrOffset: int64(buffer.Len()),
			}
			// the address to jump to if the condition for `if` is false
			// (i.e the value on the top of the stack is 0)
			binary.Write(buffer, binary.LittleEndian, int64(0))
			continue
		case ops.Loop:
			// there is no condition for entering a loop block
			curBlockDepth++
			blocks[curBlockDepth] = &block{
				offset:    int64(buffer.Len()),
				ifBlock:   false,
				loopBlock: true,
			}
			continue
		case ops.Block:
			curBlockDepth++
			blocks[curBlockDepth] = &block{
				ifBlock: false,
			}
			continue
		case ops.Else:
			// add code for jumping out of a taken if branch
			buffer.WriteByte(OpJmp)
			ifBlockEndOffset := int64(buffer.Len())
			binary.Write(buffer, binary.LittleEndian, int64(0))

			curOffset := int64(buffer.Len())
			ifBlock := blocks[curBlockDepth]
			code := buffer.Bytes()

			buffer = patchOffset(code, ifBlock.elseAddrOffset, curOffset)
			// this is no longer an if block
			ifBlock.ifBlock = false
			ifBlock.patchOffsets = append(ifBlock.patchOffsets, ifBlockEndOffset)
			continue
		case ops.End:
			depth := curBlockDepth
			block := blocks[depth]

			if instr.NewStack.StackTopDiff != 0 {
				// when exiting a block, discard elements to
				// restore stack height.
				if instr.NewStack.PreserveTop {
					// this is true when the block has a
					// signature, and therefore pushes
					// a value on to the stack
					buffer.WriteByte(OpDiscardPreserveTop)
				} else {
					buffer.WriteByte(OpDiscard)
				}
				binary.Write(buffer, binary.LittleEndian, instr.NewStack.StackTopDiff)
			}

			if !block.loopBlock { // is a normal block
				block.offset = int64(buffer.Len())
				if block.ifBlock {
					code := buffer.Bytes()
					buffer = patchOffset(code, block.elseAddrOffset, int64(block.offset))
				}
			}

			for _, offset := range block.patchOffsets {
				code := buffer.Bytes()
				buffer = patchOffset(code, offset, block.offset)
			}

			for _, table := range block.branchTables {
				table.patchTable(table.blocksLen-depth-1, int64(block.offset))
			}

			delete(blocks, curBlockDepth)
			curBlockDepth--
			continue
		case ops.Br:
			if instr.NewStack.StackTopDiff != 0 {
				if instr.NewStack.PreserveTop {
					buffer.WriteByte(OpDiscardPreserveTop)
				} else {
					buffer.WriteByte(OpDiscard)
				}
				binary.Write(buffer, binary.LittleEndian, instr.NewStack.StackTopDiff)
			}
			buffer.WriteByte(OpJmp)
			label := int(instr.Immediates[0].(uint32))
			block := blocks[curBlockDepth-int(label)]
			block.patchOffsets = append(block.patchOffsets, int64(buffer.Len()))
			// write the jump address
			binary.Write(buffer, binary.LittleEndian, int64(0))
			continue
		case ops.BrIf:
			buffer.WriteByte(OpJmpNz)
			label := int(instr.Immediates[0].(uint32))
			block := blocks[curBlockDepth-int(label)]
			block.patchOffsets = append(block.patchOffsets, int64(buffer.Len()))
			// write the jump address
			binary.Write(buffer, binary.LittleEndian, int64(0))
			// write whether we need to preserve the top
			if !instr.NewStack.PreserveTop || instr.NewStack.StackTopDiff == 0 {
				buffer.WriteByte(byte(0))
			} else {
				buffer.WriteByte(byte(1))
			}
			// write the number of elements on the stack we need to discard
			binary.Write(buffer, binary.LittleEndian, instr.NewStack.StackTopDiff)
			continue
		case ops.BrTable:
			branchTable := &BranchTable{
				blocksLen: len(blocks),
			}
			targetCount := instr.Immediates[0].(uint32)
			branchTable.Targets = make([]Target, targetCount)
			for i := range branchTable.Targets {
				label := int64(instr.Immediates[i+1].(uint32))
				branchTable.Targets[i].Addr = label
				block := blocks[curBlockDepth-int(label)]
				branchTable.Targets[i].Discard = block.discard.StackTopDiff
				branchTable.Targets[i].PreserveTop = block.discard.PreserveTop
			}
			defaultLabel := int64(instr.Immediates[len(instr.Immediates)-1].(uint32))
			branchTable.DefaultTarget.Addr = defaultLabel
			defaultBlock := blocks[curBlockDepth-int(defaultLabel)]
			branchTable.DefaultTarget.Discard = defaultBlock.discard.StackTopDiff
			branchTable.DefaultTarget.PreserveTop = defaultBlock.discard.PreserveTop
			branchTables = append(branchTables, branchTable)
			for _, block := range blocks {
				block.branchTables = append(block.branchTables, branchTable)
			}

			buffer.WriteByte(ops.BrTable)
			binary.Write(buffer, binary.LittleEndian, int64(len(branchTables)-1))
		}

		buffer.WriteByte(instr.Op.Code)
		for _, imm := range instr.Immediates {
			err := binary.Write(buffer, binary.LittleEndian, imm)
			if err != nil {
				panic(err)
			}
		}
	}

	for _, table := range branchTables {
		table.patchedAddrs = nil
	}
	return buffer.Bytes(), branchTables
}

// replace the address starting at start with addr
func patchOffset(code []byte, start int64, addr int64) *bytes.Buffer {
	var shift uint
	for i := int64(0); i < 8; i++ {
		code[start+i] = byte(addr >> shift)
		shift += 8
	}

	buf := new(bytes.Buffer)
	buf.Write(code)
	return buf
}

func (table *BranchTable) patchTable(block int, addr int64) {
	if block < 0 {
		panic("Invalid block value")
	}

	for i, target := range table.Targets {
		if !table.isAddr(target.Addr) && target.Addr == int64(block) {
			table.Targets[i].Addr = addr
		}
	}

	if table.DefaultTarget.Addr == int64(block) {
		table.DefaultTarget.Addr = addr
	}
	table.patchedAddrs = append(table.patchedAddrs, addr)
}

// Whether the given value is an instruction (or the block depth)
func (table *BranchTable) isAddr(addr int64) bool {
	for _, t := range table.patchedAddrs {
		if t == addr {
			return true
		}
	}
	return false
}
