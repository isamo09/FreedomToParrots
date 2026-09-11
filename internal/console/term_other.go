//go:build !windows

package console

import "context"

// AllocWindowsConsole is a no-op outside Windows: every other OS we target
// runs this as a normal process attached to whatever terminal launched it,
// with no "default terminal application" delegation to work around.
func AllocWindowsConsole() {}

// enablePretty is a no-op outside Windows: every other terminal we target
// (Linux, macOS, Termux, BSD) already speaks ANSI/UTF-8 out of the box.
func enablePretty() {}

// setTitle is a no-op here: the window title is set portably via an OSC
// escape sequence written straight into the dashboard output (see
// console.go), which every terminal emulator we target already honors.
func setTitle(string) {}

// watchCloseButton is Windows-only: closing a terminal tab/window on
// Linux/macOS is the terminal emulator's own business, not something a
// child process can intercept.
func watchCloseButton(context.CancelFunc) {}

// closeWarning always returns "" outside Windows - see watchCloseButton.
func closeWarning() string { return "" }
