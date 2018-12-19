// Copyright 2017 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wasm_test

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/go-interpreter/wagon/exec"
	"github.com/go-interpreter/wagon/wasm"
)

var testPaths = []string{
	"testdata",
	"../exec/testdata",
	"../exec/testdata/spec",
}

func TestReadModule(t *testing.T) {
	for _, dir := range testPaths {
		fnames, err := filepath.Glob(filepath.Join(dir, "*.wasm"))
		if err != nil {
			t.Fatal(err)
		}
		for _, fname := range fnames {
			name := fname
			t.Run(filepath.Base(name), func(t *testing.T) {
				raw, err := ioutil.ReadFile(name)
				if err != nil {
					t.Fatal(err)
				}

				r := bytes.NewReader(raw)
				m, err := wasm.ReadModule(r, nil)
				if err != nil {
					t.Fatalf("error reading module %v", err)
				}
				if m == nil {
					t.Fatalf("error reading module: (nil *Module)")
				}
			})
		}
	}
}

// A list of resolver functions crafter to trigger specific problems
// in module resolution.
var moduleResolvers = map[string]wasm.ResolveFunc{
	"TestModuleSignatureLengthCheck": func(name string) (*wasm.Module, error) {
		// Return an export with the same name but a different signature
		m := wasm.NewModule()
		m.Types = &wasm.SectionTypes{
			Entries: []wasm.FunctionSig{
				{
					ParamTypes:  []wasm.ValueType{wasm.ValueTypeI64},
					ReturnTypes: []wasm.ValueType{},
				},
			},
		}
		m.FunctionIndexSpace = []wasm.Function{
			{
				Sig:  &m.Types.Entries[0],
				Host: reflect.ValueOf(func(p *exec.Process, a int64) {}),
				Body: &wasm.FunctionBody{},
			},
		}

		entries := make(map[string]wasm.ExportEntry)
		entries["finish"] = wasm.ExportEntry{
			FieldStr: "finish",
			Kind:     wasm.ExternalFunction,
			Index:    uint32(0),
		}

		m.Export = &wasm.SectionExports{
			Entries: entries,
		}

		return m, nil
	},
	"TestModuleSignatureParamTypeCheck": func(name string) (*wasm.Module, error) {
		// Return an export with the same name but a different signature
		m := wasm.NewModule()
		m.Types = &wasm.SectionTypes{
			Entries: []wasm.FunctionSig{
				{
					ParamTypes:  []wasm.ValueType{wasm.ValueTypeI64, wasm.ValueTypeI64},
					ReturnTypes: []wasm.ValueType{},
				},
			},
		}
		m.FunctionIndexSpace = []wasm.Function{
			{
				Sig:  &m.Types.Entries[0],
				Host: reflect.ValueOf(func(p *exec.Process, a int64, b int64) {}),
				Body: &wasm.FunctionBody{},
			},
		}

		entries := make(map[string]wasm.ExportEntry)
		entries["finish"] = wasm.ExportEntry{
			FieldStr: "finish",
			Kind:     wasm.ExternalFunction,
			Index:    uint32(0),
		}

		m.Export = &wasm.SectionExports{
			Entries: entries,
		}

		return m, nil
	},
	"TestModuleSignatureReturnTypeCheck": func(name string) (*wasm.Module, error) {
		// Return an export with the same name but a different signature
		m := wasm.NewModule()
		m.Types = &wasm.SectionTypes{
			Entries: []wasm.FunctionSig{
				{
					ParamTypes:  []wasm.ValueType{wasm.ValueTypeI32, wasm.ValueTypeI32},
					ReturnTypes: []wasm.ValueType{wasm.ValueTypeI32},
				},
			},
		}
		m.FunctionIndexSpace = []wasm.Function{
			{
				Sig:  &m.Types.Entries[0],
				Host: reflect.ValueOf(func(p *exec.Process, a int64, b int64) {}),
				Body: &wasm.FunctionBody{},
			},
		}

		entries := make(map[string]wasm.ExportEntry)
		entries["finish"] = wasm.ExportEntry{
			FieldStr: "finish",
			Kind:     wasm.ExternalFunction,
			Index:    uint32(0),
		}

		m.Export = &wasm.SectionExports{
			Entries: entries,
		}

		return m, nil
	},
}

func TestModuleSignatureCheck(t *testing.T) {
	raw, err := ioutil.ReadFile("testdata/nofuncs.wasm")
	if err != nil {
		t.Fatalf("error reading module %v", err)
	}

	for name, resolver := range moduleResolvers {
		t.Run(name, func(t *testing.T) {
			r := bytes.NewReader(raw)
			_, err = wasm.ReadModule(r, resolver)
			if err == nil {
				t.Fatalf("Expected an error while reading the module")
			}
			if got, want := err.Error(), "wasm: invalid signature for import 0x0 with name 'finish' in module ethereum"; got != want {
				t.Fatalf("invalid error. got=%q, want=%q", got, want)
			}
		})
	}
}
