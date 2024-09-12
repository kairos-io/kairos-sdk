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

})
