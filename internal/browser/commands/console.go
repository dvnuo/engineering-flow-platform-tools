package commands

// BridgeLaunchCommandName is the hidden subcommand the efp-bridge:// protocol
// handler invokes.
const BridgeLaunchCommandName = "bridge-launch"

// HideConsoleForProtocolLaunch hides the console window Windows hands a
// console-subsystem program when a shell starts it, but only for the protocol
// handler entry point. It is called from main before cobra parses anything:
// the window exists from the moment the process starts, so every millisecond
// before this call is a millisecond of visible flicker. Every other command
// keeps its console, which is where its JSON envelope goes.
func HideConsoleForProtocolLaunch(args []string) {
	if len(args) == 0 || args[0] != BridgeLaunchCommandName {
		return
	}
	consoleHidden = hideOwnConsoleWindow()
}

// consoleHidden records the outcome for the bridge log: a member reporting a
// flashing window needs to know whether hiding was even attempted here.
var consoleHidden bool
