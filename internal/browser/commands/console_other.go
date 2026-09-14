//go:build !windows

package commands

// hideOwnConsoleWindow is Windows-only. macOS launches the protocol handler
// through an AppleScript applet (`do shell script`) and Linux through a desktop
// entry with Terminal=false, so neither platform opens a window to hide.
func hideOwnConsoleWindow() bool { return false }
