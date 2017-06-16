package wasm

import (
	"fmt"
)

type StackTypeError struct {
	Offset   int
	Expected BlockType
}

func (e StackTypeError) Error() string {
	return fmt.Sprintf("Incorrect/Missing stack top at offset %d (should be %s)", e.Offset, e.Expected.String())
}
