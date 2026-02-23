package utils

import (
	"bytes"
	"testing"
)

func TestNewSectorReader(t *testing.T) {
	tests := []struct {
		name       string
		sectorSize int
		wantErr    bool
	}{
		{
			name:       "512 bytes (minimum)",
			sectorSize: 512,
			wantErr:    false,
		},
		{
			name:       "1024 bytes",
			sectorSize: 1024,
			wantErr:    false,
		},
		{
			name:       "2048 bytes",
			sectorSize: 2048,
			wantErr:    false,
		},
		{
			name:       "4096 bytes",
			sectorSize: 4096,
			wantErr:    false,
		},
		{
			name:       "zero",
			sectorSize: 0,
			wantErr:    true,
		},
		{
			name:       "less than minimum (256)",
			sectorSize: 256,
			wantErr:    true,
		},
		{
			name:       "not multiple of 512 (1000)",
			sectorSize: 1000,
			wantErr:    true,
		},
		{
			name:       "negative",
			sectorSize: -1,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr, err := NewSectorReader(tt.sectorSize)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSectorReader(%d) error = %v, wantErr %v", tt.sectorSize, err, tt.wantErr)
				return
			}
			if err == nil && sr == nil {
				t.Errorf("NewSectorReader(%d) returned nil without error", tt.sectorSize)
			}
		})
	}
}

func TestSectorReaderReadSector(t *testing.T) {
	tests := []struct {
		name       string
		sectorSize int
		dataSize   int
		wantErr    bool
	}{
		{
			name:       "read 512 bytes",
			sectorSize: 512,
			dataSize:   512,
			wantErr:    false,
		},
		{
			name:       "read 1024 bytes",
			sectorSize: 1024,
			dataSize:   1024,
			wantErr:    false,
		},
		{
			name:       "read 4096 bytes",
			sectorSize: 4096,
			dataSize:   4096,
			wantErr:    false,
		},
		{
			name:       "insufficient data for 1024",
			sectorSize: 1024,
			dataSize:   512,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr, err := NewSectorReader(tt.sectorSize)
			if err != nil {
				t.Fatalf("NewSectorReader(%d) unexpected error: %v", tt.sectorSize, err)
			}

			data := make([]byte, tt.dataSize)
			for i := range data {
				data[i] = byte(i % 256)
			}
			r := bytes.NewReader(data)

			got, err := sr.ReadSector(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadSector() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if len(got) != tt.sectorSize {
					t.Errorf("ReadSector() returned %d bytes, want %d", len(got), tt.sectorSize)
				}
				for i := 0; i < len(got); i++ {
					if got[i] != byte(i%256) {
						t.Errorf("ReadSector() byte[%d] = %d, want %d", i, got[i], byte(i%256))
						break
					}
				}
			}
		})
	}
}

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
