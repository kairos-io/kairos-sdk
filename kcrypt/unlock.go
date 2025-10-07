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
func UnlockAll(tpm bool, log types.KairosLogger) error {
	bus.Manager.Initialize()
	logger := log.Logger

	blk, err := ghw.Block()
	if err != nil {
		logger.Warn().Msgf("Warning: Error reading partitions '%s \n", err.Error())

		return nil
	}

	if err := udevAdmTrigger(UdevTimeout); err != nil {
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
						}
					} else {
						err = UnlockDisk(p)
						if err != nil {
							logger.Warn().Msgf("Unlocking failed: '%s'", err.Error())
						}
						logger.Info().Msg("Unlocking succeeded")
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
func UnlockDisk(b *block.Partition) error {
	pass, err := getPassword(b)
	if err != nil {
		return fmt.Errorf("error retrieving password remotely: %w", err)
	}

	return luksUnlock(filepath.Join("/dev", b.Name), b.Name, pass)
}

// GetPassword gets the password for a block.Partition
// TODO: Ask to discovery a pass to unlock. keep waiting until we get it and a timeout is exhausted with retrials (exp backoff).
func getPassword(b *block.Partition) (password string, err error) {
	bus.Reload()

	bus.Manager.Response(bus.EventDiscoveryPassword, func(_ *pluggable.Plugin, r *pluggable.EventResponse) {
		password = r.Data
		if r.Errored() {
			err = fmt.Errorf("failed discovery: %s", r.Error)
		}
	})
	_, err = bus.Manager.Publish(bus.EventDiscoveryPassword, b)
	if err != nil {
		return password, err
	}

	if password == "" {
		return password, fmt.Errorf("received empty password")
	}

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
