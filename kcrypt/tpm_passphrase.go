package kcrypt

import (
	tpm "github.com/kairos-io/tpm-helpers"
	"github.com/mudler/yip/pkg/utils"
)

const (
	// DefaultLocalPassphraseNVIndex is the default TPM NV index for storing local passphrases
	DefaultLocalPassphraseNVIndex = "0x1500000"
)

// GetOrCreateLocalTPMPassphrase retrieves a passphrase from TPM NV memory, or generates and stores one if it doesn't exist.
// This is used for local encryption (non-UKI mode without remote KMS).
// Logic moved from kcrypt-challenger/cmd/discovery/client/enc.go
func GetOrCreateLocalTPMPassphrase(nvIndex, cIndex, tpmDevice string) (string, error) {
	// Use default NV index if not specified
	if nvIndex == "" {
		nvIndex = DefaultLocalPassphraseNVIndex
	}

	// Try to read existing passphrase from TPM NV memory
	opts := []tpm.TPMOption{tpm.WithIndex(nvIndex)}
	if tpmDevice != "" {
		opts = append(opts, tpm.WithDevice(tpmDevice))
	}

	encodedPass, err := tpm.ReadBlob(opts...)
	if err != nil {
		// Generate and store a new passphrase if reading fails
		return generateAndStoreLocalTPMPassphrase(nvIndex, cIndex, tpmDevice)
	}

	// Decode the passphrase
	decryptOpts := []tpm.TPMOption{}
	if cIndex != "" {
		decryptOpts = append(decryptOpts, tpm.WithIndex(cIndex))
	}
	if tpmDevice != "" {
		decryptOpts = append(decryptOpts, tpm.WithDevice(tpmDevice))
	}

	pass, err := tpm.DecryptBlob(encodedPass, decryptOpts...)
	return string(pass), err
}

// generateAndStoreLocalTPMPassphrase generates a new random passphrase and stores it in TPM NV memory
func generateAndStoreLocalTPMPassphrase(nvIndex, cIndex, tpmDevice string) (string, error) {
	opts := []tpm.TPMOption{}
	if tpmDevice != "" {
		opts = append(opts, tpm.WithDevice(tpmDevice))
	}
	if cIndex != "" {
		opts = append(opts, tpm.WithIndex(cIndex))
	}

	// Generate a new random passphrase
	rand := utils.RandomString(32)

	// Encrypt it with TPM
	blob, err := tpm.EncryptBlob([]byte(rand))
	if err != nil {
		return "", err
	}

	// Store in TPM NV memory
	if nvIndex == "" {
		nvIndex = DefaultLocalPassphraseNVIndex
	}
	opts = append(opts, tpm.WithIndex(nvIndex))

	return rand, tpm.StoreBlob(blob, opts...)
}
