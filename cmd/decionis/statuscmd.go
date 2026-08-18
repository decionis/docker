package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/decionis/docker/internal/govern"
	"github.com/decionis/docker/internal/version"
)

func runStatus(args []string) int {
	flags := flag.NewFlagSet("status", flag.ExitOnError)
	policy := flags.String("policy", "DECIONIS_POLICY.md", "policy file to check")
	image := flags.String("image", govern.DefaultImage, "evaluator container image")
	evaluatorCmd := flags.String("evaluator-cmd", "", "custom evaluator command instead of the container (space-separated)")
	timeout := flags.Duration("timeout", 60*time.Second, "overall timeout")
	flags.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: decionis status [flags]")
		flags.PrintDefaults()
	}
	_ = flags.Parse(args)

	cfg := govern.Config{PolicyPath: *policy, Image: *image, Timeout: *timeout}
	if *evaluatorCmd != "" {
		cfg.EvaluatorCmd = strings.Fields(*evaluatorCmd)
	}

	serverName, text, err := govern.ReadPolicy(context.Background(), cfg, version.Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decionis status: %v\n", err)
		return 1
	}
	fmt.Printf("evaluator: %s (ok)\ncli: %s\n", serverName, version.Version)

	var pretty map[string]any
	if json.Unmarshal([]byte(text), &pretty) == nil {
		encoded, _ := json.MarshalIndent(pretty, "", "  ")
		fmt.Printf("policy:\n%s\n", string(encoded))
	} else {
		fmt.Printf("policy: %s\n", text)
	}
	return 0
}
