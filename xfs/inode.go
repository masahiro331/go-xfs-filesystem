package xfs

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"io"
	"unsafe"

	"golang.org/x/xerrors"

	"github.com/masahiro331/go-xfs-filesystem/log"
	"github.com/masahiro331/go-xfs-filesystem/xfs/utils"
)

var (
	XFS_DIR2_SPACE_SIZE  = int64(1) << (32 + XFS_DIR2_DATA_ALIGN_LOG)
	XFS_DIR2_DATA_OFFSET = XFS_DIR2_DATA_SPACE * XFS_DIR2_SPACE_SIZE
	XFS_DIR2_LEAF_OFFSET = XFS_DIR2_LEAF_SPACE * XFS_DIR2_SPACE_SIZE
	XFS_DIR2_FREE_OFFSET = XFS_DIR2_FREE_SPACE * XFS_DIR2_SPACE_SIZE

	_ Entry = &Dir2DataEntry{}
	_ Entry = &Dir2SfEntry{}
)

type Inode struct {
	inodeCore InodeCore
	// Device
	device *Device

	// S_IFDIR
	directoryLocal   *DirectoryLocal
	directoryExtents *DirectoryExtents
	directoryBtree   *Btree

	// S_IFREG
	regularExtent *RegularExtent
	regularBtree  *Btree

	// S_IFLNK
	symlinkString *SymlinkString
}

type RegularExtent struct {
	bmbtRecs []BmbtRec
}

type DirectoryExtents struct {
	bmbtRecs []BmbtRec
}

