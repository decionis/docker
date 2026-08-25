package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProvisionWorkspaceSendsTheInstallationID(t *testing.T) {
	var gotPath string
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"org_id":      "0b6f8a3e-6f2a-4c3e-9b1d-2f4a5b6c7d8e",
			"raw_key":     "dcn_live_minted",
			"org_name":    "My Docker Workspace",
			"provisional": true,
		})
	}))
	defer server.Close()

	workspace, err := mustPublicClient(t, server.URL).
		ProvisionWorkspace(context.Background(), "", "dcn_install_0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	if gotPath != "/v1/public/connect/docker-desktop/provision" {
		t.Fatalf("unexpected path %q", gotPath)
	}
	if gotBody["installation_id"] != "dcn_install_0123456789abcdef0123456789abcdef" {
		t.Fatalf("installation id not forwarded verbatim: %v", gotBody)
	}
	if _, present := gotBody["docker_username"]; present {
		t.Fatalf("empty username must be omitted: %v", gotBody)
	}
	if workspace.Reused {
		t.Fatal("a fresh mint must not read as reused")
	}
	if workspace.OrgID == "" || workspace.RawKey != "dcn_live_minted" {
		t.Fatalf("unexpected workspace: %+v", workspace)
	}
}

func TestProvisionWorkspaceParsesAReusedMint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"org_id":      "0b6f8a3e-6f2a-4c3e-9b1d-2f4a5b6c7d8e",
			"raw_key":     "dcn_live_fresh",
			"org_name":    "My Docker Workspace",
			"provisional": true,
			"reused":      true,
		})
	}))
	defer server.Close()

	workspace, err := mustPublicClient(t, server.URL).
		ProvisionWorkspace(context.Background(), "", "dcn_install_0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	if !workspace.Reused || workspace.RawKey != "dcn_live_fresh" || workspace.OrgName != "My Docker Workspace" {
		t.Fatalf("reused mint parsed wrong: %+v", workspace)
	}
}

func TestProvisionWorkspaceClaimedIsATypedAnswerNotAMalformedMint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// The claimed shape deliberately carries no raw_key: possession of
		// the installation id must not re-key an account's workspace.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"claimed":     true,
			"org_id":      "0b6f8a3e-6f2a-4c3e-9b1d-2f4a5b6c7d8e",
			"sign_in_url": "https://api.decionis.com/v1/public/connect/docker-desktop/start",
		})
	}))
	defer server.Close()

	_, err := mustPublicClient(t, server.URL).
		ProvisionWorkspace(context.Background(), "", "dcn_install_0123456789abcdef0123456789abcdef")
	var claimed *WorkspaceClaimedError
	if !errors.As(err, &claimed) {
		t.Fatalf("claimed answer must map to WorkspaceClaimedError, got %v", err)
	}
	if claimed.OrgID != "0b6f8a3e-6f2a-4c3e-9b1d-2f4a5b6c7d8e" ||
		claimed.SignInURL != "https://api.decionis.com/v1/public/connect/docker-desktop/start" {
		t.Fatalf("claimed answer lost its fields: %+v", claimed)
	}
}

func TestProvisionWorkspaceOmitsAnEmptyInstallationID(t *testing.T) {
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"org_id": "o", "raw_key": "k"})
	}))
	defer server.Close()

	if _, err := mustPublicClient(t, server.URL).ProvisionWorkspace(context.Background(), "", "  "); err != nil {
		t.Fatalf("provision: %v", err)
	}
	if _, present := gotBody["installation_id"]; present {
		t.Fatalf("blank installation id must be omitted: %v", gotBody)
	}
}
