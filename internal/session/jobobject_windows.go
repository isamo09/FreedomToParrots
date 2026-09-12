//go:build windows

package session

import (
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var ( //nolint:gochecknoglobals // lazily created once, shared by every session's core process
	jobOnce   sync.Once
	jobHandle windows.Handle
)

// ensureJob lazily creates a Windows Job Object with
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE: every core process assigned to it
// (see assignToJob) is killed by the OS itself the instant this job's last
// handle closes - which happens automatically when our own process exits,
// no matter how abruptly. That matters specifically because Windows tears
// the console down a few seconds after the window's close button is
// clicked (see term_windows.go's watchCloseButton) regardless of whether
// our own graceful shutdown (stopping every session, waiting on each core
// process) has finished by then. Racing a sleep timer against Windows'
// own termination timeout is not reliable; the job object is - it doesn't
// depend on our process getting to run any more code at all.
func ensureJob() windows.Handle {
	jobOnce.Do(func() {
		h, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			return
		}

		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
			BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
				LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
			},
		}

		_, _ = windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))) //nolint:gosec // fixed Win32 struct pointer/size pattern

		jobHandle = h
	})

	return jobHandle
}

// assignToJob puts pid under the shared job object. Best-effort: on
// failure (e.g. no permission, or - pre-Windows 8 only - already in a
// non-nestable job) the process simply isn't covered by the auto-kill
// safety net; it still gets the normal graceful/forced stop attempts.
func assignToJob(pid int) {
	h := ensureJob()
	if h == 0 {
		return
	}

	ph, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid)) //nolint:gosec // pid from our own process table
	if err != nil {
		return
	}
	defer windows.CloseHandle(ph) //nolint:errcheck // best-effort cleanup

	_ = windows.AssignProcessToJobObject(h, ph)
}
