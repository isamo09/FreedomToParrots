package session

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/isamo09/FreedomToParrots/internal/procio"
	"github.com/isamo09/FreedomToParrots/internal/security"
	"github.com/isamo09/FreedomToParrots/internal/store"
)

var (
	ErrNotFound = errors.New("нет такой сессии")
	ErrNoCore   = errors.New("бинарник ядра недоступен")
	ErrLimitHit = errors.New("лимит трафика исчерпан — сбросьте счётчик или поднимите лимит")
)

// runtime is the in-memory-only state for a live or recently-live process.
// Nothing here is persisted; a restart of fzp itself starts every enabled
// session fresh (see reclaimStray).
type runtime struct {
	pid       int
	startedAt time.Time
	logBase   int64
	ioOffset  int64
	lastIO    int64
	done      chan struct{}
}

// restartState tracks the crash-loop backoff for one session, independent
// of runtime (which only exists while a process is actually alive) - so the
// backoff survives the gap between "it just crashed" and "we retry it".
type restartState struct {
	fails   int
	next    time.Time
	lastErr string
}

// Manager owns every session's persisted record and live process.
type Manager struct {
	dirs     store.Dirs
	corePath string
	coreErr  error

	mu       sync.Mutex
	sessions map[string]*store.Session
	rt       map[string]*runtime
	restart  map[string]*restartState
}

// New builds a Manager and loads any sessions persisted from a previous run.
// corePath is the path to the extracted core binary; coreErr, if set,
// explains why it isn't available (sessions can still be created but won't
// start until this is fixed).
func New(dirs store.Dirs, corePath string, coreErr error) (*Manager, error) {
	loaded, err := store.LoadSessions(dirs.Root)
	if err != nil {
		return nil, err
	}

	m := &Manager{
		dirs:     dirs,
		corePath: corePath,
		coreErr:  coreErr,
		sessions: make(map[string]*store.Session, len(loaded)),
		rt:       make(map[string]*runtime, len(loaded)),
		restart:  make(map[string]*restartState, len(loaded)),
	}

	for i := range loaded {
		s := loaded[i]
		m.sessions[s.ID] = &s
	}

	return m, nil
}

// Restore reclaims any process left running by a previous instance (best
// effort - see reclaimStray) and starts every session marked enabled.
func (m *Manager) Restore() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	for _, id := range ids {
		m.reclaimStray(id)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	changed := false

	for _, s := range m.sessions {
		if s.Enabled {
			if err := m.startLocked(s); err != nil {
				s.Enabled = false
				changed = true
			}
		}
	}

	if changed {
		m.saveLocked()
	}
}

// reclaimStray kills whatever process a *.pid file from a previous run
// points at, on the assumption a process still using that config path is
// the same tunnel and would otherwise be duplicated by the fresh start.
func (m *Manager) reclaimStray(id string) {
	data, err := os.ReadFile(pidPath(m.dirs, id)) //nolint:gosec // path built from resolved data dir
	if err != nil {
		return
	}

	pid, err := strconv.Atoi(string(data))
	if err == nil && pid > 0 {
		killStray(pid)
		time.Sleep(200 * time.Millisecond)
	}

	_ = os.Remove(pidPath(m.dirs, id))
}

// List returns every session, oldest first.
func (m *Manager) List() []Public {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]Public, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, m.publicLocked(s))
	}

	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt < out[j].CreatedAt })

	return out
}

// Get returns one session's public view.
func (m *Manager) Get(id string) (Public, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok {
		return Public{}, ErrNotFound
	}

	return m.publicLocked(s), nil
}

