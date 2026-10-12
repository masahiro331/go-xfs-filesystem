package xfs

import (
	"bytes"
	"errors"
	"io"
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
