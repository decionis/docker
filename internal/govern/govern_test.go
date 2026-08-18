package govern

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeSession struct {
	toolText  string
	isError   bool
	toolErr   error
	initErr   error
	lastTool  string
	lastArgs  any
	closeDone bool
}

func (f *fakeSession) Initialize(context.Context, string, string) (string, error) {
	return "decionis-mcp", f.initErr
}

func (f *fakeSession) CallTool(_ context.Context, name string, args any) (string, bool, error) {
	f.lastTool = name
	f.lastArgs = args
	return f.toolText, f.isError, f.toolErr
}

func (f *fakeSession) Close() error {
	f.closeDone = true
	return nil
}

func withFakeSession(t *testing.T, fake *fakeSession, startErr error) {
	t.Helper()
	original := startSession
	startSession = func(context.Context, []string, []string) (session, error) {
		if startErr != nil {
			return nil, startErr
		}
		return fake, nil
	}
	t.Cleanup(func() { startSession = original })
}

func tempPolicy(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "DECIONIS_POLICY.md")
	if err := os.WriteFile(path, []byte("# policy"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEvaluateParsesVerdict(t *testing.T) {
	fake := &fakeSession{toolText: `{"verdict":"block","outcome":"REJECT","selected_rule":"Freeze","policy_version":"v1"}`}
	withFakeSession(t, fake, nil)

	verdict, err := Evaluate(context.Background(), Config{PolicyPath: tempPolicy(t)},
		map[string]any{"action": "deploy"}, "enforce", "test")
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if verdict.Verdict != "block" || verdict.Outcome != "REJECT" || verdict.SelectedRule != "Freeze" {
		t.Fatalf("unexpected verdict: %+v", verdict)
	}
	if verdict.ExitCode() != ExitBlocked {
		t.Fatalf("block must map to exit %d, got %d", ExitBlocked, verdict.ExitCode())
	}
	if fake.lastTool != "decionis_evaluate" {
		t.Fatalf("wrong tool called: %s", fake.lastTool)
	}
	if !fake.closeDone {
		t.Fatal("session must be closed")
	}
}

func TestExitCodeMapping(t *testing.T) {
	cases := map[string]int{
		"allow":    0,
		"block":    ExitBlocked,
		"review":   ExitNeedsHuman,
		"escalate": ExitNeedsHuman,
		"???":      ExitEvaluatorFailure, // unknown verdicts fail closed
	}
	for verdict, want := range cases {
		if got := (Verdict{Verdict: verdict}).ExitCode(); got != want {
			t.Errorf("verdict %q: exit %d, want %d", verdict, got, want)
		}
	}
}

func TestEvaluateFailsClosed(t *testing.T) {
	policy := tempPolicy(t)
	payload := map[string]any{"action": "x"}

	t.Run("evaluator start failure", func(t *testing.T) {
		withFakeSession(t, nil, errors.New("no docker"))
		if _, err := Evaluate(context.Background(), Config{PolicyPath: policy}, payload, "enforce", "test"); err == nil {
			t.Fatal("start failure must be an error")
		}
	})
	t.Run("tool isError (policy does not compile)", func(t *testing.T) {
		withFakeSession(t, &fakeSession{toolText: `{"error":"no_rules_block"}`, isError: true}, nil)
		if _, err := Evaluate(context.Background(), Config{PolicyPath: policy}, payload, "enforce", "test"); err == nil {
			t.Fatal("isError must be an error")
		}
	})
	t.Run("malformed verdict", func(t *testing.T) {
		withFakeSession(t, &fakeSession{toolText: `not json`}, nil)
		if _, err := Evaluate(context.Background(), Config{PolicyPath: policy}, payload, "enforce", "test"); err == nil {
			t.Fatal("malformed verdict must be an error")
		}
	})
	t.Run("missing verdict field", func(t *testing.T) {
		withFakeSession(t, &fakeSession{toolText: `{"outcome":"REJECT"}`}, nil)
		if _, err := Evaluate(context.Background(), Config{PolicyPath: policy}, payload, "enforce", "test"); err == nil {
			t.Fatal("empty verdict must be an error")
		}
	})
	t.Run("missing policy file", func(t *testing.T) {
		withFakeSession(t, &fakeSession{}, nil)
		if _, err := Evaluate(context.Background(), Config{PolicyPath: filepath.Join(t.TempDir(), "absent.md")}, payload, "enforce", "test"); err == nil {
			t.Fatal("missing policy must be an error")
		}
	})
}

func TestArgvDockerAndCustom(t *testing.T) {
	policy := tempPolicy(t)

	argv, env, err := Config{PolicyPath: policy}.argv()
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")
	if !strings.HasPrefix(joined, "docker run -i --rm --network none --read-only -v ") ||
		!strings.HasSuffix(joined, " "+DefaultImage) ||
		!strings.Contains(joined, ":/work/DECIONIS_POLICY.md:ro") {
		t.Fatalf("unexpected docker argv: %q", joined)
	}
	if len(env) != 0 {
		t.Fatalf("docker mode needs no extra env, got %v", env)
	}

	argv, env, err = Config{PolicyPath: policy, EvaluatorCmd: []string{"npx", "-y", "@decionis/mcp"}}.argv()
	if err != nil {
		t.Fatal(err)
	}
	if argv[0] != "npx" || len(env) != 1 || !strings.HasPrefix(env[0], "DECIONIS_POLICY_PATH=") {
		t.Fatalf("unexpected custom argv/env: %v %v", argv, env)
	}
}
