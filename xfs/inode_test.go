package xfs

import (
	"encoding/binary"
	"fmt"
	"io"
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
			expected:  256 - INODE_CORE_BASE_SIZE, // 156
		},
		{
			name:      "V1, forkoff=0, inodesize=256",
			inodesize: 256,
			forkoff:   0,
			version:   1,
			expected:  256 - INODE_CORE_BASE_SIZE, // 156
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

func TestInodeCoreBaseSize(t *testing.T) {
	size := binary.Size(InodeCoreBase{})
	if size != INODE_CORE_BASE_SIZE {
		t.Errorf("InodeCoreBase binary size = %d, want %d", size, INODE_CORE_BASE_SIZE)
	}
}

func TestInodeCoreV3ExtSize(t *testing.T) {
	size := binary.Size(InodeCoreV3Ext{})
	expected := INODEV3_SIZE - INODE_CORE_BASE_SIZE
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
