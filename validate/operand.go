// Copyright 2017 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package validate

import (
	"github.com/go-interpreter/wagon/wasm"
)

const (
	noReturn    = wasm.ValueType(wasm.BlockTypeEmpty)
	unknownType = wasm.ValueType(0)
)

type operand struct {
	Type wasm.ValueType
}

// Equal returns true if the operand and given type are equivalent
// for typechecking purposes.
func (p operand) Equal(t wasm.ValueType) bool {
	if p.Type == unknownType || t == unknownType {
		return true
	}
	return p.Type == t
}
