package kcrypt

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/gofrs/uuid"
	"github.com/jaypipes/ghw"
	"github.com/jaypipes/ghw/pkg/block"
	"github.com/kairos-io/kairos-sdk/kcrypt/bus"
	"github.com/kairos-io/kairos-sdk/types"
	"github.com/kairos-io/kairos-sdk/utils"
)

const udevTimeout = 30 * time.Second

// Encrypt is the entrypoint to encrypt a partition with LUKS.
// It automatically scans for kcrypt configuration.
func Encrypt(label string, logger types.KairosLogger, argsCreate ...string) (string, error) {
	return EncryptWithConfig(label, logger, nil, argsCreate...)
}

// EncryptWithConfig encrypts a partition with explicit kcrypt config.
// If config is nil, it will scan for configuration automatically.
func EncryptWithConfig(label string, logger types.KairosLogger, kcryptConfig *bus.DiscoveryPasswordPayload, argsCreate ...string) (string, error) {
	return luksifyWithConfig(label, logger, kcryptConfig, argsCreate...)
}

// EncryptWithPcrs is the entrypoint to encrypt a partition with LUKS and bind it to PCRs.
func EncryptWithPcrs(label string, publicKeyPcrs []string, pcrs []string, logger types.KairosLogger, argsCreate ...string) error {
	return luksifyMeasurements(label, publicKeyPcrs, pcrs, logger, argsCreate...)
}

// EncryptWithLocalTPMPassphrase encrypts a partition using a passphrase stored in TPM NV memory.
// This bypasses the plugin bus and directly uses kairos-sdk TPM functions.
// Used for non-UKI local encryption (without remote KMS).
func EncryptWithLocalTPMPassphrase(label string, nvIndex, cIndex, tpmDevice string, logger types.KairosLogger, argsCreate ...string) (string, error) {
	logger.Logger.Info().Str("partition", label).Msg("Encrypting with local TPM NV passphrase")

	// Get or create passphrase from TPM NV memory
	passphrase, err := GetOrCreateLocalTPMPassphrase(nvIndex, cIndex, tpmDevice)
	if err != nil {
		return "", fmt.Errorf("failed to get/create local TPM passphrase: %w", err)
	}

	logger.Logger.Info().
		Str("partition", label).
		Int("passphrase_length", len(passphrase)).
		Msg("Retrieved passphrase from local TPM NV memory")

	// Now encrypt using the passphrase (same logic as luksifyWithConfig but without plugin)
	return luksifyWithPassphrase(label, passphrase, logger, argsCreate...)
}

// luksifyWithPassphrase encrypts a partition with an explicit passphrase (no plugin involved)
func luksifyWithPassphrase(label string, passphrase string, logger types.KairosLogger, argsCreate ...string) (string, error) {
	logger.Logger.Info().Msg("Running udevadm settle")
	if err := udevAdmTrigger(udevTimeout); err != nil {
		return "", err
	}

	logger.Logger.Info().Str("label", label).Msg("Finding partition")
	part, b, err := findPartition(label)
	if err != nil {
		logger.Err(err).Msg("find partition")
		return "", err
	}

	mapper := fmt.Sprintf("/dev/mapper/%s", b.Name)
	device := fmt.Sprintf("/dev/%s", part)

	extraArgs := []string{"--uuid", uuid.NewV5(uuid.NamespaceURL, label).String()}
	extraArgs = append(extraArgs, "--label", label)
	extraArgs = append(extraArgs, argsCreate...)

	// Unmount the device if it's mounted before attempting to encrypt it
	logger.Logger.Info().Str("device", device).Msg("Checking if device is mounted")
	if err := unmountIfMounted(device, logger); err != nil {
		logger.Err(err).Msg("unmount device")
		return "", err
	}

	logger.Logger.Info().Str("device", device).Msg("Creating LUKS container")
	if err := createLuks(device, passphrase, extraArgs...); err != nil {
		logger.Err(err).Msg("create luks")
		return "", err
	}

	logger.Logger.Info().Str("device", device).Str("label", label).Msg("Formatting LUKS container")
	err = formatLuks(device, b.Name, mapper, label, passphrase, logger)
	if err != nil {
		logger.Err(err).Msg("format luks")
		return "", err
	}

	logger.Logger.Info().Str("label", label).Msg("Partition encryption completed")
	return fmt.Sprintf("%s:%s:%s", b.FilesystemLabel, b.Name, b.UUID), nil
}

