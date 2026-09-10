// Package session manages olcrtc tunnel sessions: their on-disk config/key
// files, the core binary process for each, traffic accounting and the
// crash-loop supervisor. It's a Go port of the session-management half of
// the original panel.py prototype, with the Linux-only policy-routing
// feature (per-session network interface/uid binding) intentionally left
// out - it never worked outside rooted Linux and doesn't fit a tool whose
// whole point is "one binary, every OS".
package session

import "github.com/isamo09/FreedomToParrots/internal/store"

// Providers are the call platforms olcrtc can disguise traffic as.
var Providers = []string{"telemost", "jitsi", "wbstream"} //nolint:gochecknoglobals // fixed enum, mirrors core's auth providers

// Transports are the WebRTC channels traffic can ride over.
var Transports = []string{"vp8channel", "datachannel", "seichannel", "videochannel"} //nolint:gochecknoglobals // fixed enum

// Fields is the user-editable subset of a session, as submitted by the
// create/edit form.
type Fields struct {
	Name      string
	Provider  string
	Transport string
	Room      string // raw pasted link or bare room id - gets normalized
	LimitB    int64
	Debug     bool
}

// Public is the JSON shape sent to the web UI and console dashboard.
type Public struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Provider       string `json:"provider"`
	Transport      string `json:"transport"`
	Room           string `json:"room"`
	Enabled        bool   `json:"enabled"`
	Debug          bool   `json:"debug"`
	Running        bool   `json:"running"`
	Limited        bool   `json:"limited"`
	LimitB         int64  `json:"limit"`
	TrafficB       int64  `json:"traffic"`
	TrafficKnown   bool   `json:"traffic_known"`
	CreatedAt      int64  `json:"created"`
	StartedAt      int64  `json:"started"`
	LastConnection int64  `json:"last_connection"` // 0 = never
	Restarts       int    `json:"restarts"`
	RetryInSeconds int    `json:"retry_in"`
	StartError     string `json:"start_error,omitempty"`
	LogSizeB       int64  `json:"log_size"`
}

// Detail is the extra, sensitive-ish data shown only when a session card is
// opened explicitly (key, connection URI, QR, log tail).
type Detail struct {
	URI string `json:"uri"`
	Key string `json:"key"`
	QR  string `json:"qr"` // data: URI, empty if generation failed
	Log string `json:"log"`
}

func fromStore(s store.Session) Fields {
	return Fields{
		Name: s.Name, Provider: s.Provider, Transport: s.Transport,
		Room: s.Room, LimitB: s.LimitB, Debug: s.Debug,
	}
}
