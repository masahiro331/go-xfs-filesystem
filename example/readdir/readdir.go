package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/masahiro331/go-xfs-filesystem/xfs"
)

func main() {
	f, err := os.Open("path your linux.img")
	if err != nil {
		log.Fatal(err)
	}
	info, err := f.Stat()
	if err != nil {
		log.Fatal(err)
	}

	filesystem, err := xfs.NewFS(*io.NewSectionReader(f, 0, info.Size()))
	if err != nil {
		log.Fatal(err)
	}

	dirs, err := filesystem.ReadDir("etc/")
	if err != nil {
		log.Fatal(err)
	}
	for _, dir := range dirs {
		fmt.Println(dir.Name())
	}
}
