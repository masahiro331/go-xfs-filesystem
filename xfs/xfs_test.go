package xfs_test

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"sort"
	"testing"

	"github.com/masahiro331/go-xfs-filesystem/xfs"
	"golang.org/x/xerrors"
)

func TestFileSystemCheckFileExtents(t *testing.T) {
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

func TestFileSystemCheckWalkDir(t *testing.T) {
	f, err := os.Open("./testdata/image.xfs")
	if err != nil {
		t.Fatal(err)
	}
	fileSystem, err := xfs.NewFileSystem(f)
	if err != nil {
		t.Fatal(err)
	}

	testExecutableFileCases := []struct {
		name          string
		parentPath    string
		expectedFiles []string
	}{
		{
			name:       "search executable file",
			parentPath: "parent",
			expectedFiles: []string{
				"parent/child/child/child/child/child/executable",
				"parent/child/child/child/child/executable",
			},
		},
	}

	for _, tt := range testExecutableFileCases {
		t.Run(fmt.Sprintf(tt.name), func(t *testing.T) {
			filePaths := []string{}
			err := fs.WalkDir(fileSystem, tt.parentPath, func(path string, d fs.DirEntry, err error) error {
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

func TestFileSystemCheckReadFile(t *testing.T) {
	f, err := os.Open("./testdata/image.xfs")
	if err != nil {
		t.Fatal(err)
	}

	fileSystem, err := xfs.NewFileSystem(f)
	if err != nil {
		t.Fatal(err)
	}
	testDirectoryCases := []struct {
		name         string
		expectedFile string
	}{
		{
			name:         "etc/os-release",
			expectedFile: "testdata/os-release",
		},
	}

	for _, tt := range testDirectoryCases {
		t.Run(fmt.Sprintf("test %s read", tt.name), func(t *testing.T) {
			file, err := fileSystem.Open(tt.name)
			if err != nil {
				t.Fatalf("failed to open file: %v", err)
			}
			expectedFile, err := os.Open(tt.expectedFile)
			if err != nil {
				t.Fatal(err)
			}

			buf, err := ioutil.ReadAll(file)
			if err != nil {
				t.Fatal(err)
			}
			expectedBuf, err := ioutil.ReadAll(expectedFile)
			if err != nil {
				t.Fatal(err)
			}
			if string(expectedBuf) != string(buf) {
				t.Fatalf("expected %s, actual %s", expectedBuf, buf)
			}
		})
	}
}
