// Copyright 2020 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wasm_test

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/go-interpreter/wagon/wasm"
)

func TestSectionCustom(t *testing.T) {
	fname := "testdata/custom_funcs_locals.wasm"

	t.Run(filepath.Base(fname), func(t *testing.T) {
		raw, err := ioutil.ReadFile(fname)
		if err != nil {
			t.Fatal(err)
		}

		r := bytes.NewReader(raw)
		m, err := wasm.DecodeModule(r)
		if err != nil {
			t.Fatalf("error reading module %v", err)
		}

		nameCustom := m.Custom("name")
		if nameCustom == nil {
			t.Fatal("can not find name custom section")
		}

		var nSec wasm.NameSection
		err = nSec.UnmarshalWASM(bytes.NewReader(nameCustom.Data))
		if err != nil {
			t.Fatalf("error name Section Unmarshal %v", err)
		}

		// check FunctionNames MarshalWASM func
		if len(nSec.Types[wasm.NameFunction]) == 0 {
			t.Fatalf("%s doesn't have custom FunctionNames section", fname)
		}

		sub, err := nSec.Decode(wasm.NameFunction)
		if err != nil {
			t.Fatalf("error NameSection Decode NameFunction %v", err)
		}

		funcNames, ok := sub.(*wasm.FunctionNames)
		if !ok {
			t.Fatal("error NameSubsection type")
		}

		buf := new(bytes.Buffer)
		err = funcNames.MarshalWASM(buf)
		if err != nil {
			t.Fatalf("error FunctionNames Marshal %v", err)
		}
		if !bytes.Equal(buf.Bytes(), nSec.Types[wasm.NameFunction]) {
			t.Fatal("error Marshal and Unmarshal FunctionNames")
		}

		// check LocalNames MarshalWASM func
		if len(nSec.Types[wasm.NameLocal]) == 0 {
			t.Fatalf("%s doesn't have custom LocalNames section", fname)
		}

		sub, err = nSec.Decode(wasm.NameLocal)
		if err != nil {
			t.Fatalf("error NameSection Decode NameFunction %v", err)
		}

		localNames, ok := sub.(*wasm.LocalNames)
		if !ok {
			t.Fatal("error NameSubsection type")
		}

		buf = new(bytes.Buffer)
		err = localNames.MarshalWASM(buf)
		if err != nil {
			t.Fatalf("error LocalNames Marshal %v", err)
		}
		if !bytes.Equal(buf.Bytes(), nSec.Types[wasm.NameLocal]) {
			t.Fatal("error Marshal and Unmarshal LocalNames")
		}

	})
}
