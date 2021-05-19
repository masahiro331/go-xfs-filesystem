package xfs

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"

	"golang.org/x/xerrors"
)

const (
	BMBT_EXNTFLAG_BITLEN = 1
)

const (
	// typedef enum xfs_dinode_fmt
	XFS_DINODE_FMT_DEV = iota
	XFS_DINODE_FMT_LOCAL
	XFS_DINODE_FMT_EXTENTS
	XFS_DINODE_FMT_BTREE
	XFS_DINODE_FMT_UUID
	XFS_DINODE_FMT_RMAP
)

func ParseInode(reader io.Reader, inodeSize int64) (*Inode, error) {
	r := io.LimitReader(reader, inodeSize)

	inode := Inode{}

	if err := binary.Read(r, binary.BigEndian, &inode.inodeCore); err != nil {
		return nil, xerrors.Errorf("failed to read InodeCore: %w", err)
	}

	switch inode.inodeCore.Format {
	case XFS_DINODE_FMT_DEV:
		inode.device = &Device{}
	case XFS_DINODE_FMT_LOCAL:
		if inode.inodeCore.IsDir() {
			inode.directoryLocal = &DirectoryLocal{}
			if err := binary.Read(r, binary.BigEndian, &inode.directoryLocal.dir2SfHdr); err != nil {
				return nil, xerrors.Errorf("failed to read XFS_DINODE_FMT_LOCAL directory error: %w", err)
			}
			if inode.directoryLocal.dir2SfHdr.I8Count != 0 {
				panic("header inode number 8 byte panic")
			}
			for i := 0; i < int(inode.directoryLocal.dir2SfHdr.Count); i++ {
				entry, err := parseEntry(r)
				if err != nil {
					log.Fatal(err)
				}
				inode.directoryLocal.entries = append(inode.directoryLocal.entries, *entry)
			}
		} else if inode.inodeCore.IsSymlink() {
			inode.symlinkString = &SymlinkString{}
			buf := make([]byte, inode.inodeCore.Size)
			_, err := r.Read(buf)
			if err != nil {
				return nil, xerrors.Errorf("failed to read XFS_DINODE_FMT_LOCAL symlink error: %w", err)
			}
			inode.symlinkString.Name = string(buf)
		} else {
			panic("not support XFS_DINODE_FMT_LOCAL")
		}
	case XFS_DINODE_FMT_EXTENTS:
		if inode.inodeCore.IsDir() {
			inode.directoryExtents = &DirectoryExtents{}
			if err := binary.Read(r, binary.BigEndian, &inode.directoryExtents.bmbtRec); err != nil {
				return nil, xerrors.Errorf("failed to read xfs_bmbt_irec error: %w", err)
			}
		} else if inode.inodeCore.IsRegular() {
			inode.regularExtent = &RegularExtent{}
			if err := binary.Read(r, binary.BigEndian, &inode.regularExtent.bmbtRec); err != nil {
				return nil, xerrors.Errorf("failed to read xfs_bmbt_irec error: %w", err)
			}
		} else if inode.inodeCore.IsSymlink() {
			panic("not support XFS_DINODE_FMT_EXTENTS isSymlink")
		} else {
			panic("not support XFS_DINODE_FMT_EXTENTS")
		}
	case XFS_DINODE_FMT_BTREE:
		panic("not support XFS_DINODE_FMT_BTREE")
	case XFS_DINODE_FMT_UUID:
		panic("not support XFS_DINODE_FMT_UUID")
	case XFS_DINODE_FMT_RMAP:
		panic("not support XFS_DINODE_FMT_RMAP")
	default:
		panic("not support")
	}

	if inode.inodeCore.Forkoff != 0 {
		panic("has extend attribute fork")
	}

	// TODO: Need parse extended attribute fork.
	ioutil.ReadAll(r)
	return &inode, nil
}

func (i *Inode) String() string {
	var s string
	s = fmt.Sprintf("%+v\n", i.inodeCore)

	if i.directoryLocal != nil {
		s = s + fmt.Sprintf("%+v\n", i.directoryLocal)
	}
	if i.directoryExtents != nil {
		s = s + fmt.Sprintf("%+v\n", i.directoryExtents)
		//  fmt.Println(i.directoryExtents.bmbtRec.Unpack())
	}
	if i.regularExtent != nil {
		s = s + fmt.Sprintf("%+v\n", i.regularExtent)
	}

	if i.symlinkString != nil {
		s = s + fmt.Sprintf("%+v\n", i.symlinkString)
	}

	if i.device != nil {
		s = s + "DEVICE\n"
	}

	return s
}

