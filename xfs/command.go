package xfs

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"

	"github.com/masahiro331/go-xfs-filesystem/xfs/utils"
	"golang.org/x/xerrors"
)

func (fs *FileSystem) Search(commands ...string) (string, error) {
	var retStr string
	if len(commands) != 2 {
		return "", xerrors.New("invalid arguments error: search [hex string]")
	}
	hexBytes, err := hex.DecodeString(commands[1])
	if err != nil {
		return "", xerrors.Errorf("failed to parse hex string: %w", err)
	}

	var count int
	fs.file.Seek(0, 0)
	for {
		buf, err := fs.readBlock(1)
		if err != nil {
			if xerrors.Is(err, io.EOF) {
				break
			}
			return "", xerrors.Errorf("failed to read file(count: %d): %w", count, err)
		}
		i := bytes.Index(buf, hexBytes)
		if i >= 0 {
			retStr += fmt.Sprintf("found(%x) block count: %d, offset: %d\n", buf[i:i+len(hexBytes)], count, i)
		}
		count++
	}
	if retStr != "" {
		return retStr, nil
	}
	return fmt.Sprintf("%v bytes are not found", hexBytes), nil
}

func (fs *FileSystem) ChangeDirectory(commands ...string) (string, error) {
	if len(commands) != 2 {
		return "", xerrors.New("invalid arguments error: cd [directory]")
	}
	if commands[1] == "/" {
		fs.seekInode(fs.PrimaryAG.SuperBlock.Rootino)
		fs.CurrentInode = fs.PrimaryAG.SuperBlock.Rootino
		return "", nil
	}

	nameInodeMap, err := fs.listEntry(fs.CurrentInode)
	if err != nil {
		return "", xerrors.Errorf("failed to list entry: %w", err)
	}
	if n, ok := nameInodeMap[commands[1]]; ok {
		fs.seekInode(n)
		fs.CurrentInode = uint64(n)
		return "", nil
	}
	return "", xerrors.Errorf("no such file or directory: %s", commands[1])
}

func (fs *FileSystem) Debug(commands ...string) {
	if len(commands) != 2 {
		panic("debug arguments error")
	}
	offset, err := strconv.Atoi(commands[1])
	if err != nil {
		panic("debug arguments error: " + commands[1])
	}
	fs.seekBlock(int64(offset))
	utils.DebugBlock(utils.ReadBlock(fs.file))
}

func (fs *FileSystem) ChangeInode(commands ...string) (string, error) {
	if len(commands) != 2 {
		return "", xerrors.New("invalid arguments error: inode [inode number]")
	}

	if commands[1] == "/" {
		fs.seekInode(fs.PrimaryAG.SuperBlock.Rootino)
		fs.CurrentInode = fs.PrimaryAG.SuperBlock.Rootino
		return "", nil
	}

	inodeNumber, err := strconv.Atoi(commands[1])
	if err != nil {
		return "", xerrors.Errorf("invalid arguments error: %w", err)
	}

	fs.seekInode(uint64(inodeNumber))
	fs.CurrentInode = uint64(inodeNumber)
	return "", nil
}

func (xfs *FileSystem) ListSegments(commands ...string) (string, error) {
	var ret string

	inodeNumber := xfs.CurrentInode
	if len(commands) > 1 {
		nameInodeMap, err := xfs.listEntry(xfs.CurrentInode)
		if err != nil {
			return "", xerrors.Errorf("failed to list entry: %w", err)
		}
		if n, ok := nameInodeMap[commands[1]]; ok {
			inodeNumber = n
		} else {
			return "", xerrors.Errorf("no such file or directory: %s", commands[1])
		}
	}

	inode, err := xfs.ParseInode(inodeNumber)
	if err != nil {
		return "", xerrors.Errorf("failed to parse inode: %w", err)
	}
	if !inode.inodeCore.IsDir() {
		return "", xerrors.New("error inode is not directory")
	}
	if inode.directoryLocal != nil {
		for _, entry := range inode.directoryLocal.entries {
			inode, err := xfs.ParseInode(entry.InodeNumber())
			if err != nil {
				return "", xerrors.Errorf("failed to parse child inode: %w", err)
			}
			ret += fmt.Sprintf("%s: (mode: %d)\n", entry, inode.inodeCore.Mode)
		}
		return ret, nil
	}
	if inode.directoryExtents != nil {
		if len(inode.directoryExtents.bmbtRecs) == 0 {
			panic("directory extents tree bmbtRecs is empty error")
		}
		for _, b := range inode.directoryExtents.bmbtRecs {
			block, err := xfs.parseDir2Block(b.Unpack())
			if err != nil {
				if !xerrors.Is(err, UnsupportedDir2BlockHeaderErr) {
					return "", xerrors.Errorf("failed to parse dir2 block: %w", err)
				}
			}
			if block == nil {
				break
			}

			for _, entry := range block.Entries {
				inode, err := xfs.ParseInode(entry.InodeNumber())
				if err != nil {
					return "", xerrors.Errorf("failed to parse child inode: %w", err)
				}
				ret += fmt.Sprintf("%s: (mode: %d)\n", entry, inode.inodeCore.Mode)
			}
		}
		return ret, nil
	}
	return "", xerrors.Errorf("error inode directory is null: %+v", inode)
}

func (fs *FileSystem) Tree(commands ...string) (string, error) {
	panic("not support")
	return "", nil
}

func (fs *FileSystem) Catenate(commands ...string) (string, error) {
	panic("not support")
	return "", nil
}

func (xfs *FileSystem) Print(commands ...string) (string, error) {
	inodeNumber := xfs.CurrentInode
	if len(commands) > 1 {
		nameInodeMap, err := xfs.listEntry(xfs.CurrentInode)
		if err != nil {
			return "", xerrors.Errorf("failed to list entry: %w", err)
		}
		if n, ok := nameInodeMap[commands[1]]; ok {
			inodeNumber = n
		} else {
			return "", xerrors.Errorf("no such file or directory: %s", commands[1])
		}
	}

	inode, err := xfs.ParseInode(inodeNumber)
	if err != nil {
		return "", xerrors.Errorf("failed to parse inode: %w", err)
	}

	fileBuffer := []byte{}

	for _, rec := range inode.regularExtent.bmbtRecs {
		p := rec.Unpack()
		physicalBlockOffset := xfs.PrimaryAG.SuperBlock.BlockToPhysicalOffset(p.StartBlock)
		xfs.seekBlock(physicalBlockOffset)
		buf, err := xfs.readBlock(uint32(p.BlockCount))
		if err != nil {
			return "", xerrors.Errorf("failed to read block: %w", err)
		}

		fileBuffer = append(fileBuffer, buf...)
	}

	return string(fileBuffer[:inode.inodeCore.Size]), nil
}
