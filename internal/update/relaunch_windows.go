//go:build windows

package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

// relaunch starts the just-installed executable as a fresh, independent
// process. It's fine for it to (briefly) attach to our own console via
// AttachConsole(ATTACH_PARENT_PROCESS) - see internal/console - since this
// process is about to close that console anyway as part of its own
// graceful shutdown.
func relaunch() error {
	exe, err := runningExecutable()
	if err != nil {
		return err
	}

	cmd := exec.Command(exe) //nolint:gosec // exe is our own just-installed executable
	cmd.Dir = filepath.Dir(exe)
	cmd.Env = os.Environ()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start updated executable: %w", err)
	}

	return nil
}
