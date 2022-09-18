package main

import (
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"os"

	"github.com/masahiro331/go-xfs-filesystem/xfs"
	"golang.org/x/xerrors"
)

func main() {
	f, err := os.Open("path to your linux.img")
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

	err = fs.WalkDir(filesystem, "etc", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return xerrors.Errorf("file walk error: %w", err)
		}
		if d.IsDir() {
			return nil
		}

		if path == "etc/os-release" {
			file, err := filesystem.Open(path)
			if err != nil {
				return err
			}
			buf, err := ioutil.ReadAll(file)
			if err != nil {
				return err
			}
			fmt.Println(string(buf))
			os.Exit(0)
		}
		return nil

	})
	if err != nil {
		log.Fatal(err)
	}
}
