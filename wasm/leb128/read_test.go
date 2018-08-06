// Copyright 2017 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package leb128

import (
	"bytes"
	"fmt"
	"testing"
)

var casesUint = []struct {
	v uint32
	b []byte
}{
	{b: []byte{0x08}, v: 8},
	{b: []byte{0x80, 0x7f}, v: 16256},
	{b: []byte{0x80, 0x80, 0x80, 0xfd, 0x07}, v: 2141192192},
}

func TestReadVarUint32(t *testing.T) {
	for _, c := range casesUint {
		t.Run(fmt.Sprint(c.v), func(t *testing.T) {
			n, err := ReadVarUint32(bytes.NewReader(c.b))
			if err != nil {
				t.Fatal(err)
			}
			if n != c.v {
				t.Fatalf("got = %d; want = %d", n, c.v)
			}
		})
	}
}

var casesInt = []struct {
	v int64
	b []byte
}{
	{b: []byte{0xff, 0x7e}, v: -129},
	{b: []byte{0xe4, 0x00}, v: 100},
	{b: []byte{0x80, 0x80, 0x80, 0xfd, 0x07}, v: 2141192192},
}

func TestReadVarint32(t *testing.T) {
	for _, c := range casesInt {
		t.Run(fmt.Sprint(c.v), func(t *testing.T) {
			n, err := ReadVarint32(bytes.NewReader(c.b))
			if err != nil {
				t.Fatal(err)
			}
			if n != int32(c.v) {
				t.Fatalf("got = %d; want = %d", n, c.v)
			}
		})
	}
}
