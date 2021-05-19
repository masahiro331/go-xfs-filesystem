package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/masahiro331/go-xfs-filesystem/xfs"
	"github.com/masahiro331/go-xfs-filesystem/xfs/utils"
)

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

	ag, err := xfs.NewAG(f)
	if err != nil {
		log.Fatal(err)
	}

	// 謎のやつ //
	utils.ReadBlock(f)
	utils.ReadBlock(f)
	utils.ReadBlock(f)

	var sblockReader io.Reader
	for i := 0; i < 64; i++ {
		if i%8 == 0 {
			sblockReader = bytes.NewReader(utils.ReadBlock(f))
		}
		inode, err := xfs.ParseInode(sblockReader, int64(ag.SuperBlock.Inodesize))
		if err != nil {
			log.Fatal(err)
		}
		// Debug
		fmt.Println("=============================================================")
		fmt.Println(inode)
	}

}

type AllocRec struct {
	StartBlock uint32
	BlockCount uint32
}
