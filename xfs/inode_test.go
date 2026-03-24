package xfs

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"
)

func TestDataForkSize(t *testing.T) {
	tests := []struct {
		name      string
		inodesize uint16
		forkoff   uint8
		version   uint8
		expected  int
	}{
		{
			name:      "V3, forkoff=0, inodesize=256",
			inodesize: 256,
			forkoff:   0,
			version:   3,
			expected:  256 - 176, // 80
		},
		{
			name:      "V3, forkoff=0, inodesize=512",
			inodesize: 512,
			forkoff:   0,
			version:   3,
			expected:  512 - 176, // 336
		},
		{
			name:      "V2, forkoff=0, inodesize=256",
			inodesize: 256,
			forkoff:   0,
			version:   2,
			expected:  256 - INODEV1V2_SIZE, // 156
		},
		{
			name:      "V1, forkoff=0, inodesize=256",
			inodesize: 256,
			forkoff:   0,
			version:   1,
			expected:  256 - INODEV1V2_SIZE, // 156
		},
		{
			name:      "V3, forkoff>0, forkoff=24",
			inodesize: 256,
			forkoff:   24,
			version:   3,
			expected:  24 << 3, // 192
		},
		{
			name:      "V2, forkoff>0, forkoff=10",
			inodesize: 512,
			forkoff:   10,
			version:   2,
			expected:  10 << 3, // 80
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := &FileSystem{
				PrimaryAG: AG{
					SuperBlock: SuperBlock{
						Inodesize: tt.inodesize,
					},
				},
			}
			got := fs.DataForkSize(tt.forkoff, tt.version)
			if got != tt.expected {
				t.Errorf("DataForkSize(%d, %d) = %d, want %d", tt.forkoff, tt.version, got, tt.expected)
			}
		})
	}
}

func TestBmbrMaxRecsFromDataFork(t *testing.T) {
	tests := []struct {
		name            string
		inodesize       uint16
		forkoff         uint8
		version         uint8
		expectedMaxrecs int
	}{
		{
			name:            "V3, 256-byte inode, forkoff=0",
			inodesize:       256,
			forkoff:         0,
			version:         3,
			expectedMaxrecs: (80 - 4) / 16, // DataForkSize=80, maxrecs=4
		},
		{
			name:            "V3, 512-byte inode, forkoff=0",
			inodesize:       512,
			forkoff:         0,
			version:         3,
			expectedMaxrecs: (336 - 4) / 16, // DataForkSize=336, maxrecs=20
		},
		{
			name:            "V2, 256-byte inode, forkoff=0",
			inodesize:       256,
			forkoff:         0,
			version:         2,
			expectedMaxrecs: (156 - 4) / 16, // DataForkSize=156, maxrecs=9
		},
		{
			name:            "V3, forkoff=24",
			inodesize:       256,
			forkoff:         24,
			version:         3,
			expectedMaxrecs: (192 - 4) / 16, // DataForkSize=192, maxrecs=11
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := &FileSystem{
				PrimaryAG: AG{
					SuperBlock: SuperBlock{
						Inodesize: tt.inodesize,
					},
				},
			}
			dataForkSize := fs.DataForkSize(tt.forkoff, tt.version)
			maxrecs := (dataForkSize - 4) / 16
			if maxrecs != tt.expectedMaxrecs {
				t.Errorf("maxrecs = %d, want %d (DataForkSize=%d)", maxrecs, tt.expectedMaxrecs, dataForkSize)
			}
		})
	}
}

func TestInodeCoreBaseSize(t *testing.T) {
	size := binary.Size(InodeCoreBase{})
	if size != INODEV1V2_SIZE {
		t.Errorf("InodeCoreBase binary size = %d, want %d", size, INODEV1V2_SIZE)
	}
}

func TestInodeCoreV3ExtSize(t *testing.T) {
	size := binary.Size(InodeCoreV3Ext{})
	expected := INODEV3_SIZE - INODEV1V2_SIZE
	if size != expected {
		t.Errorf("InodeCoreV3Ext binary size = %d, want %d", size, expected)
	}
}

