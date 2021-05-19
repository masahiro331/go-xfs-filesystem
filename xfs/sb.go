package xfs

type SuperBlock struct {
	Magicnum   [4]byte
	BlockSize  uint32
	Dblocks    uint64   // rfsblock
	Rblocks    uint64   // rfsblock
	Rextens    uint64   // rtblock
	UUID       [16]byte // uuid_t
	Logstart   uint64   // fsblock
	Rootino    uint64   // ino
	Rbmino     uint64
	Rsmino     uint64
	Rextsize   uint32
	Agblocks   uint32
	Agcount    uint32
	Rbblocks   uint32
	Logblocks  uint32
	Versionnum uint16
	Sectsize   uint16
	Inodesize  uint16
	Inopblock  uint16
	Fname      [12]byte
	Blocklog   uint8
	Sectlog    uint8
	Inodelog   uint8
	Inopblog   uint8
	Agdlklog   uint8
	Rextslog   uint8
	Inprogress uint8
	ImaxPct    uint8

	Icount    uint64
	Ifree     uint64
	Fdblocks  uint64
	Frextents uint64

	Uqunotino   uint64
	Gquotino    uint64
	Qflags      uint16
	Flags       uint8
	SharedVn    uint8
	Inoalignmt  uint32
	Unit        uint32
	Width       uint32
	Dirblklog   uint8
	Logsectlog  uint8
	Logsectsize uint16
	Logsunit    uint32
	Features2   uint32

	BadFeatures2        uint32
	FeaturesCompat      uint32
	FeaturesRoCompat    uint32
	FeaturesIncompat    uint32
	FeaturesLogIncompat uint32

	CRC        uint32
	SpinoAlign uint32
	Pquotino   uint64
	Lsn        int64
	MetaUUID   [16]byte
}

// return (AG number), (Inode Block), (Inode Offset)
func (sb SuperBlock) InodeOffset(inodeNumber uint32) (int, uint64, uint64) {
	offsetAddress := sb.Inopblock + uint16(sb.Agblocks)
	lowMask := (1<<(offsetAddress+1) - 1)

	AGNumber := inodeNumber >> uint32(offsetAddress)
	relativeInodeNumber := inodeNumber & uint32(lowMask)
	InodeBlock := relativeInodeNumber / uint32(sb.Inopblock)
	InodeOffset := relativeInodeNumber % uint32(sb.Inopblock)

	return int(AGNumber), uint64(InodeBlock), uint64(InodeOffset)
}
