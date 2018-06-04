// Copyright 2018 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec

import (
	"testing"
)

var (
	smallMemoryVM      = &VM{memory: []byte{1, 2, 3}}
	emptyMemoryVM      = &VM{memory: []byte{}}
	smallMemoryProcess = &Process{vm: smallMemoryVM}
	emptyMemoryProcess = &Process{vm: emptyMemoryVM}
	tooBigABuffer      = []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0}
)

func TestNormalWrite(t *testing.T) {
	vm := &VM{memory: make([]byte, 300)}
	proc := &Process{vm: vm}
	n, err := proc.WriteAt(tooBigABuffer, 0)
	if err != nil {
		t.Fatalf("Found an error when writing: %v", err)
	}
	if n != len(tooBigABuffer) {
		t.Fatalf("Number of written bytes was %d, should have been %d", n, len(tooBigABuffer))
	}
}

func TestWriteBoundary(t *testing.T) {
	n, err := smallMemoryProcess.WriteAt(tooBigABuffer, 0)
	if err == nil {
		t.Fatal("Should have reported an error and didn't")
	}
	if n != len(smallMemoryVM.memory) {
		t.Fatalf("Number of written bytes was %d, should have been 0", n)
	}
}

func TestReadBoundary(t *testing.T) {
	buf := make([]byte, 300)
	n, err := smallMemoryProcess.ReadAt(buf, 0)
	if err == nil {
		t.Fatal("Should have reported an error and didn't")
	}
	if n != len(smallMemoryVM.memory) {
		t.Fatalf("Number of written bytes was %d, should have been 0", n)
	}
}

func TestReadEmpty(t *testing.T) {
	buf := make([]byte, 300)
	n, err := emptyMemoryProcess.ReadAt(buf, 0)
	if err == nil {
		t.Fatal("Should have reported an error and didn't")
	}
	if n != 0 {
		t.Fatalf("Number of written bytes was %d, should have been 0", n)
	}
}

func TestReadOffset(t *testing.T) {
	buf0 := make([]byte, 2)
	n0, err := smallMemoryProcess.ReadAt(buf0, 0)
	if err != nil {
		t.Fatalf("Error reading 1-byte buffer: %v", err)
	}
	if n0 != 2 {
		t.Fatalf("Read %d bytes, expected 2", n0)
	}

	buf1 := make([]byte, 1)
	n1, err := smallMemoryProcess.ReadAt(buf1, 1)
	if err != nil {
		t.Fatalf("Error reading 1-byte buffer: %v", err)
	}
	if n1 != 1 {
		t.Fatalf("Read %d bytes, expected 1.", n0)
	}

	if buf0[1] != buf1[0] {
		t.Fatal("Read two different bytes from what should be the same location")
	}
}

func TestWriteEmpty(t *testing.T) {
	n, err := emptyMemoryProcess.WriteAt(tooBigABuffer, 0)
	if err == nil {
		t.Fatal("Should have reported an error and didn't")
	}
	if n != 0 {
		t.Fatalf("Number of written bytes was %d, should have been 0", n)
	}
}

func TestWriteOffset(t *testing.T) {
	vm := &VM{memory: make([]byte, 300)}
	proc := &Process{vm: vm}

	n, err := proc.WriteAt(tooBigABuffer, 2)
	if err != nil {
		t.Fatalf("error writing to buffer: %v", err)
	}
	if n != len(tooBigABuffer) {
		t.Fatalf("Number of written bytes was %d, should have been %d", n, len(tooBigABuffer))
	}

	if vm.memory[0] != 0 || vm.memory[1] != 0 || vm.memory[2] != tooBigABuffer[0] {
		t.Fatal("Writing at offset didn't work")
	}
}