// unmountIfMounted checks if a device is mounted and unmounts it if needed
// This is necessary because cryptsetup cannot format a mounted partition
func unmountIfMounted(device string, logger types.KairosLogger) error {
	// Read /proc/mounts to check if the device is mounted
	// mount entries look like: /dev/sda6 / ext4 rw,relatime 0 0
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return fmt.Errorf("failed to open /proc/mounts: %w", err)
	}
	defer f.Close()

	var mountPoint string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		// fields[0] is device, fields[1] is mount point
		if len(fields) >= 2 && fields[0] == device {
			mountPoint = fields[1]
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading /proc/mounts: %w", err)
	}

	// If device is not mounted, nothing to do
	if mountPoint == "" {
		return nil
	}

	logger.Logger.Debug().Str("device", device).Str("mountpoint", mountPoint).Msg("Device is mounted, unmounting before encryption")
	// Unmount using syscall.Unmount with flags=0 (standard unmount)
	if err := syscall.Unmount(mountPoint, 0); err != nil {
		return fmt.Errorf("failed to unmount %s from %s: %w", device, mountPoint, err)
	}

	logger.Logger.Debug().Str("device", device).Msg("Successfully unmounted device")
	return nil
}

func createLuks(dev, password string, cryptsetupArgs ...string) error {
	args := []string{"luksFormat", "--type", "luks2", "--iter-time", "5", "-q", dev}
	args = append(args, cryptsetupArgs...)
	cmd := exec.Command("cryptsetup", args...)
	cmd.Stdin = strings.NewReader(password)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func getRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

// luksify Take a part label, and recreates it with LUKS. IT OVERWRITES DATA!.
// On success, it returns a machine parseable string with the partition information
// (label:name:uuid) so that it can be stored by the caller for later use.
// This is because the label of the encrypted partition is not accessible unless
// the partition is decrypted first and the uuid changed after encryption so
// any stored information needs to be updated (by the caller).
func luksify(label string, logger types.KairosLogger, argsCreate ...string) (string, error) {
	return luksifyWithConfig(label, logger, nil, argsCreate...)
}

func luksifyWithConfig(label string, logger types.KairosLogger, kcryptConfig *bus.DiscoveryPasswordPayload, argsCreate ...string) (string, error) {
	var pass string

	logger.Logger.Info().Msg("Running udevadm settle")
	if err := udevAdmTrigger(udevTimeout); err != nil {
		return "", err
	}

	logger.Logger.Info().Str("label", label).Msg("Finding partition")
	part, b, err := findPartition(label)
	if err != nil {
		logger.Err(err).Msg("find partition")
		return "", err
	}

	// Scan for config if not provided
	if kcryptConfig == nil {
		kcryptConfig = ScanKcryptConfig(logger)
	}

	logger.Logger.Info().Str("partition", label).Msg("Getting password from kcrypt-challenger")
	pass, err = getPassword(b, kcryptConfig)
	if err != nil {
		logger.Err(err).Msg("get password")
		return "", err
	}

	// Log that we received a passphrase (without revealing it)
	logger.Logger.Info().
		Str("partition", label).
		Int("passphrase_length", len(pass)).
		Msg("ENCRYPTION: Received passphrase from kcrypt-challenger")

	mapper := fmt.Sprintf("/dev/mapper/%s", b.Name)
	device := fmt.Sprintf("/dev/%s", part)

	extraArgs := []string{"--uuid", uuid.NewV5(uuid.NamespaceURL, label).String()}
	extraArgs = append(extraArgs, "--label", label)
	extraArgs = append(extraArgs, argsCreate...)

	// Unmount the device if it's mounted before attempting to encrypt it
	logger.Logger.Info().Str("device", device).Msg("Checking if device is mounted")
	if err := unmountIfMounted(device, logger); err != nil {
		logger.Err(err).Msg("unmount device")
		return "", err
	}

	logger.Logger.Info().Str("device", device).Msg("Creating LUKS container")
	if err := createLuks(device, pass, extraArgs...); err != nil {
		logger.Err(err).Msg("create luks")
		return "", err
	}

	logger.Logger.Info().Str("device", device).Str("label", label).Msg("Formatting LUKS container")
	err = formatLuks(device, b.Name, mapper, label, pass, logger)
	if err != nil {
		logger.Err(err).Msg("format luks")
		return "", err
	}

	logger.Logger.Info().Str("label", label).Msg("Partition encryption completed")
	return fmt.Sprintf("%s:%s:%s", b.FilesystemLabel, b.Name, b.UUID), nil
}

// luksifyMeasurements takes a label and a list if public-keys and pcrs to bind and uses the measurements.
// in the current node to encrypt the partition with those and bind those to the given pcrs
// this expects systemd 255 as it needs the SRK public key that systemd extracts
// Sets a random password, enrolls the policy, unlocks and formats the partition, closes it and tfinally removes the random password from it
// Note that there is a diff between the publicKeyPcrs and normal Pcrs
// The former links to a policy type that allows anything signed by that policy to unlcok the partitions so its
// really useful for binding to PCR11 which is the UKI measurements in order to be able to upgrade the system and still be able
// to unlock the partitions.
// The later binds to a SINGLE measurement, so if that changes, it will not unlock anything.
// This is useful for things like PCR7 which measures the secureboot state and certificates if you dont expect those to change during
// the whole lifetime of a machine
// It can also be used to bind to things like the firmware code or efi drivers that we dont expect to change
// default for publicKeyPcrs is 11
// default for pcrs is nothing, so it doesn't bind as we want to expand things like DBX and be able to blacklist certs and such.
func luksifyMeasurements(label string, publicKeyPcrs []string, pcrs []string, logger types.KairosLogger, argsCreate ...string) error {
	if err := udevAdmTrigger(udevTimeout); err != nil {
		return err
	}

	part, b, err := findPartition(label)
	if err != nil {
		return err
	}

	// On TPM locking we generate a random password that will only be used here then discarded.
	// only unlocking method will be PCR values
	pass := getRandomString(32)
	mapper := fmt.Sprintf("/dev/mapper/%s", b.Name)
	device := fmt.Sprintf("/dev/%s", part)

	extraArgs := []string{"--uuid", uuid.NewV5(uuid.NamespaceURL, label).String()}
	extraArgs = append(extraArgs, "--label", label)
	extraArgs = append(extraArgs, argsCreate...)

	// Unmount the device if it's mounted before attempting to encrypt it
	if err := unmountIfMounted(device, logger); err != nil {
		logger.Err(err).Msg("unmount device")
		return err
	}

	if err := createLuks(device, pass, extraArgs...); err != nil {
		return err
	}

	if len(publicKeyPcrs) == 0 {
		publicKeyPcrs = []string{"11"}
	}

	syscall.Sync()

	// Enroll PCR policy as a keyslot
	// We pass the current signature of the booted system to confirm that we would be able to unlock with the current booted system
	// That checks the policy against the signatures and fails if a UKI with those signatures wont be able to unlock the device
	// Files are generated by systemd automatically and are extracted from the UKI binary directly
	// public pem cert -> .pcrpkey section fo the elf file
	// signatures -> .pcrsig section of the elf file
	args := []string{
		"--tpm2-public-key=/run/systemd/tpm2-pcr-public-key.pem",
		fmt.Sprintf("--tpm2-public-key-pcrs=%s", strings.Join(publicKeyPcrs, "+")),
		fmt.Sprintf("--tpm2-pcrs=%s", strings.Join(pcrs, "+")),
		"--tpm2-signature=/run/systemd/tpm2-pcr-signature.json",
		"--tpm2-device-key=/run/systemd/tpm2-srk-public-key.tpm2b_public",
		device}
	logger.Logger.Debug().Str("args", strings.Join(args, " ")).Msg("running command")
	cmd := exec.Command("systemd-cryptenroll", args...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("PASSWORD=%s", pass), "SYSTEMD_LOG_LEVEL=debug") // cannot pass it via stdin
	// Store the output into a buffer to log it in case we need it
	// debug output goes to stderr for some reason?
	stdOut := bytes.Buffer{}
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdOut
	err = cmd.Run()
	if err != nil {
		logger.Logger.Debug().Str("output", stdOut.String()).Msg("debug from cryptenroll")
		logger.Err(err).Msg("Enrolling measurements")
		return err
	}

	logger.Logger.Debug().Str("output", stdOut.String()).Msg("debug from cryptenroll")

	err = formatLuks(device, b.Name, mapper, label, pass, logger)
	if err != nil {
		logger.Err(err).Msg("format luks")
		return err
	}

	// Delete password slot from luks device
	out, err := utils.SH(fmt.Sprintf("systemd-cryptenroll --wipe-slot=password %s", device))
	if err != nil {
		logger.Err(err).Str("out", out).Msg("Removing password")
		return err
	}
	return nil
}

// format luks will unlock the device, wait for it and then format it
// device is the actual /dev/X luks device
// label is the label we will set to the formatted partition
// password is the pass to unlock the device to be able to format the underlying mapper.
func formatLuks(device, name, mapper, label, pass string, logger types.KairosLogger) error {
	l := logger.Logger.With().Str("device", device).Str("name", name).Str("mapper", mapper).Logger()
	l.Debug().Msg("unlock")
	if err := luksUnlock(device, name, pass); err != nil {
		return fmt.Errorf("unlock err: %w", err)
	}

	l.Debug().Msg("wait device")
	if err := waitDevice(mapper, 10); err != nil {
		return fmt.Errorf("waitdevice err: %w", err)
	}

	l.Debug().Msg("format")
	cmdFormat := fmt.Sprintf("mkfs.ext4 -L %s %s", label, mapper)
	out, err := utils.SH(cmdFormat)
	if err != nil {
		return fmt.Errorf("mkfs err: %w, out: %s", err, out)
	}

	// Refresh needs the password as its doing actions on the device directly
	l.Debug().Msg("discards")
	// Note: cryptsetup v2.8+ expects the device name (not the device path) for the 'refresh' command.
	// Using 'name' with v2.7 also works, hence why no fallback is needed for backward compatibility.
	cmd := exec.Command("cryptsetup", "refresh", "--persistent", "--allow-discards", name)
	cmd.Stdin = strings.NewReader(pass)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("refresh err: %w, out: %s", err, string(output))
	}

	l.Debug().Msg("close")
	out, err = utils.SH(fmt.Sprintf("cryptsetup close %s", mapper))
	if err != nil {
		return fmt.Errorf("lock err: %w, out: %s", err, out)
	}

	return nil
}

