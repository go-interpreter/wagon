// Copyright 2017 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package validate

import (
	"errors"
	"fmt"

	"github.com/go-interpreter/wagon/wasm"
	ops "github.com/go-interpreter/wagon/wasm/operators"
)

// Error wraps validation errors with information about where the error
// was encountered.
type Error struct {
	Offset   int // Byte offset in the bytecode vector where the error occurs.
	Function int // Index into the function index space for the offending function.
	Err      error
}

func (e Error) Error() string {
	return fmt.Sprintf("error while validating function %d at offset %d: %v", e.Function, e.Offset, e.Err)
}

// ErrStackUnderflow is returned if an instruction consumes a value, but there
// are no values on the stack.
var ErrStackUnderflow = errors.New("validate: stack underflow")

// InvalidImmediateError is returned if the immediate value provided
// is invalid for the given instruction.
type InvalidImmediateError struct {
	ImmType string
	OpName  string
}

func (e InvalidImmediateError) Error() string {
	return fmt.Sprintf("invalid immediate for op %s (should be %s)", e.OpName, e.ImmType)
}

// UnmatchedOpError is returned if a block does not have a corresponding
// end instruction, or if an else instruction is encountered outside of
// an if block.
type UnmatchedOpError byte

func (e UnmatchedOpError) Error() string {
	n1, _ := ops.New(byte(e))
	return fmt.Sprintf("encountered unmatched %s", n1.Name)
}

// InvalidTypeError is returned if a branch is encountered which points to
// a block that does not exist.
type InvalidLabelError uint32

func (e InvalidLabelError) Error() string {
	return fmt.Sprintf("invalid nesting depth %d", uint32(e))
}

// UnmatchedIfValueErr is returned if an if block returns a value, but
// no else block is present.
type UnmatchedIfValueErr wasm.ValueType

func (e UnmatchedIfValueErr) Error() string {
	return fmt.Sprintf("if block returns value of type %v but no else present", wasm.ValueType(e))
}

// InvalidTableIndexError is returned if a table is referenced with an
// out-of-bounds index.
type InvalidTableIndexError struct {
	table string
	index uint32
}

func (e InvalidTableIndexError) Error() string {
	return fmt.Sprintf("invalid index %d for %s", e.index, e.table)
}

// InvalidLocalIndexError is returned if a local variable index is referenced
// which does not exist.
type InvalidLocalIndexError uint32

func (e InvalidLocalIndexError) Error() string {
	return fmt.Sprintf("invalid index for local variable %d", uint32(e))
}

// InvalidTypeError is returned if there is a mismatch between the type(s)
// an operator or function accepts, and the value provided.
type InvalidTypeError struct {
	Wanted wasm.ValueType
	Got    wasm.ValueType
}

func valueTypeStr(v wasm.ValueType) string {
	switch v {
	case noReturn:
		return "void"
	case unknownType:
		return "anytype"
	default:
		return v.String()
	}
}

func (e InvalidTypeError) Error() string {
	return fmt.Sprintf("invalid type, got: %v, wanted: %v", valueTypeStr(e.Got), valueTypeStr(e.Wanted))
}

// NoSectionError is returned if a section does not exist.
type NoSectionError wasm.SectionID

func (e NoSectionError) Error() string {
	return fmt.Sprintf("reference to non existant section (id %d) in module", wasm.SectionID(e))
}

// UnbalancedStackErr is returned if there are too many items on the stack
// than is valid for the current block or function.
type UnbalancedStackErr wasm.ValueType

func (e UnbalancedStackErr) Error() string {
	return fmt.Sprintf("unbalanced stack (top of stack is %s)", valueTypeStr(wasm.ValueType(e)))
}
