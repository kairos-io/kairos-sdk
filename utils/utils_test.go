package utils

import "testing"

func TestGetEfiGrubFiles(t *testing.T) {
	tests := []struct {
		arch     string
		expected string
	}{
		{
			arch:     "amd64",
			expected: "/usr/share/efi/x86_64/grub.efi",
		},
		{
			arch:     "arm64",
			expected: "/usr/share/efi/aarch64/grub.efi",
		},
		{
			arch:     "riscv64",
			expected: "/usr/lib/grub/riscv64-efi/grubriscv64.efi",
		},
	}

	for _, tt := range tests {
		files := GetEfiGrubFiles(tt.arch)
		if len(files) == 0 {
			t.Fatalf("GetEfiGrubFiles(%q) returned no files", tt.arch)
		}

		found := false
		for _, file := range files {
			if file == tt.expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("GetEfiGrubFiles(%q) did not include %q; got %v", tt.arch, tt.expected, files)
		}
	}
}

func TestGetEfiShimFiles(t *testing.T) {
	arm64Files := GetEfiShimFiles("arm64")
	if len(arm64Files) == 0 {
		t.Fatal("GetEfiShimFiles(\"arm64\") returned no files")
	}

	riscv64Files := GetEfiShimFiles("riscv64")
	if len(riscv64Files) != 0 {
		t.Fatalf("GetEfiShimFiles(\"riscv64\") = %v, want no shim paths", riscv64Files)
	}
}
