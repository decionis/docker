package mcpclient

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func startFake(t *testing.T, mode string, timeout time.Duration) (*Client, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)
	client, err := Start(ctx,
		[]string{os.Args[0], "-test.run=TestHelperProcess"},
		[]string{"GO_WANT_HELPER_PROCESS=1", "MCP_FAKE_MODE=" + mode})
	if err != nil {
		t.Fatalf("start fake: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client, ctx
}

func TestHandshakeAndToolCall(t *testing.T) {
	client, ctx := startFake(t, "ok", 10*time.Second)

	name, err := client.Initialize(ctx, "test", "0")
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if name != "fake-mcp" {
		t.Fatalf("server name %q", name)
	}

	result, err := client.CallTool(ctx, "decionis_evaluate", map[string]any{"payload": map[string]any{}})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if result.IsError || !strings.Contains(result.Text, `"verdict":"block"`) {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestFailsClosedWhenEvaluatorExitsEarly(t *testing.T) {
	client, ctx := startFake(t, "exit", 10*time.Second)
	if _, err := client.Initialize(ctx, "test", "0"); err == nil {
		t.Fatal("early exit must surface as an error")
	}
}

func TestFailsClosedOnSilentEvaluator(t *testing.T) {
	client, ctx := startFake(t, "silent", 2*time.Second)
	if _, err := client.Initialize(ctx, "test", "0"); err == nil {
		t.Fatal("a silent evaluator must time out into an error")
	}
}

// TestHelperProcess is the fake stdio MCP server (newline-delimited JSON),
// spawned by the tests above.
func TestHelperProcess(_ *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	mode := os.Getenv("MCP_FAKE_MODE")
	if mode == "exit" {
		os.Exit(0)
	}

	respond := func(value any) {
		encoded, _ := json.Marshal(value)
		os.Stdout.Write(append(encoded, '\n'))
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	for scanner.Scan() {
		if mode == "silent" {
			continue
		}
		var request struct {
			ID     *int64 `json:"id"`
			Method string `json:"method"`
		}
		if json.Unmarshal(scanner.Bytes(), &request) != nil || request.ID == nil {
			continue
		}
		switch request.Method {
		case "initialize":
			respond(map[string]any{"jsonrpc": "2.0", "id": *request.ID, "result": map[string]any{
				"protocolVersion": "2025-06-18",
				"serverInfo":      map[string]any{"name": "fake-mcp", "version": "0"},
			}})
		case "tools/call":
			respond(map[string]any{"jsonrpc": "2.0", "id": *request.ID, "result": map[string]any{
				"content": []map[string]any{{"type": "text", "text": `{"verdict":"block","outcome":"REJECT"}`}},
				"isError": false,
			}})
		default:
			respond(map[string]any{"jsonrpc": "2.0", "id": *request.ID, "error": map[string]any{"code": -32601, "message": "nope"}})
		}
	}
	os.Exit(0)
}
