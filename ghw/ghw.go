package ghw

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kairos-io/kairos-sdk/types"
)

const (
	sectorSize = 512
	UNKNOWN    = "unknown"
)

type Paths struct {
	SysBlock    string
	RunUdevData string
	ProcMounts  string
}

func NewPaths(withOptionalPrefix string) *Paths {
	p := &Paths{
		SysBlock:    "/sys/block/",
		RunUdevData: "/run/udev/data",
		ProcMounts:  "/proc/mounts",
	}

	// Allow overriding the paths via env var. It has precedence over anything
	val, exists := os.LookupEnv("GHW_CHROOT")
	if exists {
		val = strings.TrimSuffix(val, "/")
		p.SysBlock = fmt.Sprintf("%s%s", val, p.SysBlock)
		p.RunUdevData = fmt.Sprintf("%s%s", val, p.RunUdevData)
		p.ProcMounts = fmt.Sprintf("%s%s", val, p.ProcMounts)
		return p
	}

	if withOptionalPrefix != "" {
		withOptionalPrefix = strings.TrimSuffix(withOptionalPrefix, "/")
		p.SysBlock = fmt.Sprintf("%s%s", withOptionalPrefix, p.SysBlock)
		p.RunUdevData = fmt.Sprintf("%s%s", withOptionalPrefix, p.RunUdevData)
		p.ProcMounts = fmt.Sprintf("%s%s", withOptionalPrefix, p.ProcMounts)
	}
	return p
}

func GetDisks(paths *Paths, logger *types.KairosLogger) []*types.Disk {
	if logger == nil {
		newLogger := types.NewKairosLogger("ghw", "info", false)
		logger = &newLogger
	}
	disks := make([]*types.Disk, 0)
	logger.Logger.Debug().Str("path", paths.SysBlock).Msg("Scanning for disks")
	files, err := os.ReadDir(paths.SysBlock)
	if err != nil {
		return nil
	}
	for _, file := range files {
		logger.Logger.Debug().Str("file", file.Name()).Msg("Reading file")
		dname := file.Name()
		size := diskSizeBytes(paths, dname, logger)

		if strings.HasPrefix(dname, "loop") && size == 0 {
			// We don't care about unused loop devices...
			continue
		}
		d := &types.Disk{
			Name:      dname,
			SizeBytes: size,
			UUID:      diskUUID(paths, dname, "", logger),
		}

		parts := diskPartitions(paths, dname, logger)
		d.Partitions = parts

		disks = append(disks, d)
	}

	return disks
}

func diskSizeBytes(paths *Paths, disk string, logger *types.KairosLogger) uint64 {
	// We can find the number of 512-byte sectors by examining the contents of
	// /sys/block/$DEVICE/size and calculate the physical bytes accordingly.
	path := filepath.Join(paths.SysBlock, disk, "size")
	logger.Logger.Debug().Str("path", path).Msg("Reading disk size")
	contents, err := os.ReadFile(path)
	if err != nil {
		logger.Logger.Error().Str("path", path).Err(err).Msg("Failed to read file")
		return 0
	}
	size, err := strconv.ParseUint(strings.TrimSpace(string(contents)), 10, 64)
	if err != nil {
		logger.Logger.Error().Str("path", path).Err(err).Str("content", string(contents)).Msg("Failed to parse size")
		return 0
	}
	logger.Logger.Trace().Uint64("size", size*sectorSize).Msg("Got disk size")
	return size * sectorSize
}

