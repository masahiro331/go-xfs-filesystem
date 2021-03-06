package xfs

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/xerrors"

	"github.com/masahiro331/go-xfs-filesystem/log"
)

var (
	_ fs.FS        = &FileSystem{}
	_ fs.ReadDirFS = &FileSystem{}
	_ fs.StatFS    = &FileSystem{}

	_ fs.File     = &File{}
	_ fs.FileInfo = &FileInfo{}
	_ fs.DirEntry = dirEntry{}

	ErrOpenSymlink = xerrors.New("symlink open not support")
)

// FileSystem is implemented io/fs FS interface
type FileSystem struct {
	file      *os.File
	PrimaryAG AG
	AGs       []AG
}

func Verify(r io.Reader) bool {
	_, err := parseSuperBlock(r)
	if err != nil {
		return false
	}
	return true
}

func NewFS(f *os.File) (*FileSystem, error) {
	primaryAG, err := ParseAG(f)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse primary allocation group: %w", nil)
	}

	fileSystem := FileSystem{
		file:      f,
		PrimaryAG: *primaryAG,
		AGs:       []AG{*primaryAG},
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
		fileSystem.AGs = append(fileSystem.AGs, *ag)
	}
	return &fileSystem, nil
}

func (xfs *FileSystem) Close() error {
	return xfs.file.Close()
}

func (xfs *FileSystem) Stat(name string) (fs.FileInfo, error) {
	const op = "stat"

	f, err := xfs.Open(name)
	if err != nil {
		info, err := xfs.ReadDirInfo(name)
		if err != nil {
			return nil, xfs.wrapError(op, name, xerrors.Errorf("failed to read dir info: %w", err))
		}
		return info, nil
	}
	return f.Stat()
}

func (xfs *FileSystem) newFile(inode *Inode) ([]byte, error) {
	var buf []byte
	for _, rec := range inode.regularExtent.bmbtRecs {
		p := rec.Unpack()
		physicalBlockOffset := xfs.PrimaryAG.SuperBlock.BlockToPhysicalOffset(p.StartBlock)
		_, err := xfs.seekBlock(physicalBlockOffset)
		if err != nil {
			return nil, xerrors.Errorf("failed to seek block: %w", err)
		}
		b, err := xfs.readBlock(uint32(p.BlockCount))
		if err != nil {
			return nil, xerrors.Errorf("failed to read block: %w", err)
		}

		buf = append(buf, b...)
	}
	return buf[:inode.inodeCore.Size], nil
}

func (xfs *FileSystem) ReadDir(name string) ([]fs.DirEntry, error) {
	const op = "read directory"

	dirEntries, err := xfs.readDirEntry(name)
	if err != nil {
		return nil, xfs.wrapError(op, name, err)
	}
	return dirEntries, nil
}

func (xfs *FileSystem) ReadDirInfo(name string) (fs.FileInfo, error) {
	if name == "/" {
		inode, err := xfs.getRootInode()
		if err != nil {
			return nil, xerrors.Errorf("failed to parse root inode: %w", err)
		}
		return FileInfo{
			name:  "/",
			inode: inode,
			mode:  fs.FileMode(inode.inodeCore.Mode),
		}, nil
	}
	name = strings.TrimRight(name, string(filepath.Separator))

	dirs, dir := path.Split(name)
	dirEntries, err := xfs.readDirEntry(dirs)
	if err != nil {
		return nil, xerrors.Errorf("failed to read dir entry: %w", err)
	}
	for _, entry := range dirEntries {
		if entry.Name() == strings.Trim(dir, string(filepath.Separator)) {
			return entry.Info()
		}
	}

	return nil, fs.ErrNotExist
}

func (xfs *FileSystem) getRootInode() (*Inode, error) {
	inode, err := xfs.ParseInode(xfs.PrimaryAG.SuperBlock.Rootino)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse root inode: %w", err)
	}
	return inode, nil
}

// TODO: support ReadFile Interface
func (xfs *FileSystem) ReadFile(name string) ([]byte, error) {
	return []byte{}, nil
}

// TODO: support GlobFS Interface
func (xfs *FileSystem) Glob(pattern string) ([]string, error) {
	return []string{}, nil
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

	dirName, fileName := filepath.Split(name)
	dirEntries, err := xfs.ReadDir(dirName)
	if err != nil {
		return nil, xfs.wrapError(op, name, xerrors.Errorf("railed to read directory: %w", err))
	}

	for _, entry := range dirEntries {
		if !entry.IsDir() && entry.Name() == fileName {
			if dir, ok := entry.(dirEntry); ok {
				if dir.Type().Perm()&0xA000 != 0 {
					return nil, ErrOpenSymlink
				}

				if dir.inode.regularExtent == nil {
					return nil, xerrors.Errorf("regular extent empty: %v", fs.ErrNotExist)
				}

				buf, err := xfs.newFile(dir.inode)
				if err != nil {
					return nil, xerrors.Errorf("failed to new file: %w", err)
				}

				return &File{
					fs:       xfs,
					FileInfo: dir.FileInfo,
					buffer:   bytes.NewBuffer(buf),
				}, nil
			}
		}
	}
	return nil, fs.ErrNotExist
}

func (xfs *FileSystem) seekInode(n uint64) (int64, error) {
	return xfs.file.Seek(int64(xfs.PrimaryAG.SuperBlock.InodeAbsOffset(n)), 0)
}

func (xfs *FileSystem) seekBlock(n int64) (int64, error) {
	return xfs.file.Seek(n*int64(xfs.PrimaryAG.SuperBlock.BlockSize), 0)
}

