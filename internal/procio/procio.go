// Package procio reads a best-effort "bytes transferred" counter for a
// running process, used to estimate tunnel traffic. Support is inherently
// OS-specific; platforms without an implementation report ok=false and the
// panel shows the traffic column as unavailable rather than guessing.
package procio

// Read returns an approximate total of bytes read+written by pid since it
// started, halved so the counter reads like "bytes actually transferred"
// rather than double-counting one packet copied from one socket to another.
// ok is false when the platform or the process is not accessible.
func Read(pid int) (bytes int64, ok bool) {
	return read(pid)
}

// Supported reports whether this platform can produce real numbers at all,
// so the UI can say "unavailable on this OS" instead of always "0 B".
func Supported() bool {
	return supported
}
