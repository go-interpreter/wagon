// Copyright 2020 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec

import (
	"fmt"
	"testing"

	"github.com/go-interpreter/wagon/wasm/operators"
)

func TestF32BinOps(t *testing.T) {
	for _, tc := range []struct {
		opcode byte
		z1     float32
		z2     float32
		want   float32
	}{
		{operators.F32Sub, 3.0, 2.0, 1.0},
		{operators.F32Copysign, 3.0, 2.0, 3.0},
		{operators.F32Copysign, 3.0, -2.0, -3.0},
	} {
		name, err := operators.New(tc.opcode)
		if err != nil {
			t.Fatalf("could not lookup operator 0x%x: %+v", tc.opcode, name)
		}
		t.Run(fmt.Sprintf("%v(%v,%v)", name, tc.z1, tc.z2), func(t *testing.T) {
			vm := new(VM)
			vm.newFuncTable()
			vm.pushFloat32(tc.z1)
			vm.pushFloat32(tc.z2)
			vm.funcTable[tc.opcode]()
			if got, want := vm.popFloat32(), tc.want; got != want {
				t.Fatalf("got=%v, want=%v", got, want)
			}
		})
	}
}

func TestF64BinOps(t *testing.T) {
	for _, tc := range []struct {
		opcode byte
		z1     float64
		z2     float64
		want   float64
	}{
		{operators.F64Sub, 3.0, 2.0, 1.0},
		{operators.F64Copysign, 3.0, 2.0, 3.0},
		{operators.F64Copysign, 3.0, -2.0, -3.0},
	} {
		name, err := operators.New(tc.opcode)
		if err != nil {
			t.Fatalf("could not lookup operator 0x%x: %+v", tc.opcode, name)
		}
		t.Run(fmt.Sprintf("%v(%v,%v)", name, tc.z1, tc.z2), func(t *testing.T) {
			vm := new(VM)
			vm.newFuncTable()
			vm.pushFloat64(tc.z1)
			vm.pushFloat64(tc.z2)
			vm.funcTable[tc.opcode]()
			if got, want := vm.popFloat64(), tc.want; got != want {
				t.Fatalf("got=%v, want=%v", got, want)
			}
		})
	}
}
