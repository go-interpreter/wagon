// Copyright 2018 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package leb128

import (
	"bytes"
	"fmt"
	"testing"
)

func TestWriteVarUint32(t *testing.T) {
	for _, c := range casesUint {
		t.Run(fmt.Sprint(c.v), func(t *testing.T) {
			buf := new(bytes.Buffer)
			_, err := WriteVarUint32(buf, c.v)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(buf.Bytes(), c.b) {
				t.Fatalf("unexpected output: %x", buf.Bytes())
			}
		})
	}
}

func TestWriteVarint64(t *testing.T) {
	for _, c := range casesInt {
		t.Run(fmt.Sprint(c.v), func(t *testing.T) {
			buf := new(bytes.Buffer)
			_, err := WriteVarint64(buf, c.v)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(buf.Bytes(), c.b) {
				t.Fatalf("unexpected output: %x", buf.Bytes())
			}
		})
	}
}
