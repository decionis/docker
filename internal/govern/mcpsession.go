package govern

import (
	"context"

	"github.com/decionis/docker/internal/mcpclient"
)

// mcpSession adapts mcpclient.Client to the session interface.
type mcpSession struct {
	client *mcpclient.Client
}

func startMcpSession(ctx context.Context, argv []string, extraEnv []string) (session, error) {
	client, err := mcpclient.Start(ctx, argv, extraEnv)
	if err != nil {
		return nil, err
	}
	return &mcpSession{client: client}, nil
}

func (s *mcpSession) Initialize(ctx context.Context, clientName, clientVersion string) (string, error) {
	return s.client.Initialize(ctx, clientName, clientVersion)
}

func (s *mcpSession) CallTool(ctx context.Context, name string, args any) (string, bool, error) {
	result, err := s.client.CallTool(ctx, name, args)
	if err != nil {
		return "", false, err
	}
	return result.Text, result.IsError, nil
}

func (s *mcpSession) Close() error { return s.client.Close() }
