package main

import (
	"fmt"
	"log"
	"os"

	"github.com/masahiro331/go-xfs-filesystem/xfs"
)

func main() {
	f, err := os.Open("/Users/masahiro331/work/go/src/github.com/masahiro331/go-xfs-filesystem/Linux.img")
	if err != nil {
		log.Fatal(err)
	}
	filesystem, err := xfs.NewFileSystem(f)
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