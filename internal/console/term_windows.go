//go:build windows

package console

import "golang.org/x/sys/windows"

// enablePretty turns on ANSI escape processing and UTF-8 output on the
// legacy Windows console host, so box-drawing characters and OSC 8
// hyperlinks render instead of showing up as raw escape codes or "?????".
// Modern Windows Terminal already supports both; this is what makes the
// same output work in plain cmd.exe / the old conhost too.
func enablePretty() {
	_ = windows.SetConsoleOutputCP(65001)

	var mode uint32
	if err := windows.GetConsoleMode(windows.Stdout, &mode); err != nil {
		return
	}

	_ = windows.SetConsoleMode(windows.Stdout, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
}
