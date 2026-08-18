// decionis is the Decionis CLI for Docker-based environments: govern a
// command with the local evaluator, verify Decision Dossiers offline, check
// policy status, and wire the containerized MCP server into agent clients.
package main

import (
	"fmt"
	"os"

	"github.com/decionis/docker/internal/version"
)

const usage = `decionis — execution authority for AI agents and automated workflows

Usage:
  decionis govern [flags] -- <command> [args…]   evaluate the intent, run only on allow
  decionis verify [flags] <dossier.json>         verify a Decision Dossier's Ed25519 proof offline
  decionis status [flags]                        evaluator + policy status
  decionis hooks install [flags]                 wire the decionis/mcp container into an agent client
  decionis version                               print the CLI version

Verdicts follow the evaluator's own vocabulary — allow → proceed; block → do
not proceed; review/escalate → stop and involve a human. govern exit codes:
the governed command's own code on allow, 2 on block, 3 on review/escalate,
4 when evaluation itself fails (fail closed).

Run any subcommand with -h for its flags.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "govern":
		os.Exit(runGovern(os.Args[2:]))
	case "verify":
		os.Exit(runVerify(os.Args[2:]))
	case "status":
		os.Exit(runStatus(os.Args[2:]))
	case "hooks":
		os.Exit(runHooks(os.Args[2:]))
	case "version", "--version", "-v":
		fmt.Println(version.Version)
	case "help", "--help", "-h":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "decionis: unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
}
