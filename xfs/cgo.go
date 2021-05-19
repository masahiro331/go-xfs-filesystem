package xfs

import (
	"bytes"
	"encoding/binary"
	"os"
	"syscall"
	"unsafe"
)

//BulkReq keeps track of in process request
type BulkReq struct {
	last  uint64
	batch int32
	fsfd  *os.File
}

//NewBulkReq creates a new XFS bulk request
func NewBulkReq(path string, opts ...Option) (*BulkReq, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	b := &BulkReq{
		//default batch size of 4096
		batch: 4096,
		fsfd:  f,
	}

	for _, opt := range opts {
		opt(b)
	}

	return b, nil
}

//Option sets various options for NewBulkReq
type Option func(*BulkReq)

//WithStartNum sets the starting inode for bulk request
func WithStartNum(start uint64) Option {
	return func(m *BulkReq) { m.last = start }
}

//WithBatchSize sets the batch size for bulk request
func WithBatchSize(batch int32) Option {
	return func(m *BulkReq) { m.batch = batch }
}

//Next gets the next batch of Bstats
func (b *BulkReq) Next() ([]Bstat, error) {
	buf := make([]byte, BstatSize*b.batch)
	var count int32
	f := &fsopBulkreq{
		lastip:  unsafe.Pointer(&b.last),
		icount:  b.batch,
		ubuffer: unsafe.Pointer(&buf[0]),
		ocount:  unsafe.Pointer(&count),
	}

	err := xfsctl(b.fsfd.Fd(), IOCFSBULKSTAT, uintptr(unsafe.Pointer(f)))
	if err != nil {
		return []Bstat{}, err
	}

	rbuf := bytes.NewReader(buf)
	var bstats []Bstat
	for i := 0; i < int(count); i++ {
		var b Bstat
		err := binary.Read(rbuf, binary.LittleEndian, &b)
		if err != nil {
			return []Bstat{}, err
		}
		bstats = append(bstats, b)
	}
	return bstats, nil
}

//Release BulkReq handle and cleanup
func (b *BulkReq) Release() {
	b.fsfd.Close()
}

func xfsctl(fd, cmd, ptr uintptr) error {
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, cmd, ptr)
	if err != 0 {
		return err
	}
	return nil
}

const (
	//IOCFSBULKSTAT FSBULKSTAT xfs ioctl
	IOCFSBULKSTAT = 0xc0205865
	//IOCFSBULKSTATSINGLE FSBULKSTAT_SINGLE xfs ioctl
	IOCFSBULKSTATSINGLE = 0xc0205866
	//IOCFSINUMBERS FSINUMBERS xfs ioctl
	IOCFSINUMBERS = 0xc0205867

	//BstatSize C sizeof xfs_bstat
	BstatSize = 136
)

/* pahole for xfs_bstime
struct xfs_bstime {
	time_t                     tv_sec;               //     0     8
	__s32                      tv_nsec;              //     8     4

	// size: 16, cachelines: 1, members: 2
	// padding: 4
	// last cacheline: 16 bytes
};
*/

//BsTime XFS specific time for bulk stat
type BsTime struct {
	Sec  int64
	Nsec int32
	_    int32
}

/* pahole for xfs_bstat
struct xfs_bstat {
	__u64                      bs_ino;               //     0     8
	__u16                      bs_mode;              //     8     2
	__u16                      bs_nlink;             //    10     2
	__u32                      bs_uid;               //    12     4
	__u32                      bs_gid;               //    16     4
	__u32                      bs_rdev;              //    20     4
	__s32                      bs_blksize;           //    24     4

	// XXX 4 bytes hole, try to pack

	__s64                      bs_size;              //    32     8
	xfs_bstime_t               bs_atime;             //    40    16
	xfs_bstime_t               bs_mtime;             //    56    16
	xfs_bstime_t               bs_ctime;             //    72    16
	int64_t                    bs_blocks;            //    88     8
	__u32                      bs_xflags;            //    96     4
	__s32                      bs_extsize;           //   100     4
	__s32                      bs_extents;           //   104     4
	__u32                      bs_gen;               //   108     4
	__u16                      bs_projid_lo;         //   112     2
	__u16                      bs_forkoff;           //   114     2
	__u16                      bs_projid_hi;         //   116     2
	unsigned char              bs_pad[10];           //   118    10
	__u32                      bs_dmevmask;          //   128     4
	__u16                      bs_dmstate;           //   132     2
	__u16                      bs_aextents;          //   134     2

	// size: 136, cachelines: 3, members: 23
	// sum members: 132, holes: 1, sum holes: 4
	// last cacheline: 8 bytes
};
*/

//Bstat is XFS bulk stat structure
type Bstat struct {
	Ino      uint64
	Mode     uint16
	Nlink    uint16
	UID      uint32
	GID      uint32
	Rdev     uint32
	BlkSize  int32
	_        int32
	Size     int64
	Atime    BsTime
	Mtime    BsTime
	Ctime    BsTime
	Blocks   int64
	Xflags   uint32
	ExtSize  int32
	Extents  int32
	Gen      uint32
	ProjIDLo uint16
	ForkOff  uint16
	ProjIDHi uint16
	Pad      [10]uint8
	DevMask  uint32
	DmState  uint16
	AExtents uint16
}

/* pahole for xfs_fsop_bulkreq
struct xfs_fsop_bulkreq {
	__u64 *                    lastip;               //     0     8
	__s32                      icount;               //     8     4

	// XXX 4 bytes hole, try to pack

	void *                     ubuffer;              //    16     8
	__s32 *                    ocount;               //    24     8

	// size: 32, cachelines: 1, members: 4
	// sum members: 28, holes: 1, sum holes: 4
	// last cacheline: 32 bytes
};
*/

//fsopBulkreq is XFS request structure for bulkstat
type fsopBulkreq struct {
	lastip  unsafe.Pointer
	icount  int32
	_       int32
	ubuffer unsafe.Pointer
	ocount  unsafe.Pointer
}
