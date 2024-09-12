package ghw_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kairos-io/kairos-sdk/ghw"
	"github.com/kairos-io/kairos-sdk/types"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGHW(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GHW test suite")
}

var _ = Describe("GHW functions tests", func() {
	var ghwMock GhwMock
	BeforeEach(func() {
		ghwMock = GhwMock{}
	})
	AfterEach(func() {
		ghwMock.Clean()
	})
	Describe("With a disk", func() {
		BeforeEach(func() {
			mainDisk := ghw.Disk{
				Name:      "disk",
				UUID:      "555",
				SizeBytes: 1 * 1024,
				Partitions: []*types.Partition{
					{
						Name:            "disk1",
						FilesystemLabel: "COS_GRUB",
						FS:              "ext4",
						MountPoint:      "/efi",
						Size:            0,
						UUID:            "666",
					},
				},
			}

			ghwMock.AddDisk(mainDisk)
			ghwMock.CreateDevices()
		})

		It("Finds the disk and partition", func() {
			disks := ghw.GetDisks(ghw.NewPaths(ghwMock.chroot), nil)
			Expect(len(disks)).To(Equal(1), disks)
			Expect(disks[0].Name).To(Equal("disk"), disks)
			Expect(disks[0].UUID).To(Equal("555"), disks)
			// Expected is size * sectorsize which is 512
			Expect(disks[0].SizeBytes).To(Equal(uint64(1*1024*512)), disks)
			Expect(len(disks[0].Partitions)).To(Equal(1), disks)
			Expect(disks[0].Partitions[0].Name).To(Equal("disk1"), disks)
			Expect(disks[0].Partitions[0].FilesystemLabel).To(Equal("COS_GRUB"), disks)
			Expect(disks[0].Partitions[0].FS).To(Equal("ext4"), disks)
			Expect(disks[0].Partitions[0].MountPoint).To(Equal("/efi"), disks)
			Expect(disks[0].Partitions[0].UUID).To(Equal("666"), disks)
		})
	})
	Describe("With no disks", func() {
		It("Finds nothing", func() {
			ghwMock.CreateDevices()
			disks := ghw.GetDisks(ghw.NewPaths(ghwMock.chroot), nil)
			Expect(len(disks)).To(Equal(0), disks)
		})
	})

})

// GhwMock is used to construct a fake disk to present to ghw when scanning block devices
// The way this works is ghw will use the existing files in the system to determine the different disks, partitions and
// mountpoints. It uses /sys/block, /proc/self/mounts and /run/udev/data to gather everything
// It also has an entrypoint to overwrite the root dir from which the paths are constructed so that allows us to override
// it easily and make it read from a different location.
// This mock is used to construct a fake FS with all its needed files on a different chroot and just add a Disk with its
// partitions and let the struct do its thing creating files and mountpoints and such
// You can even just pass no disks to simulate a system in which there is no disk/no cos partitions
type GhwMock struct {
	chroot string
	paths  *ghw.Paths
	disks  []ghw.Disk
	mounts []string
}

// AddDisk adds a disk to GhwMock
func (g *GhwMock) AddDisk(disk ghw.Disk) {
	g.disks = append(g.disks, disk)
}

// AddPartitionToDisk will add a partition to the given disk and call Clean+CreateDevices, so we recreate all files
// It makes no effort checking if the disk exists
func (g *GhwMock) AddPartitionToDisk(diskName string, partition *types.Partition) {
	for _, disk := range g.disks {
		if disk.Name == diskName {
			disk.Partitions = append(disk.Partitions, partition)
			g.Clean()
			g.CreateDevices()
		}
	}
}

