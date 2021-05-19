package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/masahiro331/go-xfs-filesystem/xfs"
)

var BlockSize = 4096

type AGFL struct {
	Magicnum uint32
	Seqno    uint32
	UUID     [16]byte
	Lsn      uint64
	CRC      uint32
	Bno      [118]uint32
}

type BtreeShortBlock struct {
	Magicnum uint32
	Level    uint16
	Numrecs  uint16
	Leftsib  uint32
	Rightsib uint32
	Blkno    uint64
	Lsn      uint64
	UUID     [16]byte
	Owner    uint32
	CRC      uint32
}

func main() {

	args := os.Args
	if len(args) < 2 {
		log.Fatalf("invalid arguments error")
	}

	filename := args[1]

	f, err := os.Open(filename)
	if err != nil {
		log.Fatalf("failed to open error %+v", err)
	}

	firstBlock := readBlock(f)
	fr := bytes.NewReader(firstBlock)
	rf := func(r io.Reader) io.Reader {
		return bytes.NewReader(readSector(r))
	}

	var sb xfs.SuperBlock
	if err := binary.Read(rf(fr), binary.BigEndian, &sb); err != nil {
		log.Fatalf("binary read error: %+v", err)
	}
	fmt.Println("=========== Super block =============")
	fmt.Printf("%+v\n", sb)

	var agf xfs.AGF
	if err := binary.Read(rf(fr), binary.BigEndian, &agf); err != nil {
		log.Fatalf("binary read error: %+v", err)
	}
	fmt.Println("=========== AGF =============")
	fmt.Printf("%+v\n", agf)

	var agi xfs.AGI
	if err := binary.Read(rf(fr), binary.BigEndian, &agi); err != nil {
		log.Fatalf("binary read error: %+v", err)
	}
	fmt.Println("=========== AGI =============")
	fmt.Printf("%+v\n", agi)

	var agfl AGFL
	if err := binary.Read(rf(fr), binary.BigEndian, &agfl); err != nil {
		log.Fatalf("binary read error: %+v", err)
	}
	fmt.Println("=========== AGFL =============")
	fmt.Printf("%+v\n", agfl)

	// parse AB3B
	sblockReader := bytes.NewReader(readBlock(f))
	var free1 BtreeShortBlock
	if err := binary.Read(sblockReader, binary.BigEndian, &free1); err != nil {
		log.Fatalf("binary read error: %+v", err)
	}
	fmt.Printf("%+v\n", free1)

	// parse AB3C
	sblockReader = bytes.NewReader(readBlock(f))
	var free2 BtreeShortBlock
	if err := binary.Read(sblockReader, binary.BigEndian, &free2); err != nil {
		log.Fatalf("binary read error: %+v", err)
	}
	fmt.Printf("%+v\n", free2)

	// parse IAB3
	sblockReader = bytes.NewReader(readBlock(f))
	var inodeBlock BtreeShortBlock
	if err := binary.Read(sblockReader, binary.BigEndian, &inodeBlock); err != nil {
		log.Fatalf("binary read error: %+v", err)
	}
	fmt.Printf("%+v\n", inodeBlock)

	var inodes []xfs.InobtRec
	for i := 0; i < int(inodeBlock.Numrecs); i++ {
		var inode xfs.InobtRec
		if err := binary.Read(sblockReader, binary.BigEndian, &inode); err != nil {
			log.Fatalf("binary read error: %+v", err)
		}
		inodes = append(inodes, inode)
	}
	// FIB3
	readBlock(f)

	// read Free block
	readBlock(f)
	readBlock(f)
	readBlock(f)
	readBlock(f)

	// 謎のやつ
	sblockReader = bytes.NewReader(readBlock(f))
	var hoge BtreeShortBlock
	if err := binary.Read(sblockReader, binary.BigEndian, &hoge); err != nil {
		log.Fatalf("binary read error: %+v", err)
	}
	readBlock(f)
	readBlock(f)

	for i := 0; i < 64; i++ {
		if i%8 == 0 {
			sblockReader = bytes.NewReader(readBlock(f))
		}
		inode, err := xfs.ParseInode(sblockReader, int64(sb.Inodesize))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("=============================================================")
		fmt.Println(inode)
	}

}

type AllocRec struct {
	StartBlock uint32
	BlockCount uint32
}

var blockCount int64

func readBlock(r io.Reader) []byte {
	buf := make([]byte, 4096)
	r.Read(buf)
	// ------- debug ----------
	blockCount++
	fmt.Println(blockCount)
	// ------------------------
	return buf
}

func readSector(r io.Reader) []byte {
	buf := make([]byte, 512)
	r.Read(buf)
	return buf
}

func debugBlock(r io.Reader) {
	buf, _ := ioutil.ReadAll(r)
	lineByteSize := 16
	for i, b := range buf {
		if i%2 == 0 {
			fmt.Print(" ")
		}
		if i%lineByteSize == 0 {
			fmt.Println("")
		}
		fmt.Printf("%02x", b)
	}
}
