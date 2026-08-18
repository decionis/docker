// Package version carries the build version stamped into the Go binaries
// (daemon, CLI, authority proxy, Dev Container helper).
package version

// Version is overridden at build time via
// -ldflags "-X github.com/decionis/docker/internal/version.Version=vX.Y.Z".
var Version = "0.0.0-dev"
