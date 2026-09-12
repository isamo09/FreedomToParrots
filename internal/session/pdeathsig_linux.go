//go:build linux

package session

import "syscall"

// setPdeathsig asks the kernel to send SIGTERM to the core process if this
// process exits without ever getting to signal it itself.
func setPdeathsig(attr *syscall.SysProcAttr) {
	attr.Pdeathsig = syscall.SIGTERM
}
