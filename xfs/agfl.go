package xfs

type AGFL struct {
	Magicnum uint32
	Seqno    uint32
	UUID     [16]byte
	Lsn      uint64
	CRC      uint32
	Bno      [118]uint32
}
