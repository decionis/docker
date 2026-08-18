// decionis-daemon is the Docker Desktop extension backend: it serves the
// extension UI over the extension socket (or a loopback TCP port for
// development), holds the org connection, and verifies Decision Dossiers.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/decionis/docker/internal/daemon"
	"github.com/decionis/docker/internal/store"
	"github.com/decionis/docker/internal/version"
)

func main() {
	socketPath := flag.String("socket", "/run/guest-services/backend.sock", "unix socket to serve the extension UI on")
	listenAddr := flag.String("listen", "", "development override: loopback TCP address (e.g. 127.0.0.1:8787)")
	dataDir := flag.String("data", "/data", "private directory for connection state")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	d := daemon.New(version.Version, store.New(*dataDir), logger, daemon.DefaultClientFactory)
	d.LoadStoredConnection()

	listener, description, err := listen(*socketPath, *listenAddr)
	if err != nil {
		logger.Error("listen failed", "detail", err.Error())
		os.Exit(1)
	}
	logger.Info("decionis-daemon listening", "on", description, "version", version.Version)

	server := &http.Server{
		Handler:           d.Handler(),
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

	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped", "detail", err.Error())
		os.Exit(1)
	}
}

// listen binds either the extension unix socket or, for development, a
// loopback-only TCP address. Non-loopback binds are refused
// (rules/security.rules.md Rule 3.1).
func listen(socketPath, listenAddr string) (net.Listener, string, error) {
	if listenAddr != "" {
		host, _, err := net.SplitHostPort(listenAddr)
		if err != nil {
			return nil, "", err
		}
		ip := net.ParseIP(host)
		isLoopback := host == "localhost" || (ip != nil && ip.IsLoopback())
		if !isLoopback {
			return nil, "", errors.New("-listen must bind a loopback address")
		}
		listener, err := net.Listen("tcp", listenAddr)
		if err != nil {
			return nil, "", err
		}
		return listener, "tcp " + listenAddr, nil
	}
	_ = os.Remove(socketPath)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, "", err
	}
	return listener, "unix " + socketPath, nil
}
