package loop

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"
)

const (
	sectorSize = 512

	DM_IOCTL         = 0xfd
	DM_DEVICE_CREATE = 0
	DM_TABLE_LOAD    = 3
	DM_DEV_ARM       = 5
	DM_DEV_REMOVE    = 1
	DM_DEV_STATUS    = 4 // Added status operation for fetching device info

	DM_NAME_LEN = 128
	DM_UUID_LEN = 129

	DM_READONLY_FLAG       = (1 << 0)
	DM_SUSPEND_FLAG        = (1 << 1)
	DM_PERSISTENT_DEV_FLAG = (1 << 3)

	DM_VERSION_MAJOR      = 4
	DM_VERSION_MINOR      = 0
	DM_VERSION_PATCHLEVEL = 0
)

// dmOperations maps operation codes to human-readable names for error reporting
var dmOperations = map[uint]string{
	DM_DEVICE_CREATE: "device create",
	DM_TABLE_LOAD:    "table load",
	DM_DEV_ARM:       "device arm",
	DM_DEV_REMOVE:    "device remove",
	DM_DEV_STATUS:    "device status",
}

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
}

type dmTargetSpec struct {
	SectorStart uint64
	Length      uint64
	Status      int32
	Next        uint32
	TargetType  [16]byte
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

// DMError represents a device mapper operation error with context
type DMError struct {
	Op  uint
	Err error
}

func (e *DMError) Error() string {
	opName := "unknown"
	if name, ok := dmOperations[e.Op]; ok {
		opName = name
	}
	return fmt.Sprintf("device-mapper %s: %v", opName, e.Err)
}

// Helper: get major:minor string for a device
func getMajorMinor(dev string) (string, error) {
	st := syscall.Stat_t{}
	if err := syscall.Stat(dev, &st); err != nil {
		return "", err
	}
	major := (st.Rdev >> 8) & 0xfff
	minor := (st.Rdev & 0xff) | ((st.Rdev >> 12) & 0xfff00)
	return fmt.Sprintf("%d:%d", major, minor), nil
}

// Helper: perform a device-mapper ioctl with debug
func dmIoctlCall(fd uintptr, cmd uintptr, data []byte) error {
	if dmDebug {
		fmt.Printf("[DM-IOCTL] fd=%d cmd=0x%x len=%d\n", fd, cmd, len(data))
		fmt.Printf("[DM-IOCTL] buffer (first 128 bytes): %s\n", hex.EncodeToString(data[:min(128, len(data))]))
		if len(data) >= int(unsafe.Sizeof(dmIoctl{})) {
			hdr := (*dmIoctl)(unsafe.Pointer(&data[0]))
			fmt.Printf("[DM-IOCTL] Version=%d.%d.%d DataSize=%d DataStart=%d TargetCount=%d Flags=0x%x Name='%s'\n",
				hdr.Version[0], hdr.Version[1], hdr.Version[2], hdr.DataSize, hdr.DataStart, hdr.TargetCount, hdr.Flags, string(hdr.Name[:]))
		}
	}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, cmd, uintptr(unsafe.Pointer(&data[0])))
	if errno != 0 {
		if dmDebug {
			fmt.Printf("[DM-IOCTL] ioctl failed: errno=%d (%s)\n", errno, errno.Error())
		}
		return &DMError{Op: uint(cmd & 0xFF), Err: errno}
	}
	if dmDebug {
		fmt.Printf("[DM-IOCTL] ioctl succeeded\n")
	}
	return nil
}

// dmDebug controls whether to print debug information
var dmDebug = true

