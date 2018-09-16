// Copyright 2018 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package wast implements a WebAssembly text format.
package wast

// See https://webassembly.github.io/spec/core/text/

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"strconv"

	"github.com/go-interpreter/wagon/disasm"
	"github.com/go-interpreter/wagon/wasm"
	"github.com/go-interpreter/wagon/wasm/operators"
)

const tab = `  `

// WriteTo writes a WASM module in a text representation.
func WriteTo(w io.Writer, m *wasm.Module) error {
	wr, err := newWriter(w, m)
	if err != nil {
		return err
	}
	return wr.writeModule()
}

type writer struct {
	bw *bufio.Writer
	m  *wasm.Module

	fnames  wasm.NameMap
	funcOff int
	err     error
}

func newWriter(w io.Writer, m *wasm.Module) (*writer, error) {
	wr := &writer{bw: bufio.NewWriter(w), m: m}
	if s := m.Custom(wasm.CustomSectionName); s != nil {
		var names wasm.NameSection
		_ = names.UnmarshalWASM(bytes.NewReader(s.Data))
		sub, _ := names.Decode(wasm.NameFunction)
		funcs, ok := sub.(*wasm.FunctionNames)
		if ok {
			wr.fnames = funcs.Names
		}
	}
	return wr, nil
}

func (w *writer) writeModule() error {
	bw := w.bw
	bw.WriteString("(module")

	w.writeTypes()
	w.writeImports()
	w.writeFunctions()
	w.writeGlobals()
	w.writeTables()
	w.writeMemory()
	w.writeExports()
	w.writeElements()
	w.writeData()

	bw.WriteString(")\n")
	if err := bw.Flush(); err != nil {
		return err
	}
	return w.err
}

func (w *writer) writeTypes() {
	if w.m.Types == nil {
		return
	}
	w.WriteString("\n")
	for i, t := range w.m.Types.Entries {
		if i != 0 {
			w.WriteString("\n")
		}
		w.Print(tab+"(type (;%d;) ", i)
		w.writeFuncSignature(t)
		w.WriteString(")")
	}
}

func (w *writer) writeFuncSignature(t wasm.FunctionSig) error {
	w.WriteString("(func")
	defer w.WriteString(")")
	return w.writeFuncType(t)
}

func (w *writer) writeFuncType(t wasm.FunctionSig) error {
	if len(t.ParamTypes) != 0 {
		w.WriteString(" (param")
		for _, p := range t.ParamTypes {
			w.WriteString(" ")
			w.WriteString(p.String())
		}
		w.WriteString(")")
	}
	if len(t.ReturnTypes) != 0 {
		w.WriteString(" (result")
		for _, p := range t.ReturnTypes {
			w.WriteString(" ")
			w.WriteString(p.String())
		}
		w.WriteString(")")
	}
	return nil
}

func (w *writer) writeImports() {
	w.funcOff = 0
	if w.m.Import == nil {
		return
	}
	w.WriteString("\n")
	for i, e := range w.m.Import.Entries {
		if i != 0 {
			w.WriteString("\n")
		}
		w.WriteString(tab + "(import ")
		w.Print("%q %q ", e.ModuleName, e.FieldName)
		switch im := e.Type.(type) {
		case wasm.FuncImport:
			w.Print("(func (;%d;) (type %d))", w.funcOff, im.Type)
			if w.fnames == nil {
				w.fnames = make(wasm.NameMap)
			}
			w.fnames[uint32(w.funcOff)] = e.ModuleName + "." + e.FieldName
			w.funcOff++
		case wasm.TableImport:
			// TODO
		case wasm.MemoryImport:
			// TODO
		case wasm.GlobalVarImport:
			// TODO
		}
		w.WriteString(")")
	}
}

func (w *writer) writeFunctions() {
	if w.m.Function == nil {
		return
	}
	w.WriteString("\n")
	for i, t := range w.m.Function.Types {
		if i != 0 {
			w.WriteString("\n")
		}
		ind := w.funcOff + i
		w.bw.WriteString(tab + "(func ")
		if name, ok := w.fnames[uint32(ind)]; ok {
			w.bw.WriteString("$" + name)
		} else {
			fmt.Fprintf(w.bw, "(;%d;)", ind)
		}
		fmt.Fprintf(w.bw, " (type %d)", int(t))
		if int(t) < len(w.m.Types.Entries) {
			sig := w.m.Types.Entries[t]
			w.writeFuncType(sig)
		}
		if w.m.Code != nil && i < len(w.m.Code.Bodies) {
			b := w.m.Code.Bodies[i]
			if len(b.Locals) > 0 {
				w.WriteString("\n" + tab + tab + "(local")
				for _, l := range b.Locals {
					for i := 0; i < int(l.Count); i++ {
						w.WriteString(" ")
						w.WriteString(l.Type.String())
					}
				}
				w.WriteString(")")
			}
			w.writeCode(b.Code, false)
		}
		w.WriteString(")")
	}
}

