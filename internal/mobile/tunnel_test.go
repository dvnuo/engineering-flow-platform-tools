package mobile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"engineering-flow-platform-tools/internal/config"
)

func TestTunnelStartRejectsUnknownNetworkMode(t *testing.T) {
	store := NewStateStore(filepath.Join(t.TempDir(), "state"), filepath.Join(t.TempDir(), "artifacts"))
	mgr := &TunnelManager{Store: store, Credentials: Credentials{AccessKey: "secret"}}
	_, err := mgr.Start(TunnelStartRequest{NetworkMode: "publci"})
	if err == nil {
		t.Fatal("expected invalid network mode error")
	}
	me, ok := err.(*Error)
	if !ok || me.Code != "invalid_args" {
		t.Fatalf("unexpected error: %#v", err)
	}
}

func TestTunnelStatusDoesNotOverwriteTerminalState(t *testing.T) {
	store := NewStateStore(filepath.Join(t.TempDir(), "state"), filepath.Join(t.TempDir(), "artifacts"))
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	mgr := &TunnelManager{Store: store}
	state := TunnelState{
		Version:         1,
		RunID:           "run-1",
		Managed:         true,
		PID:             os.Getpid(),
		LocalIdentifier: "local-1",
		Owner:           "efp-mobile",
		Status:          "stopped",
	}
	if err := mgr.Save(state); err != nil {
		t.Fatal(err)
	}
	got, err := mgr.Status("run-1", "local-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "stopped" {
		t.Fatalf("status=%s", got.Status)
	}
}

func TestTunnelStatusMarksExpiredRunningTunnel(t *testing.T) {
	store := NewStateStore(filepath.Join(t.TempDir(), "state"), filepath.Join(t.TempDir(), "artifacts"))
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	mgr := &TunnelManager{Store: store}
	state := TunnelState{
		Version:         1,
		RunID:           "run-1",
		Managed:         true,
		PID:             os.Getpid(),
		LocalIdentifier: "local-1",
		Owner:           "efp-mobile",
		Status:          "running",
		Deadline:        time.Now().UTC().Add(-time.Minute),
	}
	if err := mgr.Save(state); err != nil {
		t.Fatal(err)
	}
	got, err := mgr.Status("run-1", "local-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "expired" {
		t.Fatalf("status=%s", got.Status)
	}
	if TunnelReusable(got, time.Now().UTC()) {
		t.Fatal("expired tunnel should not be reusable")
	}
}

func TestTunnelStatusMarksDeadManagedProcessExited(t *testing.T) {
	store := NewStateStore(filepath.Join(t.TempDir(), "state"), filepath.Join(t.TempDir(), "artifacts"))
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	mgr := &TunnelManager{Store: store}
	state := TunnelState{
		Version:         1,
		RunID:           "run-1",
		Managed:         true,
		PID:             99999999,
		LocalIdentifier: "local-1",
		Owner:           "efp-mobile",
		Status:          "running",
		Deadline:        time.Now().UTC().Add(time.Hour),
	}
	if err := mgr.Save(state); err != nil {
		t.Fatal(err)
	}
	got, err := mgr.Status("run-1", "local-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "exited" {
		t.Fatalf("status=%s", got.Status)
	}
	if TunnelReusable(got, time.Now().UTC()) {
		t.Fatal("exited tunnel should not be reusable")
	}
}

func TestTunnelStatusMarksOwnershipUnverifiedForMismatchedPID(t *testing.T) {
	store := NewStateStore(filepath.Join(t.TempDir(), "state"), filepath.Join(t.TempDir(), "artifacts"))
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	mgr := &TunnelManager{Store: store}
	state := TunnelState{
		Version:         1,
		RunID:           "run-1",
		Managed:         true,
		PID:             os.Getpid(),
		BinaryPath:      filepath.Join(t.TempDir(), "not-browserstack-local"),
		LocalIdentifier: "local-1",
		Owner:           "efp-mobile",
		Status:          "running",
		Deadline:        time.Now().UTC().Add(time.Hour),
	}
	if err := mgr.Save(state); err != nil {
		t.Fatal(err)
	}
	got, err := mgr.Status("run-1", "local-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "ownership_unverified" {
		t.Fatalf("status=%s", got.Status)
	}
	if TunnelReusable(state, time.Now().UTC()) {
		t.Fatal("ownership-mismatched tunnel should not be reusable")
	}
}

func TestInspectLocalReadyLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tunnel.log")
	if err := os.WriteFile(path, []byte("[SUCCESS] You can now access your local server(s) in our remote browser"), 0o600); err != nil {
		t.Fatal(err)
	}
	ready, failed := inspectLocalReadyLog(path)
	if !ready || failed != "" {
		t.Fatalf("ready=%v failed=%q", ready, failed)
	}
	if err := os.WriteFile(path, []byte("[ERROR] Could not connect to BrowserStack"), 0o600); err != nil {
		t.Fatal(err)
	}
	ready, failed = inspectLocalReadyLog(path)
	if ready || failed == "" {
		t.Fatalf("ready=%v failed=%q", ready, failed)
	}
	if err := os.WriteFile(path, []byte("[ERROR] Could not connect to BrowserStack\n[SUCCESS] You can now access your local server(s) in our remote browser"), 0o600); err != nil {
		t.Fatal(err)
	}
	ready, failed = inspectLocalReadyLog(path)
	if !ready || failed != "" {
		t.Fatalf("ready=%v failed=%q", ready, failed)
	}
	if err := os.WriteFile(path, []byte("[SUCCESS] You can now access your local server(s) in our remote browser\n[ERROR] Could not connect to BrowserStack"), 0o600); err != nil {
		t.Fatal(err)
	}
	ready, failed = inspectLocalReadyLog(path)
	if ready || failed == "" {
		t.Fatalf("ready=%v failed=%q", ready, failed)
	}
}

func TestMarkExitedOnlyWhenTunnelStillRunning(t *testing.T) {
	store := NewStateStore(filepath.Join(t.TempDir(), "state"), filepath.Join(t.TempDir(), "artifacts"))
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	mgr := &TunnelManager{Store: store}
	running := TunnelState{Version: 1, RunID: "run-1", Managed: true, PID: os.Getpid(), LocalIdentifier: "local-1", Owner: "efp-mobile", Status: "running"}
	if err := mgr.Save(running); err != nil {
		t.Fatal(err)
	}
	mgr.markExitedIfRunning(running)
	got, err := mgr.Load("run-1", "local-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "exited" {
		t.Fatalf("status=%s", got.Status)
	}

	stopped := running
	stopped.RunID = "run-2"
	stopped.Status = "stopped"
	if err := mgr.Save(stopped); err != nil {
		t.Fatal(err)
	}
	mgr.markExitedIfRunning(stopped)
	got, err = mgr.Load("run-2", "local-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "stopped" {
		t.Fatalf("status=%s", got.Status)
	}
}

func TestCleanupOrphansStopsStandaloneManagedTunnel(t *testing.T) {
	store := NewStateStore(filepath.Join(t.TempDir(), "state"), filepath.Join(t.TempDir(), "artifacts"))
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	mgr := &TunnelManager{Store: store}
	state := TunnelState{
		Version:         1,
		RunID:           "tunnel-local-1",
		Managed:         true,
		PID:             99999999,
		LocalIdentifier: "local-1",
		Owner:           "efp-mobile",
		Status:          "running",
		Deadline:        time.Now().UTC().Add(-time.Minute),
	}
	if err := mgr.Save(state); err != nil {
		t.Fatal(err)
	}
	stopped, err := mgr.CleanupOrphans()
	if err != nil {
		t.Fatal(err)
	}
	if len(stopped) != 1 || stopped[0].Status != "exited" {
		t.Fatalf("stopped=%#v", stopped)
	}
	got, err := mgr.Load("tunnel-local-1", "local-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "exited" {
		t.Fatalf("status=%s", got.Status)
	}
}

