package main

import (
	"io"
	"os"
)

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

func saveToFile(name string, data []byte) {
	// open output file
	fo, err := os.Create(name)
	if err != nil {
		panic(err)
	}
	// close fo on exit and check for its returned error
	defer func() {
		if err := fo.Close(); err != nil {
			panic(err)
		}
	}()

	// make a buffer to keep chunks that are read
	start := 0
	end := len(data)
	for {
		// write a chunk
		n, _ := fo.Write(data[start:end])
		start = start + n
		if start >= end {
			break
		}
	}
}