func parseEntry(r io.Reader) (*Dir2SfEntry, error) {
	var entry Dir2SfEntry
	if err := binary.Read(r, binary.BigEndian, &entry.Namelen); err != nil {
		return nil, err
	}

	if err := binary.Read(r, binary.BigEndian, &entry.Offset); err != nil {
		return nil, err
	}
	buf := make([]byte, entry.Namelen)
	i, err := r.Read(buf)
	if err != nil {
		return nil, err
	}
	if i != int(entry.Namelen) {
		return nil, errors.New("")
	}
	entry.Name = string(buf)
	if err := binary.Read(r, binary.BigEndian, &entry.Ftype); err != nil {
		return nil, err
	}

	if err := binary.Read(r, binary.BigEndian, &entry.Inumber); err != nil {
		return nil, err
	}

	return &entry, nil
}

type Inode struct {
	inodeCore InodeCore
	// Device
	device *Device

	// S_IFDIR
	directoryLocal   *DirectoryLocal
	directoryExtents *DirectoryExtents

	// S_IFREG
	regularExtent *RegularExtent

	// S_IFLNK
	symlinkString *SymlinkString
}

type RegularExtent struct {
	bmbtRec BmbtRec
}

type DirectoryExtents struct {
	bmbtRec BmbtRec
}

type DirectoryLocal struct {
	dir2SfHdr Dir2SfHdr
	entries   []Dir2SfEntry
}

// https://github.com/torvalds/linux/blob/d2b6f8a179194de0ffc4886ffc2c4358d86047b8/fs/xfs/libxfs/xfs_format.h#L1787
type BmbtRec struct {
	L0 uint64
	L1 uint64
}

// https://github.com/torvalds/linux/blob/d2b6f8a179194de0ffc4886ffc2c4358d86047b8/fs/xfs/libxfs/xfs_bmap_btree.c#L60
func (b BmbtRec) Unpack() BmbtIrec {
	return BmbtIrec{
		StartOff:   (b.L0 & Mask64Lo(64-BMBT_EXNTFLAG_BITLEN)) >> 9,
		StartBlock: ((b.L0 & Mask64Lo(9)) << 43) | (b.L1 >> 21),
		BlockCount: (b.L1 & Mask64Lo(21)),
	}
}

func Mask64Lo(n int) uint64 {
	return (1 << n) - 1
}

// https://github.com/torvalds/linux/blob/5bfc75d92efd494db37f5c4c173d3639d4772966/fs/xfs/libxfs/xfs_types.h#L162
type BmbtIrec struct {
	StartOff   uint64
	StartBlock uint64
	BlockCount uint64
	State      uint8
}

// https://github.com/torvalds/linux/blob/5bfc75d92efd494db37f5c4c173d3639d4772966/fs/xfs/libxfs/xfs_da_format.h#L203-L207
type Dir2SfHdr struct {
	Count   uint8
	I8Count uint8
	Parent  uint32
}

// https://github.com/torvalds/linux/blob/5bfc75d92efd494db37f5c4c173d3639d4772966/fs/xfs/libxfs/xfs_da_format.h#L209-L220
type Dir2SfEntry struct {
	Namelen uint8
	Offset  [2]uint8
	Name    string
	Ftype   uint8
	Inumber uint32
}

type Device struct{}

type SymlinkString struct {
	Name string
}

type InodeCore struct {
	Magic        [2]byte
	Mode         uint16
	Version      uint8
	Format       uint8
	OnLink       uint16
	UID          uint32
	GID          uint32
	NLink        uint32
	ProjId       uint16
	Padding      [8]byte
	Flushiter    uint16
	Atime        uint64
	Mtime        uint64
	Ctime        uint64
	Size         uint64
	Nblocks      uint64
	Extsize      uint32
	Nextents     uint32
	Anextents    uint16
	Forkoff      uint8
	Aformat      uint8
	Dmevmask     uint32
	Dmstate      uint16
	Flags        uint16
	Gen          uint32
	NextUnlinked uint32

	CRC         uint32
	Changecount uint64
	Lsn         uint64
	Flags2      uint64
	Cowextsize  uint32
	Padding2    [12]byte
	Crtime      uint64
	Ino         uint64
	MetaUUID    [16]byte
}

func (m InodeCore) IsDir() bool {
	return m.Mode&0x4000 != 0
}

func (m InodeCore) IsRegular() bool {
	return m.Mode&0x8000 != 0
}

func (m InodeCore) IsSymlink() bool {
	return m.Mode&0xA000 != 0
}

func (ic InodeCore) isSupported() bool {
	if ic.Version == 3 {
		return true
	}
	return false
}

type InobtRec struct {
	Startino  uint32
	Freecount uint32
	Free      uint64
}
