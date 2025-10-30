package kcrypt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anatol/luks.go"
	"github.com/kairos-io/kairos-sdk/types"
	"github.com/kairos-io/kairos-sdk/utils"
)

func luksUnlock(device, mapper, password string, logger *types.KairosLogger) error {
	// Check if device exists and is accessible
	if _, err := os.Stat(device); err != nil {
		return fmt.Errorf("device not accessible: %v", err)
	}

	// Construct mapper path
	mapperPath := filepath.Join("/dev", "mapper", mapper)

	// Check if mapper already exists
	if _, err := os.Stat(mapperPath); err == nil {
		// Already unlocked
		if logger != nil {
			logger.Logger.Debug().Str("mapper", mapperPath).Msg("Mapper already exists")
		}
		return nil
	}

	// Try to unlock with retries - the luks.go library sometimes has timing issues
	// when unlocking multiple partitions in sequence
	var dev luks.Device
	var unlockErr error
	maxRetries := 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
			if err := udevAdmSettle(logger, 10*time.Second); err != nil {
				if logger != nil {
					logger.Logger.Warn().
						Int("attempt", attempt+1).
						Err(err).Msg("Failed to settle")
				}
			}
		}

		dev, unlockErr = luks.Open(device)
		if unlockErr != nil {
			if logger != nil {
				logger.Logger.Warn().
					Int("attempt", attempt+1).
					Int("max_retries", maxRetries).
					Str("device", device).
					Err(unlockErr).
					Msg("Failed to open device")
			}

			continue
		}

		// Try to unlock
		unlockErr = dev.Unlock(0, []byte(password), mapper)
		if unlockErr != nil {
			_ = dev.Close() // Close on error
			if logger != nil {
				logger.Logger.Warn().
					Int("attempt", attempt+1).
					Int("max_retries", maxRetries).
					Str("device", device).
					Err(unlockErr).
					Msg("Failed to unlock device")
			}

			continue
		}

		// Success! Close the device handle immediately to release the file descriptor
		_ = dev.Close()
		if logger != nil {
			logger.Logger.Debug().Str("device", device).Msg("Successfully unlocked")
		}
		break
	}

	// If all retries failed, return the error
	if unlockErr != nil {
		return fmt.Errorf("LUKS unlock failed after %d attempts: %w", maxRetries, unlockErr)
	}

	// Wait for udev to settle and create the mapper device
	if err := udevAdmSettle(logger, 30*time.Second); err != nil {
		if logger != nil {
			logger.Logger.Warn().Err(err).Str("device", device).Msg("UdevAdmSettle failed")
		}
	}

	// Additional wait for dmsetup to register the device
	// Sometimes the filesystem node exists but dmsetup hasn't registered it yet
	for i := 0; i < 5; i++ {
		dmOutput, dmErr := utils.SH("dmsetup ls --target crypt")
		if dmErr == nil && strings.Contains(dmOutput, mapper) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Verify the mapper was created
	if _, err := os.Stat(mapperPath); err != nil {
		return fmt.Errorf("mapper device %s not created after unlock: %w", mapperPath, err)
	}

	return nil
}