// Create validates fields, persists a new session and starts it immediately.
func (m *Manager) Create(f Fields) (Public, error) {
	clean, err := cleanFields(f, Fields{})
	if err != nil {
		return Public{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	s := &store.Session{
		ID: newID(m.sessions), Key: security.NewToken(32), Enabled: true,
		Name: clean.Name, Provider: clean.Provider, Transport: clean.Transport,
		Room: clean.Room, LimitB: clean.LimitB, Debug: clean.Debug,
		CreatedAt: time.Now().Unix(),
	}
	m.sessions[s.ID] = s

	if err := m.startLocked(s); err != nil {
		s.Enabled = false
		m.saveLocked()

		return Public{}, fmt.Errorf("создано, но запустить не вышло: %w", err)
	}

	m.saveLocked()

	return m.publicLocked(s), nil
}

// Update edits an existing session. If it was running (or should be), it's
// restarted so the change takes effect immediately.
func (m *Manager) Update(id string, f Fields) (Public, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok {
		return Public{}, ErrNotFound
	}

	clean, err := cleanFields(f, fromStore(*s))
	if err != nil {
		return Public{}, err
	}

	wasRunning := m.isRunningLocked(id)
	if wasRunning {
		m.stopLocked(s)
	}

	s.Name, s.Provider, s.Transport = clean.Name, clean.Provider, clean.Transport
	s.Room, s.LimitB, s.Debug = clean.Room, clean.LimitB, clean.Debug

	if wasRunning || s.Enabled {
		if err := m.startLocked(s); err != nil {
			s.Enabled = false
			m.saveLocked()

			return Public{}, fmt.Errorf("сохранено, но запустить не вышло: %w", err)
		}
	}

	m.saveLocked()

	return m.publicLocked(s), nil
}

// Toggle flips a session between enabled/running and disabled/stopped.
func (m *Manager) Toggle(id string) (Public, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok {
		return Public{}, ErrNotFound
	}

	if s.Enabled && m.isRunningLocked(id) {
		m.stopLocked(s)
		s.Enabled = false
	} else {
		delete(m.restart, id) // manual restart - backoff counter starts fresh

		if err := m.startLocked(s); err != nil {
			m.saveLocked()

			return Public{}, err
		}

		s.Enabled = true
	}

	m.saveLocked()

	return m.publicLocked(s), nil
}

// Rekey replaces a session's encryption key (invalidating any device still
// configured with the old one) and restarts it if it was live.
func (m *Manager) Rekey(id string) (Public, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok {
		return Public{}, ErrNotFound
	}

	wasRunning := m.isRunningLocked(id)
	if wasRunning {
		m.stopLocked(s)
	}

	s.Key = security.NewToken(32)

	if wasRunning || s.Enabled {
		if err := m.startLocked(s); err != nil {
			s.Enabled = false
		}
	}

	m.saveLocked()

	return m.publicLocked(s), nil
}

// ResetTraffic zeroes a session's accumulated traffic counter.
func (m *Manager) ResetTraffic(id string) (Public, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok {
		return Public{}, ErrNotFound
	}

	s.TrafficB = 0
	s.Limited = false

	if rt := m.rt[id]; rt != nil && m.isRunningLocked(id) {
		raw, ok := procio.Read(rt.pid)
		if ok {
			rt.ioOffset = raw
		}
	}

	if s.Enabled && !m.isRunningLocked(id) {
		if err := m.startLocked(s); err != nil {
			s.Enabled = false
		}
	}

	m.saveLocked()

	return m.publicLocked(s), nil
}

// Delete stops and permanently removes a session, including its config, key
// and log files.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok {
		return ErrNotFound
	}

	m.stopLocked(s)
	delete(m.sessions, id)
	delete(m.rt, id)
	delete(m.restart, id)

	for _, p := range []string{cfgPath(m.dirs, id), keyPath(m.dirs, id), logPath(m.dirs, id), pidPath(m.dirs, id)} {
		_ = os.Remove(p)
	}

	m.saveLocked()

	return nil
}

// Detail returns the sensitive, on-demand data for one session's card.
func (m *Manager) Detail(id string) (Detail, error) {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()

		return Detail{}, ErrNotFound
	}

	uri := sessionURI(*s)
	key := s.Key
	m.mu.Unlock()

	return Detail{
		URI: uri,
		Key: key,
		QR:  qrDataURI(uri),
		Log: tailBytes(logPath(m.dirs, id), 8000),
	}, nil
}

// StopAll stops every running session; called on graceful shutdown.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, s := range m.sessions {
		m.stopLocked(s)
	}

	m.saveLocked()
}

func (m *Manager) saveLocked() {
	list := make([]store.Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		list = append(list, *s)
	}

	_ = store.SaveSessions(m.dirs.Root, list)
}

func newID(existing map[string]*store.Session) string {
	for {
		id := security.NewToken(3)
		if _, taken := existing[id]; !taken {
			return id
		}
	}
}
