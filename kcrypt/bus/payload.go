package bus

import "github.com/jaypipes/ghw/pkg/block"

// DiscoveryPasswordPayload is the data sent to kcrypt-challenger plugin.
// It contains both partition information and kcrypt configuration.
// The caller (kairos-agent/immucore) is responsible for collecting configuration
// from all available sources (files, cmdline) and passing it here.
type DiscoveryPasswordPayload struct {
	Partition *block.Partition `json:"partition"`
	// Kcrypt challenger configuration
	ChallengerServer string `json:"challenger_server,omitempty"`
	MDNS             bool   `json:"mdns,omitempty"`
	Certificate      string `json:"certificate,omitempty"`
	NVIndex          string `json:"nv_index,omitempty"`
	CIndex           string `json:"c_index,omitempty"`
	TPMDevice        string `json:"tpm_device,omitempty"`
}