func TestInodeCoreIsSupported(t *testing.T) {
	tests := []struct {
		version  uint8
		expected bool
	}{
		{version: 0, expected: false},
		{version: 1, expected: true},
		{version: 2, expected: true},
		{version: 3, expected: true},
		{version: 4, expected: false},
	}
	for _, tt := range tests {
		ic := InodeCore{}
		ic.Version = tt.version
		got := ic.isSupported()
		if got != tt.expected {
			t.Errorf("isSupported() version=%d: got %v, want %v", tt.version, got, tt.expected)
		}
	}
}

func TestParseInodeV2(t *testing.T) {
	// Build a minimal 512-byte V2 inode binary (big-endian).
	// Inodesize must be in allowedSectorSize (512 or 4096).
	buf := make([]byte, 512)
	// Magic = XFS_DINODE_MAGIC (0x494e)
	buf[0] = 0x49
	buf[1] = 0x4e
	// Mode = regular file (0x8000) | 0644
	buf[2] = 0x81
	buf[3] = 0xa4
	// Version = 2
	buf[4] = 2
	// Format = XFS_DINODE_FMT_EXTENTS
	buf[5] = XFS_DINODE_FMT_EXTENTS
	// OnLink = 1 (uint16 big-endian at offset 6)
	buf[7] = 1
	// NLink = 1 (uint32 big-endian at offset 16)
	buf[19] = 1
	// Nextents = 0 (uint32 at offset 76)
	// Size = 0
	// Everything else stays zero (valid for empty regular file)

	f := bytes.NewReader(buf)
	sr := io.NewSectionReader(f, 0, int64(len(buf)))

	xfs := &FileSystem{
		r: sr,
		PrimaryAG: AG{
			SuperBlock: SuperBlock{
				Inodesize: 512,
				Inopblock: 8,
				Inopblog:  3,
				Agblklog:  19,
				Agblocks:  524288,
				BlockSize: 4096,
			},
		},
		cache: &mockCache[string, any]{},
	}

	inode, err := xfs.ParseInode(0)
	if err != nil {
		t.Fatalf("ParseInode V2 failed: %v", err)
	}

	if inode.inodeCore.Version != 2 {
		t.Errorf("Version = %d, want 2", inode.inodeCore.Version)
	}
	// V2: Ino should be set from argument
	if inode.inodeCore.Ino != 0 {
		t.Errorf("Ino = %d, want 0", inode.inodeCore.Ino)
	}
	// V2: NLink should be read from the binary
	if inode.inodeCore.NLink != 1 {
		t.Errorf("NLink = %d, want 1", inode.inodeCore.NLink)
	}
	if !inode.inodeCore.IsRegular() {
		t.Error("expected regular file")
	}
}

func TestParseInodeV1OnLinkPromotion(t *testing.T) {
	buf := make([]byte, 512)
	// Magic
	buf[0] = 0x49
	buf[1] = 0x4e
	// Mode = regular file (0x8000)
	buf[2] = 0x80
	buf[3] = 0x00
	// Version = 1
	buf[4] = 1
	// Format = XFS_DINODE_FMT_EXTENTS
	buf[5] = XFS_DINODE_FMT_EXTENTS
	// OnLink = 5 (uint16 big-endian at offset 6)
	buf[6] = 0
	buf[7] = 5
	// NLink = 0 (V1 doesn't use this field natively)

	f := bytes.NewReader(buf)
	sr := io.NewSectionReader(f, 0, int64(len(buf)))

	xfs := &FileSystem{
		r: sr,
		PrimaryAG: AG{
			SuperBlock: SuperBlock{
				Inodesize: 512,
				Inopblock: 8,
				Inopblog:  3,
				Agblklog:  19,
				Agblocks:  524288,
				BlockSize: 4096,
			},
		},
		cache: &mockCache[string, any]{},
	}

	inode, err := xfs.ParseInode(0)
	if err != nil {
		t.Fatalf("ParseInode V1 failed: %v", err)
	}

	if inode.inodeCore.Version != 1 {
		t.Errorf("Version = %d, want 1", inode.inodeCore.Version)
	}
	// V1: OnLink should be promoted to NLink
	if inode.inodeCore.NLink != 5 {
		t.Errorf("NLink = %d, want 5 (promoted from OnLink)", inode.inodeCore.NLink)
	}
}

