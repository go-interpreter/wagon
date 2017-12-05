// Copyright 2017 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wagon

import (
	"bufio"
	"bytes"
	"os/exec"
	"testing"
)

func TestGovet(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := exec.Command("go", "list", "./...")
	cmd.Stdout = buf
	cmd.Stderr = buf
	err := cmd.Run()
	if err != nil {
		t.Fatalf("error getting package list: %v\n%s", err, string(buf.Bytes()))
	}
	var pkgs []string
	s := bufio.NewScanner(buf)
	for s.Scan() {
		pkgs = append(pkgs, s.Text())
	}
	if err = s.Err(); err != nil {
		t.Fatalf("error parsing package list: %v", err)
	}

	cmd = exec.Command("go", append([]string{"vet"}, pkgs...)...)
	buf = new(bytes.Buffer)
	cmd.Stdout = buf
	cmd.Stderr = buf
	err = cmd.Run()
	if err != nil {
		t.Fatalf("error running %s:\n%s\n%v", "go vet", string(buf.Bytes()), err)
	}
}

func TestGofmt(t *testing.T) {
	exe, err := exec.LookPath("goimports")
	if err != nil {
		switch e := err.(type) {
		case *exec.Error:
			if e.Err == exec.ErrNotFound {
				exe, err = exec.LookPath("gofmt")
			}
		}
	}
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(exe, "-d", ".")
	buf := new(bytes.Buffer)
	cmd.Stdout = buf
	cmd.Stderr = buf

	err = cmd.Run()
	if err != nil {
		t.Fatalf("error running %s:\n%s\n%v", exe, string(buf.Bytes()), err)
	}

	if len(buf.Bytes()) != 0 {
		t.Errorf("some files were not gofmt'ed:\n%s\n", string(buf.Bytes()))
	}
}
