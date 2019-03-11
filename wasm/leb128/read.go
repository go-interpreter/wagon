// Copyright 2017 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package leb128 provides functions for reading integer values encoded in the
// Little Endian Base 128 (LEB128) format: https://en.wikipedia.org/wiki/LEB128
package leb128

import (
	"io"
)

// ReadVarUint32 reads a LEB128 encoded unsigned 32-bit integer from r, and
// returns the integer value, and the error (if any).
func ReadVarUint32(r io.Reader) (uint32, error) {
	var (
		b     = make([]byte, 1)
		shift uint
		res   uint32
		err   error
	)
	for {
		if _, err = io.ReadFull(r, b); err != nil {
			return res, err
		}

		cur := uint32(b[0])
		res |= (cur & 0x7f) << (shift)
		if cur&0x80 == 0 {
			return res, nil
		}
		shift += 7
	}
}

// ReadVarint32 reads a LEB128 encoded signed 32-bit integer from r, and
// returns the integer value, and the error (if any).
func ReadVarint32(r io.Reader) (int32, error) {
	n, err := ReadVarint64(r)
	return int32(n), err
}

// ReadVarint64 reads a LEB128 encoded signed 64-bit integer from r, and
// returns the integer value, and the error (if any).
func ReadVarint64(r io.Reader) (int64, error) {
	var (
		b     = make([]byte, 1)
		shift uint
		sign  int64 = -1
		res   int64
		err   error
	)

	for {
		if _, err = io.ReadFull(r, b); err != nil {
			return res, err
		}

		cur := int64(b[0])
		res |= (cur & 0x7f) << shift
		shift += 7
		sign <<= 7
		if cur&0x80 == 0 {
			break
		}
	}

	if ((sign >> 1) & res) != 0 {
		res |= sign
	}
	return res, nil
}
