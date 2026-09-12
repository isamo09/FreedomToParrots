//go:build !windows && !linux

package session

import "syscall"

// setPdeathsig is a no-op here: PR_SET_PDEATHSIG is Linux-specific: macOS
// and the BSDs have no equivalent "signal me when my parent dies" facility.
func setPdeathsig(*syscall.SysProcAttr) {}
