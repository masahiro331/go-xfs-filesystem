package xfs

import (
	"bytes"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/xerrors"

	"github.com/masahiro331/go-xfs-filesystem/log"
	"github.com/masahiro331/go-xfs-filesystem/xfs/utils"
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

var (
	ErrReadSizeFormat   = "failed to read size error: actual(%d), expected(%d)"
	ErrSeekOffsetFormat = "failed to seek offset error: actual(%d), expected(%d)"
)

// FileSystem is implemented io/fs FS interface
type FileSystem struct {
	r         *io.SectionReader
	PrimaryAG AG
	AGs       []AG

	cache Cache[string, any]
}

func Check(r io.Reader) bool {
	_, err := parseSuperBlock(r)
	if err != nil {
		return false
	}
	return true
}

func NewFS(r io.SectionReader, cache Cache[string, any]) (*FileSystem, error) {
	primaryAG, err := ParseAG(&r)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse primary allocation group: %w", err)
	}

	if cache == nil {
		cache = &mockCache[string, any]{}
	}
	fileSystem := FileSystem{
		r:         &r,
		PrimaryAG: *primaryAG,
		AGs:       []AG{*primaryAG},
		cache:     cache,
	}

	AGSize := int64(primaryAG.SuperBlock.Agblocks) * int64(primaryAG.SuperBlock.BlockSize)
	for i := int64(1); i < int64(primaryAG.SuperBlock.Agcount); i++ {
		n, err := r.Seek(AGSize*i, 0)
		if err != nil {
			return nil, xerrors.Errorf("failed to seek file: %w", err)
		}
		if n != AGSize*i {
			return nil, xerrors.Errorf(ErrSeekOffsetFormat, n, AGSize*i)
		}
		ag, err := ParseAG(&r)
		if err != nil {
			return nil, xerrors.Errorf("failed to parse allocation group %d: %w", i, err)
		}
		fileSystem.AGs = append(fileSystem.AGs, *ag)
	}
	return &fileSystem, nil
}

func (xfs *FileSystem) Close() error {
	return nil
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

func (xfs *FileSystem) newFile(dirEntry dirEntry) (*File, error) {
	var recs []BmbtRec
	if dirEntry.inode.regularExtent != nil {
		recs = dirEntry.inode.regularExtent.bmbtRecs
	} else if dirEntry.inode.regularBtree != nil {
		recs = dirEntry.inode.regularBtree.bmbtRecs
	} else {
		return nil, xerrors.Errorf("unsupported inode: %+v", dirEntry.inode)
	}

	dt := make(dataTable)
	for _, rec := range recs {
		p := rec.Unpack()
		physicalBlockOffset := xfs.PrimaryAG.SuperBlock.BlockToPhysicalOffset(p.StartBlock)
		for i := int64(0); i < int64(p.BlockCount); i++ {
			dt[int64(p.StartOff)+i] = physicalBlockOffset + i
		}
	}

	return &File{
		fs:           xfs,
		FileInfo:     dirEntry.FileInfo,
		buffer:       bytes.NewBuffer(nil),
		blockSize:    int64(xfs.PrimaryAG.SuperBlock.BlockSize),
		currentBlock: -1,
		table:        dt,
	}, nil
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
	name = strings.TrimRight(name, "/")

	dirs, dir := path.Split(name)
	dirEntries, err := xfs.readDirEntry(dirs)
	if err != nil {
		return nil, xerrors.Errorf("failed to read dir entry: %w", err)
	}
	for _, entry := range dirEntries {
		if entry.Name() == strings.Trim(dir, "/") {
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
	panic("implement me")
	return []byte{}, nil
}

// TODO: support GlobFS Interface
func (xfs *FileSystem) Glob(pattern string) ([]string, error) {
	panic("implement me")
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

				f, err := xfs.newFile(dir)
				if err != nil {
					return nil, xerrors.Errorf("failed to new file: %w", err)
				}

				return f, nil
			}
		}
	}
	return nil, fs.ErrNotExist
}

func (xfs *FileSystem) seekInode(n uint64) (int64, error) {
	offset := int64(xfs.PrimaryAG.SuperBlock.InodeAbsOffset(n))
	off, err := xfs.r.Seek(offset, io.SeekStart)
	if err != nil {
		return 0, err
	}
	if off != offset {
		return 0, xerrors.Errorf(ErrSeekOffsetFormat, off, offset)
	}
	return off, nil
}

func (xfs *FileSystem) seekBlock(n int64) (int64, error) {
	offset := n * int64(xfs.PrimaryAG.SuperBlock.BlockSize)
	off, err := xfs.r.Seek(offset, io.SeekStart)
	if err != nil {
		return 0, err
	}
	if off != offset {
		return 0, xerrors.Errorf(ErrSeekOffsetFormat, off, offset)
	}
	return off, nil
}

func (xfs *FileSystem) readBlock(count uint32) ([]byte, error) {
	buf := make([]byte, 0, xfs.PrimaryAG.SuperBlock.BlockSize*count)
	for i := uint32(0); i < count; i++ {
		b, err := utils.ReadBlock(xfs.r)
		if err != nil {
			return nil, err
		}
		buf = append(buf, b...)
	}
	return buf, nil
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
	dirs := strings.Split(strings.Trim(filepath.Clean(name), "/"), "/")
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
		// when dir string is empty ("", "."), that is root directory
		if !found && (dir != "" && dir != ".") {
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
			blockEntries, err := xfs.parseDir2Block(p)
			if err != nil {
				log.Logger.Warn(err)
			}

			if len(blockEntries) == 0 {
				continue
			}

			for _, entry := range blockEntries {
				entries = append(entries, entry)
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

	blockSize    int64
	currentBlock int64
	table        dataTable
}

// map[offset]
type dataTable map[int64]int64

func (f *File) Stat() (fs.FileInfo, error) {
	return &f.FileInfo, nil
}

func (f *File) Read(buf []byte) (int, error) {
	if f.buffer.Len() == 0 {
		f.currentBlock++
		if f.currentBlock*f.blockSize >= f.Size() {
			f.buffer = nil
			return 0, io.EOF
		}
	} else {
		return f.buffer.Read(buf)
	}

	offset, ok := f.table[f.currentBlock]
	if !ok {
		if f.Size()-f.blockSize*f.currentBlock < f.blockSize {
			f.buffer.Write(make([]byte, f.Size()-f.blockSize*f.currentBlock))
		}
		f.buffer.Write(make([]byte, f.blockSize))
	} else {
		_, err := f.fs.seekBlock(offset)
		if err != nil {
			return 0, xerrors.Errorf("failed to seek block: %w", err)
		}
		b, err := f.fs.readBlock(1)
		if err != nil {
			return 0, xerrors.Errorf("failed to read block: %w", err)
		}

		if f.Size()-f.blockSize*f.currentBlock < f.blockSize {
			b = b[:f.Size()-f.blockSize*f.currentBlock]
		}
		n, err := f.buffer.Write(b)
		if n != len(b) {
			return 0, xerrors.Errorf("write buffer error: actual(%d), expected(%d)", n, len(b))
		}
	}

	return f.buffer.Read(buf)
}

func (f *File) Close() error {
	return nil
}