// diskPartitions takes the name of a disk (note: *not* the path of the disk,
// but just the name. In other words, "sda", not "/dev/sda" and "nvme0n1" not
// "/dev/nvme0n1") and returns a slice of pointers to Partition structs
// representing the partitions in that disk
func diskPartitions(paths *Paths, disk string, logger *types.KairosLogger) types.PartitionList {
	out := make(types.PartitionList, 0)
	path := filepath.Join(paths.SysBlock, disk)
	logger.Logger.Debug().Str("file", path).Msg("Reading disk file")
	files, err := os.ReadDir(path)
	if err != nil {
		logger.Logger.Error().Err(err).Msg("failed to read disk partitions")
		return out
	}
	for _, file := range files {
		fname := file.Name()
		if !strings.HasPrefix(fname, disk) {
			continue
		}
		logger.Logger.Debug().Str("file", fname).Msg("Reading partition file")
		size := partitionSizeBytes(paths, disk, fname, logger)
		mp, pt := partitionInfo(paths, fname, logger)
		du := diskPartUUID(paths, disk, fname, logger)
		if pt == "" {
			pt = diskPartTypeUdev(paths, disk, fname, logger)
		}
		fsLabel := diskFSLabel(paths, disk, fname, logger)
		p := &types.Partition{
			Name:            fname,
			Size:            uint(size / (1024 * 1024)),
			MountPoint:      mp,
			UUID:            du,
			FilesystemLabel: fsLabel,
			FS:              pt,
			Path:            filepath.Join("/dev", fname),
			Disk:            filepath.Join("/dev", disk),
		}
		out = append(out, p)
	}
	return out
}

func partitionSizeBytes(paths *Paths, disk string, part string, logger *types.KairosLogger) uint64 {
	path := filepath.Join(paths.SysBlock, disk, part, "size")
	logger.Logger.Debug().Str("file", path).Msg("Reading size file")
	contents, err := os.ReadFile(path)
	if err != nil {
		logger.Logger.Error().Str("file", path).Err(err).Msg("failed to read disk partition size")
		return 0
	}
	size, err := strconv.ParseUint(strings.TrimSpace(string(contents)), 10, 64)
	if err != nil {
		logger.Logger.Error().Str("contents", string(contents)).Err(err).Msg("failed to parse disk partition size")
		return 0
	}
	logger.Logger.Trace().Str("disk", disk).Str("partition", part).Uint64("size", size*sectorSize).Msg("Got partition size")
	return size * sectorSize
}

func partitionInfo(paths *Paths, part string, logger *types.KairosLogger) (string, string) {
	// Allow calling PartitionInfo with either the full partition name
	// "/dev/sda1" or just "sda1"
	if !strings.HasPrefix(part, "/dev") {
		part = "/dev/" + part
	}

	// mount entries for mounted partitions look like this:
	// /dev/sda6 / ext4 rw,relatime,errors=remount-ro,data=ordered 0 0
	var r io.ReadCloser
	logger.Logger.Debug().Str("file", paths.ProcMounts).Msg("Reading mounts file")
	r, err := os.Open(paths.ProcMounts)
	if err != nil {
		logger.Logger.Error().Str("file", paths.ProcMounts).Err(err).Msg("failed to open mounts")
		return "", ""
	}
	defer r.Close()

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		logger.Logger.Trace().Str("line", line).Msg("Parsing mount info")
		entry := parseMountEntry(line, logger)
		if entry == nil || entry.Partition != part {
			continue
		}

		return entry.Mountpoint, entry.FilesystemType
	}
	return "", ""
}

type mountEntry struct {
	Partition      string
	Mountpoint     string
	FilesystemType string
}

func parseMountEntry(line string, logger *types.KairosLogger) *mountEntry {
	// mount entries for mounted partitions look like this:
	// /dev/sda6 / ext4 rw,relatime,errors=remount-ro,data=ordered 0 0
	if line[0] != '/' {
		return nil
	}
	fields := strings.Fields(line)

	if len(fields) < 4 {
		logger.Logger.Debug().Interface("fields", fields).Msg("Mount line has less than 4 fields")
		return nil
	}

	// We do some special parsing of the mountpoint, which may contain space,
	// tab and newline characters, encoded into the mount entry line using their
	// octal-to-string representations. From the GNU mtab man pages:
	//
	//   "Therefore these characters are encoded in the files and the getmntent
	//   function takes care of the decoding while reading the entries back in.
	//   '\040' is used to encode a space character, '\011' to encode a tab
	//   character, '\012' to encode a newline character, and '\\' to encode a
	//   backslash."
	mp := fields[1]
	r := strings.NewReplacer(
		"\\011", "\t", "\\012", "\n", "\\040", " ", "\\\\", "\\",
	)
	mp = r.Replace(mp)

	res := &mountEntry{
		Partition:      fields[0],
		Mountpoint:     mp,
		FilesystemType: fields[2],
	}
	return res
}

