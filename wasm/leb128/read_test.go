// Copyright 2017 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package leb128

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"testing"
)

var casesUint = []struct {
	v uint32
	b []byte
}{
	{b: []byte{0x08}, v: 8},
	{b: []byte{0x80, 0x7f}, v: 16256},
	{b: []byte{0x80, 0x80, 0x80, 0xfd, 0x07}, v: 2141192192},
}

func TestReadVarUint32(t *testing.T) {
	for _, c := range casesUint {
		t.Run(fmt.Sprint(c.v), func(t *testing.T) {
			n, err := ReadVarUint32(bytes.NewReader(c.b))
			if err != nil {
				t.Fatal(err)
			}
			if n != c.v {
				t.Fatalf("got = %d; want = %d", n, c.v)
			}
		})
	}
}

func TestReadVarUint32Err(t *testing.T) {
	_, err := ReadVarUint32(bytes.NewReader(nil))
	if got, want := err, io.EOF; got != want {
		t.Fatalf("got err=%v, want=%v", got, want)
	}
}

var casesInt = []struct {
	v int64
	b []byte
}{
	{b: []byte{0xff, 0x7e}, v: -129},
	{b: []byte{0xe4, 0x00}, v: 100},
	{b: []byte{0x80, 0x80, 0x80, 0xfd, 0x07}, v: 2141192192},
}

var varint32Cases = []struct {
	b []byte
	v int32
}{
	{[]byte{0x80, 0x80, 0x80, 0x80, 0x78}, -2147483648}, // int32 min
	{[]byte{0xff, 0xff, 0xff, 0xff, 0x07}, 2147483647},  //int32 max
	{[]byte{0x80, 0x40}, -8192},
	{[]byte{0x80, 0xc0, 0x00}, 8192},
	{[]byte{135, 0x01}, 135},
}

func TestReadVarint32(t *testing.T) {
	for _, c := range varint32Cases {
		t.Run(fmt.Sprint(c.v), func(t *testing.T) {
			n, err := ReadVarint32(bytes.NewReader(c.b))
			if err != nil {
				t.Fatal(err)
			}
			if n != int32(c.v) {
				t.Fatalf("got = %d; want = %d", n, c.v)
			}
		})
	}
}

func TestReadVarint32Err(t *testing.T) {
	_, err := ReadVarint32(bytes.NewReader(nil))
	if got, want := err, io.EOF; got != want {
		t.Fatalf("got err=%v, want=%v", got, want)
	}
}

func TestReadWriteInt64(t *testing.T) {
	buf := make([]byte, 16)
	for i := 0; i < 1000000; i++ {
		rand.Read(buf)
		reader := bytes.NewReader(buf)
		val, err := ReadVarint64(reader)
		if err != nil {
			continue
		}
		readLen := len(buf) - reader.Len()
		if readLen > (64+6)/7 { // ceil(N/7) bytes
			t.Fatalf("read len:%d larger then ceil(N/7) bytes", readLen)
		}

		buf2 := bytes.NewBuffer(nil)
		WriteVarint64(buf2, val)
		if readLen <= len(buf2.Bytes()) {
			if !bytes.HasPrefix(buf, buf2.Bytes()) {
				t.Fatalf(fmt.Sprintf("val:%d, origin buf:%v, buf2: %v", val, buf, buf2.Bytes()))
			}
		}
	}

}

func TestReadWriteInt32(t *testing.T) {
	buf := make([]byte, 16)
	for i := 0; i < 1000000; i++ {
		rand.Read(buf)

		reader := bytes.NewReader(buf)
		val, err := ReadVarint32(reader)
		if err != nil {
			continue
		}
		readLen := len(buf) - reader.Len()
		if readLen > (32+6)/7 { // ceil(N/7) bytes
			t.Fatalf("read len:%d larger then ceil(N/7) bytes", readLen)
		}

		buf2 := bytes.NewBuffer(nil)
		WriteVarint64(buf2, int64(val))
		if readLen <= len(buf2.Bytes()) {
			if !bytes.HasPrefix(buf, buf2.Bytes()) {
				t.Fatalf(fmt.Sprintf("val:%d, origin buf:%v, buf2: %v", val, buf, buf2.Bytes()))
			}
		}
	}

}

