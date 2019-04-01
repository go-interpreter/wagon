// Copyright 2019 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compile

// NativeCodeUnit represents compiled native code.
type NativeCodeUnit interface {
	Invoke(stack, locals *[]uint64)
}
