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
// merges with cmdline, and extracts the kcrypt configuration.
// Returns nil if no kcrypt config is found.
func ScanKcryptConfig(logger types.KairosLogger, dirs ...string) *bus.KcryptConfig {
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
	logger.Debugf("ScanKcryptConfig: struct is: %#v", collectorConfig.Values)

	result := ExtractKcryptConfigFromCollector(*collectorConfig, logger)
	if result != nil {
		logger.Debugf("ScanKcryptConfig: extracted kcrypt config - challenger_server=%s", result.ChallengerServer)
	} else {
		logger.Debugf("ScanKcryptConfig: no kcrypt config found in collector results")
	}

	return result
}

// ExtractKcryptConfigFromCollector extracts kcrypt configuration from a collector.Config
// This works with kairos-agent which uses collector to merge file and cmdline configs
func ExtractKcryptConfigFromCollector(collectorConfig collector.Config, log types.KairosLogger) *bus.KcryptConfig {
	if collectorConfig.Values == nil {
		return nil
	}

	// First check for kairos.kcrypt.challenger (from cmdline like kairos.kcrypt.challenger_server=...)
	kairosVal, hasKairos := collectorConfig.Values["kairos"]
	if hasKairos {
		log.Debugf("ExtractKcryptConfig: found kairos key, type=%T", kairosVal)

		if kairosMap, ok := kairosVal.(collector.ConfigValues); ok {
			// Log the keys inside kairos to see what's there
			keys := make([]string, 0, len(kairosMap))
			for k, v := range kairosMap {
				keys = append(keys, k)
				log.Debugf("ExtractKcryptConfig: kairos.%s = %v (type=%T)", k, v, v)
			}
			log.Debugf("ExtractKcryptConfig: found kairos key with subkeys: %v", keys)

			kcryptVal, hasKcrypt := kairosMap["kcrypt"]
			if hasKcrypt {
				log.Debugf("ExtractKcryptConfig: found kcrypt key, type=%T", kcryptVal)
				if kcryptMap, ok := kcryptVal.(collector.ConfigValues); ok {
					result := extractChallengerConfig(kcryptMap)
					if result != nil {
						log.Debugf("ExtractKcryptConfig: successfully extracted challenger config from kairos.kcrypt")
					}
					return result
				} else {
					log.Debugf("ExtractKcryptConfig: kcrypt value is not ConfigValues, it's %T", kcryptVal)
				}
			} else {
				log.Debugf("ExtractKcryptConfig: no kcrypt key found under kairos")
			}
		} else {
			log.Debugf("ExtractKcryptConfig: kairos value is not ConfigValues, it's %T", kairosVal)
		}
	}

	// Fallback: check for kcrypt.challenger directly (from config files with kcrypt at top level)
	kcryptVal, hasKcrypt := collectorConfig.Values["kcrypt"]
	if hasKcrypt {
		log.Debugf("ExtractKcryptConfig: found kcrypt key at top level, type=%T", kcryptVal)
		if kcryptMap, ok := kcryptVal.(collector.ConfigValues); ok {
			result := extractChallengerConfig(kcryptMap)
			if result != nil {
				log.Debugf("ExtractKcryptConfig: successfully extracted challenger config from top-level kcrypt")
			}
			return result
		}
	}

	log.Debugf("ExtractKcryptConfig: no kcrypt config found anywhere")
	return nil
}

// extractChallengerConfig extracts kcrypt configuration from a kcrypt config map
func extractChallengerConfig(kcryptMap collector.ConfigValues) *bus.KcryptConfig {
	challengerVal, hasChallengerKey := kcryptMap["challenger"]
	if !hasChallengerKey {
		return nil
	}

	challengerMap, ok := challengerVal.(collector.ConfigValues)
	if !ok {
		return nil
	}

	config := &bus.KcryptConfig{}

	if server, ok := challengerMap["challenger_server"].(string); ok {
		config.ChallengerServer = server
	}
	if mdns, ok := challengerMap["mdns"].(bool); ok {
		config.MDNS = mdns
	}
	if cert, ok := challengerMap["certificate"].(string); ok {
		config.Certificate = cert
	}
	if nvIndex, ok := challengerMap["nv_index"].(string); ok {
		config.NVIndex = nvIndex
	}
	if cIndex, ok := challengerMap["c_index"].(string); ok {
		config.CIndex = cIndex
	}
	if tpmDevice, ok := challengerMap["tpm_device"].(string); ok {
		config.TPMDevice = tpmDevice
	}

	return config
}

// ExtractPCRBindingsFromCollector extracts bind-pcrs and bind-public-pcrs from collector config
// Returns the PCR bindings, with defaults if not found
func ExtractPCRBindingsFromCollector(collectorConfig collector.Config, log types.KairosLogger) (bindPCRs []string, bindPublicPCRs []string) {
	if collectorConfig.Values == nil {
		log.Debugf("ExtractPCRBindings: no config values")
		return nil, nil
	}

	// Try to extract bind-pcrs
	if bindPCRsVal, ok := collectorConfig.Values["bind-pcrs"]; ok {
		log.Debugf("ExtractPCRBindings: found bind-pcrs, type=%T", bindPCRsVal)
		// Handle both []string and []interface{} (from YAML unmarshaling)
		switch v := bindPCRsVal.(type) {
		case []string:
			bindPCRs = v
		case []interface{}:
			for _, item := range v {
				if str, ok := item.(string); ok {
					bindPCRs = append(bindPCRs, str)
				}
			}
		}
		log.Debugf("ExtractPCRBindings: extracted bind-pcrs=%v", bindPCRs)
	}

	// Try to extract bind-public-pcrs
	if bindPublicPCRsVal, ok := collectorConfig.Values["bind-public-pcrs"]; ok {
		log.Debugf("ExtractPCRBindings: found bind-public-pcrs, type=%T", bindPublicPCRsVal)
		// Handle both []string and []interface{} (from YAML unmarshaling)
		switch v := bindPublicPCRsVal.(type) {
		case []string:
			bindPublicPCRs = v
		case []interface{}:
			for _, item := range v {
				if str, ok := item.(string); ok {
					bindPublicPCRs = append(bindPublicPCRs, str)
				}
			}
		}
		log.Debugf("ExtractPCRBindings: extracted bind-public-pcrs=%v", bindPublicPCRs)
	}

	return bindPCRs, bindPublicPCRs
}

// ExtractKcryptConfigFromCmdline parses cmdline string and extracts kcrypt configuration
// This works with immucore which reads /proc/cmdline directly
// Returns nil if no kcrypt config is found in cmdline
func ExtractKcryptConfigFromCmdline(cmdline string) *bus.KcryptConfig {
	config := &bus.KcryptConfig{}
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
			config.ChallengerServer = value
			foundAny = true
		case "kairos.kcrypt.mdns":
			config.MDNS = value == "true"
			foundAny = true
		case "kairos.kcrypt.certificate":
			config.Certificate = value
			foundAny = true
		case "kairos.kcrypt.nv_index":
			config.NVIndex = value
			foundAny = true
		case "kairos.kcrypt.c_index":
			config.CIndex = value
			foundAny = true
		case "kairos.kcrypt.tpm_device":
			config.TPMDevice = value
			foundAny = true
		}
	}

	if !foundAny {
		return nil
	}

	return config
}
