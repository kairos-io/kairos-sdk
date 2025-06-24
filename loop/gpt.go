package loop

import (
	"encoding/binary"
	"fmt"
	"os"
)

const (
	sectorSize = 512
)

type Partition struct {
	Number     int
	Name       string
	FirstLBA   uint64
	LastLBA    uint64
	NumSectors uint64
}

func GetGPTPartitions(devicePath string) ([]Partition, error) {
	f, err := os.Open(devicePath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", devicePath, err)
	}
	defer f.Close()

	// Read GPT header at sector 1
	hdrBuf := make([]byte, sectorSize)
	if _, err := f.ReadAt(hdrBuf, sectorSize); err != nil {
		return nil, fmt.Errorf("reading GPT header: %w", err)
	}

	partitionEntryLBA := binary.LittleEndian.Uint64(hdrBuf[72:80])
	numPartitionEntries := binary.LittleEndian.Uint32(hdrBuf[80:84])
	sizeOfPartitionEntry := binary.LittleEndian.Uint32(hdrBuf[84:88])

	partitions := []Partition{}
	entryBuf := make([]byte, sizeOfPartitionEntry)

	for i := uint32(0); i < numPartitionEntries; i++ {
		offset := int64(partitionEntryLBA*sectorSize) + int64(i*sizeOfPartitionEntry)
		if _, err := f.ReadAt(entryBuf, offset); err != nil {
			return nil, fmt.Errorf("reading partition entry %d: %w", i+1, err)
		}

		firstLBA := binary.LittleEndian.Uint64(entryBuf[32:40])
		lastLBA := binary.LittleEndian.Uint64(entryBuf[40:48])

		if firstLBA == 0 && lastLBA == 0 {
			continue // Empty partition entry
		}

		nameBytes := entryBuf[56 : 56+72]
		name := decodeUTF16String(nameBytes)

		partitions = append(partitions, Partition{
			Number:     int(i + 1),
			Name:       name,
			FirstLBA:   firstLBA,
			LastLBA:    lastLBA,
			NumSectors: lastLBA - firstLBA + 1,
		})
	}

	return partitions, nil
}

// Helper to decode UTF-16LE partition names
func decodeUTF16String(b []byte) string {
	u16 := make([]uint16, 0, len(b)/2)
	for i := 0; i < len(b); i += 2 {
		ch := binary.LittleEndian.Uint16(b[i : i+2])
		if ch == 0x0000 {
			break
		}
		u16 = append(u16, ch)
	}
	return string(runeSlice(u16))
}

func runeSlice(u16 []uint16) []rune {
	r := make([]rune, len(u16))
	for i, u := range u16 {
		r[i] = rune(u)
	}
	return r
}
