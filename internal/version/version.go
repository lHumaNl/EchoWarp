// Package version provides build-time version information for EchoWarp.
// These values are set via ldflags during build:
//
//	go build -ldflags "-X main.Version=1.0.0 -X main.Commit=$(git rev-parse HEAD) -X main.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
package version

// Version is the semantic version. Set via ldflags during builds.
// Default "dev" is used when building with plain `go install` without ldflags.
var Version = "dev"

// Commit is the git commit hash. Set via ldflags during builds.
var Commit = "unknown"

// BuildDate is the ISO 8601 build timestamp. Set via ldflags during builds.
var BuildDate = "unknown"
