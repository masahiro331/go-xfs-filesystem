package xfs

import (
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
		expected  int
	}{
		{
			name:      "forkoff=0, inodesize=256",
			inodesize: 256,
			forkoff:   0,
			expected:  256 - 176, // 80
		},
		{
			name:      "forkoff=0, inodesize=512",
			inodesize: 512,
			forkoff:   0,
			expected:  512 - 176, // 336
		},
		{
			name:      "forkoff>0, forkoff=24",
			inodesize: 256,
			forkoff:   24,
			expected:  24 << 3, // 192
		},
		{
			name:      "forkoff>0, forkoff=10",
			inodesize: 512,
			forkoff:   10,
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
			got := fs.DataForkSize(tt.forkoff)
			if got != tt.expected {
				t.Errorf("DataForkSize(%d) = %d, want %d", tt.forkoff, got, tt.expected)
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