func (w *writer) writeGlobals() {
	if w.m.Global == nil {
		return
	}
	for i, e := range w.m.Global.Globals {
		w.WriteString("\n")
		w.WriteString(tab + "(global ")
		w.Print("(;%d;)", i)
		if e.Type.Mutable {
			w.WriteString(" (mut")
		}
		w.Print(" %v", e.Type.Type)
		if e.Type.Mutable {
			w.WriteString(")")
		}
		w.WriteString(" (")
		w.writeCode(e.Init, true)
		w.WriteString("))")
	}
}

func (w *writer) writeTables() {
	if w.m.Table == nil {
		return
	}
	w.WriteString("\n")
	for i, t := range w.m.Table.Entries {
		w.WriteString(tab + "(table ")
		w.Print("(;%d;)", i)
		w.Print(" %d %d ", t.Limits.Initial, t.Limits.Maximum)
		switch t.ElementType {
		case wasm.ElemTypeAnyFunc:
			w.WriteString("anyfunc")
		}
		w.WriteString(")")
	}
}

func (w *writer) writeMemory() {
	if w.m.Memory == nil {
		return
	}
	w.WriteString("\n")
	for i, e := range w.m.Memory.Entries {
		w.WriteString(tab + "(memory ")
		w.Print("(;%d;)", i)
		w.Print(" %d", e.Limits.Initial)
		if e.Limits.Maximum != 0 {
			w.Print(" %d", e.Limits.Initial)
		}
		w.WriteString(")")
	}
}

func (w *writer) writeExports() {
	if w.m.Export == nil {
		return
	}
	w.WriteString("\n")
	for i, k := range w.m.Export.Names {
		e := w.m.Export.Entries[k]
		if i != 0 {
			w.WriteString("\n")
		}
		w.Print(tab+"(export %q (", e.FieldStr)
		switch e.Kind {
		case wasm.ExternalFunction:
			w.WriteString("func")
		case wasm.ExternalMemory:
			w.WriteString("memory")
		case wasm.ExternalTable:
			w.WriteString("table")
		case wasm.ExternalGlobal:
			w.WriteString("global")
		}
		w.Print(" %d))", e.Index)
	}
}

func (w *writer) writeElements() {
	if w.m.Elements == nil {
		return
	}
	for _, d := range w.m.Elements.Entries {
		w.WriteString("\n")
		w.WriteString(tab + "(elem")
		if d.Index != 0 {
			w.Print(" %d", d.Index)
		}
		w.WriteString(" (")
		w.writeCode(d.Offset, true)
		w.WriteString(")")
		for _, v := range d.Elems {
			w.Print(" %d", v)
		}
		w.WriteString(")")
	}
}

func (w *writer) writeData() {
	if w.m.Data == nil {
		return
	}
	for _, d := range w.m.Data.Entries {
		w.WriteString("\n")
		w.WriteString(tab + "(data")
		if d.Index != 0 {
			w.Print(" %d", d.Index)
		}
		w.WriteString(" (")
		w.writeCode(d.Offset, true)
		w.Print(") %s)", quoteData(d.Data))
	}
}

func (w *writer) WriteString(s string) {
	if w.err != nil {
		return
	}
	_, w.err = w.bw.WriteString(s)
}

func (w *writer) Print(format string, args ...interface{}) {
	if w.err != nil {
		return
	}
	_, w.err = fmt.Fprintf(w.bw, format, args...)
}

