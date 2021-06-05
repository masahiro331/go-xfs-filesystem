package utils

import (
	"fmt"
	"io"

	"golang.org/x/xerrors"
)

const (
	BlockSize  = 4096
	SectorSize = 512
)

func ReadBlock(r io.Reader) ([]byte, error) {
	buf := make([]byte, BlockSize)
	i, err := r.Read(buf)
	if err != nil {
		return nil, xerrors.Errorf("failed to read: %w", err)
	}
	if i != BlockSize {
		return nil, xerrors.Errorf("block size error, read %d byte", i)
	}

	return buf, nil
}

func ReadSector(r io.Reader) ([]byte, error) {
	buf := make([]byte, SectorSize)
	i, err := r.Read(buf)
	if err != nil {
		return nil, xerrors.Errorf("failed to read: %w", err)
	}
	if i != SectorSize {
		return nil, xerrors.Errorf("sector size error, read %d byte", i)
	}

	return buf, nil
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
