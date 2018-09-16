// Copyright 2018 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wast_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-interpreter/wagon/wasm"
	"github.com/go-interpreter/wagon/wast"
)

var testPaths = []string{
	"../wasm/testdata",
	"../exec/testdata",
	"../exec/testdata/spec",
}

func TestAssemble(t *testing.T) {
	for _, dir := range testPaths {
		fnames, err := filepath.Glob(filepath.Join(dir, "*.wasm"))
		if err != nil {
			t.Fatal(err)
		}
		for _, fname := range fnames {
			name := fname
			tname := strings.TrimSuffix(name, ".wasm") + ".wat"
			if _, err := os.Stat(tname); err != nil {
				continue
			}
			t.Run(filepath.Base(name), func(t *testing.T) {
				raw, err := ioutil.ReadFile(name)
				if err != nil {
					t.Fatal(err)
				}
				exp, err := ioutil.ReadFile(tname)
				if err != nil {
					t.Fatal(err)
				}

				r := bytes.NewReader(raw)
				m, err := wasm.DecodeModule(r)
				if err != nil {
					t.Fatalf("error reading module %v", err)
				}
				buf := new(bytes.Buffer)
				err = wast.WriteTo(buf, m)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(exp, buf.Bytes()) {
					ioutil.WriteFile(tname+"_got", buf.Bytes(), 0644)
					t.Fatalf("output is different")
				} else {
					os.Remove(tname + "_got")
				}
			})
		}
	}
}
