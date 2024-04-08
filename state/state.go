package state

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/itchyny/gojq"
	"github.com/jaypipes/ghw"
	"github.com/jaypipes/ghw/pkg/block"
	"github.com/kairos-io/kairos-sdk/types"
	"github.com/kairos-io/kairos-sdk/utils"
	"github.com/rs/zerolog"
	"github.com/zcalusic/sysinfo"
	"gopkg.in/yaml.v3"
)

const (
	Active    Boot = "active_boot"
	Passive   Boot = "passive_boot"
	Recovery  Boot = "recovery_boot"
	LiveCD    Boot = "livecd_boot"
	AutoReset Boot = "autoreset_boot"
	Unknown   Boot = "unknown"

	UEFICurrentEntryFile = "/sys/firmware/efi/efivars/LoaderEntrySelected-4a67b082-0a4c-41cf-b6c7-440b29bb8c4f"
)

var Log zerolog.Logger

type Boot string

type PartitionState struct {
	Mounted         bool   `yaml:"mounted" json:"mounted"`
	Name            string `yaml:"name" json:"name"`
	Label           string `yaml:"label" json:"label"`
	FilesystemLabel string `yaml:"filesystemlabel" json:"filesystemlabel"`
	MountPoint      string `yaml:"mount_point" json:"mount_point"`
	SizeBytes       uint64 `yaml:"size_bytes" json:"size_bytes"`
	Type            string `yaml:"type" json:"type"`
	IsReadOnly      bool   `yaml:"read_only" json:"read_only"`
	Found           bool   `yaml:"found" json:"found"`
	UUID            string `yaml:"uuid" json:"uuid"` // This would be volume UUID on macOS, PartUUID on linux, empty on Windows
}

type Kairos struct {
	Flavor  string `yaml:"flavor" json:"flavor"`
	Version string `yaml:"version" json:"version"`
	Init    string `yaml:"init" json:"init"`
}

type Runtime struct {
	UUID       string          `yaml:"uuid" json:"uuid"`
	Persistent PartitionState  `yaml:"persistent" json:"persistent"`
	Recovery   PartitionState  `yaml:"recovery" json:"recovery"`
	OEM        PartitionState  `yaml:"oem" json:"oem"`
	State      PartitionState  `yaml:"state" json:"state"`
	BootState  Boot            `yaml:"boot" json:"boot"`
	System     sysinfo.SysInfo `yaml:"system" json:"system"`
	Kairos     Kairos          `yaml:"kairos" json:"kairos"`
}

type FndMnt struct {
	Filesystems []struct {
		Target    string `json:"target,omitempty"`
		FsOptions string `json:"fs-options,omitempty"`
	} `json:"filesystems,omitempty"`
}

// Lsblk is the struct to marshal the output of lsblk
type Lsblk struct {
	BlockDevices []struct {
		Path       string `json:"path,omitempty"`
		Mountpoint string `json:"mountpoint,omitempty"`
		FsType     string `json:"fstype,omitempty"`
		Size       string `json:"size,omitempty"`
		Label      string `json:"label,omitempty"`
		RO         bool   `json:"ro,omitempty"`
	} `json:"blockdevices,omitempty"`
}

func detectPartitionByFindmnt(b *block.Partition) PartitionState {
	// If mountpoint seems empty, try to get the mountpoint of the partition label also the RO status
	// This is a current shortcoming of ghw which only identifies mountpoints via device, not by label/uuid/anything else
	mountpoint := b.MountPoint
	readOnly := b.IsReadOnly
	if b.MountPoint == "" && b.FilesystemLabel != "" {
		out, err := utils.SH(fmt.Sprintf("findmnt /dev/disk/by-label/%s -f -J -o TARGET,FS-OPTIONS", b.FilesystemLabel))
		mnt := &FndMnt{}
		if err == nil {
			err = json.Unmarshal([]byte(out), mnt)
			// This should not happen, if there were no targets, the command would have returned an error, but you never know...
			if err == nil && len(mnt.Filesystems) == 1 {
				mountpoint = mnt.Filesystems[0].Target
				// Don't assume its ro or rw by default, check both. One should match
				regexRW := regexp.MustCompile("^rw,|^rw$|,rw,|,rw$")
				regexRO := regexp.MustCompile("^ro,|^ro$|,ro,|,ro$")
				if regexRW.Match([]byte(mnt.Filesystems[0].FsOptions)) {
					readOnly = false
				}
				if regexRO.Match([]byte(mnt.Filesystems[0].FsOptions)) {
					readOnly = true
				}
			}
		}
	}
	return PartitionState{
		Type:            b.Type,
		IsReadOnly:      readOnly,
		UUID:            b.UUID,
		Name:            fmt.Sprintf("/dev/%s", b.Name),
		SizeBytes:       b.SizeBytes,
		Label:           b.Label,
		FilesystemLabel: b.FilesystemLabel,
		MountPoint:      mountpoint,
		Mounted:         mountpoint != "",
		Found:           true,
	}
}

