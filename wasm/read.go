package wasm

import (
	"io"
)

func readBytes(r io.Reader, n uint) ([]byte, error) {
	bytes := make([]byte, n)
	_, err := r.Read(bytes)
	if err != nil {
		return bytes, err
	}

	return bytes, nil
}

func readString(r io.Reader, n uint) (string, error) {
	bytes, err := readBytes(r, n)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
