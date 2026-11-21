package xfs

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"io/fs"
	"os"
	"testing"
)

// eofAtEndReaderAt returns io.EOF together with a full read that ends exactly
// at the end of the data, which the io.ReaderAt contract allows.
type eofAtEndReaderAt struct {
	r *bytes.Reader
}

func (e eofAtEndReaderAt) ReadAt(p []byte, off int64) (int, error) {
	n, err := e.r.ReadAt(p, off)
	if err == nil && off+int64(n) == e.r.Size() {
		err = io.EOF
	}
	return n, err
}

// newReadTestFileSystem builds a FileSystem over data with 512-byte blocks
// and 256-byte inodes, so inode n lives at byte offset n*256 in AG 0.
func newReadTestFileSystem(r io.ReaderAt) *FileSystem {
	return &FileSystem{
		r: r,
		PrimaryAG: AG{
			SuperBlock: SuperBlock{
				BlockSize: 512,
				Inodesize: 256,
				Inopblock: 2,
				Inopblog:  1,
				Agblocks:  16,
				Agblklog:  4,
			},
		},
	}
}

func newReadTestData(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 251)
	}
	return data
}

func TestReadBlockAt(t *testing.T) {
	data := newReadTestData(4 * 512)
	readers := map[string]io.ReaderAt{
		"bytes.Reader":     bytes.NewReader(data),
		"SectionReader":    io.NewSectionReader(bytes.NewReader(data), 0, int64(len(data))),
		"eofAtEndReaderAt": eofAtEndReaderAt{r: bytes.NewReader(data)},
	}
	for name, r := range readers {
		t.Run(name, func(t *testing.T) {
			xfs := newReadTestFileSystem(r)
			for n := int64(0); n < 4; n++ {
				got, err := xfs.readBlockAt(n)
				if err != nil {
					t.Fatalf("readBlockAt(%d): %v", n, err)
				}
				if want := data[n*512 : (n+1)*512]; !bytes.Equal(got, want) {
					t.Fatalf("readBlockAt(%d): content mismatch", n)
				}
			}
			if _, err := xfs.readBlockAt(4); err == nil {
				t.Fatal("readBlockAt(4): expected error beyond end of data")
			}
		})
	}
}

func TestReadBlockAtShortRead(t *testing.T) {
	// The last block is only partially present.
	data := newReadTestData(4*512 + 100)
	xfs := newReadTestFileSystem(bytes.NewReader(data))
	if _, err := xfs.readBlockAt(4); err == nil {
		t.Fatal("readBlockAt(4): expected error on short read")
	}
}

func TestReadInode(t *testing.T) {
	data := newReadTestData(8 * 256)
	readers := map[string]io.ReaderAt{
		"bytes.Reader":     bytes.NewReader(data),
		"eofAtEndReaderAt": eofAtEndReaderAt{r: bytes.NewReader(data)},
	}
	for name, r := range readers {
		t.Run(name, func(t *testing.T) {
			xfs := newReadTestFileSystem(r)
			for n := uint64(0); n < 8; n++ {
				got, err := xfs.readInode(n)
				if err != nil {
					t.Fatalf("readInode(%d): %v", n, err)
				}
				if want := data[n*256 : (n+1)*256]; !bytes.Equal(got, want) {
					t.Fatalf("readInode(%d): content mismatch", n)
				}
			}
			if _, err := xfs.readInode(8); err == nil {
				t.Fatal("readInode(8): expected error beyond end of data")
			}
		})
	}
}

func TestReadBlockAtInvalidBlockSize(t *testing.T) {
	data := newReadTestData(65536)
	for _, blockSize := range []uint32{0, 256, 1000, 1536, 131072, 0x7FFFFE00} {
		xfs := newReadTestFileSystem(bytes.NewReader(data))
		xfs.PrimaryAG.SuperBlock.BlockSize = blockSize
		if _, err := xfs.readBlockAt(0); err == nil {
			t.Errorf("readBlockAt with BlockSize=%d: expected error", blockSize)
		}
	}
	for blockSize := uint32(512); blockSize <= 65536; blockSize *= 2 {
		xfs := newReadTestFileSystem(bytes.NewReader(data))
		xfs.PrimaryAG.SuperBlock.BlockSize = blockSize
		if _, err := xfs.readBlockAt(0); err != nil {
			t.Errorf("readBlockAt with BlockSize=%d: %v", blockSize, err)
		}
	}
}

