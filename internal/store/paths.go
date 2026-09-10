// Package store resolves the on-disk data directory and persists panel
// settings and session records as JSON, mirroring the layout the Python
// prototype (panel.py) used: everything lives next to the binary so the
// whole install stays a single self-contained folder.
package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Dirs are the resolved, already-created runtime paths.
type Dirs struct {
	Root    string // .../data
	Configs string // .../data/configs
	Keys    string // .../data/keys
	Logs    string // .../data/logs
	Bin     string // .../data/bin - extracted core binary lives here
}

// Resolve finds (and creates) the data directory. It first tries a "data"
// folder next to the running executable - the portable, flash-drive-friendly
// layout. If that location isn't writable (e.g. installed under
// C:\Program Files), it falls back to the user's per-OS config directory.
func Resolve() (Dirs, string, error) {
	if dir, err := tryNextToExecutable(); err == nil {
		return dir, "", nil
	}

	dir, note, err := fallbackUserDir()
	if err != nil {
		return Dirs{}, "", err
	}

	return dir, note, nil
}

func tryNextToExecutable() (Dirs, error) {
	exe, err := os.Executable()
	if err != nil {
		return Dirs{}, err
	}

	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return Dirs{}, err
	}

	root := filepath.Join(filepath.Dir(exe), "data")

	return build(root)
}

func fallbackUserDir() (Dirs, string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		base, err = os.UserHomeDir()
		if err != nil {
			return Dirs{}, "", fmt.Errorf("resolve data directory: %w", err)
		}
	}

	root := filepath.Join(base, "FreedomToParrots")

	dirs, err := build(root)
	if err != nil {
		return Dirs{}, "", err
	}

	note := fmt.Sprintf("папка рядом с программой недоступна для записи — данные хранятся в %s", root)

	return dirs, note, nil
}

func build(root string) (Dirs, error) {
	d := Dirs{
		Root:    root,
		Configs: filepath.Join(root, "configs"),
		Keys:    filepath.Join(root, "keys"),
		Logs:    filepath.Join(root, "logs"),
		Bin:     filepath.Join(root, "bin"),
	}

	for _, p := range []string{d.Root, d.Configs, d.Keys, d.Logs, d.Bin} {
		if err := os.MkdirAll(p, 0o700); err != nil {
			return Dirs{}, fmt.Errorf("create %s: %w", p, err)
		}
	}

	// Probe writability: MkdirAll on an already-existing read-only tree can
	// silently succeed, so touch a file too.
	probe := filepath.Join(d.Root, ".write-test")
	if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
		return Dirs{}, fmt.Errorf("data directory not writable: %w", err)
	}

	_ = os.Remove(probe)

	return d, nil
}

// ErrNotWritable is returned by callers that need to distinguish "no
// permission" from other failures; kept for future callers/tests.
var ErrNotWritable = errors.New("data directory not writable")
