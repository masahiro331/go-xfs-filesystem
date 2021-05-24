package xfs

import (
	"fmt"
	"io/fs"
	"os"
	"time"

	"golang.org/x/xerrors"
)

var _ fs.FS = &FileSystem{}
var _ fs.File = &File{}
var _ fs.FileInfo = &FileInfo{}
var _ fs.DirEntry = dirEntry{}

// FileSystem is implemented io/fs FS interface
type FileSystem struct {
	file      *os.File
	PrimaryAG AG
	AGs       []AG

	// DEBUG
	CurrentInode uint64
}

// File is implemented io/fs File interface
type File struct {
	fs *FileSystem

	inode uint32
}

// FileInfo is implemented io/fs FileInfo interface
type FileInfo struct {
}

// dirEntry is implemented io/fs DirEntry interface
type dirEntry struct {
	fs.FileInfo
}

func (d dirEntry) Type() fs.FileMode { return d.FileInfo.Mode().Type() }

func (d dirEntry) Info() (fs.FileInfo, error) { return d.FileInfo, nil }

func (xfs *FileSystem) ReadDir(name string) ([]fs.DirEntry, error) {
	const op = "read directory"

	// dirs := strings.Split(strings.Trim(name, string(filepath.Separator)), string(filepath.Separator))
	// for _, dir := range dirs {
	// 	inodeNameMap, err := xfs.listEntry(xfs.PrimaryAG.SuperBlock.Rootino)
	// 	if err != nil {
	// 		return nil, xfs.wrapError(op, name, err)
	// 	}
	// }

	// If file is not exist or directory, return fs.ErrNotExist
	// return nil, fs.ErrNotExist

	ret := make([]fs.DirEntry, 0)
	return ret, nil
}

func (xfs *FileSystem) getRootInode() (*Inode, error) {
	inode, err := xfs.ParseInode(xfs.PrimaryAG.SuperBlock.Rootino)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse root inode: %w", err)
	}
	return inode, nil
}

func (xfs *FileSystem) ReadFile(name string) ([]byte, error) {
	return []byte{}, nil
}

func (xfs *FileSystem) wrapError(op, path string, err error) error {
	return &fs.PathError{
		Op:   op,
		Path: path,
		Err:  err,
	}
}

func (xfs *FileSystem) Open(name string) (fs.File, error) {
	const op = "open"

	if !fs.ValidPath(name) {
		return nil, xfs.wrapError(op, name, fs.ErrInvalid)
	}

	// dirName, fileName := path.Split(name)

	// dirs := strings.Split(strings.Trim(dirName, string(filepath.Separator)), string(filepath.Separator))
	// for _, dir := range dirs {
	// 	inodeNameMap, err := xfs.listEntry(xfs.PrimaryAG.SuperBlock.Rootino)
	// 	if err != nil {
	// 		return nil, xfs.wrapError(op, name, err)
	// 	}
	// }

	// If file is not exist or directory, return fs.ErrNotExist
	// return nil, fs.ErrNotExist

	return &File{}, nil
}

func (xfs *FileSystem) Glob(pattern string) ([]string, error) {
	return []string{}, nil
}

func (i *FileInfo) IsDir() bool {
	return false
}

func (i *FileInfo) ModTime() time.Time {
	return time.Now()
}

func (i *FileInfo) Size() int64 {
	return 0
}

func (i *FileInfo) Name() string {
	return ""
}

func (i *FileInfo) Sys() interface{} {
	return nil
}

func (i *FileInfo) Mode() fs.FileMode {
	return fs.FileMode(0)
}

func (f *File) Stat() (fs.FileInfo, error) {
	return &FileInfo{}, nil
}

func (f *File) Read(buf []byte) (int, error) {
	return 0, nil
}

func (f *File) Close() error {
	return nil
}

