package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/isamo09/FreedomToParrots/internal/security"
)

// Settings holds panel-wide configuration, persisted as settings.json.
type Settings struct {
	Password      string `json:"password"`
	CheckInterval int    `json:"check_interval"` // seconds, traffic-limit watcher
	CreatedAt     int64  `json:"created_at"`
}

// Session is the persisted record for one tunnel/device. Runtime-only state
// (process handle, live traffic counters) is kept separately by the session
// manager and never written to disk.
type Session struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	Transport string `json:"transport"`
	Room      string `json:"room"`
	Key       string `json:"key"`
	Enabled   bool   `json:"enabled"`
	Debug     bool   `json:"debug"`
	LimitB    int64  `json:"limit"`   // 0 = unlimited
	TrafficB  int64  `json:"traffic"` // accumulated across past runs
	Limited   bool   `json:"limited"`
	CreatedAt int64  `json:"created"`
	Restarts  int    `json:"restarts"`
}

func settingsPath(root string) string { return filepath.Join(root, "settings.json") }
func sessionsPath(root string) string { return filepath.Join(root, "sessions.json") }

// LoadSettings reads settings.json, creating it with a fresh random password
// on first run (there is no "no password" mode - a run always gets a
// generated one so a freshly opened panel is never open to the network by
// accident).
func LoadSettings(root string) (Settings, bool, error) {
	p := settingsPath(root)

	data, err := os.ReadFile(p) //nolint:gosec // path built from resolved data dir
	if err == nil {
		var s Settings
		if jerr := json.Unmarshal(data, &s); jerr == nil && s.Password != "" {
			if s.CheckInterval <= 0 {
				s.CheckInterval = 20
			}

			return s, false, nil
		}
	}

	s := Settings{
		Password:      security.NewPassword(),
		CheckInterval: 20,
		CreatedAt:     time.Now().Unix(),
	}

	if err := SaveSettings(root, s); err != nil {
		return Settings{}, false, err
	}

	return s, true, nil
}

// SaveSettings writes settings.json atomically.
func SaveSettings(root string, s Settings) error {
	return writeJSONAtomic(settingsPath(root), s, 0o600)
}

// LoadSessions reads sessions.json, returning an empty slice if it doesn't exist yet.
func LoadSessions(root string) ([]Session, error) {
	data, err := os.ReadFile(sessionsPath(root)) //nolint:gosec // path built from resolved data dir
	if err != nil {
		if os.IsNotExist(err) {
			return []Session{}, nil
		}

		return nil, fmt.Errorf("read sessions.json: %w", err)
	}

	var out []Session
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("parse sessions.json: %w", err)
	}

	return out, nil
}

// SaveSessions writes sessions.json atomically.
func SaveSessions(root string, sessions []Session) error {
	return writeJSONAtomic(sessionsPath(root), sessions, 0o600)
}

func writeJSONAtomic(path string, v any, perm os.FileMode) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filepath.Base(path), err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}

	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace %s: %w", filepath.Base(path), err)
	}

	return nil
}
