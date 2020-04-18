// Copyright 2017 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wasm

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
)

type OutsizeError struct {
	ImmType string
	Size    uint64
	Max     uint64
}

func (e OutsizeError) Error() string {
	return fmt.Sprintf("validate: %s size overflow (%v), max (%v)", e.ImmType, e.Size, e.Max)
}

type InvalidTableIndexError uint32

func (e InvalidTableIndexError) Error() string {
	return fmt.Sprintf("wasm: Invalid table to table index space: %d", uint32(e))
}

type UninitializedTableEntryError uint32

func (e UninitializedTableEntryError) Error() string {
	return fmt.Sprintf("wasm: Uninitialized table entry at index: %d", uint32(e))
}

type InvalidValueTypeInitExprError struct {
	Wanted reflect.Kind
	Got    reflect.Kind
}

func (e InvalidValueTypeInitExprError) Error() string {
	return fmt.Sprintf("wasm: Wanted initializer expression to return %v value, got %v", e.Wanted, e.Got)
}

type InvalidLinearMemoryIndexError uint32

func (e InvalidLinearMemoryIndexError) Error() string {
	return fmt.Sprintf("wasm: Invalid linear memory index: %d", uint32(e))
}

// Functions for populating and looking up entries in a module's index space.
// More info: http://webassembly.org/docs/modules/#function-index-space

func (m *Module) populateFunctions() error {
	if m.Types == nil || m.Function == nil {
		return nil
	}

	// If present, extract the function names from the custom 'name' section
	var names NameMap
	if s := m.Custom(CustomSectionName); s != nil {
		var nSec NameSection
		err := nSec.UnmarshalWASM(bytes.NewReader(s.Data))
		if err != nil {
			return err
		}
		if len(nSec.Types[NameFunction]) > 0 {
			sub, err := nSec.Decode(NameFunction)
			if err != nil {
				return err
			}
			funcs, ok := sub.(*FunctionNames)
			if ok {
				names = funcs.Names
			}
		}
	}

	// If available, fill in the name field for the imported functions
	for i := range m.FunctionIndexSpace {
		m.FunctionIndexSpace[i].Name = names[uint32(i)]
	}

	// Add the functions from the wasm itself to the function list

	// when imported module not provided, len(m.FunctionIndecSpace) == 0
	// so must calculate number of imported funcs
	var numImports int
	if m.Import != nil {
		for _, importEntry := range m.Import.Entries {
			if importEntry.Type.Kind() == ExternalFunction {
				numImports++
			}
		}
	}
	for codeIndex, typeIndex := range m.Function.Types {
		if int(typeIndex) >= len(m.Types.Entries) {
			return InvalidFunctionIndexError(typeIndex)
		}

		// Create the main function structure
		fn := Function{
			Sig:  &m.Types.Entries[typeIndex],
			Body: &m.Code.Bodies[codeIndex],
			Name: names[uint32(codeIndex+numImports)], // Add the name string if we have it
		}

		m.FunctionIndexSpace = append(m.FunctionIndexSpace, fn)
	}

	funcs := make([]uint32, 0, len(m.Function.Types)+len(m.imports.Funcs))

	funcs = append(funcs, m.imports.Funcs...)
	funcs = append(funcs, m.Function.Types...)
	m.Function.Types = funcs
	return nil
}

// GetFunction returns a *Function, based on the function's index in
// the function index space. Returns nil when the index is invalid
func (m *Module) GetFunction(i int) *Function {
	if i >= len(m.FunctionIndexSpace) || i < 0 {
		return nil
	}

	return &m.FunctionIndexSpace[i]
}

func (m *Module) GetFunctionSig(i uint32) (*FunctionSig, error) {
	var funcindex uint32
	if m.Import == nil {
		if i >= uint32(len(m.Function.Types)) {
			return nil, errors.New("fsig out of len")
		}
		typeindex := m.Function.Types[i]
		return &m.Types.Entries[typeindex], nil
	}

	for _, importEntry := range m.Import.Entries {
		if importEntry.Type.Kind() == ExternalFunction {
			if funcindex == i {
				typeindex := importEntry.Type.(FuncImport).Type
				return &m.Types.Entries[typeindex], nil
			}

			funcindex++
		}
	}

	i = i - (funcindex - uint32(len(m.imports.Funcs)))
	if i >= uint32(len(m.Function.Types)) {
		return nil, errors.New("fsig out of len")
	}

	typeindex := m.Function.Types[i]
	return &m.Types.Entries[typeindex], nil
}

func (m *Module) populateGlobals() error {
	if m.Global == nil {
		return nil
	}

	m.GlobalIndexSpace = append(m.GlobalIndexSpace, m.Global.Globals...)
	logger.Printf("There are %d entries in the global index spaces.", len(m.GlobalIndexSpace))
	return nil
}

