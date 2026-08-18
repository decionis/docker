// Package govern turns "run this command" into "evaluate the intent, then
// run it only on an allowing verdict" — the CLI core of `decionis govern`.
// Evaluation happens exclusively in the real Decionis evaluator (the
// containerized decionis/mcp image, or any custom evaluator command speaking
// the same stdio protocol); this package maps verdicts to exit behavior and
// never interprets policy (rules/security.rules.md Rule 0.2).
//
// Fail closed (Rule 0.3): an unreachable evaluator, a policy that does not
// compile, a timeout, or an unknown verdict all resolve to non-execution.
package govern

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Exit codes for `decionis govern` (0 = the governed command's own exit code
// after an allow verdict).
const (
	ExitBlocked          = 2 // verdict block: never execute
	ExitNeedsHuman       = 3 // verdict review/escalate: stop and involve a human
	ExitEvaluatorFailure = 4 // evaluator unreachable/failed: fail closed
)

// DefaultImage is the containerized evaluator this repo ships.
const DefaultImage = "decionis/mcp"

// Config selects and configures the evaluator.
type Config struct {
	// PolicyPath is the DECIONIS_POLICY.md to evaluate against.
	PolicyPath string
	// Image is the evaluator container image, used when EvaluatorCmd is empty.
	Image string
	// EvaluatorCmd overrides the container: a command speaking the same stdio
	// protocol (e.g. "npx -y @decionis/mcp"). The policy path is passed via
	// DECIONIS_POLICY_PATH.
	EvaluatorCmd []string
	// Timeout bounds the whole evaluation (finite time, Rule 3.4).
	Timeout time.Duration
}

func (c Config) timeout() time.Duration {
	if c.Timeout <= 0 {
		return 60 * time.Second
	}
	return c.Timeout
}

// argv resolves the evaluator command and environment.
func (c Config) argv() (argv []string, extraEnv []string, err error) {
	policy, err := filepath.Abs(c.PolicyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("govern: resolve policy path: %w", err)
	}
	if _, err := os.Stat(policy); err != nil {
		return nil, nil, fmt.Errorf("govern: policy file %s is not readable", policy)
	}
	if len(c.EvaluatorCmd) > 0 {
		return c.EvaluatorCmd, []string{"DECIONIS_POLICY_PATH=" + policy}, nil
	}
	image := c.Image
	if image == "" {
		image = DefaultImage
	}
	return []string{
		"docker", "run", "-i", "--rm",
		"--network", "none",
		"--read-only",
		"-v", policy + ":/work/DECIONIS_POLICY.md:ro",
		image,
	}, nil, nil
}

// Verdict is the evaluator's decision for one candidate action, in the
// protocol's own vocabulary.
type Verdict struct {
	Verdict       string         `json:"verdict"`
	Outcome       string         `json:"outcome"`
	Resolution    string         `json:"resolution"`
	SelectedRule  string         `json:"selected_rule"`
	PolicyVersion string         `json:"policy_version"`
	Note          string         `json:"note"`
	Raw           map[string]any `json:"-"`
}

// ExitCode maps a verdict to the govern exit code. Unknown verdicts fail
// closed as evaluator failures.
func (v Verdict) ExitCode() int {
	switch v.Verdict {
	case "allow":
		return 0
	case "block":
		return ExitBlocked
	case "review", "escalate":
		return ExitNeedsHuman
	default:
		return ExitEvaluatorFailure
	}
}

// Allowed reports whether execution may proceed.
func (v Verdict) Allowed() bool { return v.Verdict == "allow" }

type session interface {
	Initialize(ctx context.Context, clientName, clientVersion string) (string, error)
	CallTool(ctx context.Context, name string, args any) (toolText string, isError bool, err error)
	Close() error
}

// startSession is swapped in tests.
var startSession = startMcpSession

// Evaluate runs the candidate action payload through the evaluator in the
// given mode ("enforce" or "shadow") and returns the verdict.
func Evaluate(ctx context.Context, cfg Config, payload map[string]any, mode string, clientVersion string) (*Verdict, error) {
	argv, extraEnv, err := cfg.argv()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.timeout())
	defer cancel()

	client, err := startSession(ctx, argv, extraEnv)
	if err != nil {
		return nil, fmt.Errorf("govern: evaluator did not start: %w", err)
	}
	defer client.Close()

	if _, err := client.Initialize(ctx, "decionis-cli", clientVersion); err != nil {
		return nil, fmt.Errorf("govern: evaluator handshake failed: %w", err)
	}

	text, isError, err := client.CallTool(ctx, "decionis_evaluate", map[string]any{
		"payload": payload,
		"mode":    mode,
	})
	if err != nil {
		return nil, fmt.Errorf("govern: evaluation failed: %w", err)
	}
	if isError {
		return nil, fmt.Errorf("govern: evaluator rejected the request: %s", compact(text))
	}

	verdict := &Verdict{}
	if err := json.Unmarshal([]byte(text), verdict); err != nil {
		return nil, errors.New("govern: malformed verdict from evaluator")
	}
	if verdict.Verdict == "" {
		return nil, errors.New("govern: evaluator returned no verdict")
	}
	_ = json.Unmarshal([]byte(text), &verdict.Raw)
	return verdict, nil
}

// ReadPolicy asks the evaluator for the compiled view of the policy file
// (path, sha256, rules or compile errors) as raw JSON text.
func ReadPolicy(ctx context.Context, cfg Config, clientVersion string) (serverName, text string, err error) {
	argv, extraEnv, err := cfg.argv()
	if err != nil {
		return "", "", err
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.timeout())
	defer cancel()

	client, err := startSession(ctx, argv, extraEnv)
	if err != nil {
		return "", "", fmt.Errorf("govern: evaluator did not start: %w", err)
	}
	defer client.Close()

	serverName, err = client.Initialize(ctx, "decionis-cli", clientVersion)
	if err != nil {
		return "", "", fmt.Errorf("govern: evaluator handshake failed: %w", err)
	}
	toolText, isError, err := client.CallTool(ctx, "decionis_read_policy", map[string]any{})
	if err != nil {
		return serverName, "", fmt.Errorf("govern: read policy failed: %w", err)
	}
	if isError {
		return serverName, "", fmt.Errorf("govern: %s", compact(toolText))
	}
	return serverName, toolText, nil
}

func compact(text string) string {
	if len(text) > 400 {
		return text[:400] + "…"
	}
	return text
}