func quoteData(p []byte) string {
	buf := new(bytes.Buffer)
	buf.WriteRune('"')
	for _, b := range p {
		if strconv.IsGraphic(rune(b)) && b < 0xa0 && b != '"' && b != '\\' {
			buf.WriteByte(b)
		} else {
			s := strconv.FormatInt(int64(b), 16)
			if len(s) == 1 {
				s = "0" + s
			}
			buf.WriteString(`\` + s)
		}
	}
	buf.WriteRune('"')
	return buf.String()
}

func (w *writer) writeCode(code []byte, isInit bool) {
	if w.err != nil {
		return
	}
	instr, err := disasm.Disassemble(code)
	if err != nil {
		w.err = err
		return
	}
	tabs := 2
	block := 0
	writeBlock := func(d int) {
		w.Print(" %d (;@%d;)", d, block-d)
	}
	hadEnd := false
	for i, ins := range instr {
		if !isInit {
			w.WriteString("\n")
		}
		switch ins.Op.Code {
		case operators.End, operators.Else:
			tabs--
			block--
		}
		if isInit && !hadEnd && ins.Op.Code == operators.End {
			hadEnd = true
			continue
		}
		if isInit {
			if i > 0 {
				w.WriteString(" ")
			}
		} else {
			for i := 0; i < tabs; i++ {
				w.WriteString(tab)
			}
		}
		w.WriteString(ins.Op.Name)
		switch ins.Op.Code {
		case operators.Else:
			tabs++
			block++
		case operators.Block, operators.Loop, operators.If:
			tabs++
			block++
			b := ins.Immediates[0].(wasm.BlockType)
			if b != wasm.BlockTypeEmpty {
				w.WriteString(" (result ")
				w.WriteString(b.String())
				w.WriteString(")")
			}
			w.Print("  ;; label = @%d", block)
			continue
		case operators.F32Const:
			i1 := ins.Immediates[0].(float32)
			w.WriteString(" " + formatFloat32(i1))
			continue
		case operators.F64Const:
			i1 := ins.Immediates[0].(float64)
			w.WriteString(" " + formatFloat64(i1))
			continue
		case operators.BrIf, operators.Br:
			i1 := ins.Immediates[0].(uint32)
			writeBlock(int(i1))
			continue
		case operators.BrTable:
			n := ins.Immediates[0].(uint32)
			for i := 0; i < int(n); i++ {
				v := ins.Immediates[i+1].(uint32)
				writeBlock(int(v))
			}
			def := ins.Immediates[n+1].(uint32)
			writeBlock(int(def))
			continue
		case operators.Call:
			i1 := ins.Immediates[0].(uint32)
			if name, ok := w.fnames[i1]; ok {
				w.WriteString(" $")
				w.WriteString(name)
			} else {
				w.Print(" %v", i1)
			}
			continue
		case operators.CallIndirect:
			i1 := ins.Immediates[0].(uint32)
			w.Print(" (type %d)", i1)
			continue
		case operators.CurrentMemory, operators.GrowMemory:
			r := ins.Immediates[0].(uint8)
			if r == 0 {
				continue
			}
		case operators.I32Store, operators.I64Store,
			operators.I32Store8, operators.I64Store8,
			operators.I32Store16, operators.I64Store16,
			operators.I64Store32,
			operators.F32Store, operators.F64Store,
			operators.I32Load, operators.I64Load,
			operators.I32Load8u, operators.I32Load8s,
			operators.I32Load16u, operators.I32Load16s,
			operators.I64Load8u, operators.I64Load8s,
			operators.I64Load16u, operators.I64Load16s,
			operators.I64Load32u, operators.I64Load32s,
			operators.F32Load, operators.F64Load:

			i1 := ins.Immediates[0].(uint32)
			i2 := ins.Immediates[1].(uint32)
			dst := 0 // in log 2 (i8)
			switch ins.Op.Code {
			case operators.I64Load, operators.I64Store,
				operators.F64Load, operators.F64Store:
				dst = 3
			case operators.I32Load, operators.I64Load32s, operators.I64Load32u,
				operators.I32Store, operators.I64Store32,
				operators.F32Load, operators.F32Store:
				dst = 2
			case operators.I32Load16u, operators.I32Load16s, operators.I64Load16u, operators.I64Load16s,
				operators.I32Store16, operators.I64Store16:
				dst = 1
			case operators.I32Load8u, operators.I32Load8s, operators.I64Load8u, operators.I64Load8s,
				operators.I32Store8, operators.I64Store8:
				dst = 0
			}
			if i2 != 0 {
				w.Print(" offset=%d", i2)
			}
			if int(i1) != dst {
				w.Print(" align=%d", 1<<i1)
			}
			continue
		}
		for _, a := range ins.Immediates {
			w.WriteString(" ")
			w.Print("%v", a)
		}
	}
}

func formatFloat32(v float32) string {
	s := ""
	if v == float32(int32(v)) {
		s = strconv.FormatInt(int64(v), 10)
	} else {
		s = strconv.FormatFloat(float64(v), 'g', -1, 32)
	}
	return fmt.Sprintf("%#0x (;=%s;)", math.Float32bits(v), s)
}

func formatFloat64(v float64) string {
	// TODO: https://github.com/WebAssembly/wabt/blob/master/src/literal.cc (FloatWriter<T>::WriteHex)
	s := ""
	if v == float64(int64(v)) {
		s = strconv.FormatInt(int64(v), 10)
	} else {
		s = strconv.FormatFloat(float64(v), 'g', -1, 64)
	}
	return fmt.Sprintf("%#0x (;=%v;)", math.Float64bits(v), s)
}
