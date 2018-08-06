// Copyright 2017 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wasm

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/go-interpreter/wagon/wasm/internal/readpos"
	"github.com/go-interpreter/wagon/wasm/leb128"
)

// Section is a generic WASM section interface.
type Section interface {
	// SectionID returns a section ID for WASM encoding. Should be unique across types.
	SectionID() SectionID
	// GetRawSection Returns an embedded RawSection pointer to populate generic fields.
	GetRawSection() *RawSection
	// ReadPayload reads a section payload, assuming the size was already read, and reader is limited to it.
	ReadPayload(r io.Reader) error
	// WritePayload writes a section payload without the size.
	// Caller should calculate written size and add it before the payload.
	WritePayload(w io.Writer) error
}

// SectionID is a 1-byte code that encodes the section code of both known and custom sections.
type SectionID uint8

const (
	SectionIDCustom   SectionID = 0
	SectionIDType     SectionID = 1
	SectionIDImport   SectionID = 2
	SectionIDFunction SectionID = 3
	SectionIDTable    SectionID = 4
	SectionIDMemory   SectionID = 5
	SectionIDGlobal   SectionID = 6
	SectionIDExport   SectionID = 7
	SectionIDStart    SectionID = 8
	SectionIDElement  SectionID = 9
	SectionIDCode     SectionID = 10
	SectionIDData     SectionID = 11
)

func (s SectionID) String() string {
	n, ok := map[SectionID]string{
		SectionIDCustom:   "custom",
		SectionIDType:     "type",
		SectionIDImport:   "import",
		SectionIDFunction: "function",
		SectionIDTable:    "table",
		SectionIDMemory:   "memory",
		SectionIDGlobal:   "global",
		SectionIDExport:   "export",
		SectionIDStart:    "start",
		SectionIDElement:  "element",
		SectionIDCode:     "code",
		SectionIDData:     "data",
	}[s]
	if !ok {
		return "unknown"
	}
	return n
}

// RawSection is a declared section in a WASM module.
type RawSection struct {
	Start int64
	End   int64

	ID SectionID
	// Size of this section in bytes
	PayloadLen uint32
	// Section name, empty if id != 0
	Name  string
	Bytes []byte
}

func (s *RawSection) SectionID() SectionID {
	return s.ID
}

func (s *RawSection) ReadPayload(r io.Reader) error {
	s.Bytes = make([]byte, s.PayloadLen)
	_, err := io.ReadFull(r, s.Bytes)
	return err
}

func (s *RawSection) WritePayload(w io.Writer) error {
	_, err := w.Write(s.Bytes)
	return err
}

func (s *RawSection) GetRawSection() *RawSection {
	return s
}

type InvalidSectionIDError SectionID

func (e InvalidSectionIDError) Error() string {
	return fmt.Sprintf("wasm: invalid section ID %d", e)
}

type InvalidCodeIndexError int

func (e InvalidCodeIndexError) Error() string {
	return fmt.Sprintf("wasm: invalid index to code section: %d", int(e))
}

var ErrUnsupportedSection = errors.New("wasm: unsupported section")

type MissingSectionError SectionID

func (e MissingSectionError) Error() string {
	return fmt.Sprintf("wasm: missing section %s", SectionID(e).String())
}