type Btree struct {
	bmbrBlock BmbrBlock
	bmbtRecs  []BmbtRec
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

// https://github.com/torvalds/linux/blob/5bfc75d92efd494db37f5c4c173d3639d4772966/fs/xfs/libxfs/xfs_types.h#L162
type BmbtIrec struct {
	StartOff   uint64
	StartBlock uint64
	BlockCount uint64
	State      uint8
}

// https://github.com/torvalds/linux/blob/d2b6f8a179194de0ffc4886ffc2c4358d86047b8/fs/xfs/libxfs/xfs_format.h#L1761
type BmbrBlock struct {
	Level   uint16
	Numrecs uint16
	keys    []BmbtKey
	ptrs    []BmbtPtr
}

// BtreeBlockV4 is the V4 (non-CRC) long format B+tree block header (24 bytes).
// https://github.com/torvalds/linux/blob/d2b6f8a179194de0ffc4886ffc2c4358d86047b8/fs/xfs/libxfs/xfs_format.h#L1843
type BtreeBlockV4 struct {
	Magic      uint32
	Level      uint16
	Numrecs    uint16
	BbLeftsib  int64
	BbRightsib int64
}

// BtreeBlock is the V5 (CRC) long format B+tree block header (72 bytes).
// https://github.com/torvalds/linux/blob/d2b6f8a179194de0ffc4886ffc2c4358d86047b8/fs/xfs/libxfs/xfs_format.h#L1868
type BtreeBlock struct {
	BtreeBlockV4

	// Long version header
	// https://github.com/torvalds/linux/blob/d2b6f8a179194de0ffc4886ffc2c4358d86047b8/fs/xfs/libxfs/xfs_format.h#L1855
	BbBlockNo uint64
	BbLsn     uint64
	UUID      [16]byte
	BbOwner   uint64
	CRC       uint32
	Padding   int32
}

// https://github.com/torvalds/linux/blob/d2b6f8a179194de0ffc4886ffc2c4358d86047b8/fs/xfs/libxfs/xfs_format.h#L1821
type BmbtKey uint64

type BmbtPtr uint64

// https://github.com/torvalds/linux/blob/5bfc75d92efd494db37f5c4c173d3639d4772966/fs/xfs/libxfs/xfs_da_format.h#L203-L207
type Dir2SfHdr struct {
	Count   uint8
	I8Count uint8
	Parent  uint32
}

type Dir2Block struct {
	Header  Dir3DataHdr
	Entries []Dir2DataEntry

	UnusedEntries []Dir2DataUnused
	Leafs         []Dir2LeafEntry
	Tail          Dir2BlockTail
}

type Dir2BlockTail struct {
	Count uint32
	Stale uint32
}

type Dir2LeafEntry struct {
	Hashval uint32
	Address uint32
}

// https://github.com/torvalds/linux/blob/5bfc75d92efd494db37f5c4c173d3639d4772966/fs/xfs/libxfs/xfs_da_format.h#L320-L324
type Dir3DataHdr struct {
	Dir3BlkHdr
	Frees   [XFS_DIR2_DATA_FD_COUNT]Dir2DataFree
	Padding uint32
}

// https://github.com/torvalds/linux/blob/5bfc75d92efd494db37f5c4c173d3639d4772966/fs/xfs/libxfs/xfs_da_format.h#L311-L318
type Dir3BlkHdr struct {
	Magic    uint32
	CRC      uint32
	BlockNo  uint64
	Lsn      uint64
	MetaUUID [16]byte
	Owner    uint64
}

// https://github.com/torvalds/linux/blob/5bfc75d92efd494db37f5c4c173d3639d4772966/fs/xfs/libxfs/xfs_da_format.h#L353-L358
type Dir2DataUnused struct {
	Freetag uint16
	Length  uint16
	/* variable offset */
	Tag uint16
}

type Dir2DataFree struct {
	Offset uint16
	Length uint16
}

type Entry interface {
	Name() string
	FileType() uint8
	InodeNumber() uint64
}

// https://github.com/torvalds/linux/blob/5bfc75d92efd494db37f5c4c173d3639d4772966/fs/xfs/libxfs/xfs_da_format.h#L339-L345
type Dir2DataEntry struct {
	Inumber   uint64
	Namelen   uint8
	EntryName string
	Filetype  uint8
	Tag       uint16
}

// https://github.com/torvalds/linux/blob/5bfc75d92efd494db37f5c4c173d3639d4772966/fs/xfs/libxfs/xfs_da_format.h#L209-L220
type Dir2SfEntry struct {
	Namelen   uint8
	Offset    [2]uint8
	EntryName string
	Filetype  uint8
	Inumber   uint64
	Inumber32 uint32
}

type Device struct{}

type SymlinkString struct {
	Name string
}

// InodeCoreBase is the common 96-byte inode core header shared by V1, V2, and V3 inodes.
type InodeCoreBase struct {
	Magic        uint16
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
}

// InodeCoreV3Ext is the 76-byte V3 extension (CRC, timestamps, UUID, etc.).
type InodeCoreV3Ext struct {
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

// InodeCore represents the full inode core header.
// For V3 inodes this is 176 bytes (base + v3ext).
// For V1/V2 inodes only the base (100 bytes) is populated.
type InodeCore struct {
	InodeCoreBase
	InodeCoreV3Ext
}

type InobtRec struct {
	Startino  uint32
	Freecount uint32
	Free      uint64
}

// Holemask returns the sparse inode holemask from ir_holemask (bits 31:16 of Freecount).
// Freecount is read via binary.BigEndian, so host-order bit shifts match the on-disk layout.
// Each bit represents 4 inodes; 1 = hole (no inode allocated).
func (r InobtRec) Holemask() uint16 {
	return uint16(r.Freecount >> 16)
}

// InoCount returns the number of valid inodes in this chunk (bits 15:8 of Freecount).
func (r InobtRec) InoCount() uint8 {
	return uint8(r.Freecount >> 8)
}

// InoFreecount returns the number of free inodes in this chunk (bits 7:0 of Freecount).
func (r InobtRec) InoFreecount() uint8 {
	return uint8(r.Freecount)
}

func (xfs *FileSystem) inodeFormatDevice(inode Inode) Inode {
	inode.device = &Device{}
	return inode
}

func (xfs *FileSystem) inodeFormatLocal(r io.Reader, inode Inode) (Inode, error) {
	if inode.inodeCore.IsDir() {
		inode.directoryLocal = &DirectoryLocal{}
		if err := binary.Read(r, binary.BigEndian, &inode.directoryLocal.dir2SfHdr); err != nil {
			return Inode{}, xerrors.Errorf("failed to read XFS_DINODE_FMT_LOCAL directory error: %w", err)
		}

		var isI8count bool
		if inode.directoryLocal.dir2SfHdr.I8Count != 0 {
			isI8count = true
		}
		for i := 0; i < int(inode.directoryLocal.dir2SfHdr.Count); i++ {
			entry, err := parseEntry(r, isI8count)
			if err != nil {
				return Inode{}, xerrors.Errorf("failed to parse entries[%d]: %w", i, err)
			}
			inode.directoryLocal.entries = append(inode.directoryLocal.entries, *entry)
		}
	} else if inode.inodeCore.IsSymlink() {
		inode.symlinkString = &SymlinkString{}
		buf := make([]byte, inode.inodeCore.Size)
		n, err := r.Read(buf)
		if err != nil {
			return Inode{}, xerrors.Errorf("failed to read XFS_DINODE_FMT_LOCAL symlink error: %w", err)
		}
		if uint64(n) != inode.inodeCore.Size {
			return Inode{}, xerrors.Errorf(ErrReadSizeFormat, n, inode.inodeCore.Size)
		}
		inode.symlinkString.Name = string(buf)
	} else {
		log.Logger.Warn("not support XFS_DINODE_FMT_LOCAL")
	}
	return inode, nil
}

func (xfs *FileSystem) parseBmbtRecs(r io.Reader, count uint32) ([]BmbtRec, error) {
	var bmbtRecs []BmbtRec
	for i := uint32(0); i < count; i++ {
		var bmbtRec BmbtRec
		if err := binary.Read(r, binary.BigEndian, &bmbtRec); err != nil {
			return nil, xerrors.Errorf("read xfs_bmbt_irec error: %w", err)
		}
		bmbtRecs = append(bmbtRecs, bmbtRec)
	}
	return bmbtRecs, nil
}

func (xfs *FileSystem) inodeFormatExtents(r io.Reader, inode Inode) (Inode, error) {
	var err error
	if inode.inodeCore.IsDir() {
		inode.directoryExtents = &DirectoryExtents{}
		inode.directoryExtents.bmbtRecs, err = xfs.parseBmbtRecs(r, inode.inodeCore.Nextents)
		if err != nil {
			return Inode{}, xerrors.Errorf("failed to parse directory bmbt recs: %w", err)
		}
	} else if inode.inodeCore.IsRegular() {
		inode.regularExtent = &RegularExtent{}
		inode.regularExtent.bmbtRecs, err = xfs.parseBmbtRecs(r, inode.inodeCore.Nextents)
		if err != nil {
			return Inode{}, xerrors.Errorf("failed to parse regular bmbt recs: %w", err)
		}
	} else if inode.inodeCore.IsSymlink() {
		log.Logger.Warn("not support XFS_DINODE_FMT_EXTENTS isSymlink")
	} else {
		log.Logger.Debugf("%+v\n", inode)
		log.Logger.Debug("not support XFS_DINODE_FMT_EXTENTS")
	}

	return inode, nil
}

func (xfs *FileSystem) walkBtree(level uint16, ptrs []BmbtPtr) (uint16, []BmbtPtr, error) {
	if level == 1 {
		return level, ptrs, nil
	}

	var retPtrs []BmbtPtr
	for _, ptr := range ptrs {
		_, nodePtrs, err := xfs.parseBtreeNode(int64(ptr))
		if err != nil {
			return 0, nil, xerrors.Errorf("parse btree node (ptr: %d) error: %w", ptr, err)
		}
		retPtrs = append(retPtrs, nodePtrs...)
	}
	level--
	return xfs.walkBtree(level, retPtrs)
}

func (xfs *FileSystem) parseMultiLevelBtree(level uint16, ptrs []BmbtPtr) ([]BmbtRec, error) {
	_, leafPtrs, err := xfs.walkBtree(level, ptrs)
	if err != nil {
		return nil, xerrors.Errorf("walk Btree error: %w", err)
	}
	return xfs.parseSingleLevelBtree(leafPtrs)
}

func (xfs *FileSystem) parseSingleLevelBtree(ptrs []BmbtPtr) ([]BmbtRec, error) {
	var ret []BmbtRec
	for _, ptr := range ptrs {
		recs, err := xfs.parseBtreeLeafNode(int64(ptr))
		if err != nil {
			return nil, xerrors.Errorf("parse btree leaf node(ptr: %d) error: %w", ptr, err)
		}

		ret = append(ret, recs...)
	}
	return ret, nil
}

func (xfs *FileSystem) parseBmbtKeyPtr(r io.Reader, numrecs uint16, maxrecs int) ([]BmbtKey, []BmbtPtr, error) {
	// parse bmbt keys
	var keys []BmbtKey
	for i := uint16(0); i < numrecs; i++ {
		var key BmbtKey
		if err := binary.Read(r, binary.BigEndian, &key); err != nil {
			return nil, nil, xerrors.Errorf("failed to read regular bmbt key: %w", err)
		}
		keys = append(keys, key)
	}

	// Skip tail keys padding between keys[] and ptrs[] arrays.
	// The on-disk layout is: keys[maxrecs] + ptrs[maxrecs], so we must skip
	// (maxrecs - numrecs) unused key slots to reach the pointer array.
	if int(numrecs) > maxrecs {
		return nil, nil, xerrors.Errorf("numrecs (%d) exceeds maxrecs (%d)", numrecs, maxrecs)
	}
	tailKeysCount := maxrecs - int(numrecs)
	if tailKeysCount > 0 {
		tailBuf := make([]byte, 8*tailKeysCount)
		n, err := r.Read(tailBuf)
		if err != nil {
			return nil, nil, xerrors.Errorf("failed to read tail key buf: %w", err)
		}
		if n != len(tailBuf) {
			return nil, nil, xerrors.Errorf("failed to read tail buf length actual (%d), expected (%d)", n, len(tailBuf))
		}
	}

	// parse bmbt ptr
	var ptrs []BmbtPtr
	for i := uint16(0); i < numrecs; i++ {
		var ptr BmbtPtr
		if err := binary.Read(r, binary.BigEndian, &ptr); err != nil {
			return nil, nil, xerrors.Errorf("failed to read regular bmbt ptr: %w", err)
		}
		ptrs = append(ptrs, ptr)
	}
	return keys, ptrs, nil
}

func (xfs *FileSystem) parseBmbrBlock(r io.Reader, inode Inode) (*BmbrBlock, error) {
	var bmbrBlock BmbrBlock
	var err error
	if err := binary.Read(r, binary.BigEndian, &bmbrBlock.Level); err != nil {
		return nil, xerrors.Errorf("binary read bmbr block level error: %w", err)
	}
	if err := binary.Read(r, binary.BigEndian, &bmbrBlock.Numrecs); err != nil {
		return nil, xerrors.Errorf("binary read bmbr block numerecs error: %w", err)
	}

	// BMBR root maxrecs: layout is bmdr_header (4 bytes) + keys[maxrecs] (8 each) + ptrs[maxrecs] (8 each)
	bmbrMaxrecs := (xfs.DataForkSize(inode.inodeCore.Forkoff, inode.inodeCore.Version) - 4) / 16
	bmbrBlock.keys, bmbrBlock.ptrs, err = xfs.parseBmbtKeyPtr(r, bmbrBlock.Numrecs, bmbrMaxrecs)
	if err != nil {
		return nil, xerrors.Errorf("parse bmbr key-ptr error: %w", err)
	}
	return &bmbrBlock, nil
}

func (xfs *FileSystem) inodeFormatBtree(r io.Reader, inode Inode) (Inode, error) {
	bmbrBlock, err := xfs.parseBmbrBlock(r, inode)
	if err != nil {
		return Inode{}, xerrors.Errorf("parse bmbr block error: %w", err)
	}
	btree := &Btree{
		bmbrBlock: *bmbrBlock,
	}
	if bmbrBlock.Level == 1 {
		btree.bmbtRecs, err = xfs.parseSingleLevelBtree(
			bmbrBlock.ptrs,
		)
		if err != nil {
			return Inode{}, xerrors.Errorf("parse single level btree error: %w", err)
		}
	} else if bmbrBlock.Level > 1 {
		btree.bmbtRecs, err = xfs.parseMultiLevelBtree(
			bmbrBlock.Level,
			bmbrBlock.ptrs,
		)
		if err != nil {
			return Inode{}, xerrors.Errorf("parse multi level btree error: %w", err)
		}
	}
	if inode.inodeCore.IsRegular() {
		inode.regularBtree = btree
	}
	if inode.inodeCore.IsDir() {
		inode.directoryBtree = btree
	}

	return inode, nil
}

func (xfs *FileSystem) ParseInode(ino uint64) (*Inode, error) {
	var inode Inode
	c, ok := xfs.cache.Get(inodeCacheKey(ino))
	if ok {
		i := c.(Inode)
		if ok {
			return &i, nil
		}
	}

	_, err := xfs.seekInode(ino)
	if err != nil {
		return nil, xerrors.Errorf("failed to seek inode: %w", err)
	}

	sectorReader, err := utils.NewSectorReader(int(xfs.PrimaryAG.SuperBlock.Inodesize))
	if err != nil {
		return nil, xerrors.Errorf("failed to create sector reader: %w", err)
	}
	buf, err := sectorReader.ReadSector(xfs.r)
	if err != nil {
		return nil, xerrors.Errorf("failed to read sector: %w", err)
	}
	r := bytes.NewReader(buf)

	// Stage 1: Read the base 96-byte inode core (common to V1/V2/V3)
	if err := binary.Read(r, binary.BigEndian, &inode.inodeCore.InodeCoreBase); err != nil {
		return nil, xerrors.Errorf("failed to read InodeCoreBase: %w", err)
	}

	if inode.inodeCore.Magic != XFS_DINODE_MAGIC {
		return nil, xerrors.Errorf("invalid magic byte error")
	}

	if !inode.inodeCore.isSupported() {
		return nil, xerrors.Errorf("not support inode version %d", inode.inodeCore.Version)
	}

	if inode.inodeCore.Version == 3 {
		// Stage 2: Read V3 extension (76 bytes)
		if err := binary.Read(r, binary.BigEndian, &inode.inodeCore.InodeCoreV3Ext); err != nil {
			return nil, xerrors.Errorf("failed to read InodeCoreV3Ext: %w", err)
		}
	} else {
		// V1/V2: Promote OnLink to NLink, set Ino from argument
		if inode.inodeCore.Version == 1 {
			inode.inodeCore.NLink = uint32(inode.inodeCore.OnLink)
		}
		inode.inodeCore.Ino = ino
	}

	switch inode.inodeCore.Format {
	case XFS_DINODE_FMT_DEV:
		inode = xfs.inodeFormatDevice(inode)
	case XFS_DINODE_FMT_LOCAL:
		inode, err = xfs.inodeFormatLocal(r, inode)
		if err != nil {
			log.Logger.Debug("\n", hex.Dump(buf))
			return nil, xerrors.Errorf("parse inode format local: %w", err)
		}
	case XFS_DINODE_FMT_EXTENTS:
		inode, err = xfs.inodeFormatExtents(r, inode)
		if err != nil {
			log.Logger.Debug("\n", hex.Dump(buf))
			return nil, xerrors.Errorf("parse inode format extents: %w", err)
		}
	case XFS_DINODE_FMT_BTREE:
		inode, err = xfs.inodeFormatBtree(r, inode)
		if err != nil {
			log.Logger.Debug("\n", hex.Dump(buf))
			return nil, xerrors.Errorf("parse inode format btree: %w", err)
		}
	case XFS_DINODE_FMT_UUID:
		log.Logger.Warn("not support XFS_DINODE_FMT_UUID")
	case XFS_DINODE_FMT_RMAP:
		log.Logger.Warn("not support XFS_DINODE_FMT_RMAP")
	default:
		log.Logger.Warnf("not support inode format(%d)", inode.inodeCore.Format)
	}

	// TODO: support extend attribute fork , see. Chapter 19 Extended Attributes
	// if inode.inodeCore.Forkoff != 0 {
	// 	panic("has extend attribute fork")
	// }

	xfs.cache.Add(inodeCacheKey(ino), inode)
	return &inode, nil
}

// parseBtreeBlock reads a V4 or V5 long format B+tree block header.
// It returns the parsed block and the on-disk header size in bytes.
func (xfs *FileSystem) parseBtreeBlock(r io.Reader) (*BtreeBlock, int, error) {
	// Read the V4 base header first (24 bytes) to inspect magic.
	var v4 BtreeBlockV4
	if err := binary.Read(r, binary.BigEndian, &v4); err != nil {
		return nil, 0, xerrors.Errorf("failed to read b+tree block: %w", err)
	}

	btreeBlock := &BtreeBlock{BtreeBlockV4: v4}

	switch v4.Magic {
	case XFS_BMAP_CRC_MAGIC:
		// V5: read the remaining 48 bytes (72 - 24) in one call.
		var ext struct {
			BbBlockNo uint64
			BbLsn     uint64
			UUID      [16]byte
			BbOwner   uint64
			CRC       uint32
			Padding   int32
		}
		if err := binary.Read(r, binary.BigEndian, &ext); err != nil {
			return nil, 0, xerrors.Errorf("failed to read V5 b+tree block extension: %w", err)
		}
		btreeBlock.BbBlockNo = ext.BbBlockNo
		btreeBlock.BbLsn = ext.BbLsn
		btreeBlock.UUID = ext.UUID
		btreeBlock.BbOwner = ext.BbOwner
		btreeBlock.CRC = ext.CRC
		btreeBlock.Padding = ext.Padding
		return btreeBlock, binary.Size(BtreeBlock{}), nil
	case XFS_BMAP_MAGICa:
		// V4: header is complete (24 bytes).
		return btreeBlock, binary.Size(BtreeBlockV4{}), nil
	default:
		return nil, 0, xerrors.Errorf("unsupported block header magic: 0x%x", v4.Magic)
	}
}

func (xfs *FileSystem) parseBtreeNode(blockNumber int64) ([]BmbtKey, []BmbtPtr, error) {
	physicalBlockOffset := xfs.PrimaryAG.SuperBlock.BlockToPhysicalOffset(uint64(blockNumber))
	_, err := xfs.seekBlock(physicalBlockOffset)
	if err != nil {
		return nil, nil, xerrors.Errorf("failed to seek block: %w", err)
	}
	b, err := xfs.readBlock(1)
	if err != nil {
		return nil, nil, xerrors.Errorf("failed to read block: %w", err)
	}

	r := bytes.NewReader(b)
	btreeBlock, headerSize, err := xfs.parseBtreeBlock(r)
	if err != nil {
		return nil, nil, xerrors.Errorf("parse btree node (offset: %d) error: %w", blockNumber, err)
	}

	// Intermediate/leaf node maxrecs: layout is header + keys[maxrecs] (8 each) + ptrs[maxrecs] (8 each)
	nodeMaxrecs := (int(xfs.PrimaryAG.SuperBlock.BlockSize) - headerSize) / 16
	keys, ptrs, err := xfs.parseBmbtKeyPtr(r, btreeBlock.Numrecs, nodeMaxrecs)
	if err != nil {
		return nil, nil, xerrors.Errorf("parse bmbr key-ptr error: %w", err)
	}
	return keys, ptrs, nil
}

func (xfs *FileSystem) parseBtreeLeafNode(blockNumber int64) ([]BmbtRec, error) {
	physicalBlockOffset := xfs.PrimaryAG.SuperBlock.BlockToPhysicalOffset(uint64(blockNumber))
	_, err := xfs.seekBlock(physicalBlockOffset)
	if err != nil {
		return nil, xerrors.Errorf("failed to seek block: %w", err)
	}
	b, err := xfs.readBlock(1)
	if err != nil {
		return nil, xerrors.Errorf("failed to read block: %w", err)
	}

	r := bytes.NewReader(b)
	btreeBlock, _, err := xfs.parseBtreeBlock(r)
	if err != nil {
		return nil, xerrors.Errorf("parse btree node (offset: %d) error: %w", blockNumber*int64(xfs.PrimaryAG.SuperBlock.BlockSize), err)
	}

	if btreeBlock.Level > 1 {
		return nil, xerrors.Errorf("unsupported deep b+tree level: %d", btreeBlock.Level)
	}

	recs := []BmbtRec{}
	for i := uint16(0); i < btreeBlock.Numrecs; i++ {
		var bmbtRec BmbtRec
		if err := binary.Read(r, binary.BigEndian, &bmbtRec); err != nil {
			return nil, xerrors.Errorf("failed to read extents xfs_bmbt_irec: %w", err)
		}
		recs = append(recs, bmbtRec)
	}
	return recs, nil
}

// https://github.com/torvalds/linux/blob/d2b6f8a179194de0ffc4886ffc2c4358d86047b8/fs/xfs/libxfs/xfs_bmap_btree.c#L316
func BmbrMaxRecs(blocklen int) int {
	return blocklen / 16
}

// https://github.com/torvalds/linux/blob/d2b6f8a179194de0ffc4886ffc2c4358d86047b8/fs/xfs/libxfs/xfs_format.h#L1077-L1078
func (xfs *FileSystem) DataForkSize(forkoff uint8, version uint8) int {
	if forkoff > 0 {
		return int(forkoff) << 3
	}
	coreSize := INODEV1V2_SIZE
	if version == 3 {
		coreSize = INODEV3_SIZE
	}
	return int(xfs.PrimaryAG.SuperBlock.Inodesize) - coreSize
}

func (i *Inode) AttributeOffset() uint32 {
	coreSize := uint32(INODEV3_SIZE)
	if i.inodeCore.Version < 3 {
		coreSize = uint32(INODEV1V2_SIZE)
	}
	return uint32(i.inodeCore.Forkoff)*8 + coreSize
}

// Dir2DataHdr is the V4 directory data block header (16 bytes: magic + frees[3] + no padding).
// https://github.com/torvalds/linux/blob/5bfc75d92efd494db37f5c4c173d3639d4772966/fs/xfs/libxfs/xfs_da_format.h#L295-L298
type Dir2DataHdr struct {
	Magic uint32
	Frees [XFS_DIR2_DATA_FD_COUNT]Dir2DataFree
}

// skipDirBlockHeader reads and discards the appropriate directory block header
// based on the magic number (V4 or V5 format).
func (xfs *FileSystem) skipDirBlockHeader(r io.Reader, magic uint32) error {
	switch magic {
	case XFS_DIR3_DATA_MAGIC, XFS_DIR3_BLOCK_MAGIC:
		var hdr Dir3DataHdr
		return binary.Read(r, binary.BigEndian, &hdr)
	case XFS_DIR2_DATA_MAGIC, XFS_DIR2_BLOCK_MAGIC:
		var hdr Dir2DataHdr
		return binary.Read(r, binary.BigEndian, &hdr)
	default:
		return xerrors.Errorf("unknown directory magic: 0x%x", magic)
	}
}

// Parse XDB3block, XDB3 block is single block architecture
func (xfs *FileSystem) parseXDB3Block(r io.Reader) ([]Dir2DataEntry, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, xerrors.Errorf("failed to read XDB3 block reader: %w", err)
	}
	var tail Dir2BlockTail

	tailBlockOffset := len(buf) - int(unsafe.Sizeof(tail))
	if tailBlockOffset > len(buf) {
		return nil, xerrors.Errorf("failed to calculate tail block offset: %d", tailBlockOffset)
	}
	tailReader := bytes.NewReader(buf[tailBlockOffset:])
	if err := binary.Read(tailReader, binary.BigEndian, &tail); err != nil {
		return nil, xerrors.Errorf("failed to read tail binary: %w", err)
	}

	dataEndOffset := uint32(len(buf)) - (tail.Count*LEAF_ENTRY_SIZE + uint32(unsafe.Sizeof(tail)))
	if dataEndOffset > uint32(len(buf)) {
		return nil, xerrors.Errorf("failed to calculate data end offset: %d", dataEndOffset)
	}
	reader := bytes.NewReader(buf[:dataEndOffset])

	dir2DataEntries, err := xfs.parseDir2DataEntry(reader)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse dir2 Data Entry: %w", err)
	}
	return dir2DataEntries, nil
}

