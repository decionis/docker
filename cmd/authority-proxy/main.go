// authority-proxy is the Decionis action-aware execution gate
// (image decionis/authority).
//
// An SDK or middleware POSTs a structured description of an action it is
// about to take; the gate asks the Decionis control plane for a verdict and
// answers whether the action is authorized. It performs no traffic
// interception and evaluates no policy itself.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/decionis/docker/internal/api"
	"github.com/decionis/docker/internal/authority"
	"github.com/decionis/docker/internal/version"
)

func main() {
	listenAddr := flag.String("listen", ":8080", "address to serve the gate on")
	baseURL := flag.String("base-url", "", "Decionis API base URL (default https://api.decionis.com)")
	timeout := flag.Duration("timeout", 15*time.Second, "how long to wait for a verdict before denying")
	decisionType := flag.String("decision-type", authority.DefaultToolCallDecisionType,
		"decision type used when evaluating intercepted MCP tool calls")
	forwardArguments := flag.Bool("forward-arguments", false,
		"send intercepted tool-call argument VALUES to the control plane (names are always sent)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Credentials come from the environment, never flags: a flag value is
	// visible in the process list to anything sharing the host.
	orgID := strings.TrimSpace(os.Getenv("DECIONIS_ORG_ID"))
	apiKey := strings.TrimSpace(os.Getenv("DECIONIS_API_KEY"))
	if orgID == "" || apiKey == "" {
		logger.Error("DECIONIS_ORG_ID and DECIONIS_API_KEY are required")
		os.Exit(1)
	}

	resolvedBaseURL := *baseURL
	if strings.TrimSpace(resolvedBaseURL) == "" {
		resolvedBaseURL = os.Getenv("DECIONIS_BASE_URL")
	}
	client, err := api.NewClient(api.Config{BaseURL: resolvedBaseURL, OrgID: orgID, APIKey: apiKey})
	if err != nil {
		logger.Error("invalid configuration", "detail", err.Error())
		os.Exit(1)
	}

	server := &http.Server{
		Addr: *listenAddr,
		Handler: authority.NewServer(authority.NewGate(client, *timeout), logger, version.Version).
			WithGatewayOptions(*decisionType, *forwardArguments).
			Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	logger.Info("decionis authority gate listening", "on", *listenAddr, "version", version.Version)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped", "detail", err.Error())
		os.Exit(1)
	}
}
