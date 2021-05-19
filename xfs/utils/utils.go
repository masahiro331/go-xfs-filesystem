package utils

import (
	"fmt"
	"io"
	"io/ioutil"
)

var blockCount int64

func ReadBlock(r io.Reader) []byte {
	buf := make([]byte, 4096)
	r.Read(buf)
	// ------- debug ----------
	blockCount++
	fmt.Println(blockCount)
	// ------------------------
	return buf
}

func ReadSector(r io.Reader) []byte {
	buf := make([]byte, 512)
	r.Read(buf)
	return buf
}

func DebugBlock(r io.Reader) {
	buf, _ := ioutil.ReadAll(r)
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
