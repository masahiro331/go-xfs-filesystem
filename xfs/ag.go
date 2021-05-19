package xfs

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"

	"github.com/masahiro331/go-xfs-filesystem/xfs/utils"

	"golang.org/x/xerrors"
)

type AG struct {
	SuperBlock SuperBlock
	Agi        AGI
	Agf        AGF
	Agfl       AGFL

	Ab3b AB3B
	Ab3c AB3C
	Iab3 IAB3
	Fib3 FIB3
}

func NewAG(reader io.Reader) (*AG, error) {
	// TODO: Fix AGF, AGI spec
	r := io.LimitReader(reader, int64(BlockSize*9))
	buf := utils.ReadBlock(r)
	fr := bytes.NewReader(buf)

	rf := func(r io.Reader) io.Reader {
		return bytes.NewReader(utils.ReadSector(r))
	}
	var ag AG
	if err := binary.Read(rf(fr), binary.BigEndian, &ag.SuperBlock); err != nil {
		return nil, xerrors.Errorf("failed to read superblock: %w", err)
	}

	if err := binary.Read(rf(fr), binary.BigEndian, &ag.Agf); err != nil {
		return nil, xerrors.Errorf("failed to read afg: %w", err)
	}

	if err := binary.Read(rf(fr), binary.BigEndian, &ag.Agi); err != nil {
		return nil, xerrors.Errorf("failed to read agi: %w", err)
	}

	if err := binary.Read(rf(fr), binary.BigEndian, &ag.Agfl); err != nil {
		return nil, xerrors.Errorf("failed to read agfl: %w", err)
	}

	sblockReader := bytes.NewReader(utils.ReadBlock(r))
	if err := binary.Read(sblockReader, binary.BigEndian, &ag.Ab3b); err != nil {
		log.Fatalf("binary read error: %+v", err)
	}

	sblockReader = bytes.NewReader(utils.ReadBlock(r))
	if err := binary.Read(sblockReader, binary.BigEndian, &ag.Ab3c); err != nil {
		log.Fatalf("binary read error: %+v", err)
	}

	// parse IAB3
	sblockReader = bytes.NewReader(utils.ReadBlock(r))
	if err := binary.Read(sblockReader, binary.BigEndian, &ag.Iab3); err != nil {
		log.Fatalf("binary read error: %+v", err)
	}
	var inodes []InobtRec
	for i := 0; i < int(ag.Iab3.Numrecs); i++ {
		var inode InobtRec
		if err := binary.Read(sblockReader, binary.BigEndian, &inode); err != nil {
			log.Fatalf("binary read error: %+v", err)
		}
		inodes = append(inodes, inode)
	}

	// FIB3
	utils.ReadBlock(r)

	// read Free block
	utils.ReadBlock(r)
	utils.ReadBlock(r)
	utils.ReadBlock(r)
	utils.ReadBlock(r)
	return &ag, nil
}
