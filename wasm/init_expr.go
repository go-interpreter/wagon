// Copyright 2017 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wasm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
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

var ErrEmptyInitExpr = errors.New("wasm: Initializer expression produces no value")

type InvalidInitExprOpError byte

func (e InvalidInitExprOpError) Error() string {
	return fmt.Sprintf("wasm: Invalid opcode in initializer expression: %#x", byte(e))
}

type InvalidGlobalIndexError uint32

func (e InvalidGlobalIndexError) Error() string {
	return fmt.Sprintf("wasm: Invalid index to global index space: %#x", uint32(e))
}

func readInitExpr(r io.Reader) ([]byte, error) {
	b := make([]byte, 1)
	buf := new(bytes.Buffer)
	r = io.TeeReader(r, buf)

	// For reading an initializer expression, we parse bytes
	// as if reading WASM code, but convert LEB128 encoded
	// integers into their normal little endian representation
	// One reason why we do not execute it on the fly is that
	// get_global uses indices to the global index space, which
	// might have not been populated when a function reading a module
	// section is calling this.
outer:
	for {
		_, err := io.ReadFull(r, b)
		if err != nil {
			return nil, err
		}

		buf.WriteByte(b[0])
		switch b[0] {
		case i32Const:
			_, err := leb128.ReadVarint32(r)
			if err != nil {
				return nil, err
			}
		case i64Const:
			_, err := leb128.ReadVarint64(r)
			if err != nil {
				return nil, err
			}
		case f32Const:
			var i uint64
			if err := binary.Read(r, binary.LittleEndian, &i); err != nil {
				return nil, err
			}
		case f64Const:
			var i uint64
			if err := binary.Read(r, binary.LittleEndian, &i); err != nil {
				return nil, err
			}
		case getGlobal:
			_, err := leb128.ReadVarUint32(r)
			if err != nil {
				return nil, err
			}
		case end:
			break outer
		default:
			return nil, InvalidInitExprOpError(b[0])
		}
	}

	if buf.Len() == 0 {
		return nil, ErrEmptyInitExpr
	}

	return buf.Bytes(), nil
}

// execInitExpr executes an init_expr and returns a numeral value which can
// either be int32, int64, float32 or float64.
// It returns an error if the initializer expression is inavlid.
func (m *Module) execInitExpr(expr []byte) (interface{}, error) {
	var stack []uint64
	var lastVal ValueType
	r := bytes.NewReader(expr)

	if r.Len() == 0 {
		return nil, ErrEmptyInitExpr
	}

	for {
		b, err := r.ReadByte()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		switch b {
		case i32Const:
			i, err := leb128.ReadVarint32(r)
			if err != nil {
				return nil, err
			}
			stack = append(stack, uint64(i))
			lastVal = ValueTypeI32
		case i64Const:
			i, err := leb128.ReadVarint64(r)
			if err != nil {
				return nil, err
			}
			stack = append(stack, uint64(i))
			lastVal = ValueTypeI64
		case f32Const:
			var i uint64
			if err := binary.Read(r, binary.LittleEndian, &i); err != nil {
				return nil, err
			}
			stack = append(stack, i)
			lastVal = ValueTypeF32
		case f64Const:
			var i uint64
			if err := binary.Read(r, binary.LittleEndian, &i); err != nil {
				return nil, err
			}
			stack = append(stack, i)
			lastVal = ValueTypeF64
		case getGlobal:
			index, err := leb128.ReadVarUint32(r)
			if err != nil {
				return nil, err
			}
			globalVar := m.GetGlobal(int(index))
			if globalVar == nil {
				return nil, InvalidGlobalIndexError(index)
			}
			lastVal = globalVar.Type.Type
		case end:
			break
		default:
			return nil, InvalidInitExprOpError(b)
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
		panic(fmt.Sprintf("Invalid value type produced by initializer expression: %d", int8(lastVal)))
	}
}
