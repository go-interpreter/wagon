// Copyright 2018 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package wast implements a WebAssembly text format.
package wast

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

// WriteTo writes a WASM modules in a text representation.
func WriteTo(w io.Writer, m *wasm.Module) error {
	var fnames wasm.NameMap
	if s := m.Custom(wasm.CustomSectionName); s != nil {
		var names wasm.NameSection
		_ = names.UnmarshalWASM(bytes.NewReader(s.Data))
		sub, _ := names.Decode(wasm.NameFunction)
		funcs, ok := sub.(*wasm.FunctionNames)
		if ok {
			fnames = funcs.Names
		}
	}
	_ = fnames
	bw := bufio.NewWriter(w)
	bw.WriteString("(module")
	if m.Types != nil {
		bw.WriteString("\n")
		for i, t := range m.Types.Entries {
			if i != 0 {
				bw.WriteString("\n")
			}
			fmt.Fprintf(bw, tab+"(type (;%d;) ", i)
			writeFuncSignature(bw, t)
			bw.WriteString(")")
		}
	}
	if m.Function != nil {
		bw.WriteString("\n")
		for i, t := range m.Function.Types {
			if i != 0 {
				bw.WriteString("\n")
			}
			bw.WriteString(tab + "(func ")
			if true {
				fmt.Fprintf(bw, "(;%d;)", i)
			}
			fmt.Fprintf(bw, " (type %d)", int(t))
			if int(t) < len(m.Types.Entries) {
				sig := m.Types.Entries[t]
				writeFuncType(bw, sig)
			}
			if m.Code != nil && i < len(m.Code.Bodies) {
				b := m.Code.Bodies[i]
				if len(b.Locals) > 0 {
					bw.WriteString("\n" + tab + tab + "(local")
					for _, l := range b.Locals {
						for i := 0; i < int(l.Count); i++ {
							bw.WriteString(" ")
							bw.WriteString(l.Type.String())
						}
					}
					bw.WriteString(")")
				}
				writeCode(bw, b.Code, false)
			}
			bw.WriteString(")")
		}
	}
	if m.Global != nil {
		for i, e := range m.Global.Globals {
			bw.WriteString("\n")
			bw.WriteString(tab + "(global ")
			if true {
				fmt.Fprintf(bw, "(;%d;)", i)
			}
			if e.Type.Mutable {
				bw.WriteString(" (mut")
			}
			fmt.Fprintf(bw, " %v", e.Type.Type)
			if e.Type.Mutable {
				bw.WriteString(")")
			}
			bw.WriteString(" (")
			if err := writeCode(bw, e.Init, true); err != nil {
				return err
			}
			bw.WriteString("))")
		}
	}
	if m.Table != nil {
		bw.WriteString("\n")
		for i, t := range m.Table.Entries {
			bw.WriteString(tab + "(table ")
			if true {
				fmt.Fprintf(bw, "(;%d;)", i)
			}
			fmt.Fprintf(bw, " %d %d ", t.Limits.Initial, t.Limits.Maximum)
			switch t.ElementType {
			case wasm.ElemTypeAnyFunc:
				bw.WriteString("anyfunc")
			}
			bw.WriteString(")")
		}
	}
	if m.Memory != nil {
		bw.WriteString("\n")
		for i, e := range m.Memory.Entries {
			bw.WriteString(tab + "(memory ")
			if true {
				fmt.Fprintf(bw, "(;%d;)", i)
			}
			fmt.Fprintf(bw, " %d", e.Limits.Initial)
			if e.Limits.Maximum != 0 {
				fmt.Fprintf(bw, " %d", e.Limits.Initial)
			}
			bw.WriteString(")")
		}
	}
	if m.Export != nil {
		bw.WriteString("\n")
		for i, k := range m.Export.Names {
			e := m.Export.Entries[k]
			if i != 0 {
				bw.WriteString("\n")
			}
			fmt.Fprintf(bw, tab+"(export %q (", e.FieldStr)
			switch e.Kind {
			case wasm.ExternalFunction:
				bw.WriteString("func")
			case wasm.ExternalMemory:
				bw.WriteString("memory")
			case wasm.ExternalTable:
				bw.WriteString("table")
			case wasm.ExternalGlobal:
				bw.WriteString("global")
			}
			fmt.Fprintf(bw, " %d))", e.Index)
		}
	}
	if m.Elements != nil {
		for _, d := range m.Elements.Entries {
			bw.WriteString("\n")
			bw.WriteString(tab + "(elem")
			if d.Index != 0 {
				fmt.Fprintf(bw, " %d", d.Index)
			}
			bw.WriteString(" (")
			if err := writeCode(bw, d.Offset, true); err != nil {
				return err
			}
			bw.WriteString(")")
			for _, v := range d.Elems {
				fmt.Fprintf(bw, " %d", v)
			}
			bw.WriteString(")")
		}
	}
	if m.Data != nil {
		for _, d := range m.Data.Entries {
			bw.WriteString("\n")
			bw.WriteString(tab + "(data")
			if d.Index != 0 {
				fmt.Fprintf(bw, " %d", d.Index)
			}
			bw.WriteString(" (")
			if err := writeCode(bw, d.Offset, true); err != nil {
				return err
			}
			fmt.Fprintf(bw, ") %s)", quoteData(d.Data))
		}
	}
	bw.WriteString(")\n")
	return bw.Flush()
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

func writeFuncSignature(bw *bufio.Writer, t wasm.FunctionSig) error {
	bw.WriteString("(func")
	defer bw.WriteString(")")
	return writeFuncType(bw, t)
}

func writeFuncType(bw *bufio.Writer, t wasm.FunctionSig) error {
	if len(t.ParamTypes) != 0 {
		bw.WriteString(" (param")
		for _, p := range t.ParamTypes {
			bw.WriteString(" ")
			bw.WriteString(p.String())
		}
		bw.WriteString(")")
	}
	if len(t.ReturnTypes) != 0 {
		bw.WriteString(" (result")
		for _, p := range t.ReturnTypes {
			bw.WriteString(" ")
			bw.WriteString(p.String())
		}
		bw.WriteString(")")
	}
	return nil
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

func writeCode(bw *bufio.Writer, code []byte, isInit bool) error {
	instr, err := disasm.Disassemble(code)
	if err != nil {
		return err
	}
	tabs := 2
	block := 0
	writeBlock := func(d int) {
		fmt.Fprintf(bw, " %d (;@%d;)", d, block-d)
	}
	hadEnd := false
	for i, ins := range instr {
		if !isInit {
			bw.WriteString("\n")
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
				bw.WriteString(" ")
			}
		} else {
			for i := 0; i < tabs; i++ {
				bw.WriteString(tab)
			}
		}
		bw.WriteString(ins.Op.Name)
		switch ins.Op.Code {
		case operators.Else:
			tabs++
		case operators.Block, operators.Loop, operators.If:
			tabs++
			block++
			b := ins.Immediates[0].(wasm.BlockType)
			if b != wasm.BlockTypeEmpty {
				bw.WriteString(" (result ")
				bw.WriteString(b.String())
				bw.WriteString(")")
			}
			fmt.Fprintf(bw, "  ;; label = @%d", block)
			continue
		case operators.F32Const:
			i1 := ins.Immediates[0].(float32)
			bw.WriteString(" " + formatFloat32(i1))
			continue
		case operators.F64Const:
			i1 := ins.Immediates[0].(float64)
			bw.WriteString(" " + formatFloat64(i1))
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
		case operators.CallIndirect:
			i1 := ins.Immediates[0].(uint32)
			fmt.Fprintf(bw, " (type %d)", i1)
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
			operators.I32Load, operators.I64Load,
			operators.I32Load8u, operators.I32Load8s,
			operators.I32Load16u, operators.I32Load16s,
			operators.I64Load8u, operators.I64Load8s,
			operators.I64Load16u, operators.I64Load16s,
			operators.I64Load32u, operators.I64Load32s:

			i1 := ins.Immediates[0].(uint32)
			i2 := ins.Immediates[1].(uint32)
			dst := 0 // in log 2 (i8)
			switch ins.Op.Code {
			case operators.I64Load, operators.I64Store:
				dst = 3
			case operators.I32Load, operators.I64Load32s, operators.I64Load32u,
				operators.I32Store, operators.I64Store32:
				dst = 2
			case operators.I32Load16u, operators.I32Load16s, operators.I64Load16u, operators.I64Load16s,
				operators.I32Store16, operators.I64Store16:
				dst = 1
			case operators.I32Load8u, operators.I32Load8s, operators.I64Load8u, operators.I64Load8s,
				operators.I32Store8, operators.I64Store8:
				dst = 0
			}
			if i2 != 0 {
				fmt.Fprintf(bw, " offset=%d", i2)
			}
			if int(i1) != dst {
				fmt.Fprintf(bw, " align=%d", 1<<i1)
			}
			continue
		}
		for _, a := range ins.Immediates {
			bw.WriteString(" ")
			fmt.Fprint(bw, a)
		}
	}
	_ = instr
	return nil
}
