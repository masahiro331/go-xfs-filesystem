package xfs_test

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/masahiro331/go-xfs-filesystem/xfs"
)

// TestFileSystemConcurrentRead opens and reads many files concurrently from a
// single FileSystem and checks that every result matches a sequential read.
// Run with -race to detect shared reader state.
func TestFileSystemConcurrentRead(t *testing.T) {
	for _, image := range []string{"testdata/image.xfs", "testdata/image40.xfs"} {
		image := image
		t.Run(image, func(t *testing.T) {
			f, err := os.Open(image)
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

			var names []string
			err = fs.WalkDir(fileSystem, "/", func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.Type().IsRegular() {
					names = append(names, strings.TrimPrefix(path, "/"))
				}
				return nil
			})
			if err != nil {
				t.Fatalf("failed to walk dir: %v", err)
			}
			if len(names) < 2 {
				t.Fatalf("need at least 2 regular files, found %d", len(names))
			}

			// Every Open walks from the root inode, so sample files evenly
			// across the tree to keep the test fast under -race.
			const maxFiles = 16
			sort.Strings(names)
			step := (len(names) + maxFiles - 1) / maxFiles
			expected := map[string][]byte{}
			for i := 0; i < len(names); i += step {
				b, err := readFile(fileSystem, names[i])
				if err != nil {
					t.Fatal(err)
				}
				expected[names[i]] = b
			}

			const rounds = 4
			var wg sync.WaitGroup
			errCh := make(chan error, rounds*len(expected))
			for i := 0; i < rounds; i++ {
				for path, want := range expected {
					path, want := path, want
					wg.Add(1)
					go func() {
						defer wg.Done()
						got, err := readFile(fileSystem, path)
						if err != nil {
							errCh <- err
							return
						}
						if !bytes.Equal(got, want) {
							errCh <- fmt.Errorf("%s: content mismatch", path)
						}
					}()
				}
			}
			wg.Wait()
			close(errCh)
			for err := range errCh {
				t.Error(err)
			}
		})
	}
}

// readFile avoids fs.ReadFile, because FileSystem.ReadFile is not implemented.
func readFile(fsys fs.FS, name string) ([]byte, error) {
	f, err := fsys.Open(name)
	if err != nil {
		return nil, fmt.Errorf("%s: open: %w", name, err)
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("%s: read: %w", name, err)
	}
	return b, nil
}
