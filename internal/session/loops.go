package session

import (
	"context"
	"time"
)

const (
	loopInterval = 3 * time.Second
	stableAfter  = 60 * time.Second // a restarted session running this long resets its failure count
	maxBackoff   = 300 * time.Second
)

// RunLoops starts the background goroutines that keep sessions alive
// (supervisor) and enforce traffic limits (monitor). It returns
// immediately; both loops stop when ctx is cancelled.
func (m *Manager) RunLoops(ctx context.Context) {
	go m.supervisorLoop(ctx)
	go m.monitorLoop(ctx)
}

// supervisorLoop restarts sessions that are enabled but not running (they
// crashed, or the core binary rejected their config) with a growing backoff
// so a permanently broken config doesn't spin the CPU in a restart loop.
func (m *Manager) supervisorLoop(ctx context.Context) {
	t := time.NewTicker(loopInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.supervisorTick()
		}
	}
}

func (m *Manager) supervisorTick() {
	m.mu.Lock()
	defer m.mu.Unlock()

	changed := false
	now := time.Now()

	for id, s := range m.sessions {
		if !s.Enabled || s.Limited {
			delete(m.restart, id)

			continue
		}

		if rt, ok := m.rt[id]; ok {
			if r, ok := m.restart[id]; ok && r.fails > 0 && now.Sub(rt.startedAt) > stableAfter {
				r.fails, r.lastErr = 0, ""
			}

			continue
		}

		r, ok := m.restart[id]
		if !ok {
			r = &restartState{}
			m.restart[id] = r
		}

		if now.Before(r.next) {
			continue
		}

		r.fails++
		r.next = now.Add(backoffFor(r.fails))

		if err := m.startLocked(s); err != nil {
			r.lastErr = err.Error()
		} else {
			s.Restarts++
			r.lastErr = ""
			changed = true
		}
	}

	if changed {
		m.saveLocked()
	}
}

func backoffFor(fails int) time.Duration {
	if fails <= 0 {
		fails = 1
	}

	if fails > 7 { // 5 * 2^6 = 320s already clips to the 300s cap
		return maxBackoff
	}

	d := time.Duration(5*(1<<uint(fails-1))) * time.Second //nolint:gosec // fails bounded above by the check
	if d > maxBackoff {
		return maxBackoff
	}

	return d
}

// monitorLoop refreshes traffic accounting for every running session and
// stops any that have hit their configured limit.
func (m *Manager) monitorLoop(ctx context.Context) {
	t := time.NewTicker(loopInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.monitorTick()
		}
	}
}

func (m *Manager) monitorTick() {
	m.mu.Lock()
	defer m.mu.Unlock()

	changed := false

	for id, s := range m.sessions {
		if !m.isRunningLocked(id) {
			continue
		}

		total := m.currentTrafficLocked(id, s)

		if s.LimitB > 0 && total >= s.LimitB {
			m.stopLocked(s)
			s.Limited = true
			changed = true
		}
	}

	if changed {
		m.saveLocked()
	}
}
