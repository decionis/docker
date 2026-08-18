package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/decionis/docker/internal/canonicaljson"
	"github.com/decionis/docker/internal/dossier"
)

func loadJwks(source string) (*dossier.Jwks, error) {
	var raw []byte
	if strings.HasPrefix(strings.ToLower(source), "https://") || strings.HasPrefix(strings.ToLower(source), "http://") {
		parsed, err := url.Parse(source)
		if err != nil {
			return nil, fmt.Errorf("invalid JWKS URL: %w", err)
		}
		host := parsed.Hostname()
		isLocal := host == "localhost" || host == "127.0.0.1" || host == "::1"
		if parsed.Scheme != "https" && !isLocal {
			return nil, fmt.Errorf("JWKS URLs must use https (rules/security.rules.md Rule 2.6)")
		}
		client := &http.Client{Timeout: 15 * time.Second}
		response, err := client.Get(source)
		if err != nil {
			return nil, fmt.Errorf("JWKS fetch failed: %w", err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("JWKS fetch returned HTTP %d", response.StatusCode)
		}
		raw, err = io.ReadAll(io.LimitReader(response.Body, 1<<20))
		if err != nil {
			return nil, fmt.Errorf("JWKS read failed: %w", err)
		}
	} else {
		var err error
		raw, err = os.ReadFile(source)
		if err != nil {
			return nil, fmt.Errorf("JWKS file: %w", err)
		}
	}
	jwks := &dossier.Jwks{}
	if err := json.Unmarshal(raw, jwks); err != nil {
		return nil, fmt.Errorf("JWKS is not valid JSON")
	}
	return jwks, nil
}

func runVerify(args []string) int {
	flags := flag.NewFlagSet("verify", flag.ExitOnError)
	jwksSource := flags.String("jwks", "", "public JWKS file path or https URL (default: the dossier's own public_jwks_url)")
	asJSON := flags.Bool("json", false, "print the full verification result as JSON")
	flags.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: decionis verify [flags] <dossier.json>")
		flags.PrintDefaults()
	}
	_ = flags.Parse(args)
	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}

	raw, err := os.ReadFile(flags.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "decionis verify: %v\n", err)
		return 2
	}
	decoded, err := canonicaljson.Decode(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "decionis verify: the dossier file is not valid JSON")
		return 2
	}
	payload := dossier.ExtractDossierPayload(decoded)

	source := *jwksSource
	if source == "" {
		resolved := dossier.ResolveJwksURL(payload, "")
		if strings.HasPrefix(strings.ToLower(resolved), "https://") {
			source = resolved
		} else {
			fmt.Fprintln(os.Stderr, "decionis verify: this dossier carries no absolute public_jwks_url — pass -jwks <file|https URL>")
			return 2
		}
	}
	jwks, err := loadJwks(source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decionis verify: %v\ndecionis verify: unverified (fail closed).\n", err)
		return 1
	}

	verification := dossier.VerifyProofBundle(payload, jwks)
	reproducibility := dossier.AssessReproducibility(payload)

	if *asJSON {
		encoded, _ := json.MarshalIndent(map[string]any{
			"verification":    verification,
			"reproducibility": reproducibility,
		}, "", "  ")
		fmt.Println(string(encoded))
	} else {
		fmt.Println("Decision Dossier verification")
		if verification.KeyID != nil {
			fmt.Printf("  key: %s\n", *verification.KeyID)
		}
		for _, check := range verification.Checks {
			fmt.Printf("  [%s] %s — %s\n", check.Severity, check.Label, check.Detail)
		}
		if verification.Verified {
			fmt.Println("Verified: yes (Ed25519, against the public JWKS)")
		} else {
			fmt.Println("Verified: NO")
		}
		fmt.Printf("Reproducibility: %s — %s\n", reproducibility.Posture, reproducibility.Detail)
	}

	if !verification.Verified {
		return 1
	}
	return 0
}
