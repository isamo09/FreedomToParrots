// Package console renders the always-on-screen operator dashboard: the
// panel's URL, its password, a live table of devices and any warnings -
// the thing an operator sees the instant they run the program, in the
// terminal or in the console window a double-clicked .exe opens on
// Windows. It redraws in place every tick instead of scrolling, so the
// screen never fills up with history.
package console

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/isamo09/FreedomToParrots/internal/session"
)

const (
	clear  = "\x1b[H\x1b[2J"
	dim    = "\x1b[90m"
	bold   = "\x1b[1m"
	green  = "\x1b[32m"
	yellow = "\x1b[33m"
	red    = "\x1b[31m"
	reset  = "\x1b[0m"

	refreshEvery = 2 * time.Second
)

// Dashboard owns what gets drawn on every refresh.
type Dashboard struct {
	mgr      *session.Manager
	urls     []string
	password string
	note     string // optional one-line operator notice (e.g. data dir fallback)
	version  string
}

// New builds a dashboard. urls is every address the panel can be reached
// at (LAN + loopback); the first is treated as primary.
func New(mgr *session.Manager, urls []string, password, note, version string) *Dashboard {
	return &Dashboard{mgr: mgr, urls: urls, password: password, note: note, version: version}
}

// Run redraws the dashboard every couple of seconds until ctx is cancelled.
func (d *Dashboard) Run(ctx context.Context) {
	enablePretty()

	t := time.NewTicker(refreshEvery)
	defer t.Stop()

	d.draw()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			d.draw()
		}
	}
}

func (d *Dashboard) draw() {
	snap := d.mgr.Snapshot()

	var b strings.Builder

	b.WriteString(clear)
	b.WriteString(bold + "  Freedom To Parrots" + reset + dim + " " + d.version + " — панель управления" + reset + "\n\n")

	for _, u := range d.urls {
		b.WriteString("  Панель:   " + hyperlink(u) + "\n")
	}

	b.WriteString("  Пароль:   " + bold + d.password + reset + "\n")

	if d.note != "" {
		b.WriteString("  " + yellow + d.note + reset + "\n")
	}

	b.WriteString("\n")
	writeTable(&b, snap.Sessions)
	writeProblems(&b, snap.Problems)

	b.WriteString("\n" + dim + "  Ctrl+C — остановить сервер и все туннели." + reset + "\n")

	fmt.Print(b.String())
}

func hyperlink(url string) string {
	return "\x1b]8;;" + url + "\x1b\\" + bold + url + reset + "\x1b]8;;\x1b\\"
}

const (
	colName = 24
	colStat = 20
	colConn = 16
)

func writeTable(b *strings.Builder, sessions []session.Public) {
	live := 0

	for _, s := range sessions {
		if s.Running {
			live++
		}
	}

	b.WriteString(fmt.Sprintf("  %sКлиенты (%d из %d на связи):%s\n", bold, live, len(sessions), reset))

	if len(sessions) == 0 {
		b.WriteString(dim + "  Устройств пока нет — добавьте их в панели.\n" + reset)

		return
	}

	line := "  " + strings.Repeat("─", colName+colStat+colConn+4)
	b.WriteString(line + "\n")
	b.WriteString(fmt.Sprintf("  %-*s %-*s %-*s\n", colName, "Имя", colStat, "Статус", colConn, "Связь"))
	b.WriteString(line + "\n")

	for _, s := range sessions {
		status, color := statusOf(s)
		name := truncate(s.Name, colName)
		conn := ago(s.LastConnection)

		b.WriteString(fmt.Sprintf("  %-*s %s%-*s%s %-*s\n",
			colName, name, color, colStat, status, reset, colConn, conn))
	}

	b.WriteString(line + "\n")
}

func statusOf(s session.Public) (string, string) {
	switch {
	case s.Limited:
		return "лимит исчерпан", red
	case s.Running:
		return "на связи", green
	case s.Enabled:
		return "переподключение…", yellow
	default:
		return "выключена", dim
	}
}

func writeProblems(b *strings.Builder, problems []string) {
	if len(problems) == 0 {
		return
	}

	b.WriteString("\n  " + yellow + bold + "Проблемы:" + reset + "\n")

	for _, p := range problems {
		b.WriteString("  " + yellow + "· " + reset + p + "\n")
	}
}

func ago(unix int64) string {
	if unix == 0 {
		return "никогда"
	}

	d := time.Since(time.Unix(unix, 0))

	switch {
	case d < 5*time.Second:
		return "только что"
	case d < time.Minute:
		return fmt.Sprintf("%d с назад", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d мин назад", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d ч назад", int(d.Hours()))
	default:
		return fmt.Sprintf("%d дн назад", int(d.Hours()/24))
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}

	if n <= 1 {
		return string(r[:n])
	}

	return string(r[:n-1]) + "…"
}
