package state

import (
	"testing"

	"github.com/kairos-io/kairos-sdk/types/certs"
)

func TestRuntimeQuery(t *testing.T) {
	r := Runtime{
		UUID: "test-uuid",
		Kairos: Kairos{
			Flavor:  "kairos-flavor",
			Version: "1.0.0",
			EfiCerts: certs.EfiCerts{
				PK:  []string{"cert 1", "cert 2"},
				KEK: []string{"Test KEK Cert"},
				DB:  []string{"Test DB Cert"},
			},
		},
		BootState: "active_boot",
	}

	tests := []struct {
		name   string
		query  string
		expect string
	}{
		{"uuid field", "uuid", "test-uuid"},
		{"kairos flavor", "kairos.flavor", "kairos-flavor"},
		{"kairos version", "kairos.version", "1.0.0"},
		{"boot state", "boot", "active_boot"},
		{"eficerts", "kairos.eficerts.PK.[0]", "cert 1"},
		{"eficerts", "kairos.eficerts.PK.[1]", "cert 2"},
	}

	for _, tt := range tests {
		got, err := r.Query(tt.query)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tt.name, err)
			continue
		}
		if got != tt.expect {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.expect)
		}
	}
}
