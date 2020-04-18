// Copyright 2017 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wasm_test

import (
	"bytes"
	"io/ioutil"
	"os"
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

func TestDuplicateExportError_NoStackOverflow(t *testing.T) {
	err := wasm.DuplicateExportError("h")
	_ = err.Error()
}

func TestGetFuntionSig(t *testing.T) {
	f, err := os.Open("testdata/spec/sigtest.wasm")
	if err != nil {
		t.Fatalf("%v", err)
	}
	defer f.Close()
	m, err := wasm.ReadModule(f, nil)
	if err != nil {
		t.Fatalf("error reading module %v", err)
	}

	// check first sig
	fsig, err := m.GetFunctionSig(0)
	if err != nil {
		t.Fatalf("get fsig error")
	}
	if !(len(fsig.ParamTypes) == 1 && fsig.ParamTypes[0] == wasm.ValueTypeI64) {
		t.Fatalf("error param sig, %v", fsig.ParamTypes)
	}
	if !(len(fsig.ReturnTypes) == 1 && fsig.ReturnTypes[0] == wasm.ValueTypeI64) {
		t.Fatalf("error return sig, %v", fsig.ReturnTypes)
	}

	// check second sig
	fsig, err = m.GetFunctionSig(1)
	if err != nil {
		t.Fatalf("get fsig error")
	}
	if !(len(fsig.ParamTypes) == 2 && fsig.ParamTypes[0] == wasm.ValueTypeI32 && fsig.ParamTypes[1] == wasm.ValueTypeI32) {
		t.Fatalf("error param sig, %v", fsig.ParamTypes)
	}
	if !(len(fsig.ReturnTypes) == 1 && fsig.ReturnTypes[0] == wasm.ValueTypeI32) {
		t.Fatalf("error return sig, %v", fsig.ReturnTypes)
	}

	// check third sig
	fsig, err = m.GetFunctionSig(2)
	if err != nil {
		t.Fatalf("get fsig error")
	}
	if !(len(fsig.ParamTypes) == 1 && fsig.ParamTypes[0] == wasm.ValueTypeI32) {
		t.Fatalf("error param sig, %v", fsig.ParamTypes)
	}
	if !(len(fsig.ReturnTypes) == 1 && fsig.ReturnTypes[0] == wasm.ValueTypeI32) {
		t.Fatalf("error return sig, %v", fsig.ReturnTypes)
	}

	// check fourth sig
	fsig, err = m.GetFunctionSig(3)
	if err != nil {
		t.Fatalf("get fsig error")
	}
	if !(len(fsig.ParamTypes) == 0) {
		t.Fatalf("error param sig, %v", fsig.ParamTypes)
	}
	if !(len(fsig.ReturnTypes) == 1) && fsig.ReturnTypes[0] == wasm.ValueTypeI32 {
		t.Fatalf("error return sig, %v", fsig.ReturnTypes)
	}

	fsig, err = m.GetFunctionSig(4)
	if err == nil {
		t.Fatalf("get fsig error")
	}

	// check global var sig
	gsig, err := m.GetGlobalType(0)
	if err != nil {
		t.Fatalf("get global type error")
	}

	if gsig.Type != wasm.ValueTypeI64 {
		t.Fatalf("error global type sig, %v", gsig.Type)
	}

	gsig, err = m.GetGlobalType(1)
	if err != nil {
		t.Fatalf("get global type error")
	}

	if gsig.Type != wasm.ValueTypeI32 {
		t.Fatalf("error global type sig, %v", gsig.Type)
	}

	gsig, err = m.GetGlobalType(2)
	if err == nil {
		t.Fatalf("get global type error")
	}

}

func TestFunctionName(t *testing.T) {
	f, err := os.Open("testdata/hello-world-tinygo.wasm")
	if err != nil {
		t.Fatalf("%v", err)
	}

	defer f.Close()
	m, err := wasm.ReadModule(f, nil)
	if err != nil {
		t.Fatalf("error reading module %v", err)
	}

	var names wasm.NameMap
	if s := m.Custom(wasm.CustomSectionName); s != nil {
		var nSec wasm.NameSection
		err := nSec.UnmarshalWASM(bytes.NewReader(s.Data))
		if err != nil {
			t.Fatalf("error Unmarhsal NameSection %v", err)
		}
		if len(nSec.Types[wasm.NameFunction]) > 0 {
			sub, err := nSec.Decode(wasm.NameFunction)
			if err != nil {
				t.Fatalf("error Decode NameFunction %v", err)
			}
			funcs, ok := sub.(*wasm.FunctionNames)
			if ok {
				names = funcs.Names
			}
		}
	}

	var numImports int
	if m.Import != nil {
		for _, importEntry := range m.Import.Entries {
			if importEntry.Type.Kind() == wasm.ExternalFunction {
				numImports++
			}
		}
	}

	for index, function := range m.FunctionIndexSpace {
		if function.Name != names[uint32(index+numImports)] {
			t.Fatalf("Err:function name expect %s, got %s", names[uint32(index+numImports)], function.Name)
		}
	}
}
