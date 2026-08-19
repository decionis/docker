package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

// EnrollmentExchange is the result of exchanging a single-use connector
// enrollment token (dcn_enroll_*): the org and a freshly minted, scoped org
// API key. The raw key crosses the wire exactly once, in this response, and
// goes straight into the daemon's store — never logs, never the UI
// (rules/security.rules.md Rules 2.2–2.4).
type EnrollmentExchange struct {
	OrgID         string `json:"org_id"`
	RawKey        string `json:"raw_key"`
	ConnectorSlug string `json:"connector_slug"`
	Environment   string `json:"environment"`
}

// Enrollment exchange failures the UI can act on, mapped from the exchange
// endpoint's stable error codes — response bodies are never echoed.
var (
	ErrEnrollmentInvalid = errors.New("the enrollment token is invalid or expired")
	ErrEnrollmentUsed    = errors.New("this enrollment token has already been exchanged")
)

// ExchangeEnrollment redeems a single-use enrollment token at the control
// plane's public, possession-authenticated exchange endpoint
// (POST /v1/connectors/enrollments/exchange) — the same mechanism Decionis
// connectors use to self-provision credentials.
func ExchangeEnrollment(ctx context.Context, baseURL, enrollmentToken string) (*EnrollmentExchange, error) {
	normalizedBaseURL, err := ValidateBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(enrollmentToken)) < 20 {
		return nil, ErrEnrollmentInvalid
	}

	payload, err := json.Marshal(map[string]string{"enrollment_token": strings.TrimSpace(enrollmentToken)})
	if err != nil {
		return nil, errors.New("decionis api: enrollment exchange: encode failed")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		normalizedBaseURL+"/v1/connectors/enrollments/exchange", bytes.NewReader(payload))
	if err != nil {
		return nil, errors.New("decionis api: enrollment exchange: build request failed")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, errors.New("decionis api: enrollment exchange unreachable")
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, errors.New("decionis api: enrollment exchange: response read failed")
	}

	switch response.StatusCode {
	case http.StatusOK:
		var exchange EnrollmentExchange
		if err := json.Unmarshal(body, &exchange); err != nil ||
			exchange.OrgID == "" || exchange.RawKey == "" {
			return nil, errors.New("decionis api: enrollment exchange: malformed response")
		}
		return &exchange, nil
	case http.StatusUnauthorized:
		return nil, ErrEnrollmentInvalid
	case http.StatusConflict:
		return nil, ErrEnrollmentUsed
	case http.StatusTooManyRequests:
		return nil, errors.New("decionis api: enrollment exchange rate-limited; try again shortly")
	default:
		return nil, &StatusError{Op: "enrollment exchange", StatusCode: response.StatusCode}
	}
}
