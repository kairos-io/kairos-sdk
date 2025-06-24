package loop

/*
#cgo LDFLAGS: -ldevmapper
#include <libdevmapper.h>

enum {
	DeviceCreate = 0,
	DeviceResume = 5,
};

#define ADD_NODE_ON_RESUME DM_ADD_NODE_ON_RESUME
*/
import "C"

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const (
	BLKGETSIZE64 = 0x80081272 // ioctl request for block device size
)

func ioctlGetUint64(fd uintptr, req uint) (uint64, error) {
	var size uint64
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, uintptr(req), uintptr(unsafe.Pointer(&size)))
	if errno != 0 {
		return 0, errno
	}
	return size, nil
}

func CreateMappingsFromDevice(loopDevice string) {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.Debugf("Starting device-mapper setup for %s", loopDevice)

	partitions, err := GetGPTPartitions(loopDevice)
	if err != nil {
		logrus.Fatalf("Failed to read GPT partitions from %s: %v", loopDevice, err)
	}

	for _, p := range partitions {
		dmName := fmt.Sprintf("loop%d-part%d", getLoopNumber(loopDevice), p.Number)
		logrus.Debugf("Creating mapping for partition %d (%s)", p.Number, dmName)

		taskCreate := C.dm_task_create(C.int(C.DeviceCreate))
		if taskCreate == nil {
			logrus.Fatalf("dm_task_create for DeviceCreate failed for %s", dmName)
		}
		defer C.dm_task_destroy(taskCreate)

		dmNameC := C.CString(dmName)
		defer C.free(unsafe.Pointer(dmNameC))

		if C.dm_task_set_name(taskCreate, dmNameC) != 1 {
			logrus.Fatalf("dm_task_set_name failed for %s", dmName)
		}

		targetType := C.CString("linear")
		targetParams := C.CString(fmt.Sprintf("%s %d", loopDevice, p.FirstLBA))
		defer C.free(unsafe.Pointer(targetType))
		defer C.free(unsafe.Pointer(targetParams))

		if C.dm_task_add_target(taskCreate, 0, C.uint64_t(p.NumSectors), targetType, targetParams) != 1 {
			logrus.Fatalf("dm_task_add_target failed for %s", dmName)
		}

		if C.dm_task_set_add_node(taskCreate, C.ADD_NODE_ON_RESUME) != 1 {
			logrus.Fatalf("dm_task_set_add_node failed for %s", dmName)
		}

		if C.dm_task_run(taskCreate) != 1 {
			logrus.Fatalf("dm_task_run (DeviceCreate) failed for %s", dmName)
		}

		logrus.Debugf("Device %s created (suspended state)", dmName)

		taskResume := C.dm_task_create(C.int(C.DeviceResume))
		if taskResume == nil {
			logrus.Fatalf("dm_task_create for DeviceResume failed for %s", dmName)
		}
		defer C.dm_task_destroy(taskResume)

		if C.dm_task_set_name(taskResume, dmNameC) != 1 {
			logrus.Fatalf("dm_task_set_name (resume) failed for %s", dmName)
		}

		if C.dm_task_run(taskResume) != 1 {
			logrus.Fatalf("dm_task_run (DeviceResume) failed for %s", dmName)
		}

		logrus.Debugf("Device %s resumed (active)", dmName)

		// Trigger udev or manually create device node if necessary
		if C.dm_udev_wait(C.uint32_t(0)) != 1 {
			logrus.Warnf("dm_udev_wait failed for %s, node may not be created automatically", dmName)
		}

		dmPath := "/dev/mapper/" + dmName
		if stat, err := os.Stat(dmPath); err == nil {
			rdev := stat.Sys().(*syscall.Stat_t).Rdev
			logrus.Infof("✅ Device %s ready (major:minor = %d:%d)", dmPath, unix.Major(rdev), unix.Minor(rdev))
		} else {
			logrus.Errorf("Device node %s not found: %v", dmPath, err)
		}
	}
}

func getLoopNumber(device string) int {
	base := filepath.Base(device) // "loop0"
	numStr := strings.TrimPrefix(base, "loop")
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0 // Default to 0 or handle error appropriately
	}
	return num
}