func NewFileSystem(f *os.File) (*FileSystem, error) {
	primaryAG, err := ParseAG(f)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse primary allocation group: %w", nil)
	}

	fs := FileSystem{
		file:         f,
		PrimaryAG:    *primaryAG,
		AGs:          []AG{*primaryAG},
		CurrentInode: primaryAG.SuperBlock.Rootino,
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

func (xfs *FileSystem) seekInode(n uint64) {
	xfs.file.Seek(int64(xfs.PrimaryAG.SuperBlock.InodeAbsOffset(n)), 0)
}

func (xfs *FileSystem) seekBlock(n int64) {
	xfs.file.Seek(n*int64(xfs.PrimaryAG.SuperBlock.BlockSize), 0)
}

func (xfs *FileSystem) readBlock(count uint32) ([]byte, error) {
	buf := make([]byte, xfs.PrimaryAG.SuperBlock.BlockSize*count)
	_, err := xfs.file.Read(buf)
	if err != nil {
		return nil, xerrors.Errorf("failed to read file error: %w", err)
	}

	return buf, err
}

func (xfs *FileSystem) listEntries(ino uint64) ([]Entry, error) {
	inode, err := xfs.ParseInode(ino)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse inode: %w", err)
	}
	fmt.Println(inode)
	return nil, nil

	// inode, err := ParseInode(fs.file, int64(fs.PrimaryAG.SuperBlock.Inodesize))
	// if err != nil {
	// 	return "", xerrors.Errorf("failed to parse inode: %w", err)
	// }
	// if !inode.inodeCore.IsDir() {
	// 	return "", xerrors.New("error inode is not directory")
	// }
	// if inode.directoryLocal != nil {
	// 	for _, entry := range inode.directoryLocal.entries {
	// 		fs.seekInode(uint64(entry.Inumber))
	// 		inode, err := ParseInode(fs.file, int64(fs.PrimaryAG.SuperBlock.Inodesize))
	// 		if err != nil {
	// 			return "", xerrors.Errorf("failed to parse child inode: %w", err)
	// 		}
	// 		ret += fmt.Sprintf("%s: (mode: %d)\n", entry, inode.inodeCore.Mode)
	// 	}
	// 	return ret, nil
	// }
	// if inode.directoryExtents != nil {
	// 	if len(inode.directoryExtents.bmbtRecs) == 0 {
	// 		panic("directory extents tree bmbtRecs is empty error")
	// 	}
	// 	for _, b := range inode.directoryExtents.bmbtRecs {
	// 		p := b.Unpack()
	// 		physicalBlockOffset := fs.PrimaryAG.SuperBlock.BlockToPhysicalOffset(p.StartBlock)

	// 		fs.seekBlock(physicalBlockOffset)
	// 		buf, err := fs.readBlock(uint32(p.BlockCount))
	// 		if err != nil {
	// 			return "", xerrors.Errorf("failed to read block error: %w", err)
	// 		}
	// 		block, err := parseDir2Block(bytes.NewReader(buf), fs.PrimaryAG.SuperBlock.BlockSize*uint32(p.BlockCount))
	// 		if err != nil {
	// 			if !xerrors.Is(err, UnsupportedDir2BlockHeaderErr) {
	// 				return "", xerrors.Errorf("failed to parse dir2 block: %w", err)
	// 			}
	// 		}
	// 		if block == nil {
	// 			break
	// 		}

	// 		for _, entry := range block.Entries {
	// 			fs.seekInode(entry.Inumber)
	// 			inode, err := ParseInode(fs.file, int64(fs.PrimaryAG.SuperBlock.Inodesize))
	// 			if err != nil {
	// 				return "", xerrors.Errorf("failed to parse child inode: %w", err)
	// 			}
	// 			ret += fmt.Sprintf("%s: (mode: %d)\n", entry, inode.inodeCore.Mode)
	// 		}
	// 	}
	// 	return ret, nil
	// }
}

func (xfs *FileSystem) listEntry(ino uint64) (map[string]uint64, error) {
	inode, err := xfs.ParseInode(ino)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse inode: %w", err)
	}
	if !inode.inodeCore.IsDir() {
		return nil, xerrors.New("error inode is not directory")
	}

	if inode.directoryLocal == nil {
		return nil, xerrors.Errorf("error inode directory local is null: %+v", inode)
	}

	nameInodeMap := map[string]uint64{}
	for _, entry := range inode.directoryLocal.entries {
		nameInodeMap[entry.Name()] = uint64(entry.InodeNumber())
	}
	return nameInodeMap, nil
}
