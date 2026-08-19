package daemon

import (
	"context"
	"testing"

	"github.com/decionis/docker/internal/api"
)

// stubPublicAPI stands in for the pre-auth client. Tests set only the
// behaviour they care about; anything unset returns a benign zero.
type stubPublicAPI struct {
	workspace       *api.Workspace
	workspaceErr    error
	exchange        *api.EnrollmentExchange
	exchangeErr     error
	exchangeFn      func(token string) (*api.EnrollmentExchange, error)
	seenUsername    string
	seenEmail       string
	seenPassword    string
	seenToken       string
	provisionCalls  int
	credentialCalls int
}

func (s *stubPublicAPI) ProvisionWorkspace(_ context.Context, dockerUsername string) (*api.Workspace, error) {
	s.provisionCalls++
	s.seenUsername = dockerUsername
	return s.workspace, s.workspaceErr
}

func (s *stubPublicAPI) ConnectWithAccount(_ context.Context, email, password string) (*api.Workspace, error) {
	s.credentialCalls++
	s.seenEmail, s.seenPassword = email, password
	return s.workspace, s.workspaceErr
}

func (s *stubPublicAPI) ExchangeEnrollment(_ context.Context, token string) (*api.EnrollmentExchange, error) {
	s.seenToken = token
	if s.exchangeFn != nil {
		return s.exchangeFn(token)
	}
	return s.exchange, s.exchangeErr
}

// usePublicStub installs the stub behind the daemon's pre-auth seam and
// restores the real constructor afterwards. The base URL still passes the
// real transport gate, so an invalid URL is rejected exactly as in
// production.
func usePublicStub(t *testing.T, stub *stubPublicAPI) *stubPublicAPI {
	t.Helper()
	original := newPublicClient
	newPublicClient = func(baseURL string) (publicAPI, error) {
		if _, err := api.ValidateBaseURL(baseURL); err != nil {
			return nil, err
		}
		return stub, nil
	}
	t.Cleanup(func() { newPublicClient = original })
	return stub
}
