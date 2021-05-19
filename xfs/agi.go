package xfs

type AGI struct {
	Magicnum   uint32
	Versionnum uint32
	Seqno      uint32
	Length     uint32
	Count      uint32
	Root       uint32
	Level      uint32
	Freecount  uint32
	Newino     uint32
	Dirino     uint32
	Unlinked   [256]byte
	UUID       [16]byte
	CRC        uint32
	Pad32      uint32
	Lsn        uint64
	FreeRoot   uint32
	FreeLevel  uint32
	Iblocks    uint32
	Fblocks    uint32
}

type IAB3 struct {
	BtreeShortBlock
}

type FIB3 struct {
	BtreeShortBlock
}
