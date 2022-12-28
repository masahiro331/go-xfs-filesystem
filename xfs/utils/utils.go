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
	buf := make([]byte, 0, BlockSize)
	for i := 0; i < BlockSize/SectorSize; i++ {
		b := make([]byte, SectorSize)
		i, err := r.Read(b)
		if err != nil {
			return nil, xerrors.Errorf("failed to read: %w", err)
		}
		if i != 512 {
			return nil, fmt.Errorf("failed to read sector invalid size expected(%d), actual(%d)", SectorSize, i)
		}
		buf = append(buf, b...)
	}

	if len(buf) != BlockSize {
		return nil, fmt.Errorf("block size error, expected(%d), actual(%d)", BlockSize, len(buf))
	}

	return buf, nil
}

func ReadBlockAt(r io.ReaderAt, offset int64) ([]byte, error) {
	buf := make([]byte, 0, BlockSize)
	for i := 0; i < BlockSize/SectorSize; i++ {
		b := make([]byte, SectorSize)
		i, err := r.ReadAt(b, offset+int64(i)*SectorSize)
		if err != nil {
			return nil, xerrors.Errorf("failed to read: %w", err)
		}
		if i != 512 {
			return nil, fmt.Errorf("failed to read sector invalid size expected(%d), actual(%d)", SectorSize, i)
		}
		buf = append(buf, b...)
	}

	if len(buf) != BlockSize {
		return nil, fmt.Errorf("block size error, expected(%d), actual(%d)", BlockSize, len(buf))
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