func TestInobtRec_SparseFields(t *testing.T) {
	tests := []struct {
		name         string
		freecount    uint32
		wantHolemask uint16
		wantInoCount uint8
		wantFreecnt  uint8
	}{
		{
			name:         "all zeros",
			freecount:    0x00000000,
			wantHolemask: 0x0000,
			wantInoCount: 0,
			wantFreecnt:  0,
		},
		{
			name:         "full chunk 64 inodes, 10 free",
			freecount:    0x0000400A, // holemask=0, count=64, freecount=10
			wantHolemask: 0x0000,
			wantInoCount: 64,
			wantFreecnt:  10,
		},
		{
			name:         "sparse chunk with holes",
			freecount:    0xFF002003, // holemask=0xFF00, count=32, freecount=3
			wantHolemask: 0xFF00,
			wantInoCount: 32,
			wantFreecnt:  3,
		},
		{
			name:         "all bits set",
			freecount:    0xFFFFFFFF,
			wantHolemask: 0xFFFF,
			wantInoCount: 255,
			wantFreecnt:  255,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := InobtRec{
				Freecount: tt.freecount,
			}
			if got := rec.Holemask(); got != tt.wantHolemask {
				t.Errorf("Holemask() = 0x%04X, want 0x%04X", got, tt.wantHolemask)
			}
			if got := rec.InoCount(); got != tt.wantInoCount {
				t.Errorf("InoCount() = %d, want %d", got, tt.wantInoCount)
			}
			if got := rec.InoFreecount(); got != tt.wantFreecnt {
				t.Errorf("InoFreecount() = %d, want %d", got, tt.wantFreecnt)
			}
		})
	}
}

