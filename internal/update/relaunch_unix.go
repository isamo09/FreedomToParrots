//go:build !windows

package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// relaunch starts the just-installed executable as a fresh, independent
// process (own session, detached from our process group) so it comes up
// as a normal launch rather than something tied to this process's lifetime.
func relaunch() error {
	exe, err := runningExecutable()
	if err != nil {
		return err
	}

	cmd := exec.Command(exe) //nolint:gosec // exe is our own just-installed executable
	cmd.Dir = filepath.Dir(exe)
	cmd.Env = os.Environ()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start updated executable: %w", err)
	}

	return nil
}
