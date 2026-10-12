package xfs_test

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/masahiro331/go-xfs-filesystem/xfs"
	"golang.org/x/xerrors"
)

func TestFileSystemCheckFileExtents(t *testing.T) {
	testFileCace := []struct {
		filesystem   string
		name         string
		expectedSize int
		mode         os.FileMode
		expectedErr  error
	}{
		{
			filesystem:   "testdata/image.xfs",
			name:         "fmt_extents_file_1024",
			expectedSize: 1024,
			mode:         0o644,
		},
		{
			filesystem:   "testdata/image.xfs",
			name:         "fmt_extents_file_4096",
			expectedSize: 4096,
			mode:         0o644,
		},
		{
			filesystem:   "testdata/image.xfs",
			name:         "fmt_extents_file_16384",
			expectedSize: 16384,
			mode:         0o644,
		},
		{
			filesystem:  "testdata/image.xfs",
			name:        "no_exist_file",
			expectedErr: fmt.Errorf("file does not exist"),
		},
		{
			filesystem:  "testdata/image.xfs",
			name:        "no_exist_directory/no_exist_file",
			expectedErr: fmt.Errorf("file does not exist"),
		},
	}

	for _, tt := range testFileCace {
		t.Run(fmt.Sprintf("test %s read", tt.name), func(t *testing.T) {
			f, err := os.Open(tt.filesystem)
			if err != nil {
				t.Fatal(err)
			}
			info, err := f.Stat()
			if err != nil {
				t.Fatal(err)
			}

			fileSystem, err := xfs.NewFS(*io.NewSectionReader(f, 0, info.Size()), nil)
			if err != nil {
				t.Fatal(err)
			}

			testFile, err := fileSystem.Open(tt.name)
			if err != nil {
				if tt.expectedErr == nil {
					t.Fatal(err)
				}
				if !strings.Contains(err.Error(), tt.expectedErr.Error()) {
					t.Fatalf("name: %s, expected: %s, actual %s", tt.name, tt.expectedErr.Error(), err.Error())
				}
				return
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

func TestFileSystemCheckWalkDir(t *testing.T) {
	testExecutableFileCases := []struct {
		filesystem    string
		name          string
		parentPath    string
		expectedFiles []string
	}{
		{
			filesystem: "testdata/image.xfs",
			name:       "search executable file",
			parentPath: "parent",
			expectedFiles: []string{
				"parent/child/child/child/child/child/executable",
				"parent/child/child/child/child/executable",
			},
		},
		{
			filesystem: "testdata/image.xfs",
			name:       "search executable file with root node",
			parentPath: "/",
			expectedFiles: []string{
				"/parent/child/child/child/child/child/executable",
				"/parent/child/child/child/child/executable",
			},
		},
		{
			filesystem: "testdata/image.xfs",
			name:       "search executable file with deep path",
			parentPath: "/parent/child/child/child/child/child/",
			expectedFiles: []string{
				"/parent/child/child/child/child/child/executable",
			},
		},
	}

	for _, tt := range testExecutableFileCases {
		t.Run(fmt.Sprintf(tt.name), func(t *testing.T) {
			f, err := os.Open(tt.filesystem)
			if err != nil {
				t.Fatal(err)
			}
			info, err := f.Stat()
			if err != nil {
				t.Fatal(err)
			}

			fileSystem, err := xfs.NewFS(*io.NewSectionReader(f, 0, info.Size()), nil)
			if err != nil {
				t.Fatal(err)
			}

			filePaths := []string{}
			err = fs.WalkDir(fileSystem, tt.parentPath, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return xerrors.Errorf("file walk error: %w", err)
				}
				if d.IsDir() {
					return nil
				}

				fileInfo, err := d.Info()
				if err != nil {
					t.Fatalf("failed to get file info: %v", err)
				}
				if fileInfo.Mode().Perm()&0111 == 0 {
					return nil
				}
				filePaths = append(filePaths, path)
				return nil
			})
			if err != nil {
				t.Fatalf("failed to walk dir: %v", err)
			}

			sort.Slice(filePaths, func(i, j int) bool { return filePaths[i] < filePaths[j] })
			sort.Slice(tt.expectedFiles, func(i, j int) bool { return tt.expectedFiles[i] < tt.expectedFiles[j] })
			if len(filePaths) != len(tt.expectedFiles) {
				t.Fatalf("length error: actual %d, expected %d", len(filePaths), len(tt.expectedFiles))
			}

			for i := 0; i < len(filePaths); i++ {
				if filePaths[i] != tt.expectedFiles[i] {
					t.Fatalf("%d: actual %s, expected: %s", i, filePaths[i], tt.expectedFiles[i])
				}
			}
		})
	}
}

func TestFileSystemCheckReadDir(t *testing.T) {

	testDirectoryCases := []struct {
		filesystem string
		name       string
		entriesLen int
	}{
		{
			filesystem: "testdata/image.xfs",
			name:       "fmt_extents_block_directories",
			entriesLen: 8,
		},
		{
			filesystem: "testdata/image.xfs",
			name:       "fmt_leaf_directories",
			entriesLen: 200,
		},
		{
			filesystem: "testdata/image.xfs",
			name:       "fmt_local_directory",
			entriesLen: 1,
		},
		{
			filesystem: "testdata/image.xfs",
			name:       "fmt_node_directories",
			entriesLen: 1024,
		},
	}

	for _, tt := range testDirectoryCases {
		t.Run(fmt.Sprintf("test %s read", tt.name), func(t *testing.T) {
			f, err := os.Open(tt.filesystem)
			if err != nil {
				t.Fatal(err)
			}
			info, err := f.Stat()
			if err != nil {
				t.Fatal(err)
			}

			fileSystem, err := xfs.NewFS(*io.NewSectionReader(f, 0, info.Size()), nil)
			if err != nil {
				t.Fatal(err)
			}
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

func TestFileSystemCheckReadFile(t *testing.T) {
	testDirectoryCases := []struct {
		filesystem   string
		name         string
		expectedFile string
	}{
		{
			filesystem:   "testdata/image.xfs",
			name:         "etc/os-release",
			expectedFile: "testdata/os-release",
		},
	}

	for _, tt := range testDirectoryCases {
		t.Run(fmt.Sprintf("test %s read", tt.name), func(t *testing.T) {
			f, err := os.Open(tt.filesystem)
			if err != nil {
				t.Fatal(err)
			}
			info, err := f.Stat()
			if err != nil {
				t.Fatal(err)
			}

			fileSystem, err := xfs.NewFS(*io.NewSectionReader(f, 0, info.Size()), nil)
			if err != nil {
				t.Fatal(err)
			}
			file, err := fileSystem.Open(tt.name)
			if err != nil {
				t.Fatalf("failed to open file: %v", err)
			}
			expectedFile, err := os.Open(tt.expectedFile)
			if err != nil {
				t.Fatal(err)
			}

			buf, err := io.ReadAll(file)
			if err != nil {
				t.Fatal(err)
			}
			expectedBuf, err := io.ReadAll(expectedFile)
			if err != nil {
				t.Fatal(err)
			}
			if string(expectedBuf) != string(buf) {
				t.Fatalf("expected %s, actual %s", expectedBuf, buf)
			}
		})
	}
}

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
