//go:build linux

package procio

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

const supported = true

// read parses /proc/<pid>/io - present on both desktop Linux and Termux
// (Termux is a plain aarch64 Linux userland, and this reads the tunnel's own
// process, not another app's, so no SELinux restriction applies).
func read(pid int) (int64, bool) {
	f, err := os.Open("/proc/" + strconv.Itoa(pid) + "/io")
	if err != nil {
		return 0, false
	}
	defer f.Close() //nolint:errcheck // read-only fd

	var rchar, wchar int64

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()

		key, val, found := strings.Cut(line, ":")
		if !found {
			continue
		}

		n, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
		if err != nil {
			continue
		}

		switch strings.TrimSpace(key) {
		case "rchar":
			rchar = n
		case "wchar":
			wchar = n
		}
	}

	return (rchar + wchar) / 2, true
}
