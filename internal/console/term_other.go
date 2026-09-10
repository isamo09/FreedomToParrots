//go:build !windows

package console

// enablePretty is a no-op outside Windows: every other terminal we target
// (Linux, macOS, Termux, BSD) already speaks ANSI/UTF-8 out of the box.
func enablePretty() {}
