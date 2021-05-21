package xfs

import (
	"fmt"
	"os"
	"strconv"

	"github.com/masahiro331/go-xfs-filesystem/xfs/utils"
	"golang.org/x/xerrors"
)

type FileSystem struct {
	file      *os.File
	PrimaryAG AG
	AGs       []AG

	// DEBUG
	CurrentDirectory uint64
}

func NewFileSystem(f *os.File) (*FileSystem, error) {
	primaryAG, err := ParseAG(f)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse primary allocation group: %w", nil)
	}

	fs := FileSystem{
		file:             f,
		PrimaryAG:        *primaryAG,
		AGs:              []AG{*primaryAG},
		CurrentDirectory: primaryAG.SuperBlock.Rootino,
	}

	AGSize := uint64(primaryAG.SuperBlock.Agblocks * primaryAG.SuperBlock.BlockSize)
	for i := uint64(1); i < uint64(primaryAG.SuperBlock.Agcount); i++ {
		_, err := f.Seek(int64(AGSize*i), 0)
		if err != nil {
			return nil, xerrors.Errorf("failed to seek file: %w", err)
		}
		ag, err := ParseAG(f)
		if err != nil {
			return nil, xerrors.Errorf("failed to parse allocation group %d: %w", i, err)
		}
		fs.AGs = append(fs.AGs, *ag)
	}
	return &fs, nil
}

func (fs *FileSystem) seekInode(n uint32) {
	fs.file.Seek(int64(fs.PrimaryAG.SuperBlock.InodeAbsOffset(n)), 0)
}

func (fs *FileSystem) ListSegments(commands ...string) (string, error) {
	var ret string

	inodeNumber := uint32(fs.CurrentDirectory)
	if len(commands) > 1 {
		nameInodeMap, err := fs.listEntry(uint32(fs.CurrentDirectory))
		if err != nil {
			return "", xerrors.Errorf("failed to list entry: %w", err)
		}
		if n, ok := nameInodeMap[commands[1]]; ok {
			inodeNumber = n
		} else {
			return "", xerrors.Errorf("no such file or directory: %s", commands[1])
		}
	}

	fs.seekInode(inodeNumber)
	inode, err := ParseInode(fs.file, int64(fs.PrimaryAG.SuperBlock.Inodesize))
	if err != nil {
		return "", xerrors.Errorf("failed to parse inode: %w", err)
	}
	if inode.directoryLocal == nil {
		return "", xerrors.Errorf("error inode directory local is null, extents: %+v", inode.directoryExtents)
	}

	for _, entry := range inode.directoryLocal.entries {
		fs.seekInode(entry.Inumber)
		inode, err := ParseInode(fs.file, int64(fs.PrimaryAG.SuperBlock.Inodesize))
		if err != nil {
			return "", xerrors.Errorf("failed to parse child inode: %w", err)
		}
		ret += fmt.Sprintf("%s: (mode: %d)\n", entry, inode.inodeCore.Mode)
	}
	return ret, nil
}

func (fs *FileSystem) Catenate() {
	panic("not support")
}

func (fs *FileSystem) ChangeInode(commands ...string) (string, error) {
	if len(commands) != 2 {
		return "", xerrors.New("invalid arguments error: inode [inode number]")
	}

	if commands[1] == "/" {
		fs.seekInode(uint32(fs.PrimaryAG.SuperBlock.Rootino))
		fs.CurrentDirectory = uint64(fs.PrimaryAG.SuperBlock.Rootino)
		return "", nil
	}

	inodeNumber, err := strconv.Atoi(commands[1])
	if err != nil {
		return "", xerrors.Errorf("invalid arguments error: %w", err)
	}

	fs.seekInode(uint32(inodeNumber))
	fs.CurrentDirectory = uint64(inodeNumber)
	return "", nil
}

func (fs *FileSystem) Debug(commands ...string) {
	if len(commands) != 2 {
		panic("debug arguments error")
	}
	offset, err := strconv.Atoi(commands[1])
	if err != nil {
		panic("debug arguments error: " + commands[1])
	}
	fs.file.Seek(int64(offset), 0)
	utils.DebugBlock(utils.ReadBlock(fs.file))
}

func (fs *FileSystem) ChangeDirectory(commands ...string) (string, error) {
	if len(commands) != 2 {
		return "", xerrors.New("invalid arguments error: cd [directory]")
	}
	if commands[1] == "/" {
		fs.seekInode(uint32(fs.PrimaryAG.SuperBlock.Rootino))
		fs.CurrentDirectory = fs.PrimaryAG.SuperBlock.Rootino
		return "", nil
	}

	nameInodeMap, err := fs.listEntry(uint32(fs.CurrentDirectory))
	if err != nil {
		return "", xerrors.Errorf("failed to list entry: %w", err)
	}
	if n, ok := nameInodeMap[commands[1]]; ok {
		fs.seekInode(n)
		fs.CurrentDirectory = uint64(n)
		return "", nil
	}
	return "", xerrors.Errorf("no such file or directory: %s", commands[1])
}

func (fs *FileSystem) listEntry(n uint32) (map[string]uint32, error) {
	fs.seekInode(n)
	inode, err := ParseInode(fs.file, int64(fs.PrimaryAG.SuperBlock.Inodesize))
	if err != nil {
		return nil, xerrors.Errorf("failed to parse inode: %w", err)
	}
	if inode.directoryLocal == nil {
		return nil, xerrors.Errorf("error inode directory local is null, extents: %+v", inode.directoryExtents)
	}

	nameInodeMap := map[string]uint32{}
	for _, entry := range inode.directoryLocal.entries {
		nameInodeMap[entry.Name] = uint32(entry.Inumber)
	}
	return nameInodeMap, nil
}

func (fs *FileSystem) Tree() {

}
