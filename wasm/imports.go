package wasm

import (
	"errors"
	"fmt"
)

// ImportEntry describes an import statement in a Wasm module.
type ImportEntry struct {
	// module name string
	ModuleStr string
	/// field name string
	FieldStr string
	Kind     External

	// If Kind is Function, Type is a uint32 containing the type index of the function signature
	// If Kind is Table, Type is a Table containing the type of the imported table
	// If Kind is Memory, Type is a Memory containing the type of the imported memory
	// If the Kind is Global, Type is a GlobalVar
	Type interface{}
}

var ImportMutGlobalError = errors.New("wasm: cannot import global mutable variable")

type ErrInvalidExternal uint8

func (e ErrInvalidExternal) Error() string {
	return fmt.Sprintf("wasm: invalid external_kind value %d", uint8(e))
}

func (module *Module) resolveImports(resolve ResolveFunc) error {
	if module.Import == nil {
		return nil
	}

	modules := make(map[string]*Module)

	for _, importEntry := range module.Import.Entries {
		importedModule, ok := modules[importEntry.ModuleStr]
		if !ok {
			var err error
			importedModule, err = resolve(importEntry.ModuleStr)
			if err != nil {
				return err
			}

			modules[importEntry.ModuleStr] = importedModule
		}

		if importedModule.Export == nil {
			return errors.New("Couldn't import field.")
		}

		exportEntry, ok := importedModule.Export.Entries[importEntry.FieldStr]
		if !ok {
			return errors.New("Couldn't import field.")
		}

		if exportEntry.Kind != importEntry.Kind {
			return errors.New("Mismatch import and export kinds")
		}

		index := exportEntry.Index

		switch exportEntry.Kind {
		case ExternalFunction:
			fn := importedModule.GetFunction(int(index))
			if fn == nil {
				return errors.New("Invalid index for exported function")
			}
			if fn.body == nil {
				return errors.New("Imported function doesn't have a resolved body")
			}
			module.FunctionIndexSpace = append(module.FunctionIndexSpace, *fn)
		case ExternalGlobal:
			glb := importedModule.GetGlobal(int(index))
			if glb == nil {
				return errors.New("Invalid index for exported global")
			}
			if glb.Type.Mutable {
				return ImportMutGlobalError
			}
			module.GlobalIndexSpace = append(module.GlobalIndexSpace, *glb)

			// In both cases below, index should be always 0 (according to the MVP)
			// We check it against the length of the index space anyway.
		case ExternalTable:
			if int(index) >= len(importedModule.TableIndexSpace) {
				return errors.New("Invalid index for table index space")
			}
			module.TableIndexSpace[0] = importedModule.TableIndexSpace[0]
		case ExternalMemory:
			if int(index) >= len(importedModule.LinearMemoryIndexSpace) {
				return errors.New("Invalid index for linear memory index space")
			}
			module.LinearMemoryIndexSpace[0] = importedModule.LinearMemoryIndexSpace[0]
		default:
			return errors.New("Invalid value for external_kind.")
		}

	}

	return nil
}