// Parse XDD3block, XDD3 block is multi block architecture
func (xfs *FileSystem) parseXDD3Block(r io.Reader) ([]Dir2DataEntry, error) {
	dir2DataEntries, err := xfs.parseDir2DataEntry(r)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse dir2 Data Entry: %w", err)
	}
	return dir2DataEntries, nil
}

func (xfs *FileSystem) parseDir2DataEntry(r io.Reader) ([]Dir2DataEntry, error) {
	entries := []Dir2DataEntry{}
	for {
		entry := Dir2DataEntry{}

		// Parse Inode number
		if err := binary.Read(r, binary.BigEndian, &entry.Inumber); err != nil {
			if err == io.EOF {
				return entries, nil
			}
			return nil, xerrors.Errorf("failed to read inumber binary: %w", err)
		}

		if (entry.Inumber >> 48) == XFS_DIR2_DATA_FREE_TAG {
			freeLen := (entry.Inumber >> 32) & Mask64Lo(16)
			if freeLen != 8 {
				// Read FreeTag tail
				_, err := r.Read(make([]byte, freeLen-0x08))
				if err != nil {
					return nil, xerrors.Errorf("failed to read unused padding: %w", err)
				}
			}
			continue
		}

		// Parse Name length
		if err := binary.Read(r, binary.BigEndian, &entry.Namelen); err != nil {
			return nil, xerrors.Errorf("failed to read name length: %w", err)
		}

		// Parse Name
		nameBuf := make([]byte, entry.Namelen)
		n, err := r.Read(nameBuf)
		if err != nil {
			return nil, xerrors.Errorf("failed to read name: %w", err)
		}
		if n != int(entry.Namelen) {
			return nil, xerrors.Errorf("failed to read name: expected namelen(%d) actual(%d)", entry.Namelen, n)
		}
		entry.EntryName = string(nameBuf)

		// Parse FileType
		if err := binary.Read(r, binary.BigEndian, &entry.Filetype); err != nil {
			return nil, xerrors.Errorf("failed to read file type: %w", err)
		}

		// Read Alignment, Dir2DataEntry is 8byte alignment
		align := (int(unsafe.Sizeof(entry.Inumber)) +
			int(unsafe.Sizeof(entry.Namelen)) +
			int(unsafe.Sizeof(entry.Filetype)) +
			int(unsafe.Sizeof(entry.Tag)) +
			int(entry.Namelen)) % 8
		if align != 0 {
			n, err = r.Read(make([]byte, 8-align))
			if err != nil {
				return nil, xerrors.Errorf("failed to read alignment: %w", err)
			}
			if n != int(8-align) {
				return nil, xerrors.Errorf("failed to read alignment: expected (%d) actual(%d)", 8-align, n)
			}
		}

		// Read Tag
		if err := binary.Read(r, binary.BigEndian, &entry.Tag); err != nil {
			return nil, xerrors.Errorf("failed to read tag: %w", err)
		}

		entries = append(entries, entry)
	}
}

