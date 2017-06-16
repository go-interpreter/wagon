package wasm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"

	"github.com/go-interpreter/wagon/wasm/leb128"
)

const (
	i32Const  byte = 0x41
	i64Const  byte = 0x42
	f32Const  byte = 0x43
	f64Const  byte = 0x44
	getGlobal byte = 0x23
	end       byte = 0x0b
)

var ErrEmptyInitExpr = errors.New("Initializer expression produces no value")

func readInitExpr(reader io.Reader) ([]byte, error) {
	b := make([]byte, 1)
	buf := bytes.NewBuffer([]byte{})

	// For reading an initializer expression, we parse bytes
	// as if reading WASM code, but convert LEB128 encoded
	// integers into their normal little endian representation
	// One reason why we do not execute it on the fly is that
	// get_global uses indices to the global index space, which
	// might have not been populated when a function reading a module
	// section is calling this.
outer:
	for {
		_, err := io.ReadFull(reader, b)
		if err != nil {
			return []byte{}, err
		}

		binary.Write(buf, binary.LittleEndian, b[0])
		switch b[0] {
		case i32Const:
			i, err := leb128.ReadVarint32(reader)
			if err != nil {
				return []byte{}, err
			}
			binary.Write(buf, binary.LittleEndian, i)
		case i64Const:
			i, err := leb128.ReadVarint64(reader)
			if err != nil {
				return []byte{}, err
			}
			binary.Write(buf, binary.LittleEndian, i)
		case f32Const:
			var i uint64
			if err := binary.Read(reader, binary.LittleEndian, &i); err != nil {
				return []byte{}, err
			}
			binary.Write(buf, binary.LittleEndian, i)
		case f64Const:
			var i uint64
			if err := binary.Read(reader, binary.LittleEndian, &i); err != nil {
				return []byte{}, err
			}
			binary.Write(buf, binary.LittleEndian, i)
		case getGlobal:
			index, err := leb128.ReadVarUint32(reader)
			if err != nil {
				return []byte{}, err
			}
			binary.Write(buf, binary.LittleEndian, index)
		case end:
			break outer
		default:
			return []byte{}, errors.New("Invalid opecode in initializer expression")
		}
	}

	if buf.Len() == 0 {
		return []byte{}, ErrEmptyInitExpr
	}

	return buf.Bytes(), nil
}

// execInitExpr executes an init_expr and returns a numeral value which can
// either be int32, int64, float32 or float64.
// It returns an error if the initializer expression is inavlid.
func (m *Module) execInitExpr(expr []byte) (interface{}, error) {
	var stack []uint64
	var lastVal ValueType
	reader := bytes.NewReader(expr)

	if reader.Len() == 0 {
		return nil, ErrEmptyInitExpr
	}

	for {
		byte, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		switch byte {
		case i32Const:
			var i int32
			err := binary.Read(reader, binary.LittleEndian, &i)
			if err != nil {
				return nil, err
			}
			stack = append(stack, uint64(i))
			lastVal = ValueTypeI32
		case i64Const:
			var i int64
			err := binary.Read(reader, binary.LittleEndian, &i)
			if err != nil {
				return nil, err
			}
			stack = append(stack, uint64(i))
			lastVal = ValueTypeI64
		case f32Const:
			var i uint64
			if err := binary.Read(reader, binary.LittleEndian, &i); err != nil {
				return nil, err
			}
			stack = append(stack, i)
			lastVal = ValueTypeF32
		case f64Const:
			var i uint64
			if err := binary.Read(reader, binary.LittleEndian, &i); err != nil {
				return nil, err
			}
			stack = append(stack, i)
			lastVal = ValueTypeF64
		case getGlobal:
			var index uint32
			err := binary.Read(reader, binary.LittleEndian, &index)
			if err != nil {
				return nil, err
			}

			globalVar := m.GetGlobal(int(index))
			if globalVar == nil {
				return nil, errors.New("Invalid index to global index space")
			}
			lastVal = globalVar.Type.ContentType
		case end:
			break
		default:
			return nil, errors.New("Invalid opcode in initializer expression")
		}
	}

	v := stack[len(stack)-1]
	switch lastVal {
	case ValueTypeI32:
		return int32(v), nil
	case ValueTypeI64:
		return int64(v), nil
	case ValueTypeF32:
		return math.Float32frombits(uint32(v)), nil
	case ValueTypeF64:
		return math.Float64frombits(uint64(v)), nil
	default:
		panic("Invalid lastVal value")
	}
}
