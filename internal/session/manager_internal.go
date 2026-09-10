package session

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/isamo09/FreedomToParrots/internal/procio"
	"github.com/isamo09/FreedomToParrots/internal/store"
)

const stopGrace = 5 * time.Second

func (m *Manager) isRunningLocked(id string) bool {
	_, ok := m.rt[id]

	return ok
}

// startLocked spawns the core process for s. Caller must hold m.mu.
func (m *Manager) startLocked(s *store.Session) error {
	if m.isRunningLocked(s.ID) {
		return nil
	}

	if m.corePath == "" {
		return fmt.Errorf("%w: %v", ErrNoCore, m.coreErr)
	}

	if s.LimitB > 0 && m.currentTrafficLocked(s.ID, s) >= s.LimitB {
		return ErrLimitHit
	}

	cfg, err := writeConfig(m.dirs, *s)
	if err != nil {
		return err
	}

	cmd, logf, err := newCoreCmd(m.corePath, cfg, m.dirs.Root, logPath(m.dirs, s.ID))
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		_ = logf.Close()

		return fmt.Errorf("запуск ядра: %w", err)
	}

	logBase := int64(0)
	if fi, err := logf.Stat(); err == nil {
		logBase = fi.Size()
	}

	rt := &runtime{pid: cmd.Process.Pid, startedAt: time.Now(), logBase: logBase, done: make(chan struct{})}
	m.rt[s.ID] = rt

	_ = os.WriteFile(pidPath(m.dirs, s.ID), []byte(strconv.Itoa(rt.pid)), 0o600)

	id := s.ID

	go func() {
		_ = cmd.Wait()
		_ = logf.Close()

		m.mu.Lock()
		if cur, ok := m.rt[id]; ok && cur.pid == rt.pid {
			if sess, ok := m.sessions[id]; ok {
				sess.TrafficB += cur.lastIO
			}

			delete(m.rt, id)
		}
		m.mu.Unlock()

		_ = os.Remove(pidPath(m.dirs, id))
		close(rt.done)
	}()

	return nil
}

// stopLocked stops s's process, if any, waiting up to stopGrace for a clean
// exit before forcing it. Caller must hold m.mu; it's released while
// waiting so the exit goroutine (which also needs the lock) can run.
func (m *Manager) stopLocked(s *store.Session) {
	rt, ok := m.rt[s.ID]
	if !ok {
		return
	}

	// Snapshot while the process can still be read, then zero the run's
	// delta so the exit goroutine (which unconditionally adds rt.lastIO to
	// TrafficB once the process is confirmed gone) doesn't double-count it.
	s.TrafficB = m.currentTrafficLocked(s.ID, s)
	rt.lastIO = 0

	pid, done := rt.pid, rt.done
	stopProcess(pid, true)

	m.mu.Unlock()

	select {
	case <-done:
	case <-time.After(stopGrace):
		stopProcess(pid, false)
		<-done
	}

	m.mu.Lock()
}

// currentTrafficLocked estimates total bytes moved by s: persisted history
// plus this run's contribution (if it's running and the platform can read
// process I/O counters). It also refreshes rt.lastIO, which is what a crash
// harvest falls back to once the process is gone and can't be read anymore.
func (m *Manager) currentTrafficLocked(id string, s *store.Session) int64 {
	rt, ok := m.rt[id]
	if !ok {
		return s.TrafficB
	}

	raw, ok := procio.Read(rt.pid)
	if !ok {
		return s.TrafficB
	}

	written := int64(0)
	if fi, err := os.Stat(logPath(m.dirs, id)); err == nil {
		if w := fi.Size() - rt.logBase; w > 0 {
			written = w
		}
	}

	adjusted := raw - written/2
	if adjusted < 0 {
		adjusted = 0
	}

	cur := adjusted - rt.ioOffset
	if cur < 0 {
		cur = 0
	}

	rt.lastIO = cur

	return s.TrafficB + cur
}

func (m *Manager) publicLocked(s *store.Session) Public {
	running := m.isRunningLocked(s.ID)

	var startedAt int64

	if rt, ok := m.rt[s.ID]; ok {
		startedAt = rt.startedAt.Unix()
	}

	var lastConn int64
	if t := lastConnection(logPath(m.dirs, s.ID)); !t.IsZero() {
		lastConn = t.Unix()
	}

	var retryIn int

	var startErr string

	if r, ok := m.restart[s.ID]; ok {
		startErr = r.lastErr

		if !running && s.Enabled && !s.Limited {
			if d := time.Until(r.next); d > 0 {
				retryIn = int(d.Seconds()) + 1
			}
		}
	}

	fi, _ := os.Stat(logPath(m.dirs, s.ID))

	var logSize int64
	if fi != nil {
		logSize = fi.Size()
	}

	return Public{
		ID: s.ID, Name: s.Name, Provider: s.Provider, Transport: s.Transport, Room: s.Room,
		Enabled: s.Enabled, Debug: s.Debug, Running: running, Limited: s.Limited,
		LimitB: s.LimitB, TrafficB: m.currentTrafficLocked(s.ID, s), TrafficKnown: procio.Supported(),
		CreatedAt: s.CreatedAt, StartedAt: startedAt, LastConnection: lastConn,
		Restarts: s.Restarts, RetryInSeconds: retryIn, StartError: startErr, LogSizeB: logSize,
	}
}