// SetDMDebug enables or disables device-mapper debug output
func SetDMDebug(enabled bool) {
	dmDebug = enabled
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Helper: calculate ioctl number (matches _IOWR macro in kernel)
func dmIoctlNum(cmd uint32) uintptr {
	// _IOWR(0xfd, cmd, struct dm_ioctl) = 0xc138fd00 | cmd
	return 0xc138fd00 | uintptr(cmd)
}

const DM_BUFFER_SIZE = 16384 // 16 KiB, same as dmsetup

// Helper: build buffer for DM_DEVICE_CREATE
func buildCreateBuf(name string) []byte {
	sz := unsafe.Sizeof(dmIoctl{})
	buf := make([]byte, DM_BUFFER_SIZE)
	hdr := (*dmIoctl)(unsafe.Pointer(&buf[0]))
	hdr.Version = [3]uint32{DM_VERSION_MAJOR, DM_VERSION_MINOR, DM_VERSION_PATCHLEVEL}
	hdr.DataSize = DM_BUFFER_SIZE
	hdr.DataStart = uint32(sz)
	hdr.Flags = DM_PERSISTENT_DEV_FLAG
	copy(hdr.Name[:], name)
	return buf
}

// Helper: build buffer for DM_TABLE_LOAD (dm_ioctl + dm_target_spec + params)
func buildTableBuf(name string, size, start uint64, majorMinor string) []byte {
	const alignment = 8

	ioSz := unsafe.Sizeof(dmIoctl{})
	tsSz := unsafe.Sizeof(dmTargetSpec{})

	// Device-mapper linear target parameters: "major:minor start_sector"
	params := fmt.Sprintf("%s %d", majorMinor, start)
	paramsBytes := append([]byte(params), 0) // null-terminated

	// Compute unaligned total
	total := ioSz + tsSz + uintptr(len(paramsBytes))
	// Align total size to 8-byte boundary (dm_target_spec requirement)
	aligned := ((total + alignment - 1) / alignment) * alignment

	buf := make([]byte, aligned)

	// Fill dm_ioctl header
	hdr := (*dmIoctl)(unsafe.Pointer(&buf[0]))
	hdr.Version = [3]uint32{DM_VERSION_MAJOR, DM_VERSION_MINOR, DM_VERSION_PATCHLEVEL}
	hdr.DataSize = uint32(aligned)
	hdr.DataStart = uint32(ioSz)
	hdr.TargetCount = 1
	hdr.Flags = DM_PERSISTENT_DEV_FLAG
	copy(hdr.Name[:], name)

	// Fill dm_target_spec
	tsOffset := uintptr(hdr.DataStart)
	ts := (*dmTargetSpec)(unsafe.Pointer(uintptr(unsafe.Pointer(&buf[0])) + tsOffset))
	ts.SectorStart = 0
	ts.Length = size
	copy(ts.TargetType[:], "linear")

	paramStart := uint32(ioSz + tsSz)
	copy(buf[paramStart:], paramsBytes)

	// ✅ Set .Next to the offset of the next dm_target_spec (even if none follows)
	// This is the offset from the beginning of the target_spec to the next one,
	// which is required even if there's only one target.
	nextOffset := ((paramStart + uint32(len(paramsBytes)) + alignment - 1) / alignment) * alignment
	ts.Next = nextOffset - uint32(tsOffset) // relative offset from this target_spec

	return buf
}

const DM_EXISTS_FLAG = (1 << 6) // Add this

func runDmsetupInfo(name string) {
	cmd := exec.Command("dmsetup", "info", name)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		fmt.Printf("[DM-INFO] dmsetup info %s failed: %v\nstderr: %s\n", name, err, stderr.String())
	} else {
		fmt.Printf("[DM-INFO] dmsetup info %s:\n%s\n", name, out.String())
	}
}

func buildArmBuf(name string) []byte {
	buf := make([]byte, DM_BUFFER_SIZE)
	hdr := (*dmIoctl)(unsafe.Pointer(&buf[0]))
	hdr.Version = [3]uint32{DM_VERSION_MAJOR, DM_VERSION_MINOR, DM_VERSION_PATCHLEVEL}
	hdr.DataSize = DM_BUFFER_SIZE
	hdr.Flags = DM_EXISTS_FLAG | DM_PERSISTENT_DEV_FLAG // <-- key fix!
	copy(hdr.Name[:], name)
	return buf
}

// Helper: build buffer for device status
func buildStatusBuf(name string) []byte {
	sz := unsafe.Sizeof(dmIoctl{})
	buf := make([]byte, sz)
	hdr := (*dmIoctl)(unsafe.Pointer(&buf[0]))
	hdr.Version = [3]uint32{DM_VERSION_MAJOR, DM_VERSION_MINOR, DM_VERSION_PATCHLEVEL}
	hdr.DataSize = uint32(sz)
	hdr.DataStart = uint32(sz)
	copy(hdr.Name[:], name)
	return buf
}

