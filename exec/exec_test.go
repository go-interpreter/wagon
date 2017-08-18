// Copyright 2017 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec_test

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"testing"

	"github.com/go-interpreter/wagon/exec"
	"github.com/go-interpreter/wagon/validate"
	"github.com/go-interpreter/wagon/wasm"
)

type testCase struct {
	Function string   `json:"function"`
	Args     []string `json:"args"`
	Return   string   `json:"return"`
}

type file struct {
	FileName string     `json:"file"`
	Tests    []testCase `json:"tests"`
}

var reValue = regexp.MustCompile(`(.+)\:(.+)`)

func parseValue(str string) interface{} {
	if str == "" {
		return nil
	}
	matches := reValue.FindStringSubmatch(str)
	switch matches[1] {
	case "i32":
		n, err := strconv.ParseUint(matches[2], 10, 32)
		if err != nil {
			panic(err)
		}
		return uint32(n)
	case "i64":
		n, err := strconv.ParseUint(matches[2], 10, 64)
		if err != nil {
			panic(err)
		}
		return n
	case "f32":
		n, err := strconv.ParseFloat(matches[2], 32)
		if err != nil {
			panic(err)
		}
		return float32(n)
	case "f64":
		n, err := strconv.ParseFloat(matches[2], 64)
		if err != nil {
			panic(err)
		}
		return float64(n)
	}

	return nil
}

func parseArgs(args []string) (arr []uint64) {
	for _, str := range args {
		v := parseValue(str)
		var n uint64
		switch v.(type) {
		case uint32:
			n = uint64(v.(uint32))
		case float32:
			n = uint64(math.Float32bits(v.(float32)))
		case float64:
			n = math.Float64bits(v.(float64))
		default:
			n = v.(uint64)
		}

		arr = append(arr, n)
	}

	return arr
}

func runTest(fileName string, testCases []testCase, t testing.TB) {
	file, err := os.Open("testdata/" + fileName)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	module, err := wasm.ReadModule(file, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = validate.VerifyModule(module); err != nil {
		t.Fatalf("%s: %v", fileName, err)
	}

	vm, err := exec.NewVM(module)
	if err != nil {
		t.Fatalf("%s: %v", fileName, err)
	}

	b, ok := t.(*testing.B)
	for _, testCase := range testCases {
		var expected interface{}

		index := module.Export.Entries[testCase.Function].Index
		args := parseArgs(testCase.Args)

		if testCase.Return != "" {
			expected = parseValue(testCase.Return)
		}

		times := 1

		if ok {
			times = b.N
			b.ResetTimer()
		}

		var res interface{}
		var err error

		for i := 0; i < times; i++ {
			res, err = vm.ExecCode(int64(index), args...)
		}
		if ok {
			b.StopTimer()
		}

		if err != nil {
			t.Fatalf("%s, %s: %v", fileName, testCase.Function, err)
		}

		if !reflect.DeepEqual(res, expected) {
			t.Errorf("%s, %s: unexpected return value: got=%v(%v), want=%v(%v)", fileName, testCase.Function, reflect.TypeOf(res), res, reflect.TypeOf(expected), expected)
		}
	}
}

func TestModules(t *testing.T) {
	files := []file{}
	file, err := os.Open("testdata/modules.json")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(&files)
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		fileName := file.FileName
		testCases := file.Tests
		t.Run(fileName, func(t *testing.T) {
			t.Parallel()
			runTest(fileName, testCases, t)
		})
	}
}

func BenchmarkModules(b *testing.B) {
	files := []file{}
	file, err := os.Open("testdata/modules.json")
	if err != nil {
		b.Fatal(err)
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(&files)
	if err != nil {
		b.Fatal(err)
	}

	for _, file := range files {
		fileName := file.FileName
		testCases := file.Tests
		b.Run(fileName, func(b *testing.B) {
			runTest(fileName, testCases, b)
		})
	}
}
