// Package mcpclient is a minimal stdio MCP client for driving the Decionis
// local evaluator (`decionis/mcp` — one JSON message per line, matching the
// upstream server's hand-rolled transport). It exists so the CLI can obtain
// verdicts; it interprets nothing (rules/coding.rules.md §2).
package mcpclient

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync/atomic"
)

const maxLineBytes = 4 << 20

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type incoming struct {
	id     int64
	result json.RawMessage
	err    *rpcError
}

// Client speaks MCP to a subprocess evaluator over stdio.
type Client struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	messages chan incoming
	nextID   atomic.Int64
}

// Start launches the evaluator subprocess. Its stderr passes through to ours
// (diagnostics only; the protocol rides stdout).
func Start(ctx context.Context, argv []string, extraEnv []string) (*Client, error) {
	if len(argv) == 0 {
		return nil, errors.New("mcpclient: empty evaluator command")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("mcpclient: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("mcpclient: stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcpclient: start %q: %w", argv[0], err)
	}

	client := &Client{cmd: cmd, stdin: stdin, messages: make(chan incoming, 16)}
	go func() {
		defer close(client.messages)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64<<10), maxLineBytes)
		for scanner.Scan() {
			var message struct {
				ID     *int64          `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  *rpcError       `json:"error"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &message); err != nil || message.ID == nil {
				continue // non-response noise is ignored; requests time out fail-closed
			}
			client.messages <- incoming{id: *message.ID, result: message.Result, err: message.Error}
		}
	}()
	return client, nil
}

func (c *Client) writeLine(value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("mcpclient: encode: %w", err)
	}
	if _, err := c.stdin.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("mcpclient: evaluator is not accepting input: %w", err)
	}
	return nil
}

func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	if err := c.writeLine(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		return nil, err
	}
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("mcpclient: %s timed out: %w", method, ctx.Err())
		case message, ok := <-c.messages:
			if !ok {
				return nil, fmt.Errorf("mcpclient: evaluator exited before responding to %s", method)
			}
			if message.id != id {
				continue
			}
			if message.err != nil {
				return nil, fmt.Errorf("mcpclient: %s failed: rpc %d: %s", method, message.err.Code, message.err.Message)
			}
			return message.result, nil
		}
	}
}

// Initialize performs the MCP handshake and returns the server's name.
func (c *Client) Initialize(ctx context.Context, clientName, clientVersion string) (string, error) {
	result, err := c.call(ctx, "initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": clientName, "version": clientVersion},
	})
	if err != nil {
		return "", err
	}
	var parsed struct {
		ServerInfo struct {
			Name string `json:"name"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return "", errors.New("mcpclient: malformed initialize response")
	}
	if err := c.writeLine(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}); err != nil {
		return "", err
	}
	return parsed.ServerInfo.Name, nil
}

// ToolResult is one tools/call outcome: the first text content block plus the
// server's isError flag.
type ToolResult struct {
	Text    string
	IsError bool
}

// CallTool invokes one MCP tool and returns its text payload.
func (c *Client) CallTool(ctx context.Context, name string, args any) (ToolResult, error) {
	result, err := c.call(ctx, "tools/call", map[string]any{"name": name, "arguments": args})
	if err != nil {
		return ToolResult{}, err
	}
	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return ToolResult{}, fmt.Errorf("mcpclient: malformed %s response", name)
	}
	for _, content := range parsed.Content {
		if content.Type == "text" {
			return ToolResult{Text: content.Text, IsError: parsed.IsError}, nil
		}
	}
	return ToolResult{}, fmt.Errorf("mcpclient: %s returned no text content", name)
}

// Close ends the session (EOF on stdin) and reaps the subprocess.
func (c *Client) Close() error {
	_ = c.stdin.Close()
	return c.cmd.Wait()
}
