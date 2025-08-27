package ghw_test

import (
	"testing"

	"github.com/kairos-io/kairos-sdk/ghw"
	"github.com/kairos-io/kairos-sdk/ghw/mocks"
	"github.com/kairos-io/kairos-sdk/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGHW(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GHW test suite")
}

var _ = Describe("GHW functions tests", func() {
	var ghwMock mocks.GhwMock
	BeforeEach(func() {
		ghwMock = mocks.GhwMock{}
	})
	AfterEach(func() {
		ghwMock.Clean()
	})
	Describe("With a disk", func() {
		BeforeEach(func() {
			mainDisk := types.Disk{
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
			disks := ghw.GetDisks(ghw.NewPaths(ghwMock.Chroot), nil)
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
			disks := ghw.GetDisks(ghw.NewPaths(ghwMock.Chroot), nil)
			Expect(len(disks)).To(Equal(0), disks)
		})
	})

	Describe("With multipath devices", func() {
		BeforeEach(func() {
			// Create a multipath device dm-0 with two partitions dm-1 and dm-2
			multipathDisk := types.Disk{
				Name:      "dm-0",
				UUID:      "mpath-uuid-123",
				SizeBytes: 10 * 1024 * 1024, // 10MB
				Partitions: []*types.Partition{
					{
						Name:            "dm-1",
						FilesystemLabel: "MPATH_BOOT",
						FS:              "ext4",
						MountPoint:      "/boot",
						Size:            512,
						UUID:            "part1-uuid-456",
					},
					{
						Name:            "dm-2", 
						FilesystemLabel: "MPATH_DATA",
						FS:              "xfs",
						MountPoint:      "/data",
						Size:            1024,
						UUID:            "part2-uuid-789",
					},
				},
			}

			ghwMock.AddDisk(multipathDisk)
			ghwMock.CreateMultipathDevices()
		})

		It("Identifies multipath device correctly", func() {
			disks := ghw.GetDisks(ghw.NewPaths(ghwMock.Chroot), nil)
			
			// Should find the multipath device but not the partitions as separate disks
			Expect(len(disks)).To(Equal(1), "Should find only the multipath device")
			
			disk := disks[0]
			Expect(disk.Name).To(Equal("dm-0"))
			Expect(disk.UUID).To(Equal("mpath-uuid-123"))
			Expect(disk.SizeBytes).To(Equal(uint64(10 * 1024 * 1024 * 512))) // size * sectorSize
		})

		It("Finds multipath partitions using MultipathPartitionHandler", func() {
			disks := ghw.GetDisks(ghw.NewPaths(ghwMock.Chroot), nil)
			
			Expect(len(disks)).To(Equal(1))
			disk := disks[0]
			
			// Should find both partitions
			Expect(len(disk.Partitions)).To(Equal(2))
			
			// Verify first partition
			part1 := disk.Partitions[0]
			Expect(part1.Name).To(Equal("dm-1"))
			Expect(part1.FilesystemLabel).To(Equal("MPATH_BOOT"))
			Expect(part1.FS).To(Equal("ext4"))
			Expect(part1.MountPoint).To(Equal("/boot"))
			Expect(part1.UUID).To(Equal("part1-uuid-456"))
			Expect(part1.Path).To(Equal("/dev/dm-1"))
			Expect(part1.Disk).To(Equal("/dev/dm-0"))
			
			// Verify second partition
			part2 := disk.Partitions[1]
			Expect(part2.Name).To(Equal("dm-2"))
			Expect(part2.FilesystemLabel).To(Equal("MPATH_DATA"))
			Expect(part2.FS).To(Equal("xfs"))
			Expect(part2.MountPoint).To(Equal("/data"))
			Expect(part2.UUID).To(Equal("part2-uuid-789"))
			Expect(part2.Path).To(Equal("/dev/dm-2"))
			Expect(part2.Disk).To(Equal("/dev/dm-0"))
		})

		It("Skips multipath partitions in main disk enumeration", func() {
			// This test verifies that multipath partitions (dm-1, dm-2) are not 
			// returned as separate disks in the main GetDisks call
			disks := ghw.GetDisks(ghw.NewPaths(ghwMock.Chroot), nil)
			
			// Should only find the parent multipath device, not the partition devices
			Expect(len(disks)).To(Equal(1))
			Expect(disks[0].Name).To(Equal("dm-0"))
			
			// Verify no disk named dm-1 or dm-2 is returned
			for _, disk := range disks {
				Expect(disk.Name).ToNot(Equal("dm-1"))
				Expect(disk.Name).ToNot(Equal("dm-2"))
			}
		})
	})

	Describe("With multipath devices using /dev/dm-<n> mount format", func() {
		BeforeEach(func() {
			// Create a multipath device dm-3 with partitions mounted as /dev/dm-<n> instead of /dev/mapper/<name>
			multipathDisk := types.Disk{
				Name:      "dm-3",
				UUID:      "mpath-dm-mount-uuid",
				SizeBytes: 8 * 1024 * 1024,
				Partitions: []*types.Partition{
					{
						Name:            "dm-4",
						FilesystemLabel: "DM_BOOT",
						FS:              "ext4",
						MountPoint:      "/boot",
						Size:            256,
						UUID:            "dm-part1-uuid",
					},
					{
						Name:            "dm-5", 
						FilesystemLabel: "DM_DATA",
						FS:              "xfs",
						MountPoint:      "/data",
						Size:            512,
						UUID:            "dm-part2-uuid",
					},
				},
			}

			ghwMock.AddDisk(multipathDisk)
			ghwMock.CreateMultipathDevicesWithDmMounts()
		})

		It("Finds multipath partitions mounted as /dev/dm-<n>", func() {
			disks := ghw.GetDisks(ghw.NewPaths(ghwMock.Chroot), nil)
			
			Expect(len(disks)).To(Equal(1))
			disk := disks[0]
			
			// Should find both partitions
			Expect(len(disk.Partitions)).To(Equal(2))
			
			// Verify partitions can be found regardless of mount format
			var bootPartition, dataPartition *types.Partition
			for _, part := range disk.Partitions {
				if part.MountPoint == "/boot" {
					bootPartition = part
				} else if part.MountPoint == "/data" {
					dataPartition = part
				}
			}
			
			Expect(bootPartition).ToNot(BeNil())
			Expect(bootPartition.Name).To(Equal("dm-4"))
			Expect(bootPartition.FilesystemLabel).To(Equal("DM_BOOT"))
			Expect(bootPartition.MountPoint).To(Equal("/boot"))
			Expect(bootPartition.FS).To(Equal("ext4"))
			Expect(bootPartition.Path).To(Equal("/dev/dm-4"))
			
			Expect(dataPartition).ToNot(BeNil())
			Expect(dataPartition.Name).To(Equal("dm-5"))
			Expect(dataPartition.FilesystemLabel).To(Equal("DM_DATA"))
			Expect(dataPartition.MountPoint).To(Equal("/data"))
			Expect(dataPartition.FS).To(Equal("xfs"))
			Expect(dataPartition.Path).To(Equal("/dev/dm-5"))
		})
	})

	Describe("With standalone multipath device (no partitions)", func() {
		It("Handles multipath device with no partitions", func() {
			multipathDisk := types.Disk{
				Name:      "dm-5",
				UUID:      "mpath-empty-uuid",
				SizeBytes: 5 * 1024 * 1024,
			}
			
			ghwMock.AddDisk(multipathDisk)
			ghwMock.CreateMultipathDevices()
			
			disks := ghw.GetDisks(ghw.NewPaths(ghwMock.Chroot), nil)
			
			Expect(len(disks)).To(Equal(1))
			disk := disks[0]
			Expect(disk.Name).To(Equal("dm-5"))
			Expect(len(disk.Partitions)).To(Equal(0))
		})
	})

	Describe("With mixed regular and multipath disks", func() {
		It("Handles mixed regular and multipath disks", func() {
			// Create multipath device
			multipathDeviceDef := types.Disk{
				Name:      "dm-0",
				UUID:      "mpath-uuid-123",
				SizeBytes: 10 * 1024 * 1024,
				Partitions: []*types.Partition{
					{
						Name:            "dm-1",
						FilesystemLabel: "MPATH_BOOT",
						FS:              "ext4",
						MountPoint:      "/boot",
						Size:            512,
						UUID:            "part1-uuid-456",
					},
				},
			}
			
			// Create regular disk
			regularDisk := types.Disk{
				Name:      "sda",
				UUID:      "regular-uuid-999",
				SizeBytes: 8 * 1024 * 1024,
				Partitions: []*types.Partition{
					{
						Name:            "sda1",
						FilesystemLabel: "REGULAR_ROOT",
						FS:              "ext4",
						MountPoint:      "/",
						Size:            2048,
						UUID:            "regular-part-uuid",
					},
				},
			}
			
			// Add both disks
			ghwMock.AddDisk(multipathDeviceDef)
			ghwMock.AddDisk(regularDisk)
			
			// Create multipath structure (this will handle both disks appropriately)
			ghwMock.CreateMultipathDevices()
			
			disks := ghw.GetDisks(ghw.NewPaths(ghwMock.Chroot), nil)
			
			// Should find both the regular disk and multipath device
			Expect(len(disks)).To(Equal(2))
			
			var foundMultipathDisk, foundRegularDisk *types.Disk
			for _, disk := range disks {
				if disk.Name == "dm-0" {
					foundMultipathDisk = disk
				} else if disk.Name == "sda" {
					foundRegularDisk = disk
				}
			}
			
			Expect(foundMultipathDisk).ToNot(BeNil())
			Expect(foundRegularDisk).ToNot(BeNil())
			
			// Verify multipath device has its partition
			Expect(len(foundMultipathDisk.Partitions)).To(Equal(1))
			Expect(foundMultipathDisk.Partitions[0].Name).To(Equal("dm-1"))
			
			// Verify regular device has its partition
			Expect(len(foundRegularDisk.Partitions)).To(Equal(1))
			Expect(foundRegularDisk.Partitions[0].Name).To(Equal("sda1"))
		})
	})

})
