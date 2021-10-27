package main

import (
	"fmt"
	"log"
	"os"

	"github.com/masahiro331/go-xfs-filesystem/xfs"
)

func main() {
	f, err := os.Open("path your linux.img")
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
