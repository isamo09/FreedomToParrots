package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/isamo09/FreedomToParrots/internal/store"
)

var (
	errUnknownProviderOrTransport = errors.New("неизвестный сервис или транспорт")
	errTelemostTransport          = errors.New("Телемост не умеет этим транспортом, возьмите vp8channel")
	errMissingFields              = errors.New("нужны название и Room ID")
)

var telemostUnsupported = []string{"datachannel", "seichannel"} //nolint:gochecknoglobals // fixed enum

// Room-link patterns mirrored from the web UI's normRoom() in
// internal/webui/assets/panel.js - keep both sides in sync.
var roomPatterns = map[string][]*regexp.Regexp{ //nolint:gochecknoglobals // compiled once
	"telemost": {
		regexp.MustCompile(`telemost\.yandex\.ru/j/(\d+)`),
		regexp.MustCompile(`/j/(\d+)`),
	},
	"wbstream": {
		regexp.MustCompile(`stream\.wb\.ru/room/([\w-]+)`),
		regexp.MustCompile(`/room/([\w-]+)`),
	},
}

var (
	bareID = regexp.MustCompile(`^[\w-]+$`)
	anyURL = regexp.MustCompile(`https?://\S+`)
)

// normalizeRoom extracts a room id from a pasted invite link (or passes a
// bare id through). For jitsi the URL itself is the identifier.
func normalizeRoom(provider, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}

	if provider == "jitsi" {
		u := raw
		if m := anyURL.FindString(raw); m != "" {
			u = m
		}

		u, _, _ = strings.Cut(u, "#")
		u, _, _ = strings.Cut(u, "?")

		return strings.TrimRight(u, "/")
	}

	for _, pat := range roomPatterns[provider] {
		if m := pat.FindStringSubmatch(raw); m != nil {
			return m[1]
		}
	}

	if bareID.MatchString(raw) {
		return raw
	}

	if m := anyURL.FindString(raw); m != "" {
		m, _, _ = strings.Cut(m, "#")
		m, _, _ = strings.Cut(m, "?")
		m = strings.TrimRight(m, "/")

		if seg := lastSegment(m); seg != "" {
			return seg
		}
	}

	return raw
}

func lastSegment(s string) string {
	i := strings.LastIndexByte(s, '/')
	if i < 0 {
		return s
	}

	return s[i+1:]
}

// cleanFields validates and normalizes submitted fields, filling anything
// left blank from the existing record (existing may be zero-value for a new
// session).
func cleanFields(f Fields, existing Fields) (Fields, error) {
	out := f

	if out.Name == "" {
		out.Name = existing.Name
	}

	out.Name = truncate(strings.TrimSpace(out.Name), 40)

	if out.Provider == "" {
		out.Provider = existing.Provider
	}

	if out.Transport == "" {
		out.Transport = existing.Transport
	}

	if !slices.Contains(Providers, out.Provider) || !slices.Contains(Transports, out.Transport) {
		return Fields{}, errUnknownProviderOrTransport
	}

	if out.Provider == "telemost" && slices.Contains(telemostUnsupported, out.Transport) {
		return Fields{}, errTelemostTransport
	}

	rawRoom := out.Room
	if rawRoom == "" {
		rawRoom = existing.Room
	}

	out.Room = normalizeRoom(out.Provider, strings.TrimSpace(rawRoom))

	if out.Name == "" || out.Room == "" {
		return Fields{}, errMissingFields
	}

	if out.LimitB < 0 {
		out.LimitB = 0
	}

	return out, nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}

	return string(r[:n])
}

func transportBlock(transport string) string {
	switch transport {
	case "vp8channel":
		return "vp8:\n  fps: 30\n  batch_size: 64\n"
	case "seichannel":
		return "sei:\n  fps: 30\n  batch_size: 64\n  fragment_size: 900\n  ack_timeout_ms: 2000\n"
	case "videochannel":
		return "video:\n  codec: qrcode\n  width: 1080\n  height: 1080\n  fps: 30\n  bitrate: \"5000k\"\n  hw: none\n"
	default:
		return ""
	}
}

// writeConfig (re)writes the key file and YAML config for a session. Called
// on every start so edits take effect on the next (re)start, same as the
// Python prototype.
func writeConfig(dirs store.Dirs, s store.Session) (string, error) {
	keyPath := filepath.Join(dirs.Keys, s.ID+".key")
	if err := os.WriteFile(keyPath, []byte(s.Key+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("write key file: %w", err)
	}

	absKey, err := filepath.Abs(keyPath)
	if err != nil {
		absKey = keyPath
	}

	var body strings.Builder

	body.WriteString("# сгенерировано Freedom To Parrots, править вручную бессмысленно\n")
	body.WriteString("mode: srv\n")
	body.WriteString("auth:\n  provider: " + s.Provider + "\n")
	body.WriteString("room:\n  id: \"" + s.Room + "\"\n")
	body.WriteString("crypto:\n  key_file: " + absKey + "\n")
	body.WriteString("net:\n  transport: " + s.Transport + "\n  dns: \"8.8.8.8:53\"\n")
	body.WriteString(transportBlock(s.Transport))

	if s.Debug {
		body.WriteString("debug: true\n")
	}

	cfgPath := filepath.Join(dirs.Configs, s.ID+".yaml")
	if err := os.WriteFile(cfgPath, []byte(body.String()), 0o600); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}

	return cfgPath, nil
}

func keyPath(dirs store.Dirs, id string) string { return filepath.Join(dirs.Keys, id+".key") }
func cfgPath(dirs store.Dirs, id string) string { return filepath.Join(dirs.Configs, id+".yaml") }
func logPath(dirs store.Dirs, id string) string { return filepath.Join(dirs.Logs, id+".log") }
func pidPath(dirs store.Dirs, id string) string { return filepath.Join(dirs.Root, id+".pid") }

func sessionURI(s store.Session) string {
	return fmt.Sprintf("olcrtc://%s?%s@%s#%s$%s", s.Provider, s.Transport, s.Room, s.Key, s.Name)
}
