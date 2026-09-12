// Package webui serves the Freedom To Parrots control panel: the login
// page, the session dashboard (single page, no pagination/tabs - see
// assets/panel.html) and the public client/download page. Templates and
// static assets live as separate files under assets/ for editing
// convenience and are baked into the binary via go:embed.
package webui

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"github.com/isamo09/FreedomToParrots/internal/security"
	"github.com/isamo09/FreedomToParrots/internal/session"
	"github.com/isamo09/FreedomToParrots/internal/update"
)

const cookieName = "ftp_sid"
const cookieMaxAge = 30 * 24 * time.Hour

// Server wires the session manager to HTTP. It owns login-cookie state;
// everything else lives in *session.Manager.
type Server struct {
	mgr      *session.Manager
	password string
	updates  *update.Tracker
	shutdown context.CancelFunc

	mu   sync.Mutex
	sids map[string]time.Time
}

// New builds the HTTP handler for the panel. password is the current login
// password (see internal/store.Settings). shutdown triggers the same
// graceful-shutdown path Ctrl+C does - handleUpdateApply calls it once a
// downloaded update has been installed, so the new version starts on the
// next launch.
func New(mgr *session.Manager, password string, updates *update.Tracker, shutdown context.CancelFunc) *Server {
	return &Server{mgr: mgr, password: password, updates: updates, shutdown: shutdown, sids: make(map[string]time.Time)}
}

// Handler returns the http.Handler to serve.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	assetsFS, _ := fs.Sub(assets, "assets")
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServerFS(assetsFS)))

	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("POST /login", s.handleLogin)
	mux.HandleFunc("GET /logout", s.handleLogout)
	mux.HandleFunc("GET /download", s.handleDownload)

	mux.HandleFunc("GET /api/state", s.auth(s.handleState))
	mux.HandleFunc("GET /api/sessions/{id}", s.auth(s.handleGet))
	mux.HandleFunc("GET /api/sessions/{id}/detail", s.auth(s.handleDetail))
	mux.HandleFunc("POST /api/sessions", s.auth(s.handleCreate))
	mux.HandleFunc("PATCH /api/sessions/{id}", s.auth(s.handleUpdate))
	mux.HandleFunc("POST /api/sessions/{id}/toggle", s.auth(s.handleToggle))
	mux.HandleFunc("POST /api/sessions/{id}/rekey", s.auth(s.handleRekey))
	mux.HandleFunc("POST /api/sessions/{id}/reset-traffic", s.auth(s.handleReset))
	mux.HandleFunc("DELETE /api/sessions/{id}", s.auth(s.handleDelete))
	mux.HandleFunc("POST /api/update/apply", s.auth(s.handleUpdateApply))

	return mux
}

// ── auth ─────────────────────────────────────────────────────────────────

func (s *Server) authorized(r *http.Request) bool {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	exp, ok := s.sids[c.Value]
	if !ok {
		return false
	}

	if time.Now().After(exp) {
		delete(s.sids, c.Value)

		return false
	}

	return true
}

// auth wraps an API handler so it always answers 401 JSON when not logged in.
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "нужен вход"})

			return
		}

		next(w, r)
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		serveAsset(w, "login.html")

		return
	}

	serveAsset(w, "panel.html")
}

func (s *Server) handleDownload(w http.ResponseWriter, _ *http.Request) {
	serveAsset(w, "download.html")
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?error=1", http.StatusSeeOther)

		return
	}

	if !security.Equal(r.FormValue("password"), s.password) {
		time.Sleep(time.Second) // make brute-forcing tedious

		http.Redirect(w, r, "/?error=1", http.StatusSeeOther)

		return
	}

	sid := security.NewToken(24)

	s.mu.Lock()
	s.sids[sid] = time.Now().Add(cookieMaxAge)
	s.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: sid, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteStrictMode, MaxAge: int(cookieMaxAge.Seconds()),
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		s.mu.Lock()
		delete(s.sids, c.Value)
		s.mu.Unlock()
	}

	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ── API ──────────────────────────────────────────────────────────────────

type stateResponse struct {
	Sessions   []session.Public `json:"sessions"`
	Problems   []string         `json:"problems"`
	Providers  []string         `json:"providers"`
	Transports []string         `json:"transports"`
	Update     update.Info      `json:"update"`
}

func (s *Server) handleState(w http.ResponseWriter, _ *http.Request) {
	snap := s.mgr.Snapshot()
	writeJSON(w, http.StatusOK, stateResponse{
		Sessions: snap.Sessions, Problems: snap.Problems,
		Providers: session.Providers, Transports: session.Transports,
		Update: s.updates.Snapshot(),
	})
}

// handleUpdateApply downloads and verifies the available update, replaces
// this executable with it, starts the new version, and - only once all of
// that has actually succeeded - triggers this instance's own graceful
// shutdown so the new one takes over. A failure here changes nothing on
// disk; this process just keeps running the version it was already on.
func (s *Server) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	if err := s.updates.Apply(r.Context()); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})

		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updating"})

	go func() {
		time.Sleep(500 * time.Millisecond) // let the response above actually reach the browser first
		s.shutdown()
	}()
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	pub, err := s.mgr.Get(r.PathValue("id"))
	writeResult(w, pub, err)
}

func (s *Server) handleDetail(w http.ResponseWriter, r *http.Request) {
	d, err := s.mgr.Detail(r.PathValue("id"))
	writeResult(w, d, err)
}

type sessionRequest struct {
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	Transport string `json:"transport"`
	Room      string `json:"room"`
	Limit     int64  `json:"limit"`
	Debug     bool   `json:"debug"`
}

func (req sessionRequest) fields() session.Fields {
	return session.Fields{
		Name: req.Name, Provider: req.Provider, Transport: req.Transport,
		Room: req.Room, LimitB: req.Limit, Debug: req.Debug,
	}
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req sessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "некорректный запрос"})

		return
	}

	pub, err := s.mgr.Create(req.fields())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

		return
	}

	writeJSON(w, http.StatusCreated, pub)
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	var req sessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "некорректный запрос"})

		return
	}

	pub, err := s.mgr.Update(r.PathValue("id"), req.fields())
	writeResult(w, pub, err)
}

func (s *Server) handleToggle(w http.ResponseWriter, r *http.Request) {
	pub, err := s.mgr.Toggle(r.PathValue("id"))
	writeResult(w, pub, err)
}

func (s *Server) handleRekey(w http.ResponseWriter, r *http.Request) {
	pub, err := s.mgr.Rekey(r.PathValue("id"))
	writeResult(w, pub, err)
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	pub, err := s.mgr.ResetTraffic(r.PathValue("id"))
	writeResult(w, pub, err)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.mgr.Delete(r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ── helpers ──────────────────────────────────────────────────────────────

func writeResult(w http.ResponseWriter, v any, err error) {
	if err != nil {
		code := http.StatusBadRequest
		if errors.Is(err, session.ErrNotFound) {
			code = http.StatusNotFound
		}

		writeJSON(w, code, map[string]string{"error": err.Error()})

		return
	}

	writeJSON(w, http.StatusOK, v)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func serveAsset(w http.ResponseWriter, name string) {
	data, err := fs.ReadFile(assets, "assets/"+name)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}
