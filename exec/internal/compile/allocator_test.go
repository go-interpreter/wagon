// Copyright 2019 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !appengine

package compile

import (
	"testing"
	"unsafe"
)

func TestMMapAllocator(t *testing.T) {
	a := &MMapAllocator{}
	defer a.Close()

	if _, err := a.AllocateExec([]byte{1, 2, 3, 4}); err != nil {
		t.Fatal(err)
	}
	if d := **(**[4]byte)(unsafe.Pointer(&a.last.mem)); d != [4]byte{1, 2, 3, 4} {
		t.Errorf("shortAlloc = %d, want [4]byte{1,2,3,4}", d)
	}
	if want := uint32(128); a.last.consumed != want {
		t.Errorf("a.last.consumed = %d, want %d", a.last.consumed, want)
	}
	if want := uint32(minAllocSize - allocationAlignment - 1); a.last.remaining != want {
		t.Errorf("a.last.remaining = %d, want %d", a.last.remaining, want)
	}

	if _, err := a.AllocateExec([]byte{4, 3, 2, 1}); err != nil {
		t.Fatal(err)
	}
	if want := uint32(256); a.last.consumed != want {
		t.Errorf("a.last.consumed = %d, want %d", a.last.consumed, want)
	}
	if want := uint32(minAllocSize - allocationAlignment*2 - 2); a.last.remaining != want {
		t.Errorf("a.last.remaining = %d, want %d", a.last.remaining, want)
	}

	// Test allocation of massive slice - should be 32k more & new block.
	b := make([]byte, 36*1024)
	b[1] = 5
	if _, err := a.AllocateExec(b); err != nil {
		t.Fatal(err)
	}
	if d := **(**[2]byte)(unsafe.Pointer(&a.last.mem)); d != [2]byte{0, 5} {
		t.Errorf("bigAlloc = %d, want [2]byte{0, 5}", d)
	}
	if want := uint32(36 * 1024); a.last.consumed != want {
		t.Errorf("a.last.consumed = %d, want %d", a.last.consumed, want)
	}
	if want := uint32(minAllocSize); a.last.remaining != want {
		t.Errorf("a.last.remaining = %d, want %d", a.last.remaining, want)
	}
}
