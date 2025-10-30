package kcrypt

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jaypipes/ghw"
	"github.com/jaypipes/ghw/pkg/block"
	"github.com/kairos-io/kairos-sdk/collector"
	"github.com/kairos-io/kairos-sdk/kcrypt/bus"
	"github.com/kairos-io/kairos-sdk/types"
	"github.com/kairos-io/kairos-sdk/utils"
)

// PartitionEncryptor defines the interface for encrypting and decrypting partitions
type PartitionEncryptor interface {
	// Encrypt encrypts the specified partitions
	Encrypt(partitions []string) error

	// Unlock unlocks the specified partitions and waits for them to be ready
	// The method should only return when all partitions are unlocked and visible
	Unlock(partitions []string) error

	// Name returns the name of the encryption method (for logging)
	Name() string

	// Validate checks if prerequisites for this encryption method are met
	Validate() error
}

// RemoteKMSEncryptor encrypts partitions using a remote KMS (kcrypt-challenger)
type RemoteKMSEncryptor struct {
	logger       types.KairosLogger
	kcryptConfig *bus.KcryptConfig
}

func (e *RemoteKMSEncryptor) Encrypt(partitions []string) error {
	e.logger.Logger.Info().Str("method", e.Name()).Strs("partitions", partitions).Msg("Encrypting partitions")

	for _, partition := range partitions {
		e.logger.Logger.Info().Str("partition", partition).Msg("Encrypting partition")

		_, err := EncryptWithConfig(partition, e.logger, e.kcryptConfig)
		if err != nil {
			return fmt.Errorf("failed to encrypt partition %s: %w", partition, err)
		}

		e.logger.Logger.Info().Str("partition", partition).Msg("Successfully encrypted partition")
	}

	return nil
}

func (e *RemoteKMSEncryptor) Unlock(partitions []string) error {
	e.logger.Logger.Info().Str("method", e.Name()).Strs("partitions", partitions).Msg("Unlocking encrypted partitions")

	// Unlock each partition and wait for it to be ready
	for _, partitionLabel := range partitions {
		if err := e.unlockPartition(partitionLabel); err != nil {
			return fmt.Errorf("failed to unlock partition %s: %w", partitionLabel, err)
		}
	}

	e.logger.Logger.Info().Msg("All partitions unlocked successfully")
	return nil
}

