package xfs_test

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/masahiro331/go-xfs-filesystem/xfs"
)

// testdata/dirblk8k.xfs has 8 KiB directory blocks on 4 KiB filesystem blocks
// (Dirblklog=1), which the other test images do not, so each directory block
// spans two filesystem blocks.
func TestFileSystemMultiBlockDirectoryBlocks(t *testing.T) {
	f, err := os.Open("testdata/dirblk8k.xfs")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	fileSystem, err := xfs.NewFS(*io.NewSectionReader(f, 0, info.Size()), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := fileSystem.PrimaryAG.SuperBlock.Dirblklog; got != 1 {
		t.Fatalf("Dirblklog = %d, want 1", got)
	}

	// block_dir, leaf_dir and node_dir are in block, leaf and node format.
	for _, tt := range []struct {
		dir     string
		entries int
	}{
		{dir: "block_dir", entries: 40},
		{dir: "leaf_dir", entries: 400},
		{dir: "node_dir", entries: 4000},
	} {
		t.Run(tt.dir, func(t *testing.T) {
			dirEntries, err := fileSystem.ReadDir(tt.dir)
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]bool{}
			for i := 0; i < tt.entries; i++ {
				want[fmt.Sprintf("entry_with_a_reasonably_long_name_%05d", i)] = true
			}
			for _, e := range dirEntries {
				if !want[e.Name()] {
					t.Errorf("unexpected or duplicate entry %q", e.Name())
				}
				delete(want, e.Name())
			}
			if len(want) != 0 {
				t.Errorf("%d entries missing", len(want))
			}
		})
	}

	file, err := fileSystem.Open("multi_block_file")
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	// multi_block_file holds the output of `seq -w 1 4000`.
	const wantSum = "75af5fcf1fdb4e79a5a0ec92c697ee90d1d3b87b6f2c50c1dbf668c089743894"
	if got := fmt.Sprintf("%x", sha256.Sum256(b)); len(b) != 20000 || got != wantSum {
		t.Errorf("multi_block_file: %d bytes, sha256 %s; want 20000 bytes, sha256 %s", len(b), got, wantSum)
	}
}
