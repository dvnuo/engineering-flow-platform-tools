//go:build windows

package commands

import (
	"errors"
	"strings"
	"testing"
)

func captureRegistryCommands(t *testing.T, fail func(args []string) error) *[][]string {
	t.Helper()
	original := runRegistryCommand
	var calls [][]string
	runRegistryCommand = func(args ...string) ([]byte, error) {
		calls = append(calls, append([]string{}, args...))
		if fail != nil {
			if err := fail(args); err != nil {
				return []byte("ERROR: simulated"), err
			}
		}
		return []byte("The operation completed successfully."), nil
	}
	t.Cleanup(func() { runRegistryCommand = original })
	return &calls
}

func TestRegisterBridgeProtocolWritesHKCUKeys(t *testing.T) {
	calls := captureRegistryCommands(t, nil)
	data, err := registerBridgeProtocol("https://portal.example.test")
	if err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 3 {
		t.Fatalf("reg.exe calls = %v", *calls)
	}
	root := (*calls)[0]
	if root[0] != "add" || root[1] != `HKCU\Software\Classes\efp-bridge` || root[2] != "/ve" || root[6] != "URL:EFP Bridge" || root[7] != "/f" {
		t.Fatalf("root key call = %v", root)
	}
	protocol := (*calls)[1]
	if protocol[1] != `HKCU\Software\Classes\efp-bridge` || protocol[2] != "/v" || protocol[3] != "URL Protocol" || protocol[6] != "/d" || protocol[7] != "" || protocol[8] != "/f" {
		t.Fatalf("URL Protocol call = %v", protocol)
	}
	command := (*calls)[2]
	if command[1] != `HKCU\Software\Classes\efp-bridge\shell\open\command` || command[2] != "/ve" {
		t.Fatalf("command key call = %v", command)
	}
	value := command[6]
	// Under go test the executable is commands.test.exe; production registers browser.exe.
	if !strings.HasSuffix(value, `.exe" bridge-launch "%1"`) || !strings.HasPrefix(value, `"`) {
		t.Fatalf("command value = %q", value)
	}
	if data["registered"] != true || data["command"] != value || data["origin"] != "https://portal.example.test" {
		t.Fatalf("register data = %#v", data)
	}
}

func TestRegisterBridgeProtocolReportsRegFailure(t *testing.T) {
	captureRegistryCommands(t, func(args []string) error {
		if len(args) > 1 && strings.HasSuffix(args[1], `\shell\open\command`) {
			return errors.New("exit status 1")
		}
		return nil
	})
	_, err := registerBridgeProtocol("https://portal.example.test")
	if err == nil || !strings.Contains(err.Error(), "simulated") {
		t.Fatalf("expected protocol_register_failed error, got %v", err)
	}
}

func TestUnregisterBridgeProtocol(t *testing.T) {
	calls := captureRegistryCommands(t, func(args []string) error {
		if args[0] == "query" {
			return errors.New("exit status 1")
		}
		return nil
	})
	data, err := unregisterBridgeProtocol()
	if err != nil {
		t.Fatal(err)
	}
	if data["removed"] != false || len(*calls) != 1 {
		t.Fatalf("not-registered result = %#v calls=%v", data, *calls)
	}

	calls = captureRegistryCommands(t, nil)
	data, err = unregisterBridgeProtocol()
	if err != nil {
		t.Fatal(err)
	}
	if data["removed"] != true || len(*calls) != 2 || (*calls)[1][0] != "delete" || (*calls)[1][1] != `HKCU\Software\Classes\efp-bridge` || (*calls)[1][2] != "/f" {
		t.Fatalf("delete result = %#v calls=%v", data, *calls)
	}
}

func TestConfigureWindowsDetachedCommandFlags(t *testing.T) {
	cmd := newDetachedCommand("browser.exe", []string{"serve"}, nil)
	configureWindowsDetachedCommand(cmd, true)
	want := uint32(windowsBridgeDetachedProcess | windowsBridgeCreateNewProcessGroup | windowsBridgeCreateBreakawayJob)
	if got := cmd.SysProcAttr.CreationFlags; got&want != want {
		t.Fatalf("creation flags = %#x want all %#x", got, want)
	}
	fallback := newDetachedCommand("browser.exe", []string{"serve"}, nil)
	configureWindowsDetachedCommand(fallback, false)
	if got := fallback.SysProcAttr.CreationFlags; got&windowsBridgeCreateBreakawayJob != 0 {
		t.Fatalf("fallback flags unexpectedly include breakaway: %#x", got)
	}
	if cmd.Stdin != nil {
		t.Fatal("detached command must not inherit stdin")
	}
}
