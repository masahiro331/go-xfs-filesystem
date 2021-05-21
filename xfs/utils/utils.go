package utils

import (
	"fmt"
	"io"
)

var blockCount int64

func ReadBlock(r io.Reader) []byte {
	buf := make([]byte, 4096)
	_, err := r.Read(buf)
	if err != nil {
		panic(fmt.Sprintf("Read error %+v", err))
	}
	// ------- debug ----------
	// blockCount++
	// fmt.Printf("read block: %d\n", blockCount)
	// ------------------------
	return buf
}

func ReadSector(r io.Reader) []byte {
	buf := make([]byte, 512)
	r.Read(buf)
	return buf
}

func DebugBlock(buf []byte) {
	lineByteSize := 16
	for i, b := range buf {
		if i%2 == 0 {
			fmt.Print(" ")
		}
		if i%lineByteSize == 0 {
			fmt.Println("")
		}
		fmt.Printf("%02x", b)
	}
}
