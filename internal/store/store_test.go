package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTripAndPermissions(t *testing.T) {
	dir := t.TempDir()
	s := New(filepath.Join(dir, "data"))

	if _, _, ok, err := s.Load(); err != nil || ok {
		t.Fatalf("empty store must load as not-ok without error; ok=%v err=%v", ok, err)
	}

	connection := Connection{BaseURL: "https://api.decionis.com", OrgID: "0b6f8a3e-6f2a-4c3e-9b1d-2f4a5b6c7d8e"}
	if err := s.Save(connection, "sk-test-sentinel"); err != nil {
		t.Fatalf("save: %v", err)
	}

	for _, name := range []string{"connection.json", "api-key"} {
		info, err := os.Stat(filepath.Join(dir, "data", name))
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("%s permissions = %o, want 0600 (rules/security.rules.md Rule 2.5)", name, perm)
		}
	}

	loaded, apiKey, ok, err := s.Load()
	if err != nil || !ok {
		t.Fatalf("load after save: ok=%v err=%v", ok, err)
	}
	if loaded != connection || apiKey != "sk-test-sentinel" {
		t.Fatalf("round trip mismatch: %+v %q", loaded, apiKey)
	}

	if err := s.Clear(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, _, ok, _ := s.Load(); ok {
		t.Fatal("store must be empty after clear")
	}
}

func TestSaveRejectsEmptyKey(t *testing.T) {
	s := New(t.TempDir())
	if err := s.Save(Connection{}, "   "); err == nil {
		t.Fatal("empty api key must be rejected")
	}
}

func TestInstallationIDRoundTripSurvivesClear(t *testing.T) {
	dir := t.TempDir()
	s := New(filepath.Join(dir, "data"))

	if id, err := s.LoadInstallationID(); err != nil || id != "" {
		t.Fatalf("empty store must load an empty id without error; id=%q err=%v", id, err)
	}
	if err := s.SaveInstallationID("  "); err == nil {
		t.Fatal("empty installation id must be rejected")
	}

	if err := s.SaveInstallationID("dcn_install_sentinel_0123456789abcdef"); err != nil {
		t.Fatalf("save installation id: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "data", "installation-id"))
	if err != nil {
		t.Fatalf("stat installation-id: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("installation-id permissions = %o, want 0600 (rules/security.rules.md Rule 2.5)", perm)
	}

	// Clear drops the connection, never the install's identity: the next
	// automatic signup must find the same workspace, not mint another.
	if err := s.Save(Connection{BaseURL: "https://api.decionis.com", OrgID: "org"}, "key"); err != nil {
		t.Fatalf("save connection: %v", err)
	}
	if err := s.Clear(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if id, err := s.LoadInstallationID(); err != nil || id != "dcn_install_sentinel_0123456789abcdef" {
		t.Fatalf("installation id after clear = %q (err %v), want it kept", id, err)
	}
}