func (xfs *FileSystem) parseDir2Block(bmbtIrec BmbtIrec) ([]Dir2DataEntry, error) {
	block := Dir2Block{}
	/*
		Skip Leaf and Free node.
		The "leaf" block has a special offset defined by XFS_DIR2_LEAF_OFFSET. Currently, this is 32GB and in the extent view,
		a block offset of 32GB / sb_blocksize. On a 4KB block filesystem, this is 0x800000 (8388608 decimal).
	*/
	if int64(bmbtIrec.StartOff)*int64(xfs.PrimaryAG.SuperBlock.BlockSize) >= int64(XFS_DIR2_LEAF_OFFSET) {
		return nil, nil
	}

	var buf []byte
	for blockOffset := bmbtIrec.StartBlock; blockOffset < bmbtIrec.StartBlock+bmbtIrec.BlockCount; blockOffset++ {
		physicalBlockOffset := xfs.PrimaryAG.SuperBlock.BlockToPhysicalOffset(blockOffset)
		_, err := xfs.seekBlock(physicalBlockOffset)
		if err != nil {
			return nil, xerrors.Errorf("failed to seek block: %w", err)
		}
		blockData, err := xfs.readBlock(1)
		if err != nil {
			return nil, xerrors.Errorf("failed to read block: %w", err)
		}
		buf = append(buf, blockData...)

		// if the next block is not a leader, it is a continuation of the previous block
		if blockOffset != bmbtIrec.StartBlock+bmbtIrec.BlockCount-1 && // not last block
			!xfs.nextBlockIsLeader(blockOffset) { // not leader
			continue
		}

		// if the next block is a leader, it is the last block of the directory
		magicBytes := binary.BigEndian.Uint32(buf[:4])
		reader := bytes.NewReader(buf)
		if err := xfs.skipDirBlockHeader(reader, magicBytes); err != nil {
			return nil, xerrors.Errorf("failed to skip dir block header: %w", err)
		}
		switch magicBytes {
		case XFS_DIR3_DATA_MAGIC, XFS_DIR2_DATA_MAGIC:
			entries, err := xfs.parseXDD3Block(reader)
			if err != nil {
				return nil, xerrors.Errorf("failed to parse dir data block: %w", err)
			}
			block.Entries = append(block.Entries, entries...)
		case XFS_DIR3_BLOCK_MAGIC, XFS_DIR2_BLOCK_MAGIC:
			entries, err := xfs.parseXDB3Block(reader)
			if err != nil {
				return nil, xerrors.Errorf("failed to parse dir block: %w", err)
			}
			block.Entries = append(block.Entries, entries...)
		default:
			return nil, xerrors.Errorf("unknown magic bytes: %x", magicBytes)
		}

		// reset buf
		buf = []byte{}
	}

	return block.Entries, nil
}

