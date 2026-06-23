package mobile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestLocalBinaryArgsDoNotExposeAccessKey(t *testing.T) {
	secret := "bs-secret-value"
	args := localBinaryArgs("local.yml", "local-1", config.MobileLocalConfig{})
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
