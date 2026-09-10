// Package corebin embeds the olcrtc tunnel core binary (built from
// github.com/openlibrecommunity/olcrtc, see .github/workflows/release.yml)
// inside the fzp executable and extracts it to the data directory on first
// run. This is what makes a Freedom To Parrots build a single self-contained
// file: nothing else needs to be downloaded or installed alongside it.
package corebin

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// ErrPlaceholder is returned when the embedded binary is the development
// placeholder (see bin/core, bin/core.exe) rather than a real build - i.e.
// this binary wasn't produced by the release workflow.
var ErrPlaceholder = fmt.Errorf("no real core binary embedded (placeholder build) - " +
	"build via .github/workflows/release.yml or scripts/dev-build")

// placeholderMarker is the exact content scripts/*-placeholder files carry;
// used to give a clear error instead of silently shipping a dead binary.
const placeholderMarker = "freedomtoparrots-dev-placeholder\n"

// Extract writes the embedded core binary into dir (creating it if needed)
// and returns its path, ready to exec. It's a no-op if the file already
// present matches the embedded content, so restarts don't rewrite disk.
func Extract(dir string) (string, error) {
	if string(data) == placeholderMarker {
		return "", ErrPlaceholder
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}

	dst := filepath.Join(dir, execName)
	sum := sha256.Sum256(data)
	sumHex := hex.EncodeToString(sum[:])
	sumPath := dst + ".sha256"

	if existing, err := os.ReadFile(sumPath); err == nil && string(existing) == sumHex {
		if _, err := os.Stat(dst); err == nil {
			return dst, nil
		}
	}

	if err := os.WriteFile(dst, data, 0o700); err != nil { //nolint:gosec // core binary must be executable
		return "", fmt.Errorf("write %s: %w", dst, err)
	}

	if err := os.WriteFile(sumPath, []byte(sumHex), 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", sumPath, err)
	}

	return dst, nil
}
