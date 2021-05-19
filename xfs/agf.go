package xfs

type AGF struct {
	Magicnum   [4]byte
	Versionnum uint32
	Seqno      uint32
	Length     uint32

	Roots  [3]uint32
	Levels [3]uint32

	Flfirst   uint32
	Fllast    uint32
	Flcount   uint32
	Freeblks  uint32
	Longest   uint32
	Btreeblks uint32
	UUID      [16]byte

	RmapBlocks     uint32
	RefcountBlocks uint32
	RefcountRoot   uint32
	RefcountLevel  uint32
	Spare64        [112]byte
	Lsn            uint64
	CRC            uint32
	Spare2         uint32
}
