package wasm

import (
	"fmt"
	"io"

	"github.com/go-interpreter/wagon/wasm/leb128"
)

// ValueType represents the type of a valid value in Wasm
type ValueType int8

const (
	ValueTypeI32 ValueType = -0x01
	ValueTypeI64 ValueType = -0x02
	ValueTypeF32 ValueType = -0x03
	ValueTypeF64 ValueType = -0x04
)

var valueTypeStrMap = map[ValueType]string{
	ValueTypeI32: "i32",
	ValueTypeI64: "i64",
	ValueTypeF32: "f32",
	ValueTypeF64: "f64",
}

func (t ValueType) String() string {
	str, ok := valueTypeStrMap[t]
	if !ok {
		str = fmt.Sprintf("<unknown value_type %d>", int8(t))
	}
	return str
}

// TypeFunc represents the value type of a function
const TypeFunc int = -0x20

func readValueType(r io.Reader) (ValueType, error) {
	v, err := leb128.ReadVarint32(r)
	return ValueType(v), err
}

// BlockType represents the signature of a structured block
type BlockType ValueType // varint7
const BlockTypeEmpty BlockType = -0x40

func readBlockType(r io.Reader) (BlockType, error) {
	b, err := leb128.ReadVarint32(r)
	return BlockType(b), err
}

func (b BlockType) String() string {
	if b == BlockTypeEmpty {
		return "<empty block>"
	}
	return ValueType(b).String()
}

// ElemType describes the type of a table's elements
type ElemType int // varint7
// ElemTypeAnyFunc descibres an any_func value
const ElemTypeAnyFunc ElemType = -0x10

func readElemType(r io.Reader) (ElemType, error) {
	b, err := leb128.ReadVarint32(r)
	return ElemType(b), err
}

func (t ElemType) String() string {
	if t == ElemTypeAnyFunc {
		return "anyfunc"
	}

	return "<unknown elem_type>"
}

// FunctionSig describes the signature of a declared function in a WASM module
type FunctionSig struct {
	// value for the 'func` type constructor
	Form int8
	// The parameter types of the function
	ParamTypes  []ValueType
	ReturnTypes []ValueType

	body *FunctionBody
}

// Body returns the corresponding function body of the FunctionSig object, if
// any.
func (f FunctionSig) Body() *FunctionBody {
	return f.body
}

func (f FunctionSig) String() string {
	return fmt.Sprintf("<func %v -> %v>", f.ParamTypes, f.ReturnTypes)
}

type InvalidTypeConstructorError struct {
	Wanted int
	Got    int
}

func (e InvalidTypeConstructorError) Error() string {
	return fmt.Sprintf("wasm: invalid type constructor: wanted %d, got %d", e.Wanted, e.Got)
}

func readFunction(r io.Reader) (FunctionSig, error) {
	f := FunctionSig{}

	form, err := leb128.ReadVarint32(r)
	if err != nil {
		return f, err
	}

	f.Form = int8(form)

	paramCount, err := leb128.ReadVarUint32(r)
	if err != nil {
		return f, err
	}
	f.ParamTypes = make([]ValueType, paramCount)

	for i := uint32(0); i < paramCount; i++ {
		f.ParamTypes[i], err = readValueType(r)
		if err != nil {
			return f, err
		}
	}

	returnCount, err := leb128.ReadVarUint32(r)
	if err != nil {
		return f, err
	}

	for i := uint32(0); i < returnCount; i++ {
		vt, err := readValueType(r)
		if err != nil {
			return f, err
		}
		f.ReturnTypes = append(f.ReturnTypes, vt)
	}

	return f, nil
}

// GlobalVar describes the type and mutability of a declared global variable
type GlobalVar struct {
	// Type of the value
	ContentType ValueType
	Mutable     bool
}

func readGlobalVar(r io.Reader) (*GlobalVar, error) {
	g := &GlobalVar{}
	var err error

	g.ContentType, err = readValueType(r)
	if err != nil {
		return nil, err
	}

	m, err := leb128.ReadVarUint32(r)
	if err != nil {
		return nil, err
	}

	g.Mutable = m == 1

	return g, nil
}

// Table describes a table in a Wasm module.
type Table struct {
	// The type of elements
	ElementType ElemType
	Limits      *ResizableLimits
}

func readTable(r io.Reader) (Table, error) {
	t := Table{}
	var err error

	t.ElementType, err = readElemType(r)
	if err != nil {
		return t, err
	}

	t.Limits, err = readResizableLimits(r)
	return t, err
}

type Memory struct {
	Limits *ResizableLimits
}

func readMemory(r io.Reader) (Memory, error) {
	lim, err := readResizableLimits(r)
	return Memory{lim}, err
}

// External describes the kind of the entry being imported or exported.
type External uint8

const (
	ExternalFunction External = 0
	ExternalTable    External = 1
	ExternalMemory   External = 2
	ExternalGlobal   External = 3
)

func (e External) String() string {
	switch e {
	case ExternalFunction:
		return "function"
	case ExternalTable:
		return "table"
	case ExternalMemory:
		return "memory"
	case ExternalGlobal:
		return "global"
	default:
		return "<unknown external_kind>"
	}
}
func readExternal(r io.Reader) (External, error) {
	bytes, err := readBytes(r, 1)

	return External(bytes[0]), err
}

type ResizableLimits struct {
	// true if the maximum field is present
	Flags bool
	// initial length (in units of table elements or wasm pages)
	Initial uint32
	// non-nil if specified by flags
	Maximum *uint32
}

func readResizableLimits(r io.Reader) (*ResizableLimits, error) {
	lim := &ResizableLimits{
		Maximum: nil,
	}
	f, err := leb128.ReadVarUint32(r)
	if err != nil {
		return nil, err
	}

	lim.Flags = f == 1
	lim.Initial, err = leb128.ReadVarUint32(r)
	if err != nil {
		return nil, err
	}

	if lim.Flags {
		m, err := leb128.ReadVarUint32(r)
		if err != nil {
			return nil, err
		}
		lim.Maximum = &m

	}
	return lim, nil
}
