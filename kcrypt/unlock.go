package kcrypt

import (
	"fmt"
	"path/filepath"

	"github.com/anatol/luks.go"
	"github.com/jaypipes/ghw"
	"github.com/jaypipes/ghw/pkg/block"
	"github.com/kairos-io/kairos-sdk/kcrypt/bus"
	"github.com/kairos-io/kairos-sdk/types"
	"github.com/kairos-io/kairos-sdk/utils"
	"github.com/mudler/go-pluggable"
)

// UnlockAll Unlocks all encrypted devices found in the system.
// It automatically scans for kcrypt configuration from available sources (files + cmdline)
// and passes it to the kcrypt-challenger plugin.
func UnlockAll(tpm bool, log types.KairosLogger) error {
	return UnlockAllWithConfig(tpm, log, nil)
}

// UnlockAllWithConfig unlocks all encrypted devices with an explicit config.
// If config is nil, it will scan for configuration automatically.
func UnlockAllWithConfig(tpm bool, log types.KairosLogger, kcryptConfig *bus.DiscoveryPasswordPayload) error {
	bus.Manager.Initialize()
	logger := log.Logger

	// Scan for kcrypt config if not provided
	if kcryptConfig == nil {
		kcryptConfig = ScanKcryptConfig(log)
		if kcryptConfig != nil {
			logger.Info().
				Str("challenger_server", kcryptConfig.ChallengerServer).
				Bool("mdns", kcryptConfig.MDNS).
				Msg("Scanned kcrypt config for unlock")
		} else {
			logger.Debug().Msg("No kcrypt config found, using local encryption")
		}
	}

	blk, err := ghw.Block()
	if err != nil {
		logger.Warn().Msgf("Warning: Error reading partitions '%s \n", err.Error())

		return nil
	}

	if err := udevAdmTrigger(udevTimeout); err != nil {
		return err
	}

	for _, disk := range blk.Disks {
		for _, p := range disk.Partitions {
			if p.Type == "crypto_LUKS" {
				// Check if device is already mounted
				// We mount it under /dev/mapper/DEVICE, so It's pretty easy to check
				if !utils.Exists(filepath.Join("/dev", "mapper", p.Name)) {
					logger.Info().Msgf("Unmounted Luks found at '%s'", filepath.Join("/dev", p.Name))
					if tpm {
						out, err := utils.SH(fmt.Sprintf("/usr/lib/systemd/systemd-cryptsetup attach %s %s - tpm2-device=auto", p.Name, filepath.Join("/dev", p.Name)))
						if err != nil {
							logger.Warn().Msgf("Unlocking failed: '%s'", err.Error())
							logger.Warn().Msgf("Unlocking failed, command output: '%s'", out)
						} else {
							logger.Info().Msgf("Unlocking succeeded for '%s'", filepath.Join("/dev", p.Name))
						}
					} else {
						err = UnlockDiskWithConfig(p, kcryptConfig)
						if err != nil {
							logger.Warn().Msgf("Unlocking failed for '%s': '%s'", filepath.Join("/dev", p.Name), err.Error())
						} else {
							logger.Info().Msgf("Unlocking succeeded for '%s'", filepath.Join("/dev", p.Name))
						}
					}
				} else {
					logger.Info().Msgf("Device %s seems to be mounted at %s, skipping\n", filepath.Join("/dev", p.Name), filepath.Join("/dev", "mapper", p.Name))
				}

			}
		}
	}
	return nil
}

// UnlockDisk unlocks a single block.Partition.
// Deprecated: Use UnlockAll instead which handles config automatically.
func UnlockDisk(b *block.Partition) error {
	return UnlockDiskWithConfig(b, nil)
}

// UnlockDiskWithConfig unlocks a single block.Partition with explicit config.
func UnlockDiskWithConfig(b *block.Partition, kcryptConfig *bus.DiscoveryPasswordPayload) error {
	pass, err := getPassword(b, kcryptConfig)
	if err != nil {
		return fmt.Errorf("error retrieving password remotely: %w", err)
	}

	return luksUnlock(filepath.Join("/dev", b.Name), b.Name, pass)
}

// GetPassword gets the password for a block.Partition
// TODO: Ask to discovery a pass to unlock. keep waiting until we get it and a timeout is exhausted with retrials (exp backoff).
func getPassword(b *block.Partition, kcryptConfig *bus.DiscoveryPasswordPayload) (password string, err error) {
	// Get a logger for debugging
	log := types.NewKairosLogger("kcrypt-getPassword", "info", false)
	defer log.Close()

	log.Logger.Info().
		Str("partition_name", b.Name).
		Str("partition_label", b.FilesystemLabel).
		Str("partition_uuid", b.UUID).
		Msg("Requesting password for partition")

	bus.Reload()

	bus.Manager.Response(bus.EventDiscoveryPassword, func(_ *pluggable.Plugin, r *pluggable.EventResponse) {
		password = r.Data
		if r.Errored() {
			err = fmt.Errorf("failed discovery: %s", r.Error)
			log.Logger.Error().Err(err).Msg("Plugin returned error")
		} else {
			log.Logger.Info().
				Int("password_length", len(password)).
				Str("partition", b.Name).
				Msg("DECRYPTION: Received password from plugin")
		}
	})

	var payload bus.DiscoveryPasswordPayload
	if kcryptConfig != nil {
		payload = *kcryptConfig
		log.Logger.Info().
			Str("challenger_server", payload.ChallengerServer).
			Msg("Using provided kcrypt config")
	} else {
		log.Logger.Info().Msg("No kcrypt config provided, using local encryption")
	}
	payload.Partition = b

	_, err = bus.Manager.Publish(bus.EventDiscoveryPassword, payload)
	if err != nil {
		log.Logger.Error().Err(err).Msg("Failed to publish event to bus")
		return password, err
	}

	if password == "" {
		log.Logger.Error().Msg("Received empty password from plugin")
		return password, fmt.Errorf("received empty password")
	}

	log.Logger.Info().Msg("Password retrieval successful")
	return
}

func luksUnlock(device, mapper, password string) error {
	dev, err := luks.Open(device)
	if err != nil {
		// handle error
		return err
	}
	defer dev.Close()
	err = dev.Unlock(0, []byte(password), mapper)
	if err != nil {
		return err
	}
	return nil
}
