package xfs

import (
	"testing"
	"time"
)

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		ts       uint64
		bigtime  bool
		expected time.Time
	}{
		{
			name:     "legacy: Unix epoch (sec=0, nsec=0)",
			ts:       0,
			bigtime:  false,
			expected: time.Unix(0, 0),
		},
		{
			// xfs_timestamp_t: {t_sec=1000, t_nsec=500} → big-endian uint64: sec in upper, nsec in lower
			name:     "legacy: known timestamp (sec=1000, nsec=500)",
			ts:       uint64(uint32(1000))<<32 | uint64(500),
			bigtime:  false,
			expected: time.Unix(1000, 500),
		},
		{
			name:     "legacy: negative seconds (sec=-1, nsec=0)",
			ts:       uint64(0xFFFFFFFF) << 32,
			bigtime:  false,
			expected: time.Unix(-1, 0),
		},
		{
			name:     "bigtime: epoch offset yields Unix epoch",
			ts:       uint64(XFS_BIGTIME_EPOCH_OFFSET) * 1_000_000_000,
			bigtime:  true,
			expected: time.Unix(0, 0),
		},
		{
			name:     "bigtime: known nanoseconds",
			ts:       uint64(XFS_BIGTIME_EPOCH_OFFSET)*1_000_000_000 + 1000*1_000_000_000 + 500,
			bigtime:  true,
			expected: time.Unix(1000, 500),
		},
		{
			// nsec precision: float64(9007199255000123) / 1e9 yields nsec=255000124,
			// but integer division correctly gives nsec=255000123.
			name:     "bigtime: nsec precision with integer division",
			ts:       9007199255000123,
			bigtime:  true,
			expected: time.Unix(9007199-XFS_BIGTIME_EPOCH_OFFSET, 255000123),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTimestamp(tt.ts, tt.bigtime)
			if !got.Equal(tt.expected) {
				t.Errorf("parseTimestamp(%d, %v) = %v, want %v", tt.ts, tt.bigtime, got, tt.expected)
			}
		})
	}
}
