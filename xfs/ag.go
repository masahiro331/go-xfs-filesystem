package xfs

import (
	"bytes"
	"encoding/binary"
	"io"

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

type AGFL struct {
	Magicnum uint32
	Seqno    uint32
	UUID     [16]byte
	Lsn      uint64
	CRC      uint32
	Bno      [118]uint32
}

type AGF struct {
	Magicnum   [4]byte
	Versionnum uint32
	Seqno      uint32
	Length     uint32

	Roots  [3]uint32
	Levels [3]uint32

	Flfirst   uint32
	Fllast    uint32
	Flcount   uint32
	Freeblks  uint32
	Longest   uint32
	Btreeblks uint32
	UUID      [16]byte

	RmapBlocks     uint32
	RefcountBlocks uint32
	RefcountRoot   uint32
	RefcountLevel  uint32
	Spare64        [112]byte
	Lsn            uint64
	CRC            uint32
	Spare2         uint32
}

type AGI struct {
	Magicnum   uint32
	Versionnum uint32
	Seqno      uint32
	Length     uint32
	Count      uint32
	Root       uint32
	Level      uint32
	Freecount  uint32
	Newino     uint32
	Dirino     uint32
	Unlinked   [256]byte
	UUID       [16]byte
	CRC        uint32
	Pad32      uint32
	Lsn        uint64
	FreeRoot   uint32
	FreeLevel  uint32
	Iblocks    uint32
	Fblocks    uint32
}

type IAB3 struct {
	Header BtreeShortBlock
	Inodes []InobtRec
}

type FIB3 struct {
	BtreeShortBlock
}

type AB3B struct {
	BtreeShortBlock
}

type AB3C struct {
	BtreeShortBlock
}

type BtreeShortBlock struct {
	Magicnum uint32
	Level    uint16
	Numrecs  uint16
	Leftsib  uint32
	Rightsib uint32
	Blkno    uint64
	Lsn      uint64
	UUID     [16]byte
	Owner    uint32
	CRC      uint32
}

func ParseAG(reader io.Reader) (*AG, error) {
	r := io.LimitReader(reader, int64(utils.BlockSize*5))

	var ag AG
	buf, err := utils.ReadSector(r)
	if err != nil {
		return nil, xerrors.Errorf("failed to create superblock reader: %w", err)
	}
	if err := binary.Read(bytes.NewReader(buf), binary.BigEndian, &ag.SuperBlock); err != nil {
		return nil, xerrors.Errorf("failed to read superblock: %w", err)
	}

	buf, err = utils.ReadSector(r)
	if err != nil {
		return nil, xerrors.Errorf("failed to create afg reader: %w", err)
	}
	if err := binary.Read(bytes.NewReader(buf), binary.BigEndian, &ag.Agf); err != nil {
		return nil, xerrors.Errorf("failed to read afg: %w", err)
	}

	buf, err = utils.ReadSector(r)
	if err != nil {
		return nil, xerrors.Errorf("failed to create agi reader: %w", err)
	}
	if err := binary.Read(bytes.NewReader(buf), binary.BigEndian, &ag.Agi); err != nil {
		return nil, xerrors.Errorf("failed to read agi: %w", err)
	}

	buf, err = utils.ReadSector(r)
	if err != nil {
		return nil, xerrors.Errorf("failed to create agfl reader: %w", err)
	}
	if err := binary.Read(bytes.NewReader(buf), binary.BigEndian, &ag.Agfl); err != nil {
		return nil, xerrors.Errorf("failed to read agfl: %w", err)
	}

	// parse AB3B
	buf, err = utils.ReadBlock(r)
	if err != nil {
		return nil, xerrors.Errorf("failed to create AB3B reader: %w", err)
	}
	if err := binary.Read(bytes.NewReader(buf), binary.BigEndian, &ag.Ab3b); err != nil {
		return nil, xerrors.Errorf("failed to read ab3b: %w", err)
	}

	// parse AB3C
	buf, err = utils.ReadBlock(r)
	if err != nil {
		return nil, xerrors.Errorf("failed to create AB3C reader: %w", err)
	}
	if err := binary.Read(bytes.NewReader(buf), binary.BigEndian, &ag.Ab3c); err != nil {
		return nil, xerrors.Errorf("failed to read AB3C: %w", err)
	}

	// parse IAB3
	buf, err = utils.ReadBlock(r)
	if err != nil {
		return nil, xerrors.Errorf("failed to create IAB3 reader: %w", err)
	}
	iab3Reader := bytes.NewReader(buf)
	if err := binary.Read(iab3Reader, binary.BigEndian, &ag.Iab3.Header); err != nil {
		return nil, xerrors.Errorf("failed to read IAB3: %w", err)
	}
	for i := 0; i < int(ag.Iab3.Header.Numrecs); i++ {
		var inode InobtRec
		if err := binary.Read(iab3Reader, binary.BigEndian, &inode); err != nil {
			return nil, xerrors.Errorf("failed to read inode list: %w", err)
		}
		ag.Iab3.Inodes = append(ag.Iab3.Inodes, inode)
	}

	// parse FIB3
	buf, err = utils.ReadBlock(r)
	if err != nil {
		return nil, xerrors.Errorf("failed to create FIB3 reader: %w", err)
	}
	if err := binary.Read(bytes.NewReader(buf), binary.BigEndian, &ag.Fib3); err != nil {
		return nil, xerrors.Errorf("failed to read FIB3: %w", err)
	}
	// TODO: parse Free block, 4 block
	return &ag, nil
}
