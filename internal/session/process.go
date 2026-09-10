package session

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"time"
)

const logCapBytes = 5 * 1024 * 1024 // rotate (truncate) past this size on start

var connectMarkers = [][]byte{ //nolint:gochecknoglobals // fixed set of substrings from the core's log output
	[]byte("Link connected"), []byte("state: connected"),
	[]byte("publisher state"), []byte("Connecting link"), []byte("sid="),
}

var tsRE = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})`)

// newCoreCmd builds the exec.Cmd for one session's core process. The log
// file is opened for append and returned so the caller can record its
// starting size (used later to separate this run's traffic from history).
func newCoreCmd(corePath, cfgPath, workDir, logFile string) (*exec.Cmd, *os.File, error) {
	if fi, err := os.Stat(logFile); err == nil && fi.Size() > logCapBytes {
		_ = os.Remove(logFile)
	}

	logf, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // path built from resolved data dir
	if err != nil {
		return nil, nil, fmt.Errorf("open log: %w", err)
	}

	banner := fmt.Sprintf("\n=== запуск %s ===\n", time.Now().Format("2006/01/02 15:04:05"))
	if _, err := logf.WriteString(banner); err != nil {
		_ = logf.Close()

		return nil, nil, fmt.Errorf("write log banner: %w", err)
	}

	cmd := exec.Command(corePath, cfgPath) //nolint:gosec // corePath is our own extracted binary
	cmd.Dir = workDir
	cmd.Stdout = logf
	cmd.Stderr = logf
	cmd.Stdin = nil
	applyProcAttr(cmd)

	return cmd, logf, nil
}

// tailBytes reads up to n bytes from the end of path.
func tailBytes(path string, n int64) string {
	f, err := os.Open(path) //nolint:gosec // path built from resolved data dir
	if err != nil {
		return ""
	}
	defer f.Close() //nolint:errcheck // read-only fd

	fi, err := f.Stat()
	if err != nil {
		return ""
	}

	start := fi.Size() - n
	if start < 0 {
		start = 0
	}

	if _, err := f.Seek(start, 0); err != nil {
		return ""
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(f); err != nil {
		return ""
	}

	return buf.String()
}

// lastConnection scans the log tail for a connection marker and returns the
// timestamp on that line, or the zero time if none was found.
func lastConnection(logFile string) time.Time {
	tail := tailBytes(logFile, 128*1024)
	lines := splitLines(tail)

	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]

		hit := false

		for _, m := range connectMarkers {
			if bytes.Contains([]byte(line), m) {
				hit = true

				break
			}
		}

		if !hit {
			continue
		}

		m := tsRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		if t, err := time.ParseInLocation("2006/01/02 15:04:05", m[1], time.Local); err == nil {
			return t
		}
	}

	return time.Time{}
}

func splitLines(s string) []string {
	var out []string

	sc := bufio.NewScanner(bytes.NewReader([]byte(s)))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for sc.Scan() {
		out = append(out, sc.Text())
	}

	return out
}