// reads a valid section from r. The first return value is true if and only if
// the module has been completely read.
func (m *Module) readSection(r *readpos.ReadPos) (bool, error) {
	var err error
	var id uint32

	logger.Println("Reading section ID")
	if id, err = leb128.ReadVarUint32(r); err != nil {
		if err == io.EOF { // no bytes were read, the reader is empty
			return true, nil
		}
		return false, err
	}
	s := RawSection{ID: SectionID(id)}

	logger.Println("Reading payload length")
	if s.PayloadLen, err = leb128.ReadVarUint32(r); err != nil {
		return false, nil
	}

	payloadDataLen := s.PayloadLen

	if s.ID == SectionIDCustom {
		nameLen, nameLenSize, err := leb128.ReadVarUint32Size(r)
		if err != nil {
			return false, err
		}
		payloadDataLen -= uint32(nameLenSize)
		if s.Name, err = readString(r, int(nameLen)); err != nil {
			return false, err
		}

		payloadDataLen -= uint32(len(s.Name))
	}

	logger.Printf("Section payload length: %d", payloadDataLen)

	s.Start = r.CurPos

	sectionBytes := new(bytes.Buffer)
	sectionBytes.Grow(int(payloadDataLen))
	sectionReader := io.LimitReader(io.TeeReader(r, sectionBytes), int64(payloadDataLen))

	var sec Section
	switch s.ID {
	case SectionIDCustom:
		logger.Println("section custom")
		i := len(m.Other)
		m.Other = append(m.Other, s)
		sec = &m.Other[i]
	case SectionIDType:
		logger.Println("section type")
		m.Types = &SectionTypes{}
		sec = m.Types
	case SectionIDImport:
		logger.Println("section import")
		m.Import = &SectionImports{}
		sec = m.Import
	case SectionIDFunction:
		logger.Println("section function")
		m.Function = &SectionFunctions{}
		sec = m.Function
	case SectionIDTable:
		logger.Println("section table")
		m.Table = &SectionTables{}
		sec = m.Table
	case SectionIDMemory:
		logger.Println("section memory")
		m.Memory = &SectionMemories{}
		sec = m.Memory
	case SectionIDGlobal:
		logger.Println("section global")
		m.Global = &SectionGlobals{}
		sec = m.Global
	case SectionIDExport:
		logger.Println("section export")
		m.Export = &SectionExports{}
		sec = m.Export
	case SectionIDStart:
		logger.Println("section start")
		m.Start = &SectionStartFunction{}
		sec = m.Start
	case SectionIDElement:
		logger.Println("section element")
		m.Elements = &SectionElements{}
		sec = m.Elements
	case SectionIDCode:
		logger.Println("section code")
		m.Code = &SectionCode{}
		sec = m.Code
	case SectionIDData:
		logger.Println("section data")
		m.Data = &SectionData{}
		sec = m.Data
	default:
		return false, InvalidSectionIDError(s.ID)
	}
	err = sec.ReadPayload(sectionReader)
	if err != nil {
		logger.Println(err)
		return false, err
	}
	s.End = r.CurPos
	s.Bytes = sectionBytes.Bytes()
	*sec.GetRawSection() = s
	switch s.ID {
	case SectionIDCode:
		s := m.Code
		if m.Function == nil || len(m.Function.Types) == 0 {
			return false, MissingSectionError(SectionIDFunction)
		}
		if len(m.Function.Types) != len(s.Bodies) {
			return false, errors.New("The number of entries in the function and code section are unequal")
		}
		if m.Types == nil {
			return false, MissingSectionError(SectionIDType)
		}
		for i := range s.Bodies {
			s.Bodies[i].Module = m
		}
	}
	return false, nil
}

var _ Section = (*SectionTypes)(nil)

// SectionTypes declares all function signatures that will be used in a module.
type SectionTypes struct {
	RawSection
	Entries []FunctionSig
}

func (*SectionTypes) SectionID() SectionID {
	return SectionIDType
}

func (s *SectionTypes) ReadPayload(r io.Reader) error {
	count, err := leb128.ReadVarUint32(r)
	if err != nil {
		return err
	}
	s.Entries = make([]FunctionSig, int(count))
	for i := range s.Entries {
		if err = s.Entries[i].UnmarshalWASM(r); err != nil {
			return err
		}
	}
	return nil
}

func (s *SectionTypes) WritePayload(w io.Writer) error {
	_, err := leb128.WriteVarUint32(w, uint32(len(s.Entries)))
	if err != nil {
		return err
	}
	for _, f := range s.Entries {
		if err = f.MarshalWASM(w); err != nil {
			return err
		}
	}
	return nil
}

var _ Section = (*SectionImports)(nil)

// SectionImports declares all imports that will be used in the module.
type SectionImports struct {
	RawSection
	Entries []ImportEntry
}

func (*SectionImports) SectionID() SectionID {
	return SectionIDImport
}

