package wasm

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/go-interpreter/wagon/wasm/internal/readpos"
)

const (
	Magic   uint32 = 0x6d736100
	Version uint32 = 0x1
)

type Module struct {
	Magic   uint32
	Version uint32

	Types    *SectionTypes
	Import   *SectionImports
	Function *SectionFunctions
	Table    *SectionTables
	Memory   *SectionMemories
	Global   *SectionGlobals
	Export   *SectionExports
	Start    *SectionStartFunction
	Elements *SectionElements
	Code     *SectionCode
	Data     *SectionData

	// The function index space of the module
	FunctionIndexSpace []FunctionSig
	GlobalIndexSpace   []GlobalEntry
	// function indices into the global function space
	// the limit of each table is it's capacity (cap)
	TableIndexSpace        [][]uint32
	LinearMemoryIndexSpace [][]byte

	// Name *SectionName

	Other []Section
}

// ResolveFunc is a function that takes a module name and
// returns a valid resolved module.
type ResolveFunc func(name string) (*Module, error)

// ReadModule reads a module from the reader r. resolvePath must take a string
// and a return a reader to the module pointed to by the string.
func ReadModule(r io.Reader, resolvePath ResolveFunc) (*Module, error) {
	reader := &readpos.ReadPos{
		R:      r,
		CurPos: 0,
	}
	m := &Module{}

	if err := binary.Read(reader, binary.LittleEndian, &m.Magic); err != nil {
		return nil, err
	}
	if m.Magic != Magic {
		return nil, errors.New("wasm: invalid magic number")
	}
	if err := binary.Read(reader, binary.LittleEndian, &m.Version); err != nil {
		return nil, err
	}

	for {
		done, err := m.readSection(reader)
		if err != nil {
			return nil, err
		} else if done {
			break
		}
	}

	m.LinearMemoryIndexSpace = make([][]byte, 1)
	if m.Table != nil {
		m.TableIndexSpace = make([][]uint32, int(len(m.Table.Entries)))
	}

	if m.Import != nil {
		return nil, errors.New("Imports aren't supported")
	}

	if err := m.populateGlobals(); err != nil {
		return nil, err
	}
	if err := m.populateFunctions(); err != nil {
		return nil, err
	}
	if err := m.populateTables(); err != nil {
		return nil, err
	}
	if err := m.populateLinearMemory(); err != nil {
		return nil, err
	}

	logger.Printf("There are %d entries in the function index space.", len(m.FunctionIndexSpace))
	return m, nil

}