func TestParseBmbtKeyPtr_NumrecsExceedsMaxrecs(t *testing.T) {
	// Build a binary buffer with 2 keys (each 8 bytes big-endian).
	// Set maxrecs=1 so numrecs(2) > maxrecs(1) triggers the validation error.
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, BmbtKey(100))
	binary.Write(&buf, binary.BigEndian, BmbtKey(200))

	xfs := &FileSystem{}
	_, _, err := xfs.parseBmbtKeyPtr(&buf, 2, 1)
	if err == nil {
		t.Fatal("expected error when numrecs exceeds maxrecs, got nil")
	}
	if !strings.Contains(err.Error(), "numrecs (2) exceeds maxrecs (1)") {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestParseBmbtKeyPtr_ValidParse(t *testing.T) {
	// Build a valid buffer: maxrecs=2, numrecs=1
	// Layout: keys[2] + ptrs[2], but only first slot of each is populated.
	var buf bytes.Buffer
	// key[0]
	binary.Write(&buf, binary.BigEndian, BmbtKey(42))
	// key[1] (tail padding — skipped by parseBmbtKeyPtr)
	binary.Write(&buf, binary.BigEndian, BmbtKey(0))
	// ptr[0]
	binary.Write(&buf, binary.BigEndian, BmbtPtr(99))
	// ptr[1] (unused)
	binary.Write(&buf, binary.BigEndian, BmbtPtr(0))

	xfs := &FileSystem{}
	keys, ptrs, err := xfs.parseBmbtKeyPtr(&buf, 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 1 || keys[0] != 42 {
		t.Errorf("keys = %v, want [42]", keys)
	}
	if len(ptrs) != 1 || ptrs[0] != 99 {
		t.Errorf("ptrs = %v, want [99]", ptrs)
	}
}

func TestParseBmbtKeyPtr_NumrecsEqualsMaxrecs(t *testing.T) {
	// numrecs == maxrecs: no tail padding to skip.
	// Layout: keys[2] + ptrs[2], all slots populated.
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, BmbtKey(10))
	binary.Write(&buf, binary.BigEndian, BmbtKey(20))
	binary.Write(&buf, binary.BigEndian, BmbtPtr(100))
	binary.Write(&buf, binary.BigEndian, BmbtPtr(200))

	xfs := &FileSystem{}
	keys, ptrs, err := xfs.parseBmbtKeyPtr(&buf, 2, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 2 || keys[0] != 10 || keys[1] != 20 {
		t.Errorf("keys = %v, want [10 20]", keys)
	}
	if len(ptrs) != 2 || ptrs[0] != 100 || ptrs[1] != 200 {
		t.Errorf("ptrs = %v, want [100 200]", ptrs)
	}
}

func TestAttributeOffset(t *testing.T) {
	tests := []struct {
		name     string
		version  uint8
		forkoff  uint8
		expected uint32
	}{
		{
			name:     "V3, forkoff=0",
			version:  3,
			forkoff:  0,
			expected: INODEV3_SIZE, // 176
		},
		{
			name:     "V3, forkoff=24",
			version:  3,
			forkoff:  24,
			expected: 24*8 + INODEV3_SIZE, // 368
		},
		{
			name:     "V2, forkoff=0",
			version:  2,
			forkoff:  0,
			expected: INODEV1V2_SIZE, // 100
		},
		{
			name:     "V1, forkoff=10",
			version:  1,
			forkoff:  10,
			expected: 10*8 + INODEV1V2_SIZE, // 180
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inode := &Inode{}
			inode.inodeCore.Version = tt.version
			inode.inodeCore.Forkoff = tt.forkoff
			got := inode.AttributeOffset()
			if got != tt.expected {
				t.Errorf("AttributeOffset() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestFileInfoMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     uint16
		expected fs.FileMode
	}{
		{
			name:     "regular file 0644",
			mode:     0x8000 | 0o644,
			expected: 0o644,
		},
		{
			name:     "directory 0755",
			mode:     0x4000 | 0o755,
			expected: fs.ModeDir | 0o755,
		},
		{
			name:     "symlink 0777",
			mode:     0xA000 | 0o777,
			expected: fs.ModeSymlink | 0o777,
		},
		{
			name:     "setuid (0o4000)",
			mode:     0x8000 | 0o4755,
			expected: fs.ModeSetuid | 0o755,
		},
		{
			name:     "setgid (0o2000)",
			mode:     0x8000 | 0o2755,
			expected: fs.ModeSetgid | 0o755,
		},
		{
			name:     "sticky (0o1000)",
			mode:     0x4000 | 0o1777,
			expected: fs.ModeDir | fs.ModeSticky | 0o777,
		},
		{
			name:     "setuid + setgid + sticky",
			mode:     0x8000 | 0o7755,
			expected: fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky | 0o755,
		},
		{
			name:     "socket",
			mode:     0xC000 | 0o755,
			expected: fs.ModeSocket | 0o755,
		},
		{
			name:     "block device",
			mode:     0x6000 | 0o660,
			expected: fs.ModeDevice | 0o660,
		},
		{
			name:     "char device",
			mode:     0x2000 | 0o666,
			expected: fs.ModeCharDevice | 0o666,
		},
		{
			name:     "named pipe",
			mode:     0x1000 | 0o644,
			expected: fs.ModeNamedPipe | 0o644,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inode := &Inode{}
			inode.inodeCore.Mode = tt.mode
			fi := FileInfo{name: "test", inode: inode}
			got := fi.Mode()
			if got != tt.expected {
				t.Errorf("Mode() = %v (0x%x), want %v (0x%x)", got, uint32(got), tt.expected, uint32(tt.expected))
			}
		})
	}
}

func TestParseBtreeBlock_V4Magic(t *testing.T) {
	// Build a minimal V4 btree block header (24 bytes, big-endian).
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint32(XFS_BMAP_MAGICa)) // Magic
	binary.Write(&buf, binary.BigEndian, uint16(1))               // Level
	binary.Write(&buf, binary.BigEndian, uint16(3))               // Numrecs
	binary.Write(&buf, binary.BigEndian, int64(-1))               // BbLeftsib
	binary.Write(&buf, binary.BigEndian, int64(-1))               // BbRightsib

	xfs := &FileSystem{}
	block, headerSize, err := xfs.parseBtreeBlock(&buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if block.Magic != XFS_BMAP_MAGICa {
		t.Errorf("Magic = 0x%x, want 0x%x", block.Magic, XFS_BMAP_MAGICa)
	}
	if block.Level != 1 {
		t.Errorf("Level = %d, want 1", block.Level)
	}
	if block.Numrecs != 3 {
		t.Errorf("Numrecs = %d, want 3", block.Numrecs)
	}
	expectedSize := binary.Size(BtreeBlockV4{})
	if headerSize != expectedSize {
		t.Errorf("headerSize = %d, want %d", headerSize, expectedSize)
	}
}

func TestParseBtreeBlock_V5Magic(t *testing.T) {
	// Build a V5 btree block header (72 bytes, big-endian).
	var buf bytes.Buffer
	// V4 base (24 bytes)
	binary.Write(&buf, binary.BigEndian, uint32(XFS_BMAP_CRC_MAGIC))
	binary.Write(&buf, binary.BigEndian, uint16(0))
	binary.Write(&buf, binary.BigEndian, uint16(5))
	binary.Write(&buf, binary.BigEndian, int64(-1))
	binary.Write(&buf, binary.BigEndian, int64(-1))
	// V5 extension (48 bytes)
	binary.Write(&buf, binary.BigEndian, uint64(100))  // BbBlockNo
	binary.Write(&buf, binary.BigEndian, uint64(200))  // BbLsn
	binary.Write(&buf, binary.BigEndian, [16]byte{})   // UUID
	binary.Write(&buf, binary.BigEndian, uint64(300))  // BbOwner
	binary.Write(&buf, binary.BigEndian, uint32(0xAB)) // CRC
	binary.Write(&buf, binary.BigEndian, int32(0))     // Padding

	xfs := &FileSystem{}
	block, headerSize, err := xfs.parseBtreeBlock(&buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if block.Magic != XFS_BMAP_CRC_MAGIC {
		t.Errorf("Magic = 0x%x, want 0x%x", block.Magic, XFS_BMAP_CRC_MAGIC)
	}
	if block.Numrecs != 5 {
		t.Errorf("Numrecs = %d, want 5", block.Numrecs)
	}
	if block.BbBlockNo != 100 {
		t.Errorf("BbBlockNo = %d, want 100", block.BbBlockNo)
	}
	if block.BbOwner != 300 {
		t.Errorf("BbOwner = %d, want 300", block.BbOwner)
	}
	expectedSize := binary.Size(BtreeBlock{})
	if headerSize != expectedSize {
		t.Errorf("headerSize = %d, want %d", headerSize, expectedSize)
	}
}

func TestParseBtreeBlock_UnsupportedMagic(t *testing.T) {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint32(0xDEADBEEF))
	binary.Write(&buf, binary.BigEndian, uint16(0))
	binary.Write(&buf, binary.BigEndian, uint16(0))
	binary.Write(&buf, binary.BigEndian, int64(0))
	binary.Write(&buf, binary.BigEndian, int64(0))

	xfs := &FileSystem{}
	_, _, err := xfs.parseBtreeBlock(&buf)
	if err == nil {
		t.Fatal("expected error for unsupported magic, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported block header magic") {
		t.Errorf("unexpected error: %s", err.Error())
	}
}

func TestSkipDirBlockHeader(t *testing.T) {
	tests := []struct {
		name  string
		magic uint32
		size  int // expected bytes consumed
	}{
		{"V5 data", XFS_DIR3_DATA_MAGIC, binary.Size(Dir3DataHdr{})},
		{"V5 block", XFS_DIR3_BLOCK_MAGIC, binary.Size(Dir3DataHdr{})},
		{"V4 data", XFS_DIR2_DATA_MAGIC, binary.Size(Dir2DataHdr{})},
		{"V4 block", XFS_DIR2_BLOCK_MAGIC, binary.Size(Dir2DataHdr{})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 256) // large enough for any header
			binary.BigEndian.PutUint32(buf[:4], tt.magic)
			reader := bytes.NewReader(buf)

			xfs := &FileSystem{}
			err := xfs.skipDirBlockHeader(reader, tt.magic)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			consumed := 256 - reader.Len()
			if consumed != tt.size {
				t.Errorf("consumed %d bytes, want %d", consumed, tt.size)
			}
		})
	}
}

func TestSkipDirBlockHeader_UnsupportedMagic(t *testing.T) {
	xfs := &FileSystem{}
	err := xfs.skipDirBlockHeader(bytes.NewReader(nil), 0xDEAD)
	if err == nil {
		t.Fatal("expected error for unsupported magic, got nil")
	}
}

func TestParseInode(t *testing.T) {
	testCases := []struct {
		filesystem  string
		name        string
		inodeNumber uint64

		expectedCRC uint32
		expectedErr error
	}{
		{
			filesystem:  "testdata/image.xfs",
			name:        "rootino",
			inodeNumber: 11072,
		},
		{
			filesystem:  "testdata/image.xfs",
			name:        "fmt_local_directory",
			inodeNumber: 11075,
		},
		{
			filesystem:  "testdata/image.xfs",
			name:        "fmt_extents_block_directories",
			inodeNumber: 11077,
		},
		{
			filesystem:  "testdata/image.xfs",
			name:        "fmt_leaf_directories",
			inodeNumber: 11086,
		},
		{
			filesystem:  "testdata/image.xfs",
			name:        "fmt_node_directories",
			inodeNumber: 11287,
		},
		{
			filesystem:  "testdata/image.xfs",
			name:        "fmt_extents_file_1024",
			inodeNumber: 20440,
		},
		{
			filesystem:  "testdata/image.xfs",
			name:        "fmt_extents_file_1024",
			inodeNumber: 20441,
		},
		{
			filesystem:  "testdata/image.xfs",
			name:        "fmt_extents_file_16384",
			inodeNumber: 20442,
		},
		{
			filesystem:  "testdata/image.xfs",
			name:        "no_exist_inode",
			inodeNumber: 9999,
			expectedErr: fmt.Errorf("invalid magic byte error"),
		},
		{
			filesystem:  "testdata/image.xfs",
			name:        "no_exist_inode invalid inode range",
			inodeNumber: 9999999,
			expectedErr: fmt.Errorf("EOF"),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open(tt.filesystem)
			if err != nil {
				t.Fatal(err)
			}
			info, err := f.Stat()
			if err != nil {
				t.Fatal(err)
			}

			fileSystem, err := NewFS(*io.NewSectionReader(f, 0, info.Size()), nil)
			if err != nil {
				t.Fatal(err)
			}

			inode, err := fileSystem.ParseInode(tt.inodeNumber)
			if tt.expectedErr != nil {
				if !strings.Contains(err.Error(), tt.expectedErr.Error()) {
					t.Fatalf("name: %s, expected: %s, actual %s", tt.name, tt.expectedErr.Error(), err.Error())
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}

			if tt.inodeNumber != inode.inodeCore.Ino {
				t.Fatalf("name: %s, expected %d, actual %d", tt.name, tt.inodeNumber, inode.inodeCore.Ino)
			}
		})
	}
}

func TestBmbtRecUnpackState(t *testing.T) {
	tests := []struct {
		name          string
		l0            uint64
		l1            uint64
		expectedState uint8
	}{
		{
			name:          "normal extent (state=0)",
			l0:            0x0000000000000000,
			l1:            0x0000000000000001,
			expectedState: 0,
		},
		{
			name:          "unwritten extent (state=1)",
			l0:            0x8000000000000000, // bit 63 set
			l1:            0x0000000000000001,
			expectedState: 1,
		},
		{
			name:          "normal extent with data",
			l0:            0x0000000100000002, // startoff=0x800001, startblock high bits=2
			l1:            0x000000000000000A, // startblock low + blockcount=10
			expectedState: 0,
		},
		{
			name:          "unwritten extent with data",
			l0:            0x8000000100000002, // bit 63 set + same data
			l1:            0x000000000000000A,
			expectedState: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := BmbtRec{L0: tt.l0, L1: tt.l1}
			irec := rec.Unpack()
			if irec.State != tt.expectedState {
				t.Errorf("expected State=%d, got State=%d", tt.expectedState, irec.State)
			}
		})
	}
}