func (s *SectionImports) ReadPayload(r io.Reader) error {
	count, err := leb128.ReadVarUint32(r)
	if err != nil {
		return err
	}
	s.Entries = make([]ImportEntry, count)
	for i := range s.Entries {
		err = s.Entries[i].UnmarshalWASM(r)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SectionImports) WritePayload(w io.Writer) error {
	_, err := leb128.WriteVarUint32(w, uint32(len(s.Entries)))
	if err != nil {
		return err
	}
	for _, e := range s.Entries {
		err = writeImportEntry(w, e)
		if err != nil {
			return err
		}
	}
	return nil
}

func (i *ImportEntry) UnmarshalWASM(r io.Reader) error {
	modLen, err := leb128.ReadVarUint32(r)
	if err != nil {
		return err
	}

	if i.ModuleName, err = readString(r, int(modLen)); err != nil {
		return err
	}

	fieldLen, err := leb128.ReadVarUint32(r)
	if err != nil {
		return err
	}

	if i.FieldName, err = readString(r, int(fieldLen)); err != nil {
		return err
	}
	var kind External
	err = kind.UnmarshalWASM(r)
	if err != nil {
		return err
	}

	switch kind {
	case ExternalFunction:
		logger.Println("importing function")
		var t uint32
		t, err = leb128.ReadVarUint32(r)
		i.Type = FuncImport{t}
	case ExternalTable:
		logger.Println("importing table")
		var table Table

		err = table.UnmarshalWASM(r)
		if err == nil {
			i.Type = TableImport{table}
		}
	case ExternalMemory:
		logger.Println("importing memory")
		var mem Memory

		err = mem.UnmarshalWASM(r)
		if err == nil {
			i.Type = MemoryImport{mem}
		}
	case ExternalGlobal:
		logger.Println("importing global var")
		var gl GlobalVar

		err = gl.UnmarshalWASM(r)
		if err == nil {
			i.Type = GlobalVarImport{gl}
		}
	default:
		return InvalidExternalError(kind)
	}

	return err
}

func writeImportEntry(w io.Writer, i ImportEntry) error {
	if err := writeStringUint(w, i.ModuleName); err != nil {
		return err
	}
	if err := writeStringUint(w, i.FieldName); err != nil {
		return err
	}
	if err := i.Type.Kind().MarshalWASM(w); err != nil {
		return err
	}
	return i.Type.MarshalWASM(w)
}

// SectionFunction declares the signature of all functions defined in the module (in the code section)
type SectionFunctions struct {
	RawSection
	// Sequences of indices into (FunctionSignatues).Entries
	Types []uint32
}

func (*SectionFunctions) SectionID() SectionID {
	return SectionIDFunction
}

func (s *SectionFunctions) ReadPayload(r io.Reader) error {
	count, err := leb128.ReadVarUint32(r)
	if err != nil {
		return err
	}
	s.Types = make([]uint32, count)
	for i := range s.Types {
		t, err := leb128.ReadVarUint32(r)
		if err != nil {
			return err
		}
		s.Types[i] = t
	}
	return nil
}

func (s *SectionFunctions) WritePayload(w io.Writer) error {
	if _, err := leb128.WriteVarUint32(w, uint32(len(s.Types))); err != nil {
		return err
	}
	for _, t := range s.Types {
		if _, err := leb128.WriteVarUint32(w, uint32(t)); err != nil {
			return err
		}
	}
	return nil
}

// SectionTables describes all tables declared by a module.
type SectionTables struct {
	RawSection
	Entries []Table
}

func (*SectionTables) SectionID() SectionID {
	return SectionIDTable
}

func (s *SectionTables) ReadPayload(r io.Reader) error {
	count, err := leb128.ReadVarUint32(r)
	if err != nil {
		return err
	}
	s.Entries = make([]Table, count)
	for i := range s.Entries {
		err = s.Entries[i].UnmarshalWASM(r)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SectionTables) WritePayload(w io.Writer) error {
	if _, err := leb128.WriteVarUint32(w, uint32(len(s.Entries))); err != nil {
		return err
	}
	for _, e := range s.Entries {
		if err := e.MarshalWASM(w); err != nil {
			return err
		}
	}
	return nil
}

// SectionMemories describes all linaer memories used by a module.
type SectionMemories struct {
	RawSection
	Entries []Memory
}

func (*SectionMemories) SectionID() SectionID {
	return SectionIDMemory
}

func (s *SectionMemories) ReadPayload(r io.Reader) error {
	count, err := leb128.ReadVarUint32(r)
	if err != nil {
		return err
	}
	s.Entries = make([]Memory, count)
	for i := range s.Entries {
		err = s.Entries[i].UnmarshalWASM(r)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SectionMemories) WritePayload(w io.Writer) error {
	if _, err := leb128.WriteVarUint32(w, uint32(len(s.Entries))); err != nil {
		return err
	}
	for _, e := range s.Entries {
		if err := e.MarshalWASM(w); err != nil {
			return err
		}
	}
	return nil
}

// SectionGlobals defines the value of all global variables declared in a module.
type SectionGlobals struct {
	RawSection
	Globals []GlobalEntry
}

func (*SectionGlobals) SectionID() SectionID {
	return SectionIDGlobal
}

func (s *SectionGlobals) ReadPayload(r io.Reader) error {
	count, err := leb128.ReadVarUint32(r)
	if err != nil {
		return err
	}
	s.Globals = make([]GlobalEntry, count)
	logger.Printf("%d global entries\n", count)
	for i := range s.Globals {
		err = s.Globals[i].UnmarshalWASM(r)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SectionGlobals) WritePayload(w io.Writer) error {
	if _, err := leb128.WriteVarUint32(w, uint32(len(s.Globals))); err != nil {
		return err
	}
	for _, g := range s.Globals {
		if err := g.MarshalWASM(w); err != nil {
			return err
		}
	}
	return nil
}

// GlobalEntry declares a global variable.
type GlobalEntry struct {
	Type GlobalVar // Type holds information about the value type and mutability of the variable
	Init []byte    // Init is an initializer expression that computes the initial value of the variable
}

func (g *GlobalEntry) UnmarshalWASM(r io.Reader) error {
	err := g.Type.UnmarshalWASM(r)
	if err != nil {
		return err
	}

	// init_expr is delimited by opcode "end" (0x0b)
	g.Init, err = readInitExpr(r)
	return err
}

func (g *GlobalEntry) MarshalWASM(w io.Writer) error {
	if err := g.Type.MarshalWASM(w); err != nil {
		return err
	}
	_, err := w.Write(g.Init)
	return err
}

// SectionExports declares the export section of a module
type SectionExports struct {
	RawSection
	Entries map[string]ExportEntry
}

func (*SectionExports) SectionID() SectionID {
	return SectionIDExport
}

func (s *SectionExports) ReadPayload(r io.Reader) error {
	count, err := leb128.ReadVarUint32(r)
	if err != nil {
		return err
	}
	s.Entries = make(map[string]ExportEntry, count)
	for i := uint32(0); i < count; i++ {
		var entry ExportEntry
		err = entry.UnmarshalWASM(r)
		if err != nil {
			return err
		}

		if _, exists := s.Entries[entry.FieldStr]; exists {
			return DuplicateExportError(entry.FieldStr)
		}
		s.Entries[entry.FieldStr] = entry
	}
	return nil
}

func (s *SectionExports) WritePayload(w io.Writer) error {
	if _, err := leb128.WriteVarUint32(w, uint32(len(s.Entries))); err != nil {
		return err
	}
	entries := make([]ExportEntry, 0, len(s.Entries))
	for _, e := range s.Entries {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Index < entries[j].Index
	})
	for _, e := range entries {
		if err := e.MarshalWASM(w); err != nil {
			return err
		}
	}
	return nil
}

type DuplicateExportError string

func (e DuplicateExportError) Error() string {
	return fmt.Sprintf("Duplicate export entry: %s", e)
}

// ExportEntry represents an exported entry by the module
type ExportEntry struct {
	FieldStr string
	Kind     External
	Index    uint32
}

func (e *ExportEntry) UnmarshalWASM(r io.Reader) error {
	fieldLen, err := leb128.ReadVarUint32(r)
	if err != nil {
		return err
	}

	if e.FieldStr, err = readString(r, int(fieldLen)); err != nil {
		return err
	}

	if err := e.Kind.UnmarshalWASM(r); err != nil {
		return err
	}

	e.Index, err = leb128.ReadVarUint32(r)

	return err
}

func (e *ExportEntry) MarshalWASM(w io.Writer) error {
	if err := writeStringUint(w, e.FieldStr); err != nil {
		return err
	}
	if err := e.Kind.MarshalWASM(w); err != nil {
		return err
	}
	if _, err := leb128.WriteVarUint32(w, e.Index); err != nil {
		return err
	}
	return nil
}

// SectionStartFunction represents the start function section.
type SectionStartFunction struct {
	RawSection
	Index uint32 // The index of the start function into the global index space.
}

func (*SectionStartFunction) SectionID() SectionID {
	return SectionIDStart
}

func (s *SectionStartFunction) ReadPayload(r io.Reader) error {
	var err error
	s.Index, err = leb128.ReadVarUint32(r)
	return err
}

func (s *SectionStartFunction) WritePayload(w io.Writer) error {
	_, err := leb128.WriteVarUint32(w, s.Index)
	return err
}

// SectionElements describes the initial contents of a table's elements.
type SectionElements struct {
	RawSection
	Entries []ElementSegment
}

func (*SectionElements) SectionID() SectionID {
	return SectionIDElement
}

func (s *SectionElements) ReadPayload(r io.Reader) error {
	count, err := leb128.ReadVarUint32(r)
	if err != nil {
		return err
	}
	s.Entries = make([]ElementSegment, count)
	for i := range s.Entries {
		err = s.Entries[i].UnmarshalWASM(r)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SectionElements) WritePayload(w io.Writer) error {
	if _, err := leb128.WriteVarUint32(w, uint32(len(s.Entries))); err != nil {
		return err
	}
	for _, e := range s.Entries {
		if err := e.MarshalWASM(w); err != nil {
			return err
		}
	}
	return nil
}

// ElementSegment describes a group of repeated elements that begin at a specified offset
type ElementSegment struct {
	Index  uint32 // The index into the global table space, should always be 0 in the MVP.
	Offset []byte // initializer expression for computing the offset for placing elements, should return an i32 value
	Elems  []uint32
}

func (s *ElementSegment) UnmarshalWASM(r io.Reader) error {
	var err error

	if s.Index, err = leb128.ReadVarUint32(r); err != nil {
		return err
	}
	if s.Offset, err = readInitExpr(r); err != nil {
		return err
	}

	numElems, err := leb128.ReadVarUint32(r)
	if err != nil {
		return err
	}
	s.Elems = make([]uint32, numElems)

	for i := range s.Elems {
		e, err := leb128.ReadVarUint32(r)
		if err != nil {
			return err
		}
		s.Elems[i] = e
	}

	return nil
}

func (s *ElementSegment) MarshalWASM(w io.Writer) error {
	if _, err := leb128.WriteVarUint32(w, s.Index); err != nil {
		return err
	}
	if _, err := w.Write(s.Offset); err != nil {
		return err
	}

	if _, err := leb128.WriteVarUint32(w, uint32(len(s.Elems))); err != nil {
		return err
	}
	for _, e := range s.Elems {
		if _, err := leb128.WriteVarUint32(w, e); err != nil {
			return err
		}
	}
	return nil
}

// SectionCode describes the body for every function declared inside a module.
type SectionCode struct {
	RawSection
	Bodies []FunctionBody
}

func (*SectionCode) SectionID() SectionID {
	return SectionIDCode
}

func (s *SectionCode) ReadPayload(r io.Reader) error {
	count, err := leb128.ReadVarUint32(r)
	if err != nil {
		return err
	}
	s.Bodies = make([]FunctionBody, count)
	logger.Printf("%d function bodies\n", count)

	for i := range s.Bodies {
		logger.Printf("Reading function %d\n", i)
		if err = s.Bodies[i].UnmarshalWASM(r); err != nil {
			return err
		}
	}
	return nil
}

func (s *SectionCode) WritePayload(w io.Writer) error {
	if _, err := leb128.WriteVarUint32(w, uint32(len(s.Bodies))); err != nil {
		return err
	}
	for _, b := range s.Bodies {
		if err := b.MarshalWASM(w); err != nil {
			return err
		}
	}
	return nil
}

var ErrFunctionNoEnd = errors.New("Function body does not end with 0x0b (end)")

type FunctionBody struct {
	Module *Module // The parent module containing this function body, for execution purposes
	Locals []LocalEntry
	Code   []byte
}

func (f *FunctionBody) UnmarshalWASM(r io.Reader) error {

	bodySize, err := leb128.ReadVarUint32(r)
	if err != nil {
		return err
	}

	body := make([]byte, bodySize)

	if _, err = io.ReadFull(r, body); err != nil {
		return err
	}

	bytesReader := bytes.NewBuffer(body)

	localCount, err := leb128.ReadVarUint32(bytesReader)
	if err != nil {
		return err
	}
	f.Locals = make([]LocalEntry, localCount)

	for i := range f.Locals {
		if err = f.Locals[i].UnmarshalWASM(bytesReader); err != nil {
			return err
		}
	}

	logger.Printf("bodySize: %d, localCount: %d\n", bodySize, localCount)

	code := bytesReader.Bytes()
	logger.Printf("Read %d bytes for function body", len(code))

	if code[len(code)-1] != end {
		return ErrFunctionNoEnd
	}

	f.Code = code[:len(code)-1]

	return nil
}

func (f *FunctionBody) MarshalWASM(w io.Writer) error {
	body := new(bytes.Buffer)
	if _, err := leb128.WriteVarUint32(body, uint32(len(f.Locals))); err != nil {
		return err
	}
	for _, l := range f.Locals {
		if err := l.MarshalWASM(body); err != nil {
			return err
		}
	}
	if _, err := body.Write(f.Code); err != nil {
		return err
	}
	body.WriteByte(end)
	return writeBytesUint(w, body.Bytes())
}

type LocalEntry struct {
	Count uint32    // The total number of local variables of the given Type used in the function body
	Type  ValueType // The type of value stored by the variable
}

func (l *LocalEntry) UnmarshalWASM(r io.Reader) error {
	var err error

	l.Count, err = leb128.ReadVarUint32(r)
	if err != nil {
		return err
	}

	err = l.Type.UnmarshalWASM(r)
	if err != nil {
		return err
	}

	return nil
}

func (l *LocalEntry) MarshalWASM(w io.Writer) error {
	if _, err := leb128.WriteVarUint32(w, l.Count); err != nil {
		return err
	}
	if err := l.Type.MarshalWASM(w); err != nil {
		return err
	}
	return nil
}

// SectionData describes the intial values of a module's linear memory
type SectionData struct {
	RawSection
	Entries []DataSegment
}

func (*SectionData) SectionID() SectionID {
	return SectionIDData
}

func (s *SectionData) ReadPayload(r io.Reader) error {
	count, err := leb128.ReadVarUint32(r)
	if err != nil {
		return err
	}
	s.Entries = make([]DataSegment, count)
	for i := range s.Entries {
		if err = s.Entries[i].UnmarshalWASM(r); err != nil {
			return err
		}
	}
	return nil
}

func (s *SectionData) WritePayload(w io.Writer) error {
	if _, err := leb128.WriteVarUint32(w, uint32(len(s.Entries))); err != nil {
		return err
	}
	for _, e := range s.Entries {
		if err := e.MarshalWASM(w); err != nil {
			return err
		}
	}
	return nil
}

// DataSegment describes a group of repeated elements that begin at a specified offset in the linear memory
type DataSegment struct {
	Index  uint32 // The index into the global linear memory space, should always be 0 in the MVP.
	Offset []byte // initializer expression for computing the offset for placing elements, should return an i32 value
	Data   []byte
}

func (s *DataSegment) UnmarshalWASM(r io.Reader) error {
	var err error

	if s.Index, err = leb128.ReadVarUint32(r); err != nil {
		return err
	}
	if s.Offset, err = readInitExpr(r); err != nil {
		return err
	}

	size, err := leb128.ReadVarUint32(r)
	if err != nil {
		return err
	}
	s.Data, err = readBytes(r, int(size))

	return err
}

func (s *DataSegment) MarshalWASM(w io.Writer) error {
	if _, err := leb128.WriteVarUint32(w, s.Index); err != nil {
		return err
	}
	if _, err := w.Write(s.Offset); err != nil {
		return err
	}
	return writeBytesUint(w, s.Data)
}
