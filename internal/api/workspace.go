package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
)

// WorkspaceState is what the extension needs to describe itself: whether
// this workspace gates actions, how much of the free governed-decision
// allowance is left, and where to go to lift it.
type WorkspaceState struct {
	EnforcementEnabled   bool   `json:"enforcement_enabled"`
	EnforcementAvailable bool   `json:"enforcement_available"`
	EnforcementReverted  bool   `json:"enforcement_reverted"`
	GovernedUsed         int    `json:"governed_used"`
	GovernedLimit        *int   `json:"governed_limit"`
	Remaining            *int   `json:"remaining"`
	WarnAt               int    `json:"warn_at"`
	Warn                 bool   `json:"warn"`
	AtCap                bool   `json:"at_cap"`
	Provisional          bool   `json:"provisional"`
	SubscribeURL         string `json:"subscribe_url"`
}

// ErrEnforcementLimitReached is the control plane refusing to enable
// enforcement because the free allowance is spent.
var ErrEnforcementLimitReached = errors.New("this workspace has used its free governed decisions")

// GetWorkspaceState reads the workspace's enforcement state and allowance.
func (c *Client) GetWorkspaceState(ctx context.Context) (*WorkspaceState, error) {
	query := url.Values{}
	query.Set("org_id", c.config.OrgID)
	requestURL := c.config.BaseURL + "/v1/docker-desktop/workspace?" + query.Encode()

	var state WorkspaceState
	if err := c.getJSON(ctx, "workspace state", requestURL, 64<<10, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// SetEnforcement turns enforcement on or off for this workspace. Enabling
// once the allowance is spent is refused by the control plane, and that
// refusal is surfaced verbatim rather than being retried or hidden.
func (c *Client) SetEnforcement(ctx context.Context, enabled bool) (*WorkspaceState, error) {
	query := url.Values{}
	query.Set("org_id", c.config.OrgID)
	requestURL := c.config.BaseURL + "/v1/docker-desktop/workspace/enforcement?" + query.Encode()

	payload, err := json.Marshal(map[string]bool{"enabled": enabled})
	if err != nil {
		return nil, errors.New("decionis api: enforcement: encode failed")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, requestURL, bytes.NewReader(payload))
	if err != nil {
		return nil, errors.New("decionis api: enforcement: build request failed")
	}
	request.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, errors.New("decionis api: enforcement: control plane unreachable")
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return nil, errors.New("decionis api: enforcement: response read failed")
	}

	switch response.StatusCode {
	case http.StatusOK:
		var state WorkspaceState
		if err := json.Unmarshal(body, &state); err != nil {
			return nil, errors.New("decionis api: enforcement: malformed response")
		}
		return &state, nil
	case http.StatusPaymentRequired:
		return nil, ErrEnforcementLimitReached
	default:
		return nil, &StatusError{Op: "enforcement", StatusCode: response.StatusCode}
	}
}

// ClaimStart is the browser URL that claims this workspace, plus how long
// the token behind it lasts.
type ClaimStart struct {
	ClaimURL  string `json:"claim_url"`
	ExpiresIn int    `json:"expires_in"`
}

// ErrAlreadyClaimed means this workspace already belongs to an account.
var ErrAlreadyClaimed = errors.New("this workspace already belongs to an account")

// StartClaim mints a claim token for this workspace and returns the URL a
// browser must open to finish. The token is returned once and never stored
// here: it goes straight to the browser.
func (c *Client) StartClaim(ctx context.Context) (*ClaimStart, error) {
	query := url.Values{}
	query.Set("org_id", c.config.OrgID)
	requestURL := c.config.BaseURL + "/v1/docker-desktop/workspace/claim-token?" + query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, nil)
	if err != nil {
		return nil, errors.New("decionis api: claim: build request failed")
	}
	request.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, errors.New("decionis api: claim: control plane unreachable")
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return nil, errors.New("decionis api: claim: response read failed")
	}

	switch response.StatusCode {
	case http.StatusOK:
		var start ClaimStart
		if err := json.Unmarshal(body, &start); err != nil || start.ClaimURL == "" {
			return nil, errors.New("decionis api: claim: malformed response")
		}
		return &start, nil
	case http.StatusConflict:
		return nil, ErrAlreadyClaimed
	default:
		return nil, &StatusError{Op: "claim", StatusCode: response.StatusCode}
	}
}
