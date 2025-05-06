//go:build windows

package types

import "io"

func isJournaldAvailable() bool {
	return false
}

func getJournaldWriter() io.Writer {
	return nil // No journald on Windows
}