// CreateDevices will create a new context and paths for ghw using the Chroot value as base, then set the env var GHW_ROOT so the
// ghw library picks that up and then iterate over the disks and partitions and create the necessary files
func (g *GhwMock) CreateDevices() {
	d, _ := os.MkdirTemp("", "ghwmock")
	g.chroot = d
	g.paths = ghw.NewPaths(d)
	// Create the /sys/block dir
	_ = os.MkdirAll(g.paths.SysBlock, 0755)
	// Create the /run/udev/data dir
	_ = os.MkdirAll(g.paths.RunUdevData, 0755)
	// Create only the /proc/ dir, we add the mounts file afterwards
	procDir, _ := filepath.Split(g.paths.ProcMounts)
	_ = os.MkdirAll(procDir, 0755)
	for indexDisk, disk := range g.disks {
		// For each dir we create the /sys/block/DISK_NAME
		diskPath := filepath.Join(g.paths.SysBlock, disk.Name)
		_ = os.Mkdir(diskPath, 0755)
		// We create a dev file to indicate the devicenumber for a given disk
		_ = os.WriteFile(filepath.Join(g.paths.SysBlock, disk.Name, "dev"), []byte(fmt.Sprintf("%d:0\n", indexDisk)), 0644)
		// Also write the size
		_ = os.WriteFile(filepath.Join(g.paths.SysBlock, disk.Name, "size"), []byte(strconv.FormatUint(disk.SizeBytes, 10)), 0644)
		// Create the udevdata for this disk
		_ = os.WriteFile(filepath.Join(g.paths.RunUdevData, fmt.Sprintf("b%d:0", indexDisk)), []byte(fmt.Sprintf("E:ID_PART_TABLE_UUID=%s\n", disk.UUID)), 0644)
		for indexPart, partition := range disk.Partitions {
			// For each partition we create the /sys/block/DISK_NAME/PARTITION_NAME
			_ = os.Mkdir(filepath.Join(diskPath, partition.Name), 0755)
			// Create the /sys/block/DISK_NAME/PARTITION_NAME/dev file which contains the major:minor of the partition
			_ = os.WriteFile(filepath.Join(diskPath, partition.Name, "dev"), []byte(fmt.Sprintf("%d:6%d\n", indexDisk, indexPart)), 0644)
			_ = os.WriteFile(filepath.Join(diskPath, partition.Name, "size"), []byte(fmt.Sprintf("%d\n", partition.Size)), 0644)
			// Create the /run/udev/data/bMAJOR:MINOR file with the data inside to mimic the udev database
			data := []string{fmt.Sprintf("E:ID_FS_LABEL=%s\n", partition.FilesystemLabel)}
			if partition.FS != "" {
				data = append(data, fmt.Sprintf("E:ID_FS_TYPE=%s\n", partition.FS))
			}
			if partition.UUID != "" {
				data = append(data, fmt.Sprintf("E:ID_PART_ENTRY_UUID=%s\n", partition.UUID))
			}
			_ = os.WriteFile(filepath.Join(g.paths.RunUdevData, fmt.Sprintf("b%d:6%d", indexDisk, indexPart)), []byte(strings.Join(data, "")), 0644)
			// If we got a mountpoint, add it to our fake /proc/self/mounts
			if partition.MountPoint != "" {
				// Check if the partition has a fs, otherwise default to ext4
				if partition.FS == "" {
					partition.FS = "ext4"
				}
				// Prepare the g.mounts with all the mount lines
				g.mounts = append(
					g.mounts,
					fmt.Sprintf("%s %s %s ro,relatime 0 0\n", filepath.Join("/dev", partition.Name), partition.MountPoint, partition.FS))
			}
		}
	}
	// Finally, write all the mounts
	_ = os.WriteFile(g.paths.ProcMounts, []byte(strings.Join(g.mounts, "")), 0644)
}

// RemoveDisk will remove the files for a disk. It makes no effort to check if the disk exists or not
func (g *GhwMock) RemoveDisk(disk string) {
	// This could be simpler I think, just removing the /sys/block/DEVICE should make ghw not find anything and not search
	// for partitions, but just in case do it properly
	var newMounts []string
	diskPath := filepath.Join(g.paths.SysBlock, disk)
	_ = os.RemoveAll(diskPath)

	// Try to find any mounts that match the disk given and remove them from the mounts
	for _, mount := range g.mounts {
		fields := strings.Fields(mount)
		// If first field does not contain the /dev/DEVICE, add it to the newmounts
		if !strings.Contains(fields[0], filepath.Join("/dev", disk)) {
			newMounts = append(newMounts, mount)
		}
	}
	g.mounts = newMounts
	// Write the mounts again
	_ = os.WriteFile(g.paths.ProcMounts, []byte(strings.Join(g.mounts, "")), 0644)
}

// RemovePartitionFromDisk will remove the files for a partition
// It makes no effort checking if the disk/partition/files exist
func (g *GhwMock) RemovePartitionFromDisk(diskName string, partitionName string) {
	var newMounts []string
	diskPath := filepath.Join(g.paths.SysBlock, diskName)
	// Read the dev major:minor
	devName, _ := os.ReadFile(filepath.Join(diskPath, partitionName, "dev"))
	// Remove the MAJOR:MINOR file from the udev database
	_ = os.RemoveAll(filepath.Join(g.paths.RunUdevData, fmt.Sprintf("b%s", devName)))
	// Remove the /sys/block/DISK/PARTITION dir
	_ = os.RemoveAll(filepath.Join(diskPath, partitionName))

	// Try to find any mounts that match the partition given and remove them from the mounts
	for _, mount := range g.mounts {
		fields := strings.Fields(mount)
		// If first field does not contain the /dev/PARTITION, add it to the newmounts
		if !strings.Contains(fields[0], filepath.Join("/dev", partitionName)) {
			newMounts = append(newMounts, mount)
		}
	}
	g.mounts = newMounts
	// Write the mounts again
	_ = os.WriteFile(g.paths.ProcMounts, []byte(strings.Join(g.mounts, "")), 0644)
	// Remove it from the partitions list
	for index, disk := range g.disks {
		if disk.Name == diskName {
			var newPartitions types.PartitionList
			for _, partition := range disk.Partitions {
				if partition.Name != partitionName {
					newPartitions = append(newPartitions, partition)
				}
			}
			g.disks[index].Partitions = newPartitions
		}
	}
}

// Clean will remove the chroot dir and unset the env var
func (g *GhwMock) Clean() {
	_ = os.RemoveAll(g.chroot)
}
