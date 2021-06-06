package utils

import (
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
