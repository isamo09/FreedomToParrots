//go:build windows

package procio

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const supported = true

var ( //nolint:gochecknoglobals // lazy-loaded Win32 procedure, standard pattern for x/sys/windows
	modkernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procGetProcessIoCounters = modkernel32.NewProc("GetProcessIoCounters")
)

func getProcessIoCounters(h windows.Handle, io *windows.IO_COUNTERS) bool {
	r1, _, _ := procGetProcessIoCounters.Call(uintptr(h), uintptr(unsafe.Pointer(io))) //nolint:gosec // straight Win32 call

	return r1 != 0
}

func read(pid int) (int64, bool) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid)) //nolint:gosec // pid from our own process table
	if err != nil {
		return 0, false
	}
	defer windows.CloseHandle(h) //nolint:errcheck // best-effort cleanup

	var io windows.IO_COUNTERS
	if !getProcessIoCounters(h, &io) {
		return 0, false
	}

	total := int64(io.ReadTransferCount+io.WriteTransferCount) / 2 //nolint:gosec // counters fit in int64 in practice

	return total, true
}
