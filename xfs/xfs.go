package xfs

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
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
func (fs *FileSystem) seekBlock(n uint32) {
	fs.file.Seek(int64(n*fs.PrimaryAG.SuperBlock.BlockSize), 0)
}
func (fs *FileSystem) readBlock(count uint32) ([]byte, error) {
	buf := make([]byte, fs.PrimaryAG.SuperBlock.BlockSize*count)
	_, err := fs.file.Read(buf)
	if err != nil {
		return nil, xerrors.Errorf("failed to read file error: %w", err)
	}

	return buf, err
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
	if inode.directoryLocal != nil {
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
	if inode.directoryExtents != nil {
		if len(inode.directoryExtents.bmbtRecs) == 0 {
			panic("directory extents tree bmbtRecs is empty error")
		}
		for _, b := range inode.directoryExtents.bmbtRecs {
			p := b.Unpack()
			physicalBlockOffset := fs.PrimaryAG.SuperBlock.BlockToPhysicalOffset(p.StartBlock)

			fs.seekBlock(uint32(physicalBlockOffset))
			buf, err := fs.readBlock(uint32(p.BlockCount))
			if err != nil {
				return "", xerrors.Errorf("failed to read block error: %w", err)
			}
			block, err := parseDir2Block(bytes.NewReader(buf), fs.PrimaryAG.SuperBlock.BlockSize*uint32(p.BlockCount))
			if err != nil {
				if !xerrors.Is(err, UnsupportedDir2BlockHeaderErr) {
					return "", xerrors.Errorf("failed to parse dir2 block: %w", err)
				}
			}
			if block == nil {
				break
			}

			for _, entry := range block.Entries {
				fs.seekInode(uint32(entry.Inumber))
				inode, err := ParseInode(fs.file, int64(fs.PrimaryAG.SuperBlock.Inodesize))
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
	fs.seekBlock(uint32(offset))
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
		return nil, xerrors.Errorf("error inode directory local is null: %+v", inode)
	}

	nameInodeMap := map[string]uint32{}
	for _, entry := range inode.directoryLocal.entries {
		nameInodeMap[entry.Name] = uint32(entry.Inumber)
	}
	return nameInodeMap, nil
}

func (fs *FileSystem) Tree() {

}

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
