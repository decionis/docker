package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/decionis/docker/internal/govern"
)

// mergeMcpConfig sets mcpServers.decionis in a .mcp.json document (creating
// it when absent), preserving every other entry.
func mergeMcpConfig(existing []byte, image, absPolicyPath string) ([]byte, error) {
	document := map[string]any{}
	if len(existing) > 0 {
		if err := json.Unmarshal(existing, &document); err != nil {
			return nil, fmt.Errorf(".mcp.json exists but is not valid JSON — fix or remove it first")
		}
	}
	servers, ok := document["mcpServers"].(map[string]any)
	if !ok {
		servers = map[string]any{}
	}
	servers["decionis"] = map[string]any{
		"command": "docker",
		"args": []string{
			"run", "-i", "--rm", "--network", "none", "--read-only",
			"-v", absPolicyPath + ":/work/DECIONIS_POLICY.md:ro",
			image,
		},
	}
	document["mcpServers"] = servers
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func runHooks(args []string) int {
	if len(args) == 0 || args[0] != "install" {
		fmt.Fprintln(os.Stderr, "usage: decionis hooks install [flags]")
		return 2
	}
	flags := flag.NewFlagSet("hooks install", flag.ExitOnError)
	agent := flags.String("agent", "claude", "agent client to wire (supported: claude)")
	image := flags.String("image", govern.DefaultImage, "evaluator container image to reference")
	policy := flags.String("policy", "DECIONIS_POLICY.md", "policy file the evaluator mounts")
	dir := flags.String("dir", ".", "repository directory holding .mcp.json")
	dryRun := flags.Bool("dry-run", false, "print the resulting .mcp.json without writing")
	flags.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: decionis hooks install [flags]")
		flags.PrintDefaults()
	}
	_ = flags.Parse(args[1:])

	if *agent != "claude" {
		fmt.Fprintf(os.Stderr, "decionis hooks: agent %q is not supported yet (supported: claude). "+
			"Codex and Copilot wiring ships with the upstream @decionis/mcp hook templates.\n", *agent)
		return 2
	}

	absPolicy, err := filepath.Abs(filepath.Join(*dir, *policy))
	if err != nil {
		fmt.Fprintf(os.Stderr, "decionis hooks: %v\n", err)
		return 1
	}
	// Note: docker -v needs an absolute path, so the generated .mcp.json is
	// machine-specific. Inside a dev container the workspace path is stable,
	// which is the primary use of this command.
	configPath := filepath.Join(*dir, ".mcp.json")
	existing, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "decionis hooks: %v\n", err)
		return 1
	}

	merged, err := mergeMcpConfig(existing, *image, absPolicy)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decionis hooks: %v\n", err)
		return 1
	}
	if *dryRun {
		fmt.Print(string(merged))
		return 0
	}
	if err := os.WriteFile(configPath, merged, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "decionis hooks: %v\n", err)
		return 1
	}
	fmt.Printf("decionis: wired mcpServers.decionis into %s (image %s)\n", configPath, *image)
	if _, err := os.Stat(absPolicy); err != nil {
		fmt.Printf("decionis: note — %s does not exist yet; create it so the evaluator has rules to enforce\n", absPolicy)
	}
	return 0
}
