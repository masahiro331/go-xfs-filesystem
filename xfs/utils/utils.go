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

func DefaultChunkReader() *chunkReader {
	return &chunkReader{
		blockSize:  BlockSize,
		sectorSize: SectorSize,
	}
}

var allowedSectorSize = []int{512, 4096}

func NewChunkReader(sectorSize int) (*chunkReader, error) {
	validSectorSize := false
	for _, s := range allowedSectorSize {
		if s == sectorSize {
			validSectorSize = true
			break
		}
	}
	if !validSectorSize {
		return nil, fmt.Errorf("failed to instantiate chunk reader, invalid sector size: %d", sectorSize)
	}

	return &chunkReader{
		blockSize:  BlockSize,
		sectorSize: sectorSize,
	}, nil
}

type chunkReader struct {
	blockSize  int
	sectorSize int
}

func (c chunkReader) ReadBlock(r io.Reader) ([]byte, error) {
	buf := make([]byte, 0, c.blockSize)
	for i := 0; i < c.blockSize/c.sectorSize; i++ {
		b := make([]byte, c.sectorSize)
		i, err := r.Read(b)
		if err != nil {
			return nil, xerrors.Errorf("failed to read: %w", err)
		}
		if i != c.sectorSize {
			return nil, fmt.Errorf("failed to read sector invalid size expected(%d), actual(%d)", c.sectorSize, i)
		}
		buf = append(buf, b...)
	}

	if len(buf) != c.blockSize {
		return nil, fmt.Errorf("block size error, expected(%d), actual(%d)", c.blockSize, len(buf))
	}

	return buf, nil
}

func (c chunkReader) ReadSector(r io.Reader) ([]byte, error) {
	buf := make([]byte, c.sectorSize)
	i, err := r.Read(buf)
	if err != nil {
		return nil, xerrors.Errorf("failed to read: %w", err)
	}
	if i != c.sectorSize {
		return nil, xerrors.Errorf("sector size error, read %d byte", i)
	}

	return buf, nil
}
