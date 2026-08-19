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
