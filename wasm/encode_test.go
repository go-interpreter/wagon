// Copyright 2018 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wasm_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/go-interpreter/wagon/wasm"
)

var skipModuleFilenames = map[string]bool{
	// Contains debug information section of type Custom. Custom types
	// can be inserted in any order in the sequence, hence the encodings
	// mismatch is valid.
	"rust-basic.wasm": true,
}

func TestEncode(t *testing.T) {
	for _, dir := range testPaths {
		fnames, err := filepath.Glob(filepath.Join(dir, "*.wasm"))
		if err != nil {
			t.Fatal(err)
		}
		for _, fname := range fnames {
			if skipModuleFilenames[path.Base(fname)] {
				continue
			}

			name := fname
			t.Run(filepath.Base(name), func(t *testing.T) {
				raw, err := ioutil.ReadFile(name)
				if err != nil {
					t.Fatal(err)
				}

				r := bytes.NewReader(raw)
				m, err := wasm.DecodeModule(r)
				if err != nil {
					t.Fatalf("error reading module %v", err)
				}
				buf := new(bytes.Buffer)
				err = wasm.EncodeModule(buf, m)
				if err != nil {
					t.Fatalf("error writing module %v", err)
				}
				if !bytes.Equal(buf.Bytes(), raw) {
					ioutil.WriteFile(name+"_got", buf.Bytes(), 0644)
					t.Fatal("modules are different")
				} else {
					os.Remove(name + "_got")
				}
			})
		}
	}
}
