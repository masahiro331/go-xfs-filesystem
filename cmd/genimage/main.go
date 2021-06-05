package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/masahiro331/go-vmdk-parser/pkg/disk"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("invalid arguments error")
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatalf("failed to open: %v", err)
	}

	d, err := disk.NewDriver(f)
	if err != nil {
		log.Fatalf("failed to create disk driver: %v", err)
	}
	for _, partition := range d.GetPartitions() {
		df, err := os.Create(partition.Name())
		if err != nil {
			log.Fatalf("failed to create %s: %v", partition.Name(), err)
		}

		_, err = f.Seek(int64(partition.GetStartSector()*512), 0)
		if err != nil {
			log.Fatalf("failed to seek: %v", err)
		}
		fmt.Println(partition.GetSize() * 512)
		fmt.Println(partition.Name())
		reader := io.LimitReader(f, int64(partition.GetSize()*512))
		_, err = io.Copy(df, reader)
		if err != nil {
			log.Fatalf("failed to copy: %v", err)
		}
	}
}

/** bash
# make xfs image

# minimum xfs filesystem , BootSector 4 block, xfs 16 block
dd of=Linux.img count=0 seek=1 bs=20971520
sudo losetup -f
sudo losetup /dev/loop0 Linux.img
sudo parted /dev/loop0 -s mklabel gpt -s mkpart primary xfs 0 100%
sudo mkfs.xfs /dev/loop0p1


# mount
sudo mkdir /mnt/xfs
sudo mount /dev/loop0p1 /mnt/xfs
chmod 755 /mnt/xfs

# Write test datas
## local directory
mkdir /mnt/xfs/fmt_local_directory
mkdir /mnt/xfs/fmt_local_directory/short_form

## block directories
mkdir /mnt/xfs/fmt_extents_block_directories
mkdir /mnt/xfs/fmt_extents_block_directories/1
mkdir /mnt/xfs/fmt_extents_block_directories/2
mkdir /mnt/xfs/fmt_extents_block_directories/3
mkdir /mnt/xfs/fmt_extents_block_directories/4
mkdir /mnt/xfs/fmt_extents_block_directories/5
mkdir /mnt/xfs/fmt_extents_block_directories/6
mkdir /mnt/xfs/fmt_extents_block_directories/7
mkdir /mnt/xfs/fmt_extents_block_directories/8

## leaf directories
mkdir /mnt/xfs/fmt_leaf_directories/

for i in `seq 1 200`
do
    cp 4096 /mnt/xfs/fmt_leaf_directories/$i
done

# node directories
mkdir /mnt/xfs/fmt_node_directories/

for i in `seq 1 1024`
do
    cp 4096 /mnt/xfs/fmt_node_directories/$i
done


## extents files
cp 1024  /mnt/xfs/fmt_extents_file_1024
cp 4096  /mnt/xfs/fmt_extents_file_4096
cp 16384 /mnt/xfs/fmt_extents_file_16384

## nested directories
mkdir -p /mnt/xfs/parent/child/child/child/child/child/
cp 1024  /mnt/xfs/parent/child/child/child/child/child/executable
chmod +x /mnt/xfs/parent/child/child/child/child/child/executable

## Btree directories


# Remove
sudo umount /mnt/xfs
sudo losetup -d /dev/loop0
*/
