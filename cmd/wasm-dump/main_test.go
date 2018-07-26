// Copyright 2018 The go-interpreter Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"io/ioutil"
	"testing"
)

func TestProcess(t *testing.T) {
	opts := []string{"-h", "-x", "-s", "-d"}
	err := flag.CommandLine.Parse(opts)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		want string
	}{
		{
			name: "../../exec/testdata/basic.wasm",
			want: "testdata/basic.wasm.txt",
		},
		{
			name: "../../exec/testdata/add-ex.wasm",
			want: "testdata/add-ex.wasm.txt",
		},
		{
			name: "../../exec/testdata/add-ex-main.wasm",
			want: "testdata/add-ex-main.wasm.txt",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := new(bytes.Buffer)
			process(out, tc.name)

			want, err := ioutil.ReadFile(tc.want)
			if err != nil {
				t.Fatal(err)
			}

			if got, want := out.Bytes(), want; !bytes.Equal(got, want) {
				t.Fatalf("invalid output.\ngot:\n%s\nwant:\n%s\n", string(got), string(want))
			}
		})
	}
}
