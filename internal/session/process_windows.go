//go:build windows

package session

import (
	"os/exec"
	"strconv"
	"syscall"

	"golang.org/x/sys/windows"
)

// applyProcAttr starts the core process in its own console process group.
// That's what lets us send it a CTRL_BREAK_EVENT independently of whatever
// happens to our own console (e.g. the user hitting Ctrl+C on us), and keeps
// its console window hidden when we were double-clicked from Explorer.
func applyProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW,
	}
}

// stopProcess asks pid (a process-group leader started via applyProcAttr)
// to shut down gracefully via CTRL_BREAK_EVENT, or force-kills its whole
// tree via taskkill when graceful is false.
func stopProcess(pid int, graceful bool) {
	if graceful {
		_ = windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(pid)) //nolint:gosec // pid from our own process table

		return
	}

	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run() //nolint:gosec // fixed args, pid from our own process table
}

// killStray force-kills a process (and its tree) left running by a
// previous, now-gone instance of this program.
func killStray(pid int) {
	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run() //nolint:gosec // fixed args, pid from our own process table
}