func (xfs *FileSystem) readBlock(count uint32) ([]byte, error) {
	buf := make([]byte, xfs.PrimaryAG.SuperBlock.BlockSize*count)
	_, err := xfs.file.Read(buf)
	if err != nil {
		return nil, xerrors.Errorf("failed to read file error: %w", err)
	}

	return buf, err
}

func (xfs *FileSystem) readDirEntry(name string) ([]fs.DirEntry, error) {
	inode, err := xfs.getRootInode()
	if err != nil {
		return nil, xerrors.Errorf("failed to get root inode: %w", err)
	}

	fileInfos, err := xfs.listFileInfo(inode.inodeCore.Ino)
	if err != nil {
		return nil, xerrors.Errorf("failed to list root inode directory entries: %w", err)
	}

	currentInode := inode
	dirs := strings.Split(strings.Trim(filepath.Clean(name), string(filepath.Separator)), string(filepath.Separator))
	for i, dir := range dirs {
		found := false
		for _, fileInfo := range fileInfos {
			if fileInfo.Name() == dir {
				if !fileInfo.IsDir() {
					return nil, xerrors.Errorf("%s is file, directory: %w", fileInfo.Name(), fs.ErrNotExist)
				}
				found = true
				currentInode = fileInfo.inode
				break
			}
		}
		if !found {
			return nil, fs.ErrNotExist
		}

		fileInfos, err = xfs.listFileInfo(currentInode.inodeCore.Ino)
		if err != nil {
			return nil, xerrors.Errorf("failed to list directory entries inode: %d: %w", currentInode.inodeCore.Ino, err)
		}

		// list last directory
		if i == len(dirs)-1 {
			var dirEntries []fs.DirEntry
			for _, fileInfo := range fileInfos {
				// Skip current directory and parent directory
				// infinit loop in walkDir
				if fileInfo.Name() == "." || fileInfo.Name() == ".." {
					continue
				}

				dirEntries = append(dirEntries, dirEntry{fileInfo})
			}
			return dirEntries, nil
		}
	}
	return nil, fs.ErrNotExist
}

func (xfs *FileSystem) listFileInfo(ino uint64) ([]FileInfo, error) {
	entries, err := xfs.listEntries(ino)
	if err != nil {
		return nil, xerrors.Errorf("failed to list entries: %w", err)
	}

	var fileInfos []FileInfo
	for _, entry := range entries {
		inode, err := xfs.ParseInode(entry.InodeNumber())
		if err != nil {
			return nil, xerrors.Errorf("failed to parse inode %d: %w", entry.InodeNumber(), err)
		}
		// TODO: mode use inode.InodeCore.Mode
		fileInfos = append(fileInfos,
			FileInfo{
				name:  entry.Name(),
				inode: inode,
				mode:  fs.FileMode(inode.inodeCore.Mode),
			},
		)
	}
	return fileInfos, nil
}

func (xfs *FileSystem) listEntries(ino uint64) ([]Entry, error) {
	inode, err := xfs.ParseInode(ino)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse inode: %w", err)
	}

	if !inode.inodeCore.IsDir() {
		return nil, xerrors.New("error inode is not directory")
	}

	var entries []Entry
	if inode.directoryLocal != nil {
		for _, entry := range inode.directoryLocal.entries {
			entries = append(entries, entry)
		}
	} else if inode.directoryExtents != nil {
		if len(inode.directoryExtents.bmbtRecs) == 0 {
			return nil, xerrors.New("directory extents tree bmbtRecs is empty error")
		}

		for _, b := range inode.directoryExtents.bmbtRecs {
			p := b.Unpack()
			for i := 0; i < int(p.BlockCount); i++ {
				block, err := xfs.parseDir2Block(p)
				if err != nil {
					if !xerrors.Is(err, UnsupportedDir2BlockHeaderErr) {
						return nil, xerrors.Errorf("failed to parse dir2 block: %w", err)
					}
					log.Logger.Warn(err)
				}

				if block == nil {
					break
				}

				for _, entry := range block.Entries {
					entries = append(entries, entry)
				}
				p.StartBlock++
				p.StartOff++
			}
		}
	} else {
		return nil, xerrors.New("not found entries")
	}
	return entries, nil
}

// FileInfo is implemented io/fs FileInfo interface
type FileInfo struct {
	name  string
	inode *Inode

	mode fs.FileMode
}

func (i FileInfo) IsDir() bool {
	return i.inode.inodeCore.IsDir()
}

func (i FileInfo) ModTime() time.Time {
	return time.Unix(int64(i.inode.inodeCore.Mtime), 0)
}

func (i FileInfo) Size() int64 {
	return int64(i.inode.inodeCore.Size)
}

func (i FileInfo) Name() string {
	return i.name
}

func (i FileInfo) Sys() interface{} {
	return nil
}

func (i FileInfo) Mode() fs.FileMode {
	return i.mode
}

// dirEntry is implemented io/fs DirEntry interface
type dirEntry struct {
	FileInfo
}

func (d dirEntry) Type() fs.FileMode {
	return d.FileInfo.Mode().Type()
}

func (d dirEntry) Info() (fs.FileInfo, error) { return d.FileInfo, nil }

// File is implemented io/fs File interface
type File struct {
	fs *FileSystem
	FileInfo

	buffer *bytes.Buffer
}

func (f *File) Stat() (fs.FileInfo, error) {
	return &f.FileInfo, nil
}

func (f *File) Read(buf []byte) (int, error) {
	return f.buffer.Read(buf)
}

func (f *File) Close() error {
	return nil
}
