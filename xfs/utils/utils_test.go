package utils

import (
	"bytes"
	"testing"
)

func TestReadBlockN(t *testing.T) {
	tests := []struct {
		name      string
		blockSize int
		dataSize  int
		wantErr   bool
	}{
		{
			name:      "512 byte block",
			blockSize: 512,
			dataSize:  512,
			wantErr:   false,
		},
		{
			name:      "4096 byte block",
			blockSize: 4096,
			dataSize:  4096,
			wantErr:   false,
		},
		{
			name:      "65536 byte block",
			blockSize: 65536,
			dataSize:  65536,
			wantErr:   false,
		},
		{
			name:      "insufficient data",
			blockSize: 4096,
			dataSize:  1000,
			wantErr:   true,
		},
		{
			name:      "invalid block size not multiple of sector",
			blockSize: 1000,
			dataSize:  1000,
			wantErr:   true,
		},
		{
			name:      "zero block size",
			blockSize: 0,
			dataSize:  0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, tt.dataSize)
			// Fill with pattern for verification
			for i := range data {
				data[i] = byte(i % 256)
			}
			r := bytes.NewReader(data)

			got, err := ReadBlockN(r, tt.blockSize)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadBlockN() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if len(got) != tt.blockSize {
					t.Errorf("ReadBlockN() returned %d bytes, want %d", len(got), tt.blockSize)
				}
				// Verify content
				for i := 0; i < len(got); i++ {
					if got[i] != byte(i%256) {
						t.Errorf("ReadBlockN() byte[%d] = %d, want %d", i, got[i], byte(i%256))
						break
					}
				}
			}
		})
	}
}
