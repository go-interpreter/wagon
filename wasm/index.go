package wasm

import (
	"errors"
)

// Functions for populating and looking up entries in a module's index space.
// More info: http://webassembly.org/docs/modules/#function-index-space

func (m *Module) populateFunctions() error {
	if m.Types == nil {
		return nil
	}

	if m.Function != nil {
		for _, index := range m.Function.Types {
			if int(index) >= len(m.Types.Entries) {
				return errors.New("Invalid type index")
			}

			m.FunctionIndexSpace = append(m.FunctionIndexSpace, m.Types.Entries[int(index)])
		}
	}

	return nil
}

// GetFunction returns a *Function, based on the function's index in
// the function index space. Returns nil when the index is invalid
func (m *Module) GetFunction(i int) *FunctionSig {
	if i >= len(m.FunctionIndexSpace) || i < 0 {
		return nil
	}

	return &m.FunctionIndexSpace[i]
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

func (m *Module) populateTables() error {
	if m.Table == nil || len(m.Table.Entries) == 0 || m.Elements == nil || len(m.Elements.Entries) == 0 {
		return nil
	}

	for _, elem := range m.Elements.Entries {
		// the MVP dictates that index should always be zero, we shuold
		// probably check this
		if int(elem.Index) >= len(m.TableIndexSpace) {
			return errors.New("Invalid index to table section")
		}

		val, err := m.execInitExpr(elem.Offset)
		if err != nil {
			return err
		}
		offset, ok := val.(int32)
		if !ok {
			return errors.New("The offset initializer expression does not return i32")
		}

		table := m.TableIndexSpace[int(elem.Index)]
		switch {
		case int(offset) > len(table):
			// make space for offset - 1 elements, so the n'th element
			// of this segment is located at offset + n
			table = append(table, make([]uint32, offset-1)...)
			fallthrough
		case int(offset) == len(table):
			table = append(table, elem.Elems...)
		case int(offset) < len(table):
			lastIndex := int(offset) + len(elem.Elems) - 1
			if lastIndex >= len(table) {
				table = append(table, make([]uint32, lastIndex+1-len(table))...)
			}

			tableIndex := offset
			for _, funcIndex := range elem.Elems {
				table[tableIndex] = funcIndex
				tableIndex++
			}
		}

		m.TableIndexSpace[int(elem.Index)] = table
	}

	logger.Printf("There are %d entries in the table index space.", len(m.TableIndexSpace))
	return nil
}

// GetTableElement returns an element from the tableindex  space indexed
// by the integer index. It returns an error if index is invalid.
func (m *Module) GetTableElement(index int) (uint32, error) {
	if index >= len(m.TableIndexSpace[0]) {
		return 0, errors.New("Invalid index into the table index space")
	}

	return m.TableIndexSpace[0][index], nil
}

func (m *Module) populateLinearMemory() error {
	if m.Data == nil || len(m.Data.Entries) == 0 {
		return nil
	}
	// each module can only have a single linear memory in the MVP

	for _, entry := range m.Data.Entries {
		if entry.Index != 0 {
			return errors.New("Invalid index for linear memory")
		}

		val, err := m.execInitExpr(entry.Offset)
		if err != nil {
			return err
		}
		offset, ok := val.(int32)
		if !ok {
			return errors.New("The offset initializer expression does not return i32")
		}
		memory := m.LinearMemoryIndexSpace[int(entry.Index)]

		switch {
		case int(offset) > len(memory):
			memory = append(memory, make([]byte, offset-1)...)
			fallthrough
		case int(offset) == len(memory):
			memory = append(memory, entry.Data...)
		case int(offset) < len(memory):
			lastIndex := int(offset) + len(entry.Data) - 1
			if lastIndex >= len(memory) {
				memory = append(memory, make([]byte, lastIndex+1-len(memory))...)
			}

			memIndex := offset
			for _, byte := range entry.Data {
				memory[memIndex] = byte
				memIndex++
			}
		}

		m.LinearMemoryIndexSpace[int(entry.Index)] = memory
	}

	return nil
}

func (m *Module) GetLinearMemoryData(index int) (byte, error) {
	if index >= len(m.LinearMemoryIndexSpace[0]) {
		return 0, errors.New("Invalid index to linear memory index space")

	}

	return m.LinearMemoryIndexSpace[0][index], nil
}
