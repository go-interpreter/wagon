package wasm

import (
	"io/ioutil"
	"log"
	"os"
)

var PrintDebugInfo = false

var logger *log.Logger

func init() {
	w := ioutil.Discard

	if PrintDebugInfo {
		w = os.Stderr
	}

	logger = log.New(w, "", log.Lshortfile)
}