func TestReadWriteUint32(t *testing.T) {
	buf := make([]byte, 16)
	for i := 0; i < 100000; i++ {
		rand.Read(buf)

		reader := bytes.NewReader(buf)
		val, err := ReadVarUint32(reader)
		if err != nil {
			continue
		}
		readLen := len(buf) - reader.Len()
		if readLen > (32+6)/7 { // ceil(N/7) bytes
			t.Fatalf("read len:%d larger then ceil(N/7) bytes", readLen)
		}

		buf2 := bytes.NewBuffer(nil)
		WriteVarUint32(buf2, val)
		if readLen <= len(buf2.Bytes()) {
			if !bytes.HasPrefix(buf, buf2.Bytes()) {
				t.Fatalf(fmt.Sprintf("val:%d, origin buf:%v, buf2: %v", val, buf, buf2.Bytes()))
			}
		}
	}
}

func TestCompareReadVarint(t *testing.T) {
	buf := make([]byte, 16)
	for n := uint(1); n <= 64; n++ {
		for i := 0; i < 100000; i++ {
			rand.Read(buf)

			val2, err2 := readVarintRecur(bytes.NewReader(buf), n)
			val1, err1 := readVarint(bytes.NewReader(buf), n)
			if fmt.Sprint(err1) != fmt.Sprint(err2) || val1 != val2 {
				t.Fatalf(fmt.Sprintf("buf: %v, val1:%d, val2: %d", buf, val1, val2))
			}
		}

	}
}

func TestCompareReadVarUint(t *testing.T) {
	buf := make([]byte, 16)
	for n := uint(1); n <= 64; n++ {
		for i := 0; i < 100000; i++ {
			rand.Read(buf)

			val2, err2 := readVarUintRecur(bytes.NewReader(buf), n)
			val1, err1 := readVarUint(bytes.NewReader(buf), n)
			if fmt.Sprint(err1) != fmt.Sprint(err2) || val1 != val2 {
				t.Fatalf(fmt.Sprintf("buf: %v, val1:%d, val2: %d", buf, val1, val2))
			}
		}

	}
}

func readVarUintRecur(r io.Reader, n uint) (uint64, error) {
	if n > 64 {
		panic(errors.New("leb128: n must <= 64"))
	}
	p := make([]byte, 1)
	_, err := io.ReadFull(r, p)
	if err != nil {
		return 0, err
	}
	b := uint64(p[0])
	switch {
	case b < 1<<7 && b < 1<<n:
		return b, nil
	case b >= 1<<7 && n > 7:
		m, err := readVarUint(r, n-7)
		if err != nil {
			return 0, err
		}

		return (1<<7)*m + (b - 1<<7), nil
	default:
		return 0, errors.New("leb128: invalid uint")
	}
}

func readVarintRecur(r io.Reader, n uint) (int64, error) {
	if n > 64 {
		panic(errors.New("leb128: n must <= 64"))
	}
	p := make([]byte, 1)
	_, err := io.ReadFull(r, p)
	if err != nil {
		return 0, err
	}
	b := int64(p[0])
	switch {
	case b < 1<<6 && uint64(b) < uint64(1<<(n-1)):
		return b, nil
	case b >= 1<<6 && b < 1<<7 && uint64(b)+1<<(n-1) >= 1<<7:
		return b - 1<<7, nil
	case b >= 1<<7 && n > 7:
		m, err := readVarint(r, n-7)
		if err != nil {
			return 0, err
		}

		return (1<<7)*m + (b - 1<<7), nil
	default:
		return 0, errors.New("leb128: invalid int")
	}
}
