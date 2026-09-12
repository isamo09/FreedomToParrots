//go:build !linux

package procio

// Windows falls under this file too: GetProcessIoCounters technically
// exists there, but in practice it reliably reflects file/disk I/O and not
// network socket traffic - testing showed it staying flat at the process's
// one-time startup disk reads (config/key files) and never growing with
// real WebRTC media/data-channel traffic, which made the traffic column
// actively misleading (and would have silently broken the traffic-limit
// feature, since it never sees usage grow). Better to say "unavailable"
// honestly than show a frozen, wrong number.
const supported = false

func read(int) (int64, bool) {
	return 0, false
}
