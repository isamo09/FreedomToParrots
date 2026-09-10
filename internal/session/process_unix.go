//go:build !windows

package session

import (
	"os/exec"
	"syscall"
)

// applyProcAttr puts the core process in its own process group so a signal
// sent to the whole group (below) also reaches children it spawns itself -
// videochannel launches ffmpeg as a separate process, and we don't want to
// orphan it.
func applyProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// stopProcess signals the process group led by pid: SIGTERM for a graceful
// shutdown (the core binary has its own shutdown grace period), SIGKILL to
// force it once that grace period has passed.
func stopProcess(pid int, graceful bool) {
	sig := syscall.SIGKILL
	if graceful {
		sig = syscall.SIGTERM
	}

	if err := syscall.Kill(-pid, sig); err != nil {
		_ = syscall.Kill(pid, sig) // not a group leader (already reaped?) - hit the pid directly
	}
}

// killStray best-effort terminates a process left running by a previous,
// now-gone instance of this program (see Manager.reclaimStray).
func killStray(pid int) {
	_ = syscall.Kill(pid, syscall.SIGTERM)
}