func TestReadInodeInvalidInodeSize(t *testing.T) {
	data := newReadTestData(8 * 4096)
	for _, inodeSize := range []uint16{0, 100, 128, 384, 4096} {
		xfs := newReadTestFileSystem(bytes.NewReader(data))
		xfs.PrimaryAG.SuperBlock.Inodesize = inodeSize
		if _, err := xfs.readInode(1); err == nil {
			t.Errorf("readInode with Inodesize=%d: expected error", inodeSize)
		}
	}
	for _, inodeSize := range []uint16{256, 512, 1024, 2048} {
		xfs := newReadTestFileSystem(bytes.NewReader(data))
		xfs.PrimaryAG.SuperBlock.Inodesize = inodeSize
		if _, err := xfs.readInode(1); err != nil {
			t.Errorf("readInode with Inodesize=%d: %v", inodeSize, err)
		}
	}
}

func TestReadAtShortReadIsUnexpectedEOF(t *testing.T) {
	// The last block is only partially present.
	data := newReadTestData(4*512 + 100)
	readers := map[string]io.ReaderAt{
		"bytes.Reader":  bytes.NewReader(data),
		"SectionReader": io.NewSectionReader(bytes.NewReader(data), 0, int64(len(data))),
	}
	for name, r := range readers {
		t.Run(name, func(t *testing.T) {
			xfs := newReadTestFileSystem(r)
			for _, n := range []int64{4, 5} { // partial block, block past the end
				_, err := xfs.readBlockAt(n)
				if !errors.Is(err, io.ErrUnexpectedEOF) {
					t.Errorf("readBlockAt(%d): got %v, want io.ErrUnexpectedEOF", n, err)
				}
				if errors.Is(err, io.EOF) {
					t.Errorf("readBlockAt(%d): error must not wrap io.EOF: %v", n, err)
				}
			}
		})
	}
}

// errOnFullReadReaderAt fills p completely but still reports err.
type errOnFullReadReaderAt struct {
	r   *bytes.Reader
	err error
}

func (e errOnFullReadReaderAt) ReadAt(p []byte, off int64) (int, error) {
	n, _ := e.r.ReadAt(p, off)
	return n, e.err
}

func TestReadAtFullReadWithError(t *testing.T) {
	data := newReadTestData(4 * 512)
	errDevice := errors.New("device error")
	xfs := newReadTestFileSystem(errOnFullReadReaderAt{r: bytes.NewReader(data), err: errDevice})
	if _, err := xfs.readBlockAt(1); !errors.Is(err, errDevice) {
		t.Errorf("readBlockAt: got %v, want %v", err, errDevice)
	}
	if _, err := xfs.readInode(1); !errors.Is(err, errDevice) {
		t.Errorf("readInode: got %v, want %v", err, errDevice)
	}
}

// A snapshot of a mounted filesystem can have a directory entry whose inode
// was freed by a change that is only in the log. Such an entry must not show
// up as a file.
func TestDirectoryEntryToFreeInode(t *testing.T) {
	const name = "fmt_extents_file_1024"

	data, err := os.ReadFile("testdata/image.xfs")
	if err != nil {
		t.Fatal(err)
	}
	orig, err := NewFS(*io.NewSectionReader(bytes.NewReader(data), 0, int64(len(data))), nil)
	if err != nil {
		t.Fatal(err)
	}
	fileInfos, err := orig.listFileInfo(orig.PrimaryAG.SuperBlock.Rootino)
	if err != nil {
		t.Fatal(err)
	}
	var ino uint64
	for _, fi := range fileInfos {
		if fi.Name() == name {
			ino = fi.inode.inodeCore.Ino
		}
	}
	if ino == 0 {
		t.Fatalf("%s not found in the root directory", name)
	}
	entries, err := orig.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	// di_mode is at byte 2 of the inode; xfs_ifree sets it to 0.
	off := orig.PrimaryAG.SuperBlock.InodeAbsOffset(ino)
	binary.BigEndian.PutUint16(data[off+2:], 0)
	fileSystem, err := NewFS(*io.NewSectionReader(bytes.NewReader(data), 0, int64(len(data))), nil)
	if err != nil {
		t.Fatal(err)
	}

	got, err := fileSystem.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(entries)-1 {
		t.Errorf("ReadDir returned %d entries, want %d", len(got), len(entries)-1)
	}
	for _, e := range got {
		if e.Name() == name {
			t.Errorf("ReadDir returned %s, whose inode is free", name)
		}
	}
	if _, err := fileSystem.Open(name); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Open(%s) error = %v, want fs.ErrNotExist", name, err)
	}
	if _, err := fileSystem.Stat(name); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat(%s) error = %v, want fs.ErrNotExist", name, err)
	}
}
