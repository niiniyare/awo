package version_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"awo.so/awo/version"
)

func TestGet_ContainsGoVersion(t *testing.T) {
	info := version.Get()
	assert.Contains(t, info.GoVersion, "go")
	assert.NotEmpty(t, info.OS)
	assert.NotEmpty(t, info.Arch)
}

func TestGet_DefaultVersion(t *testing.T) {
	info := version.Get()
	// In tests, Version is the package-level var default ("dev").
	assert.NotEmpty(t, info.Version)
}

func TestBanner_ContainsServiceName(t *testing.T) {
	b := version.Banner("awo-server")
	assert.True(t, strings.HasPrefix(b, "awo-server"))
}

func TestBanner_ContainsVersion(t *testing.T) {
	b := version.Banner("svc")
	assert.Contains(t, b, version.Get().Version)
}

func TestIsRelease_Dev(t *testing.T) {
	// Default is "dev" so IsRelease should be false unless overridden by ldflags.
	if version.Version == "dev" {
		assert.False(t, version.IsRelease())
	}
}

func TestParseBuildTime_Unknown(t *testing.T) {
	// When BuildTime == "unknown" (dev build), ParseBuildTime returns zero.
	if version.BuildTime == "unknown" {
		bt := version.ParseBuildTime()
		assert.True(t, bt.IsZero())
	}
}
