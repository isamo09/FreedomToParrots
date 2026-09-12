// Package console renders the always-on-screen operator dashboard: the
// panel's URL, its password, a live table of devices and any warnings -
// the thing an operator sees the instant they run the program, in the
// terminal or in the console window a double-clicked .exe opens on
// Windows. It redraws in place every tick instead of scrolling, so the
// screen never fills up with history.
package console

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/isamo09/FreedomToParrots/internal/session"
	"github.com/isamo09/FreedomToParrots/internal/update"
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

// windowTitle is the console/terminal window title - set once at startup so
// the taskbar/tab shows the app's name instead of an exe path. The window
// *icon* on Windows comes for free from the exe's own resource (see
// term_windows.go); other OSes don't have an equivalent for a plain
// terminal-hosted process.
const windowTitle = "Freedom To Parrots"

// Dashboard owns what gets drawn on every refresh.
type Dashboard struct {
	mgr      *session.Manager
	urls     []string
	password string
	note     string // optional one-line operator notice (e.g. data dir fallback)
	version  string
	cancel   context.CancelFunc
	updates  *update.Tracker

	updMu       sync.Mutex
	updApplying bool
	updErr      string
}

// New builds a dashboard. urls is every address the panel can be reached
// at (LAN + loopback); the first is treated as primary. cancel is called to
// trigger graceful shutdown - Ctrl+C already goes through the caller's own
// context, but on Windows the dashboard also wires it up to the window's
// close button (see term_windows.go), and typing "u" here does the same
// after a successful self-update (see watchUpdateKey).
func New(mgr *session.Manager, urls []string, password, note, version string,
	cancel context.CancelFunc, updates *update.Tracker,
) *Dashboard {
	return &Dashboard{
		mgr: mgr, urls: urls, password: password, note: note,
		version: version, cancel: cancel, updates: updates,
	}
}

// Run redraws the dashboard every couple of seconds until ctx is cancelled.
func (d *Dashboard) Run(ctx context.Context) {
	enablePretty()
	setTitle(windowTitle)
	watchCloseButton(d.cancel)
	go d.watchUpdateKey(ctx)

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

// watchUpdateKey lets the operator type "u" + Enter to install an available
// update without needing the web panel open. Reading stdin like this is
// harmless even when nobody's typing - Scan just blocks until either a line
// arrives or the console goes away with the process.
func (d *Dashboard) watchUpdateKey(ctx context.Context) {
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		if ctx.Err() != nil {
			return
		}

		if strings.TrimSpace(strings.ToLower(sc.Text())) != "u" {
			continue
		}

		if !d.updates.Snapshot().Available {
			continue
		}

		d.updMu.Lock()
		d.updApplying, d.updErr = true, ""
		d.updMu.Unlock()
		d.draw()

		if err := d.updates.Apply(context.Background()); err != nil {
			d.updMu.Lock()
			d.updApplying, d.updErr = false, err.Error()
			d.updMu.Unlock()

			continue
		}

		d.cancel()

		return
	}
}

func (d *Dashboard) draw() {
	snap := d.mgr.Snapshot()

	var b strings.Builder

	b.WriteString(oscTitle(windowTitle))
	b.WriteString(clear)
	b.WriteString(bold + "  " + windowTitle + reset + dim + " " + d.version + " — панель управления" + reset + "\n\n")

	for _, u := range d.urls {
		b.WriteString("  Панель:   " + hyperlink(u) + "\n")
	}

	b.WriteString("  Пароль:   " + bold + d.password + reset + "\n")

	if d.note != "" {
		b.WriteString("  " + yellow + d.note + reset + "\n")
	}

	if w := closeWarning(); w != "" {
		b.WriteString("\n  " + red + bold + "! " + reset + red + w + reset + "\n")
	}

	d.writeUpdateBanner(&b)

	b.WriteString("\n")
	writeTable(&b, snap.Sessions)
	writeProblems(&b, snap.Problems)

	b.WriteString("\n" + dim + "  Ctrl+C — остановить сервер и все туннели." + reset + "\n")

	fmt.Print(b.String())
}

func (d *Dashboard) writeUpdateBanner(b *strings.Builder) {
	d.updMu.Lock()
	applying, applyErr := d.updApplying, d.updErr
	d.updMu.Unlock()

	if applying {
		b.WriteString("\n  " + green + "↻ Устанавливаю обновление, панель скоро перезапустится…" + reset + "\n")

		return
	}

	info := d.updates.Snapshot()

	if applyErr != "" {
		b.WriteString("\n  " + red + "Не удалось обновиться: " + applyErr + reset + "\n")
	}

	if !info.Available {
		return
	}

	b.WriteString(fmt.Sprintf("\n  %s↑ Доступна версия %s%s%s (сейчас %s) — введите %sU%s и Enter "+
		"здесь, или нажмите «Обновить» в панели.%s\n",
		green, bold, info.Latest, reset+green, info.Current, bold, reset+green, reset))
}

func hyperlink(url string) string {
	return "\x1b]8;;" + url + "\x1b\\" + bold + url + reset + "\x1b]8;;\x1b\\"
}

// oscTitle sets the terminal window/tab title via the OSC 0 escape
// sequence, understood by essentially every terminal emulator (xterm,
// gnome-terminal, iTerm2, Windows Terminal). Harmless no-op bytes on
// anything that doesn't support it, including when stdout is redirected to
// a file. On Windows this complements the native SetConsoleTitleW call in
// term_windows.go, which covers the legacy console host too.
func oscTitle(title string) string {
	return "\x1b]0;" + title + "\x07"
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
