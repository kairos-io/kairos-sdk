package kcrypt

import (
	"strings"

	"github.com/kairos-io/kairos-sdk/collector"
	"github.com/kairos-io/kairos-sdk/kcrypt/bus"
	"github.com/kairos-io/kairos-sdk/types"
)

// DefaultConfigDirs are the default directories to scan for Kairos configuration
var DefaultConfigDirs = []string{"/oem", "/sysroot/oem", "/run/cos/oem"}

// ScanKcryptConfig scans for Kairos configuration in the given directories (or defaults),
// merges with cmdline, and extracts just the kcrypt.challenger configuration.
// Returns nil if no kcrypt config is found.
func ScanKcryptConfig(logger types.KairosLogger, dirs ...string) *bus.DiscoveryPasswordPayload {
	if len(dirs) == 0 {
		dirs = DefaultConfigDirs
	}

	logger.Debugf("ScanKcryptConfig: scanning directories: %v", dirs)

	o := &collector.Options{NoLogs: true, MergeBootCMDLine: true}
	if err := o.Apply(collector.Directories(dirs...)); err != nil {
		logger.Debugf("ScanKcryptConfig: error applying collector options: %v", err)
		return nil
	}

	collectorConfig, err := collector.Scan(o, func(d []byte) ([]byte, error) {
		return d, nil
	})
	if err != nil {
		logger.Debugf("ScanKcryptConfig: error scanning for config: %v", err)
		return nil
	}

	if collectorConfig == nil {
		logger.Debugf("ScanKcryptConfig: collector returned nil config")
		return nil
	}

	logger.Debugf("ScanKcryptConfig: collector found config with %d keys", len(collectorConfig.Values))
	if len(collectorConfig.Values) > 0 {
		// Log the top-level keys
		keys := make([]string, 0, len(collectorConfig.Values))
		for k := range collectorConfig.Values {
			keys = append(keys, k)
		}
		logger.Debugf("ScanKcryptConfig: top-level keys: %v", keys)
	}

	result := ExtractKcryptConfigFromCollector(*collectorConfig)
	if result != nil {
		logger.Debugf("ScanKcryptConfig: extracted kcrypt config - challenger_server=%s", result.ChallengerServer)
	} else {
		logger.Debugf("ScanKcryptConfig: no kcrypt config found in collector results")
	}

	return result
}

// ExtractKcryptConfigFromCollector extracts kcrypt.challenger configuration from a collector.Config
// This works with kairos-agent which uses collector to merge file and cmdline configs
func ExtractKcryptConfigFromCollector(collectorConfig collector.Config) *bus.DiscoveryPasswordPayload {
	if collectorConfig.Values == nil {
		return nil
	}

	// First check for kairos.kcrypt.challenger (from cmdline like kairos.kcrypt.challenger_server=...)
	kairosVal, hasKairos := collectorConfig.Values["kairos"]
	if hasKairos {
		if kairosMap, ok := kairosVal.(collector.ConfigValues); ok {
			kcryptVal, hasKcrypt := kairosMap["kcrypt"]
			if hasKcrypt {
				if kcryptMap, ok := kcryptVal.(collector.ConfigValues); ok {
					return extractChallengerConfig(kcryptMap)
				}
			}
		}
	}

	// Fallback: check for kcrypt.challenger directly (from config files with kcrypt at top level)
	kcryptVal, hasKcrypt := collectorConfig.Values["kcrypt"]
	if hasKcrypt {
		if kcryptMap, ok := kcryptVal.(collector.ConfigValues); ok {
			return extractChallengerConfig(kcryptMap)
		}
	}

	return nil
}

// extractChallengerConfig extracts challenger configuration from a kcrypt config map
func extractChallengerConfig(kcryptMap collector.ConfigValues) *bus.DiscoveryPasswordPayload {
	challengerVal, hasChallengerKey := kcryptMap["challenger"]
	if !hasChallengerKey {
		return nil
	}

	challengerMap, ok := challengerVal.(collector.ConfigValues)
	if !ok {
		return nil
	}

	payload := &bus.DiscoveryPasswordPayload{}

	if server, ok := challengerMap["challenger_server"].(string); ok {
		payload.ChallengerServer = server
	}
	if mdns, ok := challengerMap["mdns"].(bool); ok {
		payload.MDNS = mdns
	}
	if cert, ok := challengerMap["certificate"].(string); ok {
		payload.Certificate = cert
	}
	if nvIndex, ok := challengerMap["nv_index"].(string); ok {
		payload.NVIndex = nvIndex
	}
	if cIndex, ok := challengerMap["c_index"].(string); ok {
		payload.CIndex = cIndex
	}
	if tpmDevice, ok := challengerMap["tpm_device"].(string); ok {
		payload.TPMDevice = tpmDevice
	}

	return payload
}

// ExtractKcryptConfigFromCmdline parses cmdline string and extracts kcrypt.challenger configuration
// This works with immucore which reads /proc/cmdline directly
// Returns nil if no kcrypt config is found in cmdline
func ExtractKcryptConfigFromCmdline(cmdline string) *bus.DiscoveryPasswordPayload {
	payload := &bus.DiscoveryPasswordPayload{}
	foundAny := false

	parts := strings.Fields(cmdline)
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, value := kv[0], kv[1]

		switch key {
		case "kairos.kcrypt.challenger_server":
			payload.ChallengerServer = value
			foundAny = true
		case "kairos.kcrypt.mdns":
			payload.MDNS = value == "true"
			foundAny = true
		case "kairos.kcrypt.certificate":
			payload.Certificate = value
			foundAny = true
		case "kairos.kcrypt.nv_index":
			payload.NVIndex = value
			foundAny = true
		case "kairos.kcrypt.c_index":
			payload.CIndex = value
			foundAny = true
		case "kairos.kcrypt.tpm_device":
			payload.TPMDevice = value
			foundAny = true
		}
	}

	if !foundAny {
		return nil
	}

	return payload
}
