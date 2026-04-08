package kcrypt

import (
	"testing"

	"github.com/kairos-io/kairos-sdk/types/partitions"
)

func TestLuksHeaderLabel(t *testing.T) {
	if got := luksHeaderLabel("COS_PERSISTENT"); got != "LUKS_COS_PERSISTENT" {
		t.Fatalf("luksHeaderLabel: got %q, want LUKS_COS_PERSISTENT", got)
	}
}

func TestResolvePartitionByLabel(t *testing.T) {
	const fsLabel = "COS_PERSISTENT"
	const headerLabel = "LUKS_COS_PERSISTENT"
	const mapperPath = "/dev/mapper/vda3"
	const rawPath = "/dev/vda3"

	tests := []struct {
		name           string
		blkid          map[string]string
		parts          []*partitions.Partition
		wantErr        bool
		wantPath       string
		wantLabelMatch string
	}{
		{
			name:  "unlocked: inner FS label resolves to mapper",
			blkid: map[string]string{fsLabel: mapperPath},
			parts: []*partitions.Partition{
				{FilesystemLabel: fsLabel, Path: mapperPath, Name: "vda3"},
			},
			wantPath:       mapperPath,
			wantLabelMatch: fsLabel,
		},
		{
			name:  "locked: only LUKS header label is visible, fallback used",
			blkid: map[string]string{headerLabel: rawPath},
			parts: []*partitions.Partition{
				{FilesystemLabel: headerLabel, Path: rawPath, Name: "vda3"},
			},
			wantPath:       rawPath,
			wantLabelMatch: headerLabel,
		},
		{
			name: "both labels visible: inner FS label takes precedence",
			blkid: map[string]string{
				fsLabel:     mapperPath,
				headerLabel: rawPath,
			},
			parts: []*partitions.Partition{
				{FilesystemLabel: headerLabel, Path: rawPath, Name: "vda3"},
				{FilesystemLabel: fsLabel, Path: mapperPath, Name: "vda3"},
			},
			wantPath:       mapperPath,
			wantLabelMatch: fsLabel,
		},
		{
			name:    "label not found anywhere",
			blkid:   map[string]string{},
			parts:   []*partitions.Partition{},
			wantErr: true,
		},
		{
			name:  "blkid finds device but ghw doesn't list it",
			blkid: map[string]string{fsLabel: mapperPath},
			parts: []*partitions.Partition{
				{FilesystemLabel: "COS_OEM", Path: "/dev/vda2"},
			},
			wantErr: true,
		},
		{
			name:    "ghw scan failed",
			blkid:   map[string]string{fsLabel: mapperPath},
			parts:   nil,
			wantErr: true,
		},
		{
			name:  "partition with empty Path/Name gets them populated from blkid",
			blkid: map[string]string{headerLabel: rawPath},
			parts: []*partitions.Partition{
				{FilesystemLabel: headerLabel},
			},
			wantPath:       rawPath,
			wantLabelMatch: headerLabel,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			blkidFn := func(label string) string { return tc.blkid[label] }
			listFn := func() []*partitions.Partition { return tc.parts }

			got, err := resolvePartitionByLabel(fsLabel, blkidFn, listFn)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got partition %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Path != tc.wantPath {
				t.Errorf("Path: got %q, want %q", got.Path, tc.wantPath)
			}
			if got.FilesystemLabel != tc.wantLabelMatch {
				t.Errorf("FilesystemLabel: got %q, want %q", got.FilesystemLabel, tc.wantLabelMatch)
			}
			if got.Name == "" {
				t.Errorf("Name should be populated, got empty")
			}
		})
	}
}

// TestResolvePartitionByLabel_LocksDoNotCollide is the regression test for
// kairos-io/kairos#4033: when only the LUKS header label exists (because the
// partition is still locked), lookup must still succeed via the LUKS_-prefixed
// fallback rather than failing or pointing at the wrong device.
func TestResolvePartitionByLabel_LocksDoNotCollide(t *testing.T) {
	const fsLabel = "COS_PERSISTENT"

	// Simulate a locked LUKS partition: blkid -L COS_PERSISTENT returns
	// nothing (no inner FS visible), but blkid -L LUKS_COS_PERSISTENT
	// returns the raw device.
	blkid := func(label string) string {
		if label == "LUKS_COS_PERSISTENT" {
			return "/dev/vda3"
		}
		return ""
	}
	list := func() []*partitions.Partition {
		return []*partitions.Partition{
			{FilesystemLabel: "LUKS_COS_PERSISTENT", Path: "/dev/vda3", Name: "vda3"},
		}
	}

	got, err := resolvePartitionByLabel(fsLabel, blkid, list)
	if err != nil {
		t.Fatalf("locked-partition lookup must succeed via header-label fallback, got: %v", err)
	}
	if got.Path != "/dev/vda3" {
		t.Errorf("Path: got %q, want /dev/vda3", got.Path)
	}
}