func detectBoot(logger zerolog.Logger) Boot {
	logger.Info().Msg("detecting boot state")
	cmdline, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		logger.Debug().Err(err).Msg("Error reading /proc/cmdline file " + err.Error())
		return Unknown
	}

	cmdlineS := string(cmdline)

	if DetectUKIboot(cmdlineS) {
		logger.Debug().Msg("Detected uki boot")
		return getUKIBootState(logger)
	}

	return getNonUKIBootState(cmdlineS)
}

func getUKIBootState(logger zerolog.Logger) Boot {
	if !EfiBootFromInstall(logger) {
		return LiveCD
	}

	currentEntryBytes, err := os.ReadFile(UEFICurrentEntryFile)
	if err != nil {
		logger.Debug().Err(err).Msg(fmt.Sprintf("Error reading %s file %s", UEFICurrentEntryFile, err.Error()))
		return Unknown
	}

	// Create a regular expression to remove non-printable characters
	regex := regexp.MustCompile("[[:cntrl:]]")
	currentEntry := regex.ReplaceAllString(string(currentEntryBytes), "")

	logger.Debug().Msg("Current entry: " + currentEntry)

	if !strings.HasSuffix(currentEntry, ".conf") {
		return Unknown
	}

	switch {
	case strings.HasPrefix(currentEntry, "active"):
		return Active
	case strings.HasPrefix(currentEntry, "passive"):
		return Passive
	case strings.HasPrefix(currentEntry, "recovery"):
		return Recovery
	case strings.HasPrefix(currentEntry, "statereset"):
		return AutoReset
	default:
		return Unknown
	}
}

func getNonUKIBootState(cmdline string) Boot {
	switch {
	case strings.Contains(cmdline, "COS_ACTIVE"):
		return Active
	case strings.Contains(cmdline, "COS_PASSIVE"):
		return Passive
	case strings.Contains(cmdline, "COS_RECOVERY"), strings.Contains(cmdline, "COS_SYSTEM"), strings.Contains(cmdline, "recovery-mode"):
		return Recovery
	case strings.Contains(cmdline, "live:LABEL"), strings.Contains(cmdline, "live:CDLABEL"), strings.Contains(cmdline, "netboot"):
		return LiveCD
	default:
		return Unknown
	}
}

// Detects if we are on uki mode
func DetectUKIboot(cmdline string) bool {
	Log.Info().Msg("checking cmdline for uki:" + cmdline)
	return strings.Contains(cmdline, "rd.immucore.uki")
}

// EfiBootFromInstall will try to check the /sys/firmware/efi/LoaderDevicePartUUID-4a67b082-0a4c-41cf-b6c7-440b29bb8c4f
// systemd vendor Id is 4a67b082-0a4c-41cf-b6c7-440b29bb8c4f and will never change
// LoaderDevicePartUUID contains the partition UUID of the EFI System Partition the boot loader was run from. Set by the boot loader.
// This will return true if we are running from a DISK device, which sets the efivar
// This wil return false when running from a volatile media, like CD or netboot as it cannot infer where it was booted from
// Useful to check if we are on install phase or not
// This efi var is VOLATILE so once we reboot is GONE. No way of keeping it across reboots, its set by the bootloader.
func EfiBootFromInstall(logger zerolog.Logger) bool {
	file := "/sys/firmware/efi/efivars/LoaderDevicePartUUID-4a67b082-0a4c-41cf-b6c7-440b29bb8c4f"
	readFile, err := os.ReadFile(file)
	if err != nil {
		logger.Debug().Err(err).Msg("Error reading LoaderDevicePartUUID file")
		return false
	}
	if len(readFile) == 0 || string(readFile) == "" {
		logger.Debug().Str("file", string(readFile)).Msg("Error reading LoaderDevicePartUUID file")
		return false
	}
	return true
}

