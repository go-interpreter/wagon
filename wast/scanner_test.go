// Copyright 2017 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wast

import (
	"fmt"
	"testing"
)

func TestScanner(t *testing.T) {
	for _, fname := range []string{
		//		"../exec/testdata/spec/address.wast", // FIXME
		"../exec/testdata/spec/block.wast",
		"../exec/testdata/spec/break-drop.wast",
		"../exec/testdata/spec/br_if.wast",
		//		"../exec/testdata/spec/br_table.wast", // FIXME
		//		"../exec/testdata/spec/br.wast", // FIXME
		"../exec/testdata/spec/call_indirect.wast",
		//		"../exec/testdata/spec/endianness.wast", // FIXME
		"../exec/testdata/spec/fac.wast",
		"../exec/testdata/spec/forward.wast",
		"../exec/testdata/spec/get_local.wast",
		"../exec/testdata/spec/globals.wast",
		"../exec/testdata/spec/if.wast",
		//		"../exec/testdata/spec/loop.wast", // FIXME
		"../exec/testdata/spec/memory_redundancy.wast",
		"../exec/testdata/spec/names.wast",
		"../exec/testdata/spec/nop.wast",
		"../exec/testdata/spec/resizing.wast",
		//		"../exec/testdata/spec/return.wast", // FIXME
		"../exec/testdata/spec/select.wast",
		"../exec/testdata/spec/switch.wast",
		"../exec/testdata/spec/tee_local.wast",
		"../exec/testdata/spec/traps_int_div.wast",
		"../exec/testdata/spec/traps_int_rem.wast",
		//		"../exec/testdata/spec/traps_mem.wast", // FIXME
		"../exec/testdata/spec/unwind.wast",
	} {
		t.Run(fname, func(t *testing.T) {

			s := NewScanner(fname)
			if len(s.Errors) > 0 {
				fmt.Println(s.Errors[0])
				return
			}

			var tok *Token
			tok = s.Next()
			for tok.Kind != EOF {
				fmt.Printf("%d:%d %s\n", tok.Line, tok.Column, tok.String())
				tok = s.Next()
			}
			fmt.Printf("%d:%d %s\n", tok.Line, tok.Column, tok.String())

			for _, err := range s.Errors {
				fmt.Print(err)
			}

			if len(s.Errors) > 0 {
				t.Errorf("wast: failed with %d errors", len(s.Errors))
			}
		})
	}
}
