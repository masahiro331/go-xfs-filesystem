package xfs_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/masahiro331/go-xfs-filesystem/xfs"
)

func TestFileSystemOpen(t *testing.T) {
	f, err := os.Open("./testdata/image.xfs")
	if err != nil {
		t.Fatal(err)
	}

	fileSystem, err := xfs.NewFileSystem(f)
	if err != nil {
		t.Fatal(err)
	}

	testFileCace := []struct {
		name         string
		expectedSize int
		mode         os.FileMode
	}{
		{
			name:         "fmt_extents_file_1024",
			expectedSize: 1024,
			mode:         33188,
		},
		{
			name:         "fmt_extents_file_4096",
			expectedSize: 4096,
			mode:         33188,
		},
		{
			name:         "fmt_extents_file_16384",
			expectedSize: 16384,
			mode:         33188,
		},
	}

	for _, tt := range testFileCace {
		t.Run(fmt.Sprintf("test %s read", tt.name), func(t *testing.T) {
			testFile, err := fileSystem.Open(tt.name)
			if err != nil {
				t.Fatal(err)
			}
			stat, err := testFile.Stat()
			if err != nil {
				t.Fatal(err)
			}

			if stat.Size() != int64(tt.expectedSize) {
				t.Errorf("expected %d, actual %d", tt.expectedSize, stat.Size())
			}
			if stat.Name() != tt.name {
				t.Errorf("expected %s, actual %s", tt.name, stat.Name())
			}
			if stat.Mode() != tt.mode {
				t.Errorf("expected %s, actual %s", tt.mode, stat.Mode())
			}
		})
	}
}

func TestFileSystemReadDir(t *testing.T) {
	f, err := os.Open("./testdata/image.xfs")
	if err != nil {
		t.Fatal(err)
	}

	fileSystem, err := xfs.NewFileSystem(f)
	if err != nil {
		t.Fatal(err)
	}

	testDirectoryCases := []struct {
		name       string
		entriesLen int
	}{
		{
			name:       "fmt_extents_block_directories",
			entriesLen: 8,
		},
		{
			name:       "fmt_leaf_directories",
			entriesLen: 200,
		},
		{
			name:       "fmt_local_directory",
			entriesLen: 1,
		},
		{
			name:       "fmt_node_directories",
			entriesLen: 1024,
		},
	}

	for _, tt := range testDirectoryCases {
		t.Run(fmt.Sprintf("test %s read", tt.name), func(t *testing.T) {
			dirEntries, err := fileSystem.ReadDir(tt.name)
			if err != nil {
				t.Fatal(err)
			}
			if len(dirEntries) != tt.entriesLen {
				t.Errorf("expected %d, actual %d", len(dirEntries), tt.entriesLen)
			}
		})
	}
}
