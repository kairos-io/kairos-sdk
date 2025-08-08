package ghw

import (
	"fmt"
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

func isMultipathDevice(paths *Paths, entry os.DirEntry, logger *types.KairosLogger) bool {
	hasPrefix := strings.HasPrefix(entry.Name(), "dm-")
	if !hasPrefix {
		return false
	}

	// Check if the device has a "slaves" directory, which is a common indicator
	_, err := os.Stat(filepath.Join(paths.SysBlock, entry.Name(), "slaves"))
	if err != nil {
		var msg string
		if os.IsNotExist(err) {
			msg = "No slaves directory, not a multipath device"
		} else {
			msg = "Error checking slaves directory"
		}
		
		logger.Logger.Debug().Str("devNo", entry.Name()).Msg(msg)
		return false
	}

	// If the device has a "slaves" directory, we can check its udev info
	// to confirm it's a multipath device.
	// This is a more reliable check than just the name.
	udevInfo, err := udevInfoPartition(paths, entry.Name(), logger)
	if err != nil {
		logger.Logger.Error().Err(err).Str("devNo", entry.Name()).Msg("Failed to get udev info")
		return false
	}
	// Check if the udev info contains DM_NAME indicating it's a multipath device
	_, ok := udevInfo["DM_NAME"]
	if !ok {
		logger.Logger.Debug().Str("devNo", entry.Name()).Msg("Not a multipath device")
	}

	return ok
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
		var partitionHandler PartitionHandler;
		logger.Logger.Debug().Str("file", file.Name()).Msg("Reading file")
		dname := file.Name()
		size := diskSizeBytes(paths, dname, logger)

		// Skip entries that are multipath partitions
		// we will handle them when we parse this disks partitions
		if isMultipathPartition(file, paths, logger) {
			logger.Logger.Debug().Str("file", dname).Msg("Skipping multipath partition")
			continue
		}

		if strings.HasPrefix(dname, "loop") && size == 0 {
			// We don't care about unused loop devices...
			continue
		}
		d := &types.Disk{
			Name:      dname,
			SizeBytes: size,
			UUID:      diskUUID(paths, dname, logger),
		}

		if(isMultipathDevice(paths, file, logger)) {
			partitionHandler = NewMultipathPartitionHandler(dname)
		} else {
			partitionHandler = NewDiskPartitionHandler(dname)
		}
		

		parts := partitionHandler.GetPartitions(paths, logger)
		d.Partitions = parts

		disks = append(disks, d)
	}

	return disks
}

func isMultipathPartition(entry os.DirEntry, paths *Paths, logger *types.KairosLogger) bool {
    // Must be a dm device to be a multipath partition
    if !isMultipathDevice(paths, entry, logger) {
		return false
	}

	deviceName := entry.Name()
	udevInfo, err := udevInfoPartition(paths, deviceName, logger)
	if err != nil {
		logger.Logger.Error().Err(err).Str("devNo", deviceName).Msg("Failed to get udev info")
		return false
	}

	// Check if the udev info contains DM_PART indicating it's a partition
	// this is the primary check for multipath partitions and should be safe.
	_, ok := udevInfo["DM_PART"]
	return ok
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