// getDeviceInfo gets the device info including the device ID
func getDeviceInfo(control *os.File, name string) (dmIoctl, error) {
	if dmDebug {
		fmt.Printf("[DM] Getting device info for %s\n", name)
	}

	buf := buildStatusBuf(name)
	if err := dmIoctlCall(control.Fd(), dmIoctlNum(DM_DEV_STATUS), buf); err != nil {
		if dmDebug {
			fmt.Printf("[DM] Failed to get device info for %s: %v\n", name, err)
		}
		return dmIoctl{}, err
	}

	hdr := (*dmIoctl)(unsafe.Pointer(&buf[0]))
	if dmDebug {
		fmt.Printf("[DM] Got device info for %s, dev=%d\n", name, hdr.Dev)
	}
	return *hdr, nil
}

// openDeviceMapper opens the device-mapper control device
func openDeviceMapper() (*os.File, error) {
	if dmDebug {
		fmt.Printf("[DM] Opening /dev/mapper/control\n")
	}
	control, err := os.OpenFile("/dev/mapper/control", os.O_RDWR, 0)
	if err != nil {
		if dmDebug {
			fmt.Printf("[DM] Failed to open /dev/mapper/control: %v\n", err)
		}
		return nil, fmt.Errorf("open /dev/mapper/control: %w", err)
	}
	return control, nil
}

// removeDevice removes a device-mapper device by name
func removeDevice(control *os.File, name string) error {
	if dmDebug {
		fmt.Printf("[DM] DM_DEV_REMOVE for %s\n", name)
	}

	createBuf := buildCreateBuf(name)
	if err := dmIoctlCall(control.Fd(), dmIoctlNum(DM_DEV_REMOVE), createBuf); err != nil {
		if dmDebug {
			fmt.Printf("[DM] DM_DEV_REMOVE failed for %s: %v\n", name, err)
		}
		return err
	}

	if dmDebug {
		fmt.Printf("[DM] DM_DEV_REMOVE succeeded for %s\n", name)
	}
	return nil
}

// CreateDeviceMapping creates a single device-mapper device with the specified name, size and offset
func CreateDeviceMapping(control *os.File, name string, size, start uint64, majorMinor string) error {
	// 1. Create device
	if dmDebug {
		fmt.Printf("[DM] DM_DEVICE_CREATE for %s\n", name)
	}
	createBuf := buildCreateBuf(name)
	if err := dmIoctlCall(control.Fd(), dmIoctlNum(DM_DEVICE_CREATE), createBuf); err != nil {
		if dmDebug {
			fmt.Printf("[DM] DM_DEVICE_CREATE failed for %s: %v\n", name, err)
		}
		return fmt.Errorf("DM_DEVICE_CREATE %s: %w", name, err)
	}
	if dmDebug {
		fmt.Printf("[DM] DM_DEVICE_CREATE succeeded for %s\n", name)
	}

	// Handle cleanup on failure
	var success bool
	defer func() {
		if !success && control != nil {
			_ = removeDevice(control, name)
		}
	}()

	// 2. Table load
	if dmDebug {
		fmt.Printf("[DM] DM_TABLE_LOAD for %s\n", name)
	}
	tableBuf := buildTableBuf(name, size, start, majorMinor)
	if err := dmIoctlCall(control.Fd(), dmIoctlNum(DM_TABLE_LOAD), tableBuf); err != nil {
		if dmDebug {
			fmt.Printf("[DM] DM_TABLE_LOAD failed for %s: %v\n", name, err)
		}
		return fmt.Errorf("DM_TABLE_LOAD %s: %w", name, err)
	}
	if dmDebug {
		fmt.Printf("[DM] DM_TABLE_LOAD succeeded for %s\n", name)
	}

	runDmsetupInfo(name)

	// 3. Arm - Use the device info to build the arm buffer
	if dmDebug {
		fmt.Printf("[DM] DM_DEV_ARM for %s\n", name)
	}

	// Use the buildArmBuf function with the device ID
	armBuf := buildArmBuf(name)

	if err := dmIoctlCall(control.Fd(), dmIoctlNum(DM_DEV_ARM), armBuf); err != nil {
		if dmDebug {
			fmt.Printf("[DM] DM_DEV_ARM failed for %s: %v\n", name, err)
		}
		return fmt.Errorf("DM_DEV_ARM %s: %w", name, err)
	}
	if dmDebug {
		fmt.Printf("[DM] DM_DEV_ARM succeeded for %s\n", name)
	}

	success = true
	return nil
}

