// Copyright 2018 The go-interpreter Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"io/ioutil"
	"testing"
)

func TestRun(t *testing.T) {
	for _, tc := range []struct {
		name   string
		verify bool
		want   string
	}{
		{
			name: "../../exec/testdata/basic.wasm",
			want: "testdata/basic.wasm.txt",
		},
		{
			name:   "../../exec/testdata/basic.wasm",
			verify: true,
			want:   "testdata/basic.wasm.txt",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := new(bytes.Buffer)
			run(out, tc.name, tc.verify)

			want, err := ioutil.ReadFile(tc.want)
			if err != nil {
				t.Fatal(err)
			}

			if got, want := string(out.Bytes()), string(want); got != want {
				t.Fatalf("invalid output.\ngot:\n%s\nwant:\n%s\n", got, want)
			}
		})
	}
}
