// Package api is the daemon's thin client for the hosted Decionis control
// plane, hand-written against the published OpenAPI contract
// (openapi/decionis-api.yaml upstream). It is a temporary bridge until the
// public Go SDK module is live (build-plan decision 3, 2026-08-18); it calls
// only published endpoints and never interprets policy.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/decionis/docker/internal/canonicaljson"
	"github.com/decionis/docker/internal/dossier"
)

// DefaultBaseURL matches the upstream Go SDK's default.
const DefaultBaseURL = "https://api.decionis.com"

const (
	maxReportsResponseBytes = 5 << 20
	maxDossierResponseBytes = 5 << 20
	maxJwksResponseBytes    = 1 << 20
)

// Config identifies one org connection. The API key is held only here and in
// the store — it must never reach logs or the extension UI
// (rules/security.rules.md Rules 2.2–2.4).
type Config struct {
	BaseURL string
	OrgID   string
	APIKey  string
}

// StatusError is a sanitized upstream failure: operation and status code
// only — never response bodies, URLs with credentials, or headers.
type StatusError struct {
	Op         string
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("decionis api: %s returned HTTP %d", e.Op, e.StatusCode)
}

// IsAuthError reports whether err is an upstream 401/403.
func IsAuthError(err error) bool {
	var statusErr *StatusError
	return errors.As(err, &statusErr) && (statusErr.StatusCode == http.StatusUnauthorized || statusErr.StatusCode == http.StatusForbidden)
}

// Client calls the hosted control plane with finite timeouts and bounded
// response reads (rules/security.rules.md Rule 3.4).
type Client struct {
	config     Config
	httpClient *http.Client
}

func localhostHost(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// NewClient validates the connection config. HTTPS is mandatory except for
// localhost (development and tests) — rules/security.rules.md Rule 2.6.
func NewClient(config Config) (*Client, error) {
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if config.BaseURL == "" {
		config.BaseURL = DefaultBaseURL
	}
	if !strings.HasPrefix(config.BaseURL, "https://") && !localhostHost(config.BaseURL) {
		return nil, errors.New("base URL must use https")
	}
	if parsed, err := url.Parse(config.BaseURL); err != nil || parsed.User != nil {
		return nil, errors.New("base URL must be a plain origin without credentials")
	}
	if strings.TrimSpace(config.OrgID) == "" {
		return nil, errors.New("org id is required")
	}
	if strings.TrimSpace(config.APIKey) == "" {
		return nil, errors.New("api key is required")
	}
	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
				TLSHandshakeTimeout:   5 * time.Second,
				ResponseHeaderTimeout: 15 * time.Second,
				IdleConnTimeout:       5 * time.Second,
				MaxIdleConns:          4,
			},
		},
	}, nil
}

// BaseURL returns the validated origin (no credentials by construction).
func (c *Client) BaseURL() string { return c.config.BaseURL }

// OrgID returns the connected org id.
func (c *Client) OrgID() string { return c.config.OrgID }

func (c *Client) getJSON(ctx context.Context, op, requestURL string, limit int64, out any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return fmt.Errorf("decionis api: %s: build request failed", op)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("decionis api: %s unreachable", op)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return &StatusError{Op: op, StatusCode: response.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, limit))
	if err != nil {
		return fmt.Errorf("decionis api: %s: response read failed", op)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decionis api: %s: malformed response", op)
	}
	return nil
}

// Health calls GET /v1/health (operationId getHealth).
func (c *Client) Health(ctx context.Context) error {
	var ignored map[string]any
	return c.getJSON(ctx, "health", c.config.BaseURL+"/v1/health", 64<<10, &ignored)
}

// Report mirrors ShadowEvaluateDecisionReport in the published contract.
type Report struct {
	EvaluationID               string   `json:"evaluation_id"`
	DossierID                  string   `json:"dossier_id"`
	CreatedAt                  string   `json:"created_at"`
	DecisionType               string   `json:"decision_type"`
	DecisionDomain             *string  `json:"decision_domain,omitempty"`
	Amount                     *float64 `json:"amount,omitempty"`
	RiskScore                  *float64 `json:"risk_score,omitempty"`
	Channel                    *string  `json:"channel,omitempty"`
	PolicyVersion              string   `json:"policy_version"`
	Mode                       string   `json:"mode"`
	Outcome                    string   `json:"outcome"`
	Confidence                 float64  `json:"confidence"`
	WouldExecute               bool     `json:"would_execute"`
	ExecutionAction            string   `json:"execution_action"`
	Reason                     string   `json:"reason"`
	PolicyGuardReason          *string  `json:"policy_guard_reason,omitempty"`
	PolicyEvaluationResolution *string  `json:"policy_evaluation_resolution,omitempty"`
	SelectedRuleID             *string  `json:"selected_rule_id,omitempty"`
	DossierAPIPath             string   `json:"dossier_api_path"`
}