// DetectBootWithVFS will detect the boot state using a vfs so it can be used for tests as well
func DetectBootWithVFS(fs types.KairosFS) (Boot, error) {
	cmdline, err := fs.ReadFile("/proc/cmdline")
	if err != nil {
		return Unknown, err
	}
	cmdlineS := string(cmdline)
	switch {
	case strings.Contains(cmdlineS, "COS_ACTIVE"):
		return Active, nil
	case strings.Contains(cmdlineS, "COS_PASSIVE"):
		return Passive, nil
	case strings.Contains(cmdlineS, "COS_RECOVERY"), strings.Contains(cmdlineS, "COS_SYSTEM"), strings.Contains(cmdlineS, "recovery-mode"):
		return Recovery, nil
	case strings.Contains(cmdlineS, "live:LABEL"), strings.Contains(cmdlineS, "live:CDLABEL"), strings.Contains(cmdlineS, "netboot"):
		return LiveCD, nil
	default:
		return Unknown, nil
	}
}

func detectRuntimeState(r *Runtime) error {
	blockDevices, err := block.New(ghw.WithDisableTools(), ghw.WithDisableWarnings())
	// ghw currently only detects if partitions are mounted via the device
	// If we mount them via label, then its set as not mounted.
	if err != nil {
		return err
	}
	for _, d := range blockDevices.Disks {
		for _, part := range d.Partitions {
			switch part.FilesystemLabel {
			case "COS_PERSISTENT":
				r.Persistent = detectPartitionByFindmnt(part)
			case "COS_RECOVERY":
				r.Recovery = detectPartitionByFindmnt(part)
			case "COS_OEM":
				r.OEM = detectPartitionByFindmnt(part)
			case "COS_STATE":
				r.State = detectPartitionByFindmnt(part)
			}
		}
	}
	if !r.OEM.Found {
		r.OEM = detectPartitionByLsblk("COS_OEM")
	}
	if !r.Recovery.Found {
		r.Recovery = detectPartitionByLsblk("COS_RECOVERY")
	}
	return nil
}

// detectPartitionByLsblk will try to detect info about a partition by using lsblk
// Useful for LVM partitions which ghw is unable to find
func detectPartitionByLsblk(label string) PartitionState {
	out, err := utils.SH(fmt.Sprintf("lsblk /dev/disk/by-label/%s -o PATH,FSTYPE,MOUNTPOINT,SIZE,RO,LABEL -J", label))
	mnt := &Lsblk{}
	part := PartitionState{}
	if err == nil {
		err = json.Unmarshal([]byte(out), mnt)
		// This should not happen, if there were no targets, the command would have returned an error, but you never know...
		if err == nil && len(mnt.BlockDevices) == 1 {
			blk := mnt.BlockDevices[0]
			part.Found = true
			part.Name = blk.Path
			part.Mounted = blk.Mountpoint != ""
			part.MountPoint = blk.Mountpoint
			part.Type = blk.FsType
			part.FilesystemLabel = blk.Label
			// this seems to report always false. We can try to use findmnt here to know if its ro/rw
			part.IsReadOnly = blk.RO
		}
	}

	return part
}

func detectSystem(r *Runtime) {
	var si sysinfo.SysInfo

	si.GetSysInfo()
	r.System = si
}

func detectKairos(r *Runtime) {
	k := &Kairos{}
	k.Flavor = utils.Flavor()

	v, err := utils.OSRelease("VERSION")
	if err == nil {
		k.Version = v
	}
	k.Init = utils.GetInit()
	r.Kairos = *k
}

func NewRuntimeWithLogger(logger zerolog.Logger) (Runtime, error) {
	logger.Info().Msg("creating a runtime")
	runtime := &Runtime{
		BootState: detectBoot(logger),
		UUID:      utils.UUID(),
	}

	detectSystem(runtime)
	detectKairos(runtime)
	err := detectRuntimeState(runtime)

	return *runtime, err
}

func NewRuntime() (Runtime, error) {
	return NewRuntimeWithLogger(Log)
}

func (r Runtime) String() string {
	dat, err := yaml.Marshal(r)
	if err == nil {
		return string(dat)
	}
	return ""
}

func (r Runtime) Query(s string) (res string, err error) {
	s = fmt.Sprintf(".%s", s)
	jsondata := map[string]interface{}{}
	var dat []byte
	dat, err = json.Marshal(r)
	if err != nil {
		return
	}
	err = json.Unmarshal(dat, &jsondata)
	if err != nil {
		return
	}
	query, err := gojq.Parse(s)
	if err != nil {
		return res, err
	}
	iter := query.Run(jsondata) // or query.RunWithContext
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			return res, err
		}
		res += fmt.Sprint(v)
	}
	return
}