func findPartition(label string) (string, *block.Partition, error) {
	b, err := ghw.Block()
	if err == nil {
		for _, disk := range b.Disks {
			for _, p := range disk.Partitions {
				if p.FilesystemLabel == label {
					return p.Name, p, nil
				}

			}
		}
	} else {
		return "", nil, err
	}

	return "", nil, fmt.Errorf("not found label %s", label)
}

func waitDevice(device string, attempts int) error {
	for tries := 0; tries < attempts; tries++ {
		_, err := utils.SH("udevadm settle")
		if err != nil {
			return err
		}
		syscall.Sync()
		_, err = os.Lstat(device)
		if !os.IsNotExist(err) {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("no device found %s", device)
}

// udevAdmTrigger runs `udevadm trigger` and waits for the results to be
// visible. It returns an error if the command fails or if the "settle"
// timeout is exceeded.
func udevAdmTrigger(timeout time.Duration) error {
	// Make sure ghw will see all partitions correctly.
	// older versions don't have --type=all. Try the simpler version then.
	out, err := utils.SH("udevadm trigger --type=all || udevadm trigger")
	if err != nil {
		return fmt.Errorf("udevadm trigger failed: %w, out: %s", err, out)
	}
	syscall.Sync()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "udevadm", "settle")
	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("udevadm settle timed out after %s", timeout)
	}
	if err != nil {
		return fmt.Errorf("udevadm settle failed: %v (output: %s)", err, string(output))
	}

	return nil
}
