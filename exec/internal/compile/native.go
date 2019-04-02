// Copyright 2019 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compile

import (
	"bytes"
	"os"
	"os/exec"
)

// NativeCodeUnit represents compiled native code.
type NativeCodeUnit interface {
	Invoke(stack, locals *[]uint64)
}

func debugPrintAsm(asm []byte) {
	cmd := exec.Command("ndisasm", "-b64", "-")
	cmd.Stdin = bytes.NewReader(asm)
	cmd.Stdout = os.Stdout
	cmd.Run()
}
