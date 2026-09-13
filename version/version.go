// Package version provides build metadata for the Awo framework.
//
// Values are injected at build time via ldflags:
//
//	go build -ldflags "
//	  -X awo.so/awo/version.Version=1.0.0
//	  -X awo.so/awo/version.GitCommit=$(git rev-parse --short HEAD)
//	  -X awo.so/awo/version.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)
//	" ./cmd/server
//
// In development (no ldflags), the values fall back to "dev", "unknown",
// and "unknown" respectively.
//
// # Usage in startup banner
//
//	fmt.Println(version.Banner())
//
// # Usage in health check endpoint
//
//	c.JSON(version.Info())
package version

import (
	"fmt"
	"runtime"
	"time"
)

// Build-time variables. Injected via -ldflags. Defaults signal a dev build.
var (
	// Version is the semantic version string (e.g. "1.0.0").
	Version = "dev"

	// GitCommit is the short git commit SHA at build time.
	GitCommit = "unknown"

	// BuildTime is the RFC 3339 timestamp of the build.
	BuildTime = "unknown"
)

// Info holds the structured build metadata. Returned as JSON by the build
// information endpoint (GET /api/v1/build).
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// Get returns the build Info for this process.
func Get() Info {
	return Info{
		Version:   Version,
		GitCommit: GitCommit,
		BuildTime: BuildTime,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

// Banner returns a multi-line startup banner suitable for log output.
func Banner(serviceName string) string {
	i := Get()
	return fmt.Sprintf(
		"%s v%s (commit: %s, built: %s, %s/%s, %s)",
		serviceName, i.Version, i.GitCommit, i.BuildTime,
		i.OS, i.Arch, i.GoVersion,
	)
}

// IsRelease reports whether this is a release build (Version is not "dev").
func IsRelease() bool { return Version != "dev" }

// ParseBuildTime parses BuildTime as time.Time.
// Returns the zero time if BuildTime is "unknown" or unparseable.
func ParseBuildTime() time.Time {
	if BuildTime == "unknown" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, BuildTime)
	if err != nil {
		return time.Time{}
	}
	return t
}
