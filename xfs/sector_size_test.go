package xfs_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"testing"
	"testing/iotest"

	"github.com/masahiro331/go-xfs-filesystem/xfs"
)

// newLargeSectorImage lays out the AG headers of testdata/image.xfs as if it
// had been made with sectSize, since no test image has sectors over 4 KiB.
func newLargeSectorImage(t *testing.T, sectSize int) []byte {
	t.Helper()
	orig, err := os.ReadFile("testdata/image.xfs")
	if err != nil {
		t.Fatal(err)
	}
	const origSectSize = 512

	var sb xfs.SuperBlock
	if err := binary.Read(bytes.NewReader(orig), binary.BigEndian, &sb); err != nil {
		t.Fatal(err)
	}
	if sb.Sectsize != origSectSize {
		t.Fatalf("unexpected sector size of testdata/image.xfs: %d", sb.Sectsize)
	}
	sb.Sectsize = uint16(sectSize)
	sb.Agcount = 1
	// XFS requires the sector size to be no larger than the block size.
	if uint32(sectSize) > sb.BlockSize {
		sb.BlockSize = uint32(sectSize)
	}

	img := make([]byte, 4*sectSize)
	var w bytes.Buffer
	if err := binary.Write(&w, binary.BigEndian, &sb); err != nil {
		t.Fatal(err)
	}
	copy(img, w.Bytes())
	for i := 1; i < 4; i++ {
		copy(img[i*sectSize:], orig[i*origSectSize:(i+1)*origSectSize])
	}
	return img
}

func TestNewFSLargeSectorSize(t *testing.T) {
	for _, sectSize := range []int{4096, 8192, 32768} {
		img := newLargeSectorImage(t, sectSize)
		fileSystem, err := xfs.NewFS(*io.NewSectionReader(bytes.NewReader(img), 0, int64(len(img))), nil)
		if err != nil {
			t.Fatalf("sector size %d: %v", sectSize, err)
		}
		if got := fileSystem.PrimaryAG.SuperBlock.Sectsize; int(got) != sectSize {
			t.Errorf("sector size %d: parsed Sectsize = %d", sectSize, got)
		}
	}
}

func TestCheckLargeSectorSizeShortReads(t *testing.T) {
	img := newLargeSectorImage(t, 8192)
	if !xfs.Check(iotest.OneByteReader(bytes.NewReader(img))) {
		t.Fatal("Check failed on a reader that returns one byte per Read")
	}
}

func TestCheckInvalidSectorSize(t *testing.T) {
	orig, err := os.ReadFile("testdata/image.xfs")
	if err != nil {
		t.Fatal(err)
	}
	for _, sectSize := range []uint16{0, 256, 1536, 65024} {
		var sb xfs.SuperBlock
		if err := binary.Read(bytes.NewReader(orig), binary.BigEndian, &sb); err != nil {
			t.Fatal(err)
		}
		sb.Sectsize = sectSize
		// Enough data that a read of the bogus remainder would succeed.
		img := make([]byte, 128*1024)
		var w bytes.Buffer
		if err := binary.Write(&w, binary.BigEndian, &sb); err != nil {
			t.Fatal(err)
		}
		copy(img, w.Bytes())
		if xfs.Check(bytes.NewReader(img)) {
			t.Errorf("Check accepted Sectsize=%d", sectSize)
		}
	}
}
