package loop

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"encoding/binary"
)

const (
	sectorSize       = 512
	DM_IOCTL         = 0xfd
	DM_DEVICE_CREATE = 0
	DM_TABLE_LOAD    = 3
	DM_DEV_ARM       = 5

	DM_NAME_LEN = 128
	DM_UUID_LEN = 129

	DM_IOCTL_BASE = (2 << 30) | (unsafe.Sizeof(dmIoctl{}) << 16) | (DM_IOCTL << 8)
)

type dmIoctl struct {
	Version     [3]uint32
	DataSize    uint32
	DataStart   uint32
	TargetCount uint32
	OpenCount   int32
	Flags       uint32
	EventNr     uint32
	Padding     uint32
	Dev         uint64
	Name        [DM_NAME_LEN]byte
	Uuid        [DM_UUID_LEN]byte
	Data        [7]byte
}

type dmTargetSpec struct {
	SectorStart uint64
	Length      uint64
	TargetType  [16]byte
	Next        uint32
	_           uint32
}

type GPTHeader struct {
	Signature      [8]byte
	Revision       uint32
	HeaderSize     uint32
	CRC32          uint32
	_              uint32 // reserved
	MyLBA          uint64
	AlternateLBA   uint64
	FirstUsableLBA uint64
	LastUsableLBA  uint64
	DiskGUID       [16]byte
	PartEntryLBA   uint64
	NumPartEntries uint32
	PartEntrySize  uint32
	PartEntryCRC32 uint32
	_              [420]byte // padding to 512 bytes
}

type GPTEntry struct {
	TypeGUID [16]byte
	PartGUID [16]byte
	FirstLBA uint64
	LastLBA  uint64
	Attrs    uint64
	Name     [72]byte // UTF-16
}

func ioctl(fd uintptr, req uintptr, arg uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, arg)
	if errno != 0 {
		return errno
	}
	return nil
}

func CreateMappingsFromImage(loopDevice string) ([]string, error) {
	var created []string

	control, err := os.OpenFile("/dev/mapper/control", os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open /dev/mapper/control: %w", err)
	}
	defer control.Close()

	img, err := os.Open(loopDevice)
	if err != nil {
		return nil, fmt.Errorf("open loop device: %w", err)
	}
	defer img.Close()

	// Seek GPT header (LBA 1)
	_, err = img.Seek(512, 0)
	if err != nil {
		return nil, fmt.Errorf("seek gpt: %w", err)
	}

	var gpt GPTHeader
	if err := binary.Read(img, binary.LittleEndian, &gpt); err != nil {
		return nil, fmt.Errorf("read gpt: %w", err)
	}
	if string(gpt.Signature[:]) != "EFI PART" {
		return nil, fmt.Errorf("invalid gpt signature")
	}

	// Seek GPT entries
	_, err = img.Seek(int64(gpt.PartEntryLBA*512), 0)
	if err != nil {
		return nil, fmt.Errorf("seek entries: %w", err)
	}

	for i := 0; i < int(gpt.NumPartEntries); i++ {
		var entry GPTEntry
		if err := binary.Read(img, binary.LittleEndian, &entry); err != nil {
			break
		}
		if entry.FirstLBA == 0 || entry.LastLBA == 0 {
			continue
		}

		start := entry.FirstLBA
		size := entry.LastLBA - entry.FirstLBA + 1
		name := fmt.Sprintf("gptp%d", i+1)

		fmt.Printf("Partition %s: start=%d sectors, size=%d sectors\n", name, start, size)

		// DM_DEVICE_CREATE
		var create dmIoctl
		create.Version = [3]uint32{4, 0, 0}
		create.DataSize = uint32(unsafe.Sizeof(create))
		create.DataStart = uint32(unsafe.Offsetof(create.Data))
		copy(create.Name[:], name)
		if err := ioctl(control.Fd(), DM_IOCTL_BASE|DM_DEVICE_CREATE, uintptr(unsafe.Pointer(&create))); err != nil {
			return created, fmt.Errorf("DM_DEVICE_CREATE %s: %w", name, err)
		}

		// DM_TABLE_LOAD
		params := fmt.Sprintf("%s %d\x00", loopDevice, start)
		specLen := uint32(unsafe.Sizeof(dmTargetSpec{}))
		totalSize := uint32(unsafe.Sizeof(dmIoctl{})) + specLen + uint32(len(params))

		data := make([]byte, totalSize)
		io := (*dmIoctl)(unsafe.Pointer(&data[0]))
		io.Version = [3]uint32{4, 0, 0}
		io.DataSize = totalSize
		io.DataStart = uint32(unsafe.Offsetof(io.Data))
		io.TargetCount = 1
		copy(io.Name[:], name)

		specOffset := uintptr(unsafe.Offsetof(io.Data))
		spec := (*dmTargetSpec)(unsafe.Pointer(uintptr(unsafe.Pointer(io)) + specOffset))
		spec.SectorStart = 0
		spec.Length = size
		copy(spec.TargetType[:], []byte("linear"))
		spec.Next = specLen + uint32(len(params))

		paramOffset := int(specOffset + uintptr(specLen))
		copy(data[paramOffset:], []byte(params))

		if err := ioctl(control.Fd(), DM_IOCTL_BASE|DM_TABLE_LOAD, uintptr(unsafe.Pointer(io))); err != nil {
			return created, fmt.Errorf("DM_TABLE_LOAD %s: %w", name, err)
		}

		if err := ioctl(control.Fd(), DM_IOCTL_BASE|DM_DEV_ARM, uintptr(unsafe.Pointer(io))); err != nil {
			return created, fmt.Errorf("DM_DEV_ARM %s: %w", name, err)
		}

		created = append(created, "/dev/mapper/"+name)
	}

	return created, nil
}