func (e *RemoteKMSEncryptor) unlockPartition(partitionLabel string) error {
	var lastErr error

	for attempt := 0; attempt < 10; attempt++ {
		if attempt > 0 {
			e.logger.Logger.Info().
				Str("partition", partitionLabel).
				Int("attempt", attempt).
				Msg("Retrying unlock")
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		// Find the partition device by label
		devicePath, err := utils.SH(fmt.Sprintf("blkid -L %s", partitionLabel))
		devicePath = strings.TrimSpace(devicePath)

		if err != nil || devicePath == "" {
			lastErr = fmt.Errorf("partition not found")
			e.logger.Logger.Debug().
				Str("partition", partitionLabel).
				Msg("Partition not found, will retry")
			continue
		}

		// Get partition name from device path (e.g., /dev/sda1 -> sda1)
		partitionName := filepath.Base(devicePath)
		mapperPath := filepath.Join("/dev", "mapper", partitionName)

		// Check if already unlocked
		if utils.Exists(mapperPath) {
			e.logger.Logger.Info().
				Str("partition", partitionLabel).
				Str("mapper", mapperPath).
				Msg("Partition already unlocked")
			return nil
		}

		// Find the block.Partition for this device
		blk, err := ghw.Block()
		if err != nil {
			lastErr = fmt.Errorf("failed to scan block devices: %w", err)
			e.logger.Logger.Warn().Err(lastErr).Msg("Block scan failed, will retry")
			continue
		}

		var partition *block.Partition
		for _, disk := range blk.Disks {
			for _, p := range disk.Partitions {
				if p.FilesystemLabel == partitionLabel {
					partition = p
					break
				}
			}
			if partition != nil {
				break
			}
		}

		if partition == nil {
			lastErr = fmt.Errorf("partition not found in block devices")
			e.logger.Logger.Warn().Str("partition", partitionLabel).Msg("Partition not found, will retry")
			continue
		}

		e.logger.Logger.Debug().Msg("partition name: " + partition.Name)
		// Attempt to unlock using remote KMS (via plugin bus)
		err = UnlockWithKMS(partition, e.kcryptConfig, e.logger)
		if err != nil {
			lastErr = fmt.Errorf("unlock failed: %w", err)
			e.logger.Logger.Warn().
				Str("partition", partitionLabel).
				Err(lastErr).
				Msg("Unlock failed, will retry")
			continue
		}

		// Verify the partition is now visible
		checkPath, _ := utils.SH(fmt.Sprintf("blkid -L %s", partitionLabel))
		if strings.TrimSpace(checkPath) != "" {
			e.logger.Logger.Info().
				Str("partition", partitionLabel).
				Msg("Partition unlocked and verified")
			return nil
		}

		lastErr = fmt.Errorf("partition unlocked but not visible")
	}

	return fmt.Errorf("failed after 10 attempts: %w", lastErr)
}

func (e *RemoteKMSEncryptor) Name() string {
	return "Remote KMS"
}

func (e *RemoteKMSEncryptor) Validate() error {
	// No special validation needed for remote KMS
	// The actual connection will be validated when encrypting
	return nil
}

// TPMWithPCREncryptor encrypts partitions using TPM with PCR policy (UKI mode)
type TPMWithPCREncryptor struct {
	logger         types.KairosLogger
	bindPublicPCRs []string
	bindPCRs       []string
}

func (e *TPMWithPCREncryptor) Encrypt(partitions []string) error {
	e.logger.Logger.Info().Str("method", e.Name()).Strs("partitions", partitions).Msg("Encrypting partitions")

	_ = os.Setenv("SYSTEMD_LOG_LEVEL", "debug")
	defer os.Unsetenv("SYSTEMD_LOG_LEVEL")

	for _, partition := range partitions {
		e.logger.Logger.Info().Str("partition", partition).Msg("Encrypting partition")

		err := EncryptWithPcrs(partition, e.bindPublicPCRs, e.bindPCRs, e.logger)
		if err != nil {
			return fmt.Errorf("failed to encrypt partition %s: %w", partition, err)
		}

		e.logger.Logger.Info().Str("partition", partition).Msg("Successfully encrypted partition")
	}

	return nil
}

func (e *TPMWithPCREncryptor) Unlock(partitions []string) error {
	e.logger.Logger.Info().Str("method", e.Name()).Strs("partitions", partitions).Msg("Unlocking encrypted partitions")

	// TPM with PCR uses TPM-based unlock with systemd
	_ = os.Setenv("SYSTEMD_LOG_LEVEL", "debug")
	defer os.Unsetenv("SYSTEMD_LOG_LEVEL")

	// Unlock each partition and wait for it to be ready
	for _, partitionLabel := range partitions {
		if err := e.unlockPartition(partitionLabel); err != nil {
			return fmt.Errorf("failed to unlock partition %s: %w", partitionLabel, err)
		}
	}

	e.logger.Logger.Info().Msg("All partitions unlocked successfully")
	return nil
}

func (e *TPMWithPCREncryptor) unlockPartition(partitionLabel string) error {
	var lastErr error

	for attempt := 0; attempt < 10; attempt++ {
		if attempt > 0 {
			e.logger.Logger.Info().
				Str("partition", partitionLabel).
				Int("attempt", attempt).
				Msg("Retrying unlock")
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		// Find the partition device by label
		devicePath, err := utils.SH(fmt.Sprintf("blkid -L %s", partitionLabel))
		devicePath = strings.TrimSpace(devicePath)

		if err != nil || devicePath == "" {
			lastErr = fmt.Errorf("partition not found")
			e.logger.Logger.Debug().
				Str("partition", partitionLabel).
				Msg("Partition not found, will retry")
			continue
		}

		// Get partition name from device path (e.g., /dev/sda1 -> sda1)
		partitionName := filepath.Base(devicePath)
		mapperPath := filepath.Join("/dev", "mapper", partitionName)

		// Check if already unlocked
		if utils.Exists(mapperPath) {
			e.logger.Logger.Info().
				Str("partition", partitionLabel).
				Str("mapper", mapperPath).
				Msg("Partition already unlocked")
			return nil
		}

		// Attempt to unlock with TPM
		out, err := utils.SH(fmt.Sprintf("/usr/lib/systemd/systemd-cryptsetup attach %s %s - tpm2-device=auto", partitionName, devicePath))
		if err != nil {
			lastErr = fmt.Errorf("TPM unlock failed: %w (output: %s)", err, out)
			e.logger.Logger.Warn().
				Str("partition", partitionLabel).
				Err(lastErr).
				Msg("TPM unlock failed, will retry")
			continue
		}

		// Verify the partition is now visible
		checkPath, _ := utils.SH(fmt.Sprintf("blkid -L %s", partitionLabel))
		if strings.TrimSpace(checkPath) != "" {
			e.logger.Logger.Info().
				Str("partition", partitionLabel).
				Msg("Partition unlocked and verified")
			return nil
		}

		lastErr = fmt.Errorf("partition unlocked but not visible")
	}

	return fmt.Errorf("failed after 10 attempts: %w", lastErr)
}

func (e *TPMWithPCREncryptor) Name() string {
	return "TPM with PCR policy"
}

func (e *TPMWithPCREncryptor) Validate() error {
	// Validate systemd version (need >= 252 for systemd-cryptenroll)
	if err := validateSystemdVersion(e.logger, 252); err != nil {
		return err
	}

	// Validate TPM 2.0 device exists
	if err := validateTPMDevice(e.logger); err != nil {
		return err
	}

	return nil
}

// LocalTPMNVEncryptor encrypts partitions using local TPM NV passphrase storage
type LocalTPMNVEncryptor struct {
	logger       types.KairosLogger
	kcryptConfig *bus.KcryptConfig
}

func (e *LocalTPMNVEncryptor) Encrypt(partitions []string) error {
	e.logger.Logger.Info().Str("method", e.Name()).Strs("partitions", partitions).Msg("Encrypting partitions")

	// Extract TPM configuration
	nvIndex := DefaultLocalPassphraseNVIndex
	cIndex := ""
	tpmDevice := ""

	if e.kcryptConfig != nil {
		if e.kcryptConfig.NVIndex != "" {
			nvIndex = e.kcryptConfig.NVIndex
		}
		cIndex = e.kcryptConfig.CIndex
		tpmDevice = e.kcryptConfig.TPMDevice
	}

	for _, partition := range partitions {
		e.logger.Logger.Info().Str("partition", partition).Msg("Encrypting partition")

		_, err := EncryptWithLocalTPMPassphrase(partition, nvIndex, cIndex, tpmDevice, e.logger)
		if err != nil {
			return fmt.Errorf("failed to encrypt partition %s: %w", partition, err)
		}

		e.logger.Logger.Info().Str("partition", partition).Msg("Successfully encrypted partition")
	}

	return nil
}

func (e *LocalTPMNVEncryptor) Unlock(partitions []string) error {
	e.logger.Logger.Info().Str("method", e.Name()).Strs("partitions", partitions).Msg("Unlocking encrypted partitions")

	// Unlock each partition and wait for it to be ready
	for _, partitionLabel := range partitions {
		if err := e.unlockPartition(partitionLabel); err != nil {
			return fmt.Errorf("failed to unlock partition %s: %w", partitionLabel, err)
		}
	}

	e.logger.Logger.Info().Msg("All partitions unlocked successfully")
	return nil
}

func (e *LocalTPMNVEncryptor) unlockPartition(partitionLabel string) error {
	var lastErr error

	for attempt := 0; attempt < 10; attempt++ {
		if attempt > 0 {
			e.logger.Logger.Info().
				Str("partition", partitionLabel).
				Int("attempt", attempt).
				Msg("Retrying unlock")
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		// Find the partition device by label
		devicePath, err := utils.SH(fmt.Sprintf("blkid -L %s", partitionLabel))
		devicePath = strings.TrimSpace(devicePath)

		if err != nil || devicePath == "" {
			lastErr = fmt.Errorf("partition not found")
			e.logger.Logger.Debug().
				Str("partition", partitionLabel).
				Msg("Partition not found, will retry")
			continue
		}

		// Get partition name from device path (e.g., /dev/sda1 -> sda1)
		partitionName := filepath.Base(devicePath)
		mapperPath := filepath.Join("/dev", "mapper", partitionName)

		// Check if already unlocked
		if utils.Exists(mapperPath) {
			e.logger.Logger.Info().
				Str("partition", partitionLabel).
				Str("mapper", mapperPath).
				Msg("Partition already unlocked")
			return nil
		}

		// Find the block.Partition for this device
		blk, err := ghw.Block()
		if err != nil {
			lastErr = fmt.Errorf("failed to scan block devices: %w", err)
			e.logger.Logger.Warn().Err(lastErr).Msg("Block scan failed, will retry")
			continue
		}

		var partition *block.Partition
		for _, disk := range blk.Disks {
			for _, p := range disk.Partitions {
				if p.FilesystemLabel == partitionLabel {
					partition = p
					break
				}
			}
			if partition != nil {
				break
			}
		}

		if partition == nil {
			lastErr = fmt.Errorf("partition not found in block devices")
			e.logger.Logger.Warn().Str("partition", partitionLabel).Msg("Partition not found, will retry")
			continue
		}

		// Extract TPM configuration
		nvIndex := DefaultLocalPassphraseNVIndex
		cIndex := ""
		tpmDevice := ""
		if e.kcryptConfig != nil {
			if e.kcryptConfig.NVIndex != "" {
				nvIndex = e.kcryptConfig.NVIndex
			}
			cIndex = e.kcryptConfig.CIndex
			tpmDevice = e.kcryptConfig.TPMDevice
		}

		// Get passphrase from local TPM NV memory (not from remote)
		passphrase, err := GetOrCreateLocalTPMPassphrase(nvIndex, cIndex, tpmDevice)
		if err != nil {
			lastErr = fmt.Errorf("failed to get passphrase from local TPM: %w", err)
			e.logger.Logger.Warn().
				Str("partition", partitionLabel).
				Err(lastErr).
				Msg("Failed to get local TPM passphrase, will retry")
			continue
		}

		// Unlock directly with the local passphrase (bypass plugin bus)
		err = luksUnlock(devicePath, partitionName, passphrase, &e.logger)
		if err != nil {
			lastErr = fmt.Errorf("unlock failed: %w", err)
			e.logger.Logger.Warn().
				Str("partition", partitionLabel).
				Err(lastErr).
				Msg("Unlock failed, will retry")
			continue
		}

		// Verify the partition is now visible
		checkPath, _ := utils.SH(fmt.Sprintf("blkid -L %s", partitionLabel))
		if strings.TrimSpace(checkPath) != "" {
			e.logger.Logger.Info().
				Str("partition", partitionLabel).
				Msg("Partition unlocked and verified")
			return nil
		}

		lastErr = fmt.Errorf("partition unlocked but not visible")
	}

	return fmt.Errorf("failed after 10 attempts: %w", lastErr)
}

func (e *LocalTPMNVEncryptor) Name() string {
	return "Local TPM NV passphrase"
}

func (e *LocalTPMNVEncryptor) Validate() error {
	// Validate TPM 2.0 device exists
	return validateTPMDevice(e.logger)
}

// GetEncryptor returns the appropriate encryptor based on system configuration.
// It automatically:
// 1. Scans for kcrypt configuration (challenger server, mdns, etc.)
// 2. Detects UKI mode
// 3. Extracts PCR bindings from config
// 4. Returns the appropriate encryptor implementation
//
// Decision logic:
// 1. If challenger_server configured OR mdns enabled -> Remote KMS
// 2. Else if UKI mode -> TPM + PCR policy (requires systemd >= 252 and TPM 2.0)
// 3. Else (non-UKI, no remote) -> Local TPM NV passphrase
func GetEncryptor(logger types.KairosLogger) (PartitionEncryptor, error) {
	// 1. Scan for kcrypt configuration
	kcryptConfig := ScanKcryptConfig(logger)

	// 2. Detect UKI mode by checking if we're running from a UKI boot
	isUKI := detectUKIMode(logger)

	// 3. Extract PCR bindings from config (only used in UKI mode)
	var bindPCRs, bindPublicPCRs []string
	if isUKI {
		// Scan config again to get PCR bindings
		collectorConfig := scanCollectorConfig(logger)
		if collectorConfig != nil {
			bindPCRs, bindPublicPCRs = ExtractPCRBindingsFromCollector(*collectorConfig, logger)
		}
	}

	// 4. Determine which encryptor to use
	useRemoteKMS := kcryptConfig != nil && (kcryptConfig.ChallengerServer != "" || kcryptConfig.MDNS)

	var encryptor PartitionEncryptor

	if useRemoteKMS {
		logger.Logger.Info().
			Str("challenger_server", kcryptConfig.ChallengerServer).
			Bool("mdns", kcryptConfig.MDNS).
			Msg("Using remote KMS for encryption")
		encryptor = &RemoteKMSEncryptor{
			logger:       logger,
			kcryptConfig: kcryptConfig,
		}
	} else if isUKI {
		logger.Logger.Info().Msg("Using TPM with PCR policy for encryption (UKI mode)")
		encryptor = &TPMWithPCREncryptor{
			logger:         logger,
			bindPublicPCRs: bindPublicPCRs,
			bindPCRs:       bindPCRs,
		}
	} else {
		// Non-UKI local mode
		logger.Logger.Info().Msg("Using local TPM NV passphrase for encryption")
		encryptor = &LocalTPMNVEncryptor{
			logger:       logger,
			kcryptConfig: kcryptConfig,
		}
	}

	// Validate the encryptor
	if err := encryptor.Validate(); err != nil {
		return nil, fmt.Errorf("encryptor validation failed: %w", err)
	}

	return encryptor, nil
}

// scanCollectorConfig scans for configuration and returns the collector config
func scanCollectorConfig(logger types.KairosLogger) *collector.Config {
	o := &collector.Options{NoLogs: true, MergeBootCMDLine: true}
	if err := o.Apply(collector.Directories(DefaultConfigDirs...)); err != nil {
		logger.Debugf("scanCollectorConfig: error applying collector options: %v", err)
		return nil
	}

	collectorConfig, err := collector.Scan(o, func(d []byte) ([]byte, error) {
		return d, nil
	})
	if err != nil {
		logger.Debugf("scanCollectorConfig: error scanning for config: %v", err)
		return nil
	}

	return collectorConfig
}

// detectUKIMode detects if the system is running in UKI mode
// This checks for the presence of UKI-specific indicators
func detectUKIMode(logger types.KairosLogger) bool {
	// Check if we're booted from a UKI by looking for systemd UKI indicators
	// The most reliable way is to check if /run/systemd/tpm2-pcr-signature.json exists
	// This file is created by systemd when booting from a UKI with PCR signatures
	if _, err := os.Stat("/run/systemd/tpm2-pcr-signature.json"); err == nil {
		logger.Logger.Debug().Msg("Detected UKI mode: found /run/systemd/tpm2-pcr-signature.json")
		return true
	}

	// Alternative: check for /run/systemd/tpm2-pcr-public-key.pem
	if _, err := os.Stat("/run/systemd/tpm2-pcr-public-key.pem"); err == nil {
		logger.Logger.Debug().Msg("Detected UKI mode: found /run/systemd/tpm2-pcr-public-key.pem")
		return true
	}

	logger.Logger.Debug().Msg("Not running in UKI mode")
	return false
}

// validateSystemdVersion checks if systemd version is >= required version
func validateSystemdVersion(logger types.KairosLogger, minVersion int) error {
	run, err := utils.SH("systemctl --version | head -1 | awk '{ print $2}'")
	systemdVersion := strings.TrimSpace(string(run))
	if err != nil {
		logger.Errorf("could not get systemd version: %s", err)
		return fmt.Errorf("could not get systemd version: %w", err)
	}
	if systemdVersion == "" {
		return fmt.Errorf("could not get systemd version: empty output")
	}

	// Extract the numeric portion of the version string using a regular expression
	re := regexp.MustCompile(`\d+`)
	matches := re.FindString(systemdVersion)
	if matches == "" {
		return fmt.Errorf("could not extract numeric part from systemd version: %s", systemdVersion)
	}

	// Convert to int
	systemdVersionInt, err := strconv.Atoi(matches)
	if err != nil {
		return fmt.Errorf("could not convert systemd version to int: %w", err)
	}

	// Check minimum version
	if systemdVersionInt < minVersion {
		return fmt.Errorf("systemd version is %d, we need %d or higher for encrypting partitions with PCR policy", systemdVersionInt, minVersion)
	}

	logger.Logger.Info().Int("version", systemdVersionInt).Int("required", minVersion).Msg("Systemd version check passed")
	return nil
}

// validateTPMDevice checks if TPM 2.0 device exists
func validateTPMDevice(logger types.KairosLogger) error {
	// Check for a TPM 2.0 device as it's needed to encrypt
	// Exposed by the kernel to userspace as /dev/tpmrm0 since kernel 4.12
	// https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/commit/?id=fdc915f7f71939ad5a3dda3389b8d2d7a7c5ee66
	_, err := os.Stat("/dev/tpmrm0")
	if err != nil {
		logger.Warnf("Could not find TPM 2.0 device at /dev/tpmrm0")
		return fmt.Errorf("could not find TPM 2.0 device at /dev/tpmrm0: %w", err)
	}

	logger.Logger.Info().Msg("TPM 2.0 device found at /dev/tpmrm0")
	return nil
}
