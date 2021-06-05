# How to create test data

Use Linux OS. (This command use CentOS 8)

```
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

# block directories
mkdir /mnt/xfs/fmt_extents_block_directories
mkdir /mnt/xfs/fmt_extents_block_directories/1
mkdir /mnt/xfs/fmt_extents_block_directories/2
mkdir /mnt/xfs/fmt_extents_block_directories/3
mkdir /mnt/xfs/fmt_extents_block_directories/4
mkdir /mnt/xfs/fmt_extents_block_directories/5
mkdir /mnt/xfs/fmt_extents_block_directories/6
mkdir /mnt/xfs/fmt_extents_block_directories/7
mkdir /mnt/xfs/fmt_extents_block_directories/8

# leaf directories
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


# extents files
cp 1024  /mnt/xfs/fmt_extents_file_1024
cp 4096  /mnt/xfs/fmt_extents_file_4096
cp 16384 /mnt/xfs/fmt_extents_file_16384


# remove
sudo umount /mnt/xfs
sudo losetup -d /dev/loop0
```