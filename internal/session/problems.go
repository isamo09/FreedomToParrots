package session

import "fmt"

// activeWarnThreshold is the point past which running more tunnels at once
// starts risking instability (each is a disguised WebRTC call competing for
// the same uplink/CPU).
const activeWarnThreshold = 5

// crashLoopThreshold is how many failed (re)starts, with the tunnel having
// never once connected, before we suggest the room link itself is the
// problem rather than a transient network hiccup.
const crashLoopThreshold = 3

// Snapshot bundles everything the console dashboard and the web panel need
// for one refresh: the session list plus a human-readable problem digest.
type Snapshot struct {
	Sessions      []Public
	Problems      []string
	CoreAvailable bool
	CoreError     string
}

// Snapshot computes the current state and derived warnings in one pass.
func (m *Manager) Snapshot() Snapshot {
	sessions := m.List()

	snap := Snapshot{
		Sessions:      sessions,
		CoreAvailable: m.corePath != "",
	}

	if !snap.CoreAvailable && m.coreErr != nil {
		snap.CoreError = m.coreErr.Error()
	}

	snap.Problems = problemsFor(sessions, snap.CoreAvailable, snap.CoreError)

	return snap
}

func problemsFor(sessions []Public, coreAvailable bool, coreError string) []string {
	var out []string

	if !coreAvailable {
		out = append(out, fmt.Sprintf(
			"Ядро тоннеля недоступно: %s. Соберите программу официальным релизом "+
				"(GitHub Actions) вместо go build напрямую.", coreError))
	}

	active := 0

	for _, s := range sessions {
		if s.Running {
			active++
		}
	}

	if active > activeWarnThreshold {
		out = append(out, fmt.Sprintf(
			"Активных клиентов: %d — больше %d одновременно не рекомендуется, соединение может стать нестабильным.",
			active, activeWarnThreshold))
	}

	for _, s := range sessions {
		switch {
		case s.Limited:
			out = append(out, fmt.Sprintf("«%s»: лимит трафика исчерпан, сессия остановлена.", s.Name))
		case s.Enabled && !s.Running && s.Restarts >= crashLoopThreshold && s.LastConnection == 0:
			out = append(out, fmt.Sprintf(
				"«%s»: не удаётся подключиться (%d попыт.) и связь ни разу не установилась — "+
					"похоже, ссылка на звонок больше не действительна или комната закрыта. Проверьте Room ID/ссылку.",
				s.Name, s.Restarts))
		case s.Enabled && !s.Running && s.StartError != "":
			out = append(out, fmt.Sprintf("«%s»: не запускается — %s", s.Name, s.StartError))
		}
	}

	return out
}