// ReportSummary mirrors ShadowEvaluateDecisionReportSummary.
type ReportSummary struct {
	TotalEvaluations    int            `json:"total_evaluations"`
	WouldApproveCount   int            `json:"would_approve_count"`
	WouldBlockCount     int            `json:"would_block_count"`
	WouldEscalateCount  int            `json:"would_escalate_count"`
	ReviewRequiredCount int            `json:"review_required_count"`
	NonApproveCount     int            `json:"non_approve_count"`
	NonApproveRate      float64        `json:"non_approve_rate"`
	PolicyMismatchCount int            `json:"policy_mismatch_count"`
	PolicyMismatchRate  float64        `json:"policy_mismatch_rate"`
	NearMissCount       int            `json:"near_miss_count"`
	NearMissRate        float64        `json:"near_miss_rate"`
	OutcomeCounts       map[string]int `json:"outcome_counts,omitempty"`
}

// ReportsResponse mirrors ShadowEvaluateDecisionReportsResponse.
type ReportsResponse struct {
	Service         string        `json:"service"`
	ProtocolVersion string        `json:"protocol_version"`
	GeneratedAt     string        `json:"generated_at"`
	OrgID           string        `json:"org_id"`
	Mode            string        `json:"mode"`
	Since           *string       `json:"since,omitempty"`
	Count           int           `json:"count"`
	Reports         []Report      `json:"reports"`
	Summary         ReportSummary `json:"summary"`
}

// ListReports calls GET /v1/protocol/shadow/evaluate-decision/reports
// (operationId listShadowEvaluateDecisionReports). Mode is one of
// SHADOW | PARALLEL | ENFORCEMENT per the contract.
func (c *Client) ListReports(ctx context.Context, mode string, limit int) (*ReportsResponse, error) {
	query := url.Values{}
	query.Set("org_id", c.config.OrgID)
	if mode != "" {
		query.Set("mode", mode)
	}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	requestURL := c.config.BaseURL + "/v1/protocol/shadow/evaluate-decision/reports?" + query.Encode()
	var response ReportsResponse
	if err := c.getJSON(ctx, "list reports", requestURL, maxReportsResponseBytes, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetDossier calls GET /v1/protocol/dossiers/{dossier_id} (operationId
// getDecisionDossier) and returns the extracted signed payload plus the URL
// it was served from (for relative JWKS resolution).
func (c *Client) GetDossier(ctx context.Context, dossierID string) (map[string]any, string, error) {
	requestURL := c.config.BaseURL + "/v1/protocol/dossiers/" + url.PathEscape(dossierID) +
		"?org_id=" + url.QueryEscape(c.config.OrgID)
	var raw json.RawMessage
	if err := c.getJSON(ctx, "get dossier", requestURL, maxDossierResponseBytes, &raw); err != nil {
		return nil, "", err
	}
	decoded, err := canonicaljson.Decode(raw)
	if err != nil {
		return nil, "", errors.New("decionis api: get dossier: malformed response")
	}
	return dossier.ExtractDossierPayload(decoded), requestURL, nil
}

// FetchJwks fetches a public JWKS. The URL comes from the dossier's own
// rotation policy; HTTPS is required except for localhost.
func (c *Client) FetchJwks(ctx context.Context, jwksURL string) (*dossier.Jwks, error) {
	if !strings.HasPrefix(strings.ToLower(jwksURL), "https://") && !localhostHost(jwksURL) {
		return nil, errors.New("jwks url must use https")
	}
	var jwks dossier.Jwks
	if err := c.getJSON(ctx, "fetch jwks", jwksURL, maxJwksResponseBytes, &jwks); err != nil {
		return nil, err
	}
	return &jwks, nil
}
