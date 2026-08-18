package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/decionis/docker/internal/govern"
	"github.com/decionis/docker/internal/version"
)

type contextFlags []string

func (c *contextFlags) String() string { return strings.Join(*c, ",") }
func (c *contextFlags) Set(v string) error {
	*c = append(*c, v)
	return nil
}

var actionCharset = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// deriveAction turns the governed command into a default action label:
// "terraform apply …" → "terraform.apply".
func deriveAction(command []string) string {
	if len(command) == 0 {
		return ""
	}
	label := filepath.Base(command[0])
	if len(command) > 1 && !strings.HasPrefix(command[1], "-") {
		label += "." + command[1]
	}
	return actionCharset.ReplaceAllString(label, "_")
}

// parseContextEntries turns repeated -context key=value flags into payload
// fields, coercing booleans and numbers.
func parseContextEntries(entries []string) (map[string]any, error) {
	payload := map[string]any{}
	for _, entry := range entries {
		key, value, found := strings.Cut(entry, "=")
		if !found || key == "" {
			return nil, fmt.Errorf("-context %q must be key=value", entry)
		}
		switch value {
		case "true":
			payload[key] = true
		case "false":
			payload[key] = false
		default:
			if number, err := strconv.ParseFloat(value, 64); err == nil {
				payload[key] = number
			} else {
				payload[key] = value
			}
		}
	}
	return payload, nil
}

func runGovern(args []string) int {
	flags := flag.NewFlagSet("govern", flag.ExitOnError)
	action := flags.String("action", "", "action label to evaluate (default: derived from the command)")
	policy := flags.String("policy", "DECIONIS_POLICY.md", "policy file to evaluate against")
	image := flags.String("image", govern.DefaultImage, "evaluator container image")
	evaluatorCmd := flags.String("evaluator-cmd", "", "custom evaluator command instead of the container (space-separated)")
	mode := flags.String("mode", "enforce", "evaluation mode: enforce or shadow")
	timeout := flags.Duration("timeout", 60*time.Second, "overall evaluation timeout")
	agentGenerated := flags.Bool("agent-generated", false, "mark the action as agent-generated in the payload")
	dryRun := flags.Bool("dry-run", false, "evaluate and print the verdict without executing")
	var contexts contextFlags
	flags.Var(&contexts, "context", "payload field key=value (repeatable)")
	flags.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: decionis govern [flags] -- <command> [args…]")
		flags.PrintDefaults()
	}
	_ = flags.Parse(args)

	command := flags.Args()
	if len(command) == 0 && !*dryRun {
		fmt.Fprintln(os.Stderr, "decionis govern: no command given (put it after --), or use -dry-run with -action")
		return 2
	}
	if *mode != "enforce" && *mode != "shadow" {
		fmt.Fprintln(os.Stderr, "decionis govern: -mode must be enforce or shadow")
		return 2
	}

	payload, err := parseContextEntries(contexts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decionis govern: %v\n", err)
		return 2
	}
	actionLabel := *action
	if actionLabel == "" {
		actionLabel = deriveAction(command)
	}
	if actionLabel == "" {
		fmt.Fprintln(os.Stderr, "decionis govern: -action is required with -dry-run and no command")
		return 2
	}
	payload["action"] = actionLabel
	if *agentGenerated {
		payload["agent_generated"] = true
	}

	cfg := govern.Config{PolicyPath: *policy, Image: *image, Timeout: *timeout}
	if *evaluatorCmd != "" {
		cfg.EvaluatorCmd = strings.Fields(*evaluatorCmd)
	}

	verdict, err := govern.Evaluate(context.Background(), cfg, payload, *mode, version.Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decionis govern: %v\ndecionis govern: failing closed — the command was NOT executed.\n", err)
		return govern.ExitEvaluatorFailure
	}

	rule := verdict.SelectedRule
	if rule == "" {
		rule = "(no rule matched)"
	}
	fmt.Fprintf(os.Stderr, "decionis: action=%s verdict=%s outcome=%s rule=%q policy=%s\n",
		actionLabel, verdict.Verdict, verdict.Outcome, rule, verdict.PolicyVersion)

	if *dryRun {
		encoded, _ := json.MarshalIndent(verdict.Raw, "", "  ")
		fmt.Println(string(encoded))
		return verdict.ExitCode()
	}

	if !verdict.Allowed() {
		switch verdict.Verdict {
		case "block":
			fmt.Fprintln(os.Stderr, "decionis: blocked — do not proceed.")
		default:
			fmt.Fprintln(os.Stderr, "decionis: held — stop and involve a human.")
		}
		return verdict.ExitCode()
	}

	child := exec.Command(command[0], command[1:]...)
	child.Stdin, child.Stdout, child.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := child.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "decionis govern: command failed to start: %v\n", err)
		return 126
	}
	return 0
}
