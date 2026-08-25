package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

// Workspace is a connected workspace and the scoped key that reaches it.
// Both the automatic-signup mint and the account sign-in return this shape,
// and it matches EnrollmentExchange field-for-field, so the daemon stores
// every connect path through one code path.
//
// Reused marks a provision the control plane resolved to this install's
// existing workspace instead of minting a new one. RawKey is then a fresh
// key and every previously issued key for the org was just revoked, so the
// caller must overwrite whatever it had stored.
type Workspace struct {
	OrgID       string `json:"org_id"`
	RawKey      string `json:"raw_key"`
	OrgName     string `json:"org_name"`
	Provisional bool   `json:"provisional"`
	Reused      bool   `json:"reused"`
}

// WorkspaceClaimedError is the control plane refusing to re-key: the
// workspace this install minted has since been claimed by a real account,
// and possession of the installation id must not mint credentials for
// someone's account. The user signs in instead; SignInURL is the plain
// browser fallback for doing so.
type WorkspaceClaimedError struct {
	OrgID     string
	SignInURL string
}

func (e *WorkspaceClaimedError) Error() string {
	return "this workspace belongs to an account — sign in to reconnect it"
}

// Account sign-in failures the UI can act on. Response bodies are never
// echoed: the control plane deliberately answers the same message for an
// unknown address and a wrong password, and this preserves that.
var (
	ErrCredentialsInvalid = errors.New("that email and password did not match a Decionis account")
	ErrCredentialsLocked  = errors.New("too many failed attempts; wait a few minutes and try again")
	ErrNoWorkspace        = errors.New("that account has no Decionis workspace yet")
	ErrSignupUnavailable  = errors.New("Decionis could not create a workspace right now")
)

// ProvisionWorkspace creates a workspace with no account and no credential —
// the automatic signup behind the extension's first run. The scoped key in
// the response is the credential from then on.
//
// dockerUsername names the workspace when the caller can supply one
// honestly. The extension sends nothing: Docker Desktop does not expose the
// signed-in Hub user to extensions, and the only local source is the
// credential store, which would mean reading the user's Hub secret to learn
// a display name. The control plane then names the workspace itself.
//
// installationID is this install's persisted possession secret (hashed at
// rest server-side). Sent on every provision, it lets the control plane
// answer with the workspace this install already minted — Reused, with a
// fresh key — or a *WorkspaceClaimedError once that workspace belongs to an
// account, instead of minting a new workspace per call. An older control
// plane ignores the field and mints exactly as before.
func (c *Client) ProvisionWorkspace(ctx context.Context, dockerUsername, installationID string) (*Workspace, error) {
	payload := map[string]string{}
	if trimmed := strings.TrimSpace(dockerUsername); trimmed != "" {
		payload["docker_username"] = trimmed
	}
	if trimmed := strings.TrimSpace(installationID); trimmed != "" {
		payload["installation_id"] = trimmed
	}
	return c.postWorkspace(ctx, "/v1/public/connect/docker-desktop/provision", payload)
}

// ConnectWithAccount resolves an account's email and password into its
// workspace and a freshly minted scoped key. The password is used for this
// one request and never stored — only the returned key is.
func (c *Client) ConnectWithAccount(ctx context.Context, email, password string) (*Workspace, error) {
	payload := map[string]string{
		"email":    strings.TrimSpace(email),
		"password": password,
	}
	return c.postWorkspace(ctx, "/v1/public/connect/docker-desktop/credentials", payload)
}

// postWorkspace issues one pre-auth request.
//
// It takes no URL: the destination is the client's own base URL, already
// normalized by ValidateBaseURL when the client was constructed (rebuilt
// from parsed scheme + validated host + path; https off loopback;
// credentials, query, and fragment rejected). `path` is a package constant.
// The host stays operator-configurable on purpose — self-hosted control
// planes — which is why the UI names the destination wherever a password is
// typed.
func (c *Client) postWorkspace(
	ctx context.Context,
	path string,
	payload map[string]string,
) (*Workspace, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.New("decionis api: workspace request: encode failed")
	}
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.config.BaseURL+path, bytes.NewReader(encoded),
	)
	if err != nil {
		return nil, errors.New("decionis api: workspace request: build failed")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, ErrSignupUnavailable
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, ErrSignupUnavailable
	}

	switch response.StatusCode {
	case http.StatusOK, http.StatusCreated:
		// The claimed shape carries no raw_key on purpose — decode it before
		// the credential validation below can mistake it for a broken mint.
		var answer struct {
			Workspace
			Claimed   bool   `json:"claimed"`
			SignInURL string `json:"sign_in_url"`
		}
		if err := json.Unmarshal(body, &answer); err != nil {
			return nil, errors.New("decionis api: workspace request: malformed response")
		}
		if answer.Claimed {
			return nil, &WorkspaceClaimedError{OrgID: answer.OrgID, SignInURL: answer.SignInURL}
		}
		if answer.OrgID == "" || answer.RawKey == "" {
			return nil, errors.New("decionis api: workspace request: malformed response")
		}
		workspace := answer.Workspace
		return &workspace, nil
	case http.StatusUnauthorized:
		return nil, ErrCredentialsInvalid
	case http.StatusForbidden:
		return nil, ErrNoWorkspace
	case http.StatusTooManyRequests:
		return nil, ErrCredentialsLocked
	default:
		return nil, ErrSignupUnavailable
	}
}