func CreateMappingsFromImage(loopDevice string) ([]string, error) {
	var created []string

	control, err := openDeviceMapper()
	if err != nil {
		return nil, err
	}
	defer control.Close()

	if dmDebug {
		fmt.Printf("[DM] Getting major:minor for %s\n", loopDevice)
	}
	majorMinor, err := getMajorMinor(loopDevice)
	if err != nil {
		if dmDebug {
			fmt.Printf("[DM] Failed to get major:minor: %v\n", err)
		}
		return nil, fmt.Errorf("get major:minor: %w", err)
	}
	if dmDebug {
		fmt.Printf("[DM] major:minor = %s\n", majorMinor)
	}

	img, err := os.Open(loopDevice)
	if err != nil {
		if dmDebug {
			fmt.Printf("[DM] Failed to open loop device: %v\n", err)
		}
		return nil, fmt.Errorf("open loop device: %w", err)
	}
	defer img.Close()

	if dmDebug {
		fmt.Printf("[DM] Seeking GPT header\n")
	}
	_, err = img.Seek(512, 0)
	if err != nil {
		if dmDebug {
			fmt.Printf("[DM] Failed to seek gpt: %v\n", err)
		}
		return nil, fmt.Errorf("seek gpt: %w", err)
	}

	var gpt GPTHeader
	if err := binary.Read(img, binary.LittleEndian, &gpt); err != nil {
		if dmDebug {
			fmt.Printf("[DM] Failed to read gpt: %v\n", err)
		}
		return nil, fmt.Errorf("read gpt: %w", err)
	}

	if dmDebug {
		fmt.Printf("[DM] GPT signature: %q\n", gpt.Signature[:])
	}
	if string(gpt.Signature[:]) != "EFI PART" {
		if dmDebug {
			fmt.Printf("[DM] Invalid GPT signature\n")
		}
		return nil, fmt.Errorf("invalid gpt signature")
	}

	if dmDebug {
		fmt.Printf("[DM] Seeking GPT entries at LBA %d\n", gpt.PartEntryLBA)
	}
	_, err = img.Seek(int64(gpt.PartEntryLBA*512), 0)
	if err != nil {
		if dmDebug {
			fmt.Printf("[DM] Failed to seek entries: %v\n", err)
		}
		return nil, fmt.Errorf("seek entries: %w", err)
	}

	for i := 0; i < int(gpt.NumPartEntries); i++ {
		var entry GPTEntry
		if err := binary.Read(img, binary.LittleEndian, &entry); err != nil {
			if dmDebug {
				fmt.Printf("[DM] Failed to read GPT entry %d: %v\n", i, err)
			}
			break
		}

		if entry.FirstLBA == 0 || entry.LastLBA == 0 {
			continue
		}

		start := entry.FirstLBA
		size := entry.LastLBA - entry.FirstLBA + 1
		name := fmt.Sprintf("%sp%d", filepath.Base(loopDevice), i+1)

		if dmDebug {
			fmt.Printf("[DM] Partition %s: start=%d size=%d\n", name, start, size)
		}

		err = CreateDeviceMapping(control, name, size, start, majorMinor)
		if err != nil {
			// Clean up already created mappings
			for _, devPath := range created {
				devName := filepath.Base(devPath)
				_ = removeDevice(control, devName)
			}
			return created, err
		}

		created = append(created, "/dev/mapper/"+name)
	}

	if dmDebug {
		fmt.Printf("[DM] Created %d device-mapper mappings\n", len(created))
	}
	return created, nil
}

// CleanupMappings removes all device-mapper mappings created for a loop device
func CleanupMappings(mappings []string) error {
	control, err := openDeviceMapper()
	if err != nil {
		return err
	}
	defer control.Close()

	var lastErr error
	for _, devPath := range mappings {
		devName := filepath.Base(devPath)
		if err := removeDevice(control, devName); err != nil {
			if dmDebug {
				fmt.Printf("[DM] Failed to remove mapping %s: %v\n", devName, err)
			}
			lastErr = err
		}
	}

	return lastErr
}
