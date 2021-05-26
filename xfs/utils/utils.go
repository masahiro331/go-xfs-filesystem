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

	for y := 0; y < len(buf)/lineByteSize; y++ {
		str := lineStr(buf[lineByteSize*y : lineByteSize*(y+1)])
		fmt.Println(string(str))
	}
}

func lineStr(buf []byte) []rune {
	var binaryStr string
	for i, b := range buf {
		if i%2 == 0 {
			binaryStr = binaryStr + " "
		}
		binaryStr = binaryStr + fmt.Sprintf("%02x", b)
	}
	return []rune(fmt.Sprintf("%s  %s", binaryStr, formatBinaryString(buf)))
}

func formatBinaryString(buf []byte) (str string) {
	for _, b := range buf {
		if b > 0x20 && b < 0x7f {
			str = str + string(b)
		} else {
			str = str + "."
		}
	}
	return
}