func (xfs *FileSystem) nextBlockIsLeader(blockOffset uint64) bool {
	physicalBlockOffset := xfs.PrimaryAG.SuperBlock.BlockToPhysicalOffset(blockOffset + 1)
	if _, err := xfs.seekBlock(physicalBlockOffset); err != nil {
		return false
	}
	blockData, err := xfs.readBlock(1)
	if err != nil {
		return false
	}

	magic := binary.BigEndian.Uint32(blockData[:4])
	return magic == XFS_DIR3_DATA_MAGIC || magic == XFS_DIR3_BLOCK_MAGIC ||
		magic == XFS_DIR2_DATA_MAGIC || magic == XFS_DIR2_BLOCK_MAGIC
}

func parseEntry(r io.Reader, i8count bool) (*Dir2SfEntry, error) {
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
		return nil, xerrors.Errorf("read name error: %s", string(buf))
	}
	entry.EntryName = string(buf)
	if err := binary.Read(r, binary.BigEndian, &entry.Filetype); err != nil {
		return nil, err
	}

	if i8count {
		if err := binary.Read(r, binary.BigEndian, &entry.Inumber); err != nil {
			return nil, err
		}
	} else {
		if err := binary.Read(r, binary.BigEndian, &entry.Inumber32); err != nil {
			return nil, err
		}
		entry.Inumber = uint64(entry.Inumber32)
	}

	return &entry, nil
}

