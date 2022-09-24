# go-xfs-filesystem

A Go library for parsing xfs(FileSystem)

go-xfs-filesystem is a library for read directory and read files.

This library implementation io/fs

## Quick start

```
package main

import (
	"fmt"
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
	filesystem, err := xfs.NewFileSystem(f)
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
```

# How to create test data

## make image data with xfs

Use Linux OS. (This command use CentOS 8)

```bash
# make xfs image

# minimum xfs filesystem , BootSector 4 block, xfs 16 block
dd of=Linux.img count=0 seek=1 bs=41943040 

sudo losetup -f
DEVICE=/dev/loop6
sudo losetup $DEVICE Linux.img
sudo parted $DEVICE -s mklabel gpt -s mkpart primary xfs 0 100%


sudo mkfs.xfs ${DEVICE}p1


# mount
sudo mkdir /mnt/xfs
sudo mount ${DEVICE}p1 /mnt/xfs
sudo chmod 755 /mnt/xfs

# Write test datas

## local directory
sudo mkdir /mnt/xfs/fmt_local_directory
sudo mkdir /mnt/xfs/fmt_local_directory/short_form

# block directories
sudo mkdir /mnt/xfs/fmt_extents_block_directories
sudo mkdir /mnt/xfs/fmt_extents_block_directories/1
sudo mkdir /mnt/xfs/fmt_extents_block_directories/2
sudo mkdir /mnt/xfs/fmt_extents_block_directories/3
sudo mkdir /mnt/xfs/fmt_extents_block_directories/4
sudo mkdir /mnt/xfs/fmt_extents_block_directories/5
sudo mkdir /mnt/xfs/fmt_extents_block_directories/6
sudo mkdir /mnt/xfs/fmt_extents_block_directories/7
sudo mkdir /mnt/xfs/fmt_extents_block_directories/8

# leaf directories
sudo mkdir /mnt/xfs/fmt_leaf_directories/

dd bs=4096 count=1 if=/dev/zero of=4096
dd bs=1024 count=1 if=/dev/zero of=1024
dd bs=16384  count=1 if=/dev/zero of=16384
dd bs=8388608 count=1 if=/dev/zero of=8388608

for i in `seq 1 200`
do
    sudo cp 4096 /mnt/xfs/fmt_leaf_directories/$i
done

# node directories
sudo mkdir /mnt/xfs/fmt_node_directories/

for i in `seq 1 1024`
do
    sudo cp 4096 /mnt/xfs/fmt_node_directories/$i
done


# extents files
sudo cp 1024  /mnt/xfs/fmt_extents_file_1024
sudo cp 4096  /mnt/xfs/fmt_extents_file_4096
sudo cp 16384 /mnt/xfs/fmt_extents_file_16384
sudo cp 8388608 /mnt/xfs/fmt_extents_file_8388608

# nested directories
sudo mkdir -p /mnt/xfs/parent/child/child/child/child/child/
sudo cp 1024  /mnt/xfs/parent/child/child/child/child/child/executable
sudo chmod +x /mnt/xfs/parent/child/child/child/child/child/executable
sudo cp 1024  /mnt/xfs/parent/child/child/child/child/executable
sudo chmod +x /mnt/xfs/parent/child/child/child/child/executable
sudo cp 1024  /mnt/xfs/parent/child/child/child/child/nonexecutable

# etc/os-release
sudo mkdir -p /mnt/xfs/etc
sudo cp /etc/os-release  /mnt/xfs/etc/os-release

# remove
sudo umount /mnt/xfs
sudo losetup -d ${DEVICE}
```

## extract xfs data in Linux.img

```
# cp Linux.img local
go build -o genimage cmd/genimage/main.go
./genimage Linux.img
mv primary xfs/testdata/image.xfs
```