func diskUUID(paths *Paths, disk string, partition string, logger *types.KairosLogger) string {
	info, err := udevInfoPartition(paths, disk, partition, logger)
	logger.Logger.Trace().Interface("info", info).Msg("Disk UUID")
	if err != nil {
		logger.Logger.Error().Str("disk", disk).Str("partition", partition).Interface("info", info).Err(err).Msg("failed to read disk UUID")
		return UNKNOWN
	}

	if pType, ok := info["ID_PART_TABLE_UUID"]; ok {
		logger.Logger.Trace().Str("disk", disk).Str("partition", partition).Str("uuid", pType).Msg("Got disk uuid")
		return pType
	}

	return UNKNOWN
}

func diskPartUUID(paths *Paths, disk string, partition string, logger *types.KairosLogger) string {
	info, err := udevInfoPartition(paths, disk, partition, logger)
	logger.Logger.Trace().Interface("info", info).Msg("Disk Part UUID")
	if err != nil {
		logger.Logger.Error().Str("disk", disk).Str("partition", partition).Interface("info", info).Err(err).Msg("Disk Part UUID")
		return UNKNOWN
	}

	if pType, ok := info["ID_PART_ENTRY_UUID"]; ok {
		logger.Logger.Trace().Str("disk", disk).Str("partition", partition).Str("uuid", pType).Msg("Got partition uuid")
		return pType
	}
	return UNKNOWN
}

// diskPartTypeUdev gets the partition type from the udev database directly and its only used as fallback when
// the partition is not mounted, so we cannot get the type from paths.ProcMounts from the partitionInfo function
func diskPartTypeUdev(paths *Paths, disk string, partition string, logger *types.KairosLogger) string {
	info, err := udevInfoPartition(paths, disk, partition, logger)
	logger.Logger.Trace().Interface("info", info).Msg("Disk Part Type")
	if err != nil {
		logger.Logger.Error().Str("disk", disk).Str("partition", partition).Interface("info", info).Err(err).Msg("Disk Part Type")
		return UNKNOWN
	}

	if pType, ok := info["ID_FS_TYPE"]; ok {
		logger.Logger.Trace().Str("disk", disk).Str("partition", partition).Str("FS", pType).Msg("Got partition fs type")
		return pType
	}
	return UNKNOWN
}

func diskFSLabel(paths *Paths, disk string, partition string, logger *types.KairosLogger) string {
	info, err := udevInfoPartition(paths, disk, partition, logger)
	logger.Logger.Trace().Interface("info", info).Msg("Disk FS label")
	if err != nil {
		logger.Logger.Error().Str("disk", disk).Str("partition", partition).Interface("info", info).Err(err).Msg("Disk FS label")
		return UNKNOWN
	}

	if label, ok := info["ID_FS_LABEL"]; ok {
		logger.Logger.Trace().Str("disk", disk).Str("partition", partition).Str("uuid", label).Msg("Got partition label")
		return label
	}
	return UNKNOWN
}

func udevInfoPartition(paths *Paths, disk string, partition string, logger *types.KairosLogger) (map[string]string, error) {
	// Get device major:minor numbers
	devNo, err := os.ReadFile(filepath.Join(paths.SysBlock, disk, partition, "dev"))
	if err != nil {
		logger.Logger.Error().Err(err).Str("path", filepath.Join(paths.SysBlock, disk, partition, "dev")).Msg("failed to read udev info")
		return nil, err
	}
	return UdevInfo(paths, string(devNo), logger)
}

// UdevInfo will return information on udev database about a device number
func UdevInfo(paths *Paths, devNo string, logger *types.KairosLogger) (map[string]string, error) {
	// Look up block device in udev runtime database
	udevID := "b" + strings.TrimSpace(devNo)
	udevBytes, err := os.ReadFile(filepath.Join(paths.RunUdevData, udevID))
	if err != nil {
		logger.Logger.Error().Err(err).Str("path", filepath.Join(paths.RunUdevData, udevID)).Msg("failed to read udev info for device")
		return nil, err
	}

	udevInfo := make(map[string]string)
	for _, udevLine := range strings.Split(string(udevBytes), "\n") {
		if strings.HasPrefix(udevLine, "E:") {
			if s := strings.SplitN(udevLine[2:], "=", 2); len(s) == 2 {
				udevInfo[s[0]] = s[1]
			}
		}
	}
	return udevInfo, nil
}