func TestLocalBinaryArgsDoNotExposeAccessKey(t *testing.T) {
	secret := "bs-secret-value"
	args := localBinaryArgs("local.yml", "local-1", config.MobileLocalConfig{
		IncludeHosts: []string{"api.internal", "*.corp"},
		ExcludeHosts: []string{"blocked.internal", "8.8.8.8"},
	})
	if !containsArg(args, "--enable-logging-for-api") {
		t.Fatalf("local API logging flag missing: %#v", args)
	}
	assertFlagValues(t, args, "--include-hosts", []string{"api.internal", "*.corp"})
	assertFlagValues(t, args, "--exclude-hosts", []string{"blocked.internal", "8.8.8.8"})
	for _, arg := range args {
		if arg == "--key" {
			t.Fatalf("argv should not pass --key: %#v", args)
		}
	}
	if strings.Contains(strings.Join(args, " "), secret) {
		t.Fatalf("secret leaked into argv: %#v", args)
	}
	cfg := string(localBinaryConfig(Credentials{AccessKey: secret}))
	if !strings.Contains(cfg, secret) {
		t.Fatalf("secret missing from local config: %s", cfg)
	}
	if strings.Contains(cfg, "--key") {
		t.Fatalf("config should not use argv flag syntax: %s", cfg)
	}
}

func TestTunnelStopRefusesUnverifiedRunningPID(t *testing.T) {
	store := NewStateStore(filepath.Join(t.TempDir(), "state"), filepath.Join(t.TempDir(), "artifacts"))
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	mgr := &TunnelManager{Store: store}
	state := TunnelState{
		Version:         1,
		RunID:           "run-1",
		Managed:         true,
		PID:             os.Getpid(),
		BinaryPath:      filepath.Join(t.TempDir(), "not-browserstack-local"),
		LocalIdentifier: "local-1",
		Owner:           "efp-mobile",
		Status:          "running",
	}
	if err := mgr.Save(state); err != nil {
		t.Fatal(err)
	}
	_, err := mgr.Stop(state)
	var me *Error
	if !errors.As(err, &me) || me.Code != "local_tunnel_ownership_mismatch" {
		t.Fatalf("expected ownership mismatch, got %#v", err)
	}
}

func TestCleanupOrphansMarksUnverifiedRunningPID(t *testing.T) {
	store := NewStateStore(filepath.Join(t.TempDir(), "state"), filepath.Join(t.TempDir(), "artifacts"))
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	mgr := &TunnelManager{Store: store}
	state := TunnelState{
		Version:         1,
		RunID:           "run-1",
		Managed:         true,
		PID:             os.Getpid(),
		BinaryPath:      filepath.Join(t.TempDir(), "not-browserstack-local"),
		LocalIdentifier: "local-1",
		Owner:           "efp-mobile",
		Status:          "running",
		Deadline:        time.Now().UTC().Add(-time.Minute),
	}
	if err := mgr.Save(state); err != nil {
		t.Fatal(err)
	}
	stopped, err := mgr.CleanupOrphans()
	if err != nil {
		t.Fatal(err)
	}
	if len(stopped) != 1 || stopped[0].Status != "ownership_unverified" {
		t.Fatalf("stopped=%#v", stopped)
	}
	got, err := mgr.Load("run-1", "local-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "ownership_unverified" {
		t.Fatalf("status=%s", got.Status)
	}
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func assertFlagValues(t *testing.T, args []string, flag string, want []string) {
	t.Helper()
	for i, arg := range args {
		if arg != flag {
			continue
		}
		got := []string{}
		for _, value := range args[i+1:] {
			if strings.HasPrefix(value, "--") {
				break
			}
			got = append(got, value)
		}
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Fatalf("%s values=%#v want %#v in %#v", flag, got, want, args)
		}
		for _, value := range got {
			if strings.Contains(value, ",") {
				t.Fatalf("%s should pass variadic args, not comma-joined value %q", flag, value)
			}
		}
		return
	}
	t.Fatalf("%s missing in %#v", flag, args)
}