// GetGlobal returns a *GlobalEntry, based on the global index space.
// Returns nil when the index is invalid
func (m *Module) GetGlobal(i int) *GlobalEntry {
	if i >= len(m.GlobalIndexSpace) || i < 0 {
		return nil
	}

	return &m.GlobalIndexSpace[i]
}

func (m *Module) GetGlobalType(i uint32) (*GlobalVar, error) {
	var globalindex uint32

	if m.Import == nil {
		if i >= uint32(len(m.Global.Globals)) {
			return nil, errors.New("global index out of len")
		}
		return &m.Global.Globals[i].Type, nil
	}

	for _, importEntry := range m.Import.Entries {
		if importEntry.Type.Kind() == ExternalGlobal {
			if globalindex == i {
				v := importEntry.Type.(GlobalVarImport).Type
				return &v, nil
			}
			globalindex++
		}
	}

	i = i - (globalindex - uint32(m.imports.Globals))
	if i >= uint32(len(m.Global.Globals)) {
		return nil, errors.New("global index out of len")
	}
	return &m.Global.Globals[i].Type, nil
}

func (m *Module) populateTables() error {
	if m.Table == nil || len(m.Table.Entries) == 0 || m.Elements == nil || len(m.Elements.Entries) == 0 {
		return nil
	}

	for _, elem := range m.Elements.Entries {
		// the MVP dictates that index should always be zero, we should
		// probably check this
		if elem.Index >= uint32(len(m.TableIndexSpace)) {
			return InvalidTableIndexError(elem.Index)
		}

		val, err := m.ExecInitExpr(elem.Offset)
		if err != nil {
			return err
		}
		off, ok := val.(int32)
		if !ok {
			return InvalidValueTypeInitExprError{reflect.Int32, reflect.TypeOf(val).Kind()}
		}
		offset := uint32(off)

		table := m.TableIndexSpace[elem.Index]
		//use uint64 to avoid overflow
		totalSize := uint64(offset) + uint64(len(elem.Elems))
		if totalSize > uint64(len(table)) {
			maxAllowSize := uint64(m.Table.Entries[elem.Index].Limits.Maximum)
			if totalSize > maxAllowSize {
				return OutsizeError{"Table", totalSize, maxAllowSize}
			}
			data := make([]TableEntry, totalSize)
			copy(data, table)
			for i, index := range elem.Elems {
				data[offset+uint32(i)] = TableEntry{Index: index, Initialized: true}
			}
			m.TableIndexSpace[elem.Index] = data
		} else {
			for i, index := range elem.Elems {
				table[offset+uint32(i)] = TableEntry{Index: index, Initialized: true}
			}
		}
	}

	logger.Printf("There are %d entries in the table index space.", len(m.TableIndexSpace))
	return nil
}

// GetTableElement returns an element from the tableindex space indexed
// by the integer index. It returns an error if index is invalid.
func (m *Module) GetTableElement(index int) (uint32, error) {
	if index >= len(m.TableIndexSpace[0]) {
		return 0, InvalidTableIndexError(index)
	}

	entry := m.TableIndexSpace[0][index]
	if !entry.Initialized {
		return 0, UninitializedTableEntryError(index)
	}

	return entry.Index, nil
}

func (m *Module) populateLinearMemory() error {
	if m.Data == nil || len(m.Data.Entries) == 0 {
		return nil
	}
	// each module can only have a single linear memory in the MVP

	for _, entry := range m.Data.Entries {
		if entry.Index != 0 {
			return InvalidLinearMemoryIndexError(entry.Index)
		}

		val, err := m.ExecInitExpr(entry.Offset)
		if err != nil {
			return err
		}
		off, ok := val.(int32)
		if !ok {
			return InvalidValueTypeInitExprError{reflect.Int32, reflect.TypeOf(val).Kind()}
		}
		offset := uint32(off)

		memory := m.LinearMemoryIndexSpace[entry.Index]
		if uint64(offset)+uint64(len(entry.Data)) > uint64(len(memory)) {
			data := make([]byte, uint64(offset)+uint64(len(entry.Data)))
			copy(data, memory)
			copy(data[offset:], entry.Data)
			m.LinearMemoryIndexSpace[int(entry.Index)] = data
		} else {
			copy(memory[offset:], entry.Data)
		}
	}

	return nil
}

func (m *Module) GetLinearMemoryData(index int) (byte, error) {
	if index >= len(m.LinearMemoryIndexSpace[0]) {
		return 0, InvalidLinearMemoryIndexError(uint32(index))

	}

	return m.LinearMemoryIndexSpace[0][index], nil
}
