//go:build windows

package console

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var ( //nolint:gochecknoglobals // lazy-loaded Win32 procedures, standard pattern for x/sys/windows
	modkernel32               = windows.NewLazySystemDLL("kernel32.dll")
	procSetConsoleTitleW      = modkernel32.NewProc("SetConsoleTitleW")
	procSetConsoleCtrlHandler = modkernel32.NewProc("SetConsoleCtrlHandler")
	procAttachConsole         = modkernel32.NewProc("AttachConsole")
	procAllocConsole          = modkernel32.NewProc("AllocConsole")
)

const attachParentProcess = ^uintptr(0) // ATTACH_PARENT_PROCESS (-1 as DWORD)

// AllocWindowsConsole gives the process a real console of its own, built
// with the GUI subsystem (see the -H windowsgui linker flag in
// release.yml/scripts) specifically so Windows doesn't auto-launch it
// through Windows Terminal's "default terminal application" delegation -
// that delegation shows Windows Terminal's own tab/window icon instead of
// this exe's icon (cmd/fzp/rsrc_windows_amd64.syso), no matter what's
// embedded in the binary.
//
// Run from an existing shell/terminal, the parent process already has a
// console - AttachConsole reuses it, so output lands inline exactly like
// any normal CLI tool instead of popping a separate window. Only when
// there's no parent console to attach to (double-clicked from Explorer, or
// started by Task Scheduler) does it fall back to AllocConsole, which
// creates a brand new window owned directly by this process - and that one
// does get this exe's own icon.
func AllocWindowsConsole() {
	if r1, _, _ := procAttachConsole.Call(attachParentProcess); r1 == 0 { //nolint:gosec // fixed ATTACH_PARENT_PROCESS constant
		_, _, _ = procAllocConsole.Call()
	}

	// O_RDWR, not O_WRONLY/O_RDONLY: GetConsoleMode/SetConsoleMode require
	// GENERIC_READ on the handle even for the *output* buffer - a
	// write-only CONOUT$ handle fails GetConsoleMode silently, which is
	// exactly what broke enablePretty() below (ANSI processing never
	// actually turned on, so every escape code - clear-screen included -
	// printed as literal garbage instead of being interpreted, and the
	// screen never cleared between redraws).
	if f, err := os.OpenFile("CONOUT$", os.O_RDWR, 0); err == nil { //nolint:gosec // fixed console pseudo-filename
		os.Stdout, os.Stderr = f, f
	}

	if f, err := os.OpenFile("CONIN$", os.O_RDWR, 0); err == nil { //nolint:gosec // fixed console pseudo-filename
		os.Stdin = f
	}
}

// enablePretty turns on ANSI escape processing and UTF-8 output on the
// legacy Windows console host, so box-drawing characters and OSC 8
// hyperlinks render instead of showing up as raw escape codes or "?????".
// Modern Windows Terminal already supports both; this is what makes the
// same output work in plain cmd.exe / the old conhost too.
//
// Reads the console handle fresh off os.Stdout rather than the x/sys/windows
// package-level Stdout var: that var is captured once at process startup,
// before AllocWindowsConsole above has necessarily run, so it can be stale.
func enablePretty() {
	_ = windows.SetConsoleOutputCP(65001)

	h := windows.Handle(os.Stdout.Fd()) //nolint:gosec // os.File.Fd() truncation is a documented Windows-handle pattern

	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return
	}

	_ = windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
}

// setTitle sets the console window's title (shown in its title bar and in
// the taskbar) - without this Windows just shows the exe's path. The
// window's taskbar/title-bar *icon* doesn't need a separate call: Windows
// takes it straight from the running exe's own icon resource (see
// cmd/fzp/rsrc_windows_amd64.syso, embedded via go-winres), which is why
// only the title needs setting here.
func setTitle(title string) {
	p, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return
	}

	_, _, _ = procSetConsoleTitleW.Call(uintptr(unsafe.Pointer(p))) //nolint:gosec // straight Win32 call
}

const closeConfirmWindow = 8 * time.Second

var closeState struct { //nolint:gochecknoglobals // shared between the OS callback thread and draw()
	mu      sync.Mutex
	pending bool
	armedAt time.Time
}

// watchCloseButton makes the window's X button a two-step close: Windows
// destroys a console window (and everything attached to it) as soon as the
// user clicks X, so a child process can delay that by at most a few
// seconds of cleanup - it can never truly veto the close. What we *can* do
// is treat the first click as "are you sure" (show a banner via
// closeWarning, handled by draw() below, and NOT shut down) and only tear
// tunnels down gracefully on an actual second click, or immediately on
// logoff/shutdown (those aren't a stray click and Windows won't wait for a
// prompt regardless).
func watchCloseButton(cancel context.CancelFunc) {
	handler := func(ctrlType uint32) uintptr {
		switch ctrlType {
		case windows.CTRL_CLOSE_EVENT:
			closeState.mu.Lock()
			confirmed := closeState.pending && time.Since(closeState.armedAt) < closeConfirmWindow
			if confirmed {
				closeState.pending = false
			} else {
				closeState.pending, closeState.armedAt = true, time.Now()
			}
			closeState.mu.Unlock()

			if confirmed {
				cancel()
				time.Sleep(3 * time.Second) // best-effort head start before Windows tears the console down
			}

			return 1 // handled
		case windows.CTRL_LOGOFF_EVENT, windows.CTRL_SHUTDOWN_EVENT:
			cancel()
			time.Sleep(3 * time.Second)

			return 1
		default:
			return 0
		}
	}

	cb := windows.NewCallback(handler)
	_, _, _ = procSetConsoleCtrlHandler.Call(cb, 1) //nolint:gosec // standard SetConsoleCtrlHandler(handler, TRUE) call
}

// closeWarning returns the banner to show while a close is pending
// confirmation (see watchCloseButton), or "" the rest of the time.
func closeWarning() string {
	closeState.mu.Lock()
	defer closeState.mu.Unlock()

	if !closeState.pending {
		return ""
	}

	left := closeConfirmWindow - time.Since(closeState.armedAt)
	if left <= 0 {
		closeState.pending = false

		return ""
	}

	return fmt.Sprintf("Закрыть окно? Это остановит сервер и все туннели. Нажмите на "+
		"крестик ещё раз в течение %d с, чтобы закрыть — или просто сверните окно, чтобы "+
		"продолжить работу в фоне.", int(left.Seconds())+1)
}
