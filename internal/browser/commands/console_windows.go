//go:build windows

package commands

import (
	"syscall"
	"unsafe"
)

// Windows gives a console-subsystem program its own console window when a shell
// starts it, and the efp-bridge:// protocol handler is exactly that: Chrome
// ShellExecutes `browser.exe bridge-launch "<url>"`, so a black window appears
// and stays until the command exits, which can be several seconds while it
// waits for the bridge to answer /ping. Members read that flashing window as a
// crash. Hiding the console we were given removes it; the Connectors panel is
// what reports whether the bridge came up.

var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	user32                    = syscall.NewLazyDLL("user32.dll")
	procGetConsoleWindow      = kernel32.NewProc("GetConsoleWindow")
	procGetConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
	procShowWindow            = user32.NewProc("ShowWindow")
)

const swHide = 0

// ownsFreshConsole reports whether this process is the only one attached to its
// console, which means the console was created for it rather than inherited
// from the member's shell. The window handle cannot be used for this: since
// Windows 7 the console window belongs to conhost.exe, so its owning process id
// is never ours. GetConsoleProcessList answers the question directly -- a
// console created for this process has exactly one client, while a terminal the
// member typed into also has the shell attached.
func ownsFreshConsole() bool {
	var pids [8]uint32
	count, _, _ := procGetConsoleProcessList.Call(uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)))
	return count == 1
}

// hideOwnConsoleWindow hides the console window when this process owns it, so
// running `browser bridge-launch ...` by hand never blanks the terminal it was
// typed into. Returns whether a window was hidden.
func hideOwnConsoleWindow() bool {
	handle, _, _ := procGetConsoleWindow.Call()
	if handle == 0 || !ownsFreshConsole() {
		return false
	}
	ret, _, _ := procShowWindow.Call(handle, swHide)
	return ret != 0
}
