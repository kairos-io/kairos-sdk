package ghw

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kairos-io/kairos-sdk/types"
)

type DiskPartitionHandler struct {
	DiskName string
}

// Validate that DiskPartitionHandler implements PartitionHandler interface.
var _ PartitionHandler = &DiskPartitionHandler{}

func NewDiskPartitionHandler(diskName string) *DiskPartitionHandler {
	return &DiskPartitionHandler{DiskName: diskName}
}

func (d *DiskPartitionHandler) GetPartitions(paths *Paths, logger *types.KairosLogger) types.PartitionList {
	out := make(types.PartitionList, 0)
	path := filepath.Join(paths.SysBlock, d.DiskName)
	logger.Logger.Debug().Str("file", path).Msg("Reading disk file")
	files, err := os.ReadDir(path)
	if err != nil {
		logger.Logger.Error().Err(err).Msg("failed to read disk partitions")
		return out
	}
	for _, file := range files {
		fname := file.Name()
		if !strings.HasPrefix(fname, d.DiskName) {
			continue
		}
		logger.Logger.Debug().Str("file", fname).Msg("Reading partition file")
		partitionPath := filepath.Join(d.DiskName, fname)
		size := partitionSizeBytes(paths, partitionPath, logger)
		mp, pt := partitionInfo(paths, fname, logger)
		du := diskPartUUID(paths, partitionPath, logger)
		if pt == "" {
			pt = diskPartTypeUdev(paths, partitionPath, logger)
		}
		fsLabel := diskFSLabel(paths, partitionPath, logger)
		p := &types.Partition{
			Name:            fname,
			Size:            uint(size / (1024 * 1024)),
			MountPoint:      mp,
			UUID:            du,
			FilesystemLabel: fsLabel,
			FS:              pt,
			Path:            filepath.Join("/dev", fname),
			Disk:            filepath.Join("/dev", d.DiskName),
		}
		out = append(out, p)
	}
	return out
}