func (ic InodeCore) IsDir() bool {
	return ic.Mode&0xF000 == 0x4000
}

func (ic InodeCore) IsRegular() bool {
	return ic.Mode&0xF000 == 0x8000
}

func (ic InodeCore) IsSocket() bool {
	return ic.Mode&0xF000 == 0xC000
}

func (ic InodeCore) IsSymlink() bool {
	return ic.Mode&0xF000 == 0xA000
}

func (ic InodeCore) isSupported() bool {
	return ic.Version >= 1 && ic.Version <= 3
}

// https://github.com/torvalds/linux/blob/d2b6f8a179194de0ffc4886ffc2c4358d86047b8/fs/xfs/libxfs/xfs_bmap_btree.c#L60
func (b BmbtRec) Unpack() BmbtIrec {
	return BmbtIrec{
		StartOff:   (b.L0 & Mask64Lo(64-BMBT_EXNTFLAG_BITLEN)) >> 9,
		StartBlock: ((b.L0 & Mask64Lo(9)) << 43) | (b.L1 >> 21),
		BlockCount: b.L1 & Mask64Lo(21),
		State:      uint8(b.L0 >> 63),
	}
}

func Mask64Lo(n int64) uint64 {
	return (1 << n) - 1
}

func (e Dir2SfEntry) FileType() uint8 {
	return e.Filetype
}

func (e Dir2DataEntry) FileType() uint8 {
	return e.Filetype
}

func (e Dir2SfEntry) Name() string {
	return e.EntryName
}

func (e Dir2DataEntry) Name() string {
	return e.EntryName
}

func (e Dir2SfEntry) InodeNumber() uint64 {
	return e.Inumber
}

func (e Dir2DataEntry) InodeNumber() uint64 {
	return e.Inumber
}
