package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// check is a single doctor check result.
type check struct {
	name   string
	ok     bool
	detail string
}

// runDoctor performs a series of environment prerequisite checks and prints
// a summary to stdout. Returns a non-nil error if any required check fails.
func runDoctor() error {
	checks := []check{
		checkGoVersion(),
		checkEnv("DATABASE_URL", true),
		checkEnv("REDIS_URL", true),
		checkEnv("TEMPORAL_HOST", false),
		checkPostgres(),
		checkRedis(),
		checkTemporal(),
		checkMigrationDir(),
	}

	pass := 0
	fail := 0
	for _, c := range checks {
		icon := "✓"
		if !c.ok {
			icon = "✗"
			fail++
		} else {
			pass++
		}
		fmt.Printf("  %s  %-30s %s\n", icon, c.name, c.detail)
	}

	fmt.Printf("\n%d passed, %d failed\n", pass, fail)
	if fail > 0 {
		return fmt.Errorf("doctor: %d check(s) failed — fix the issues above before running the server", fail)
	}
	return nil
}

// checkGoVersion verifies Go ≥ 1.21.
func checkGoVersion() check {
	ver := runtime.Version() // e.g. "go1.22.3"
	parts := strings.TrimPrefix(ver, "go")
	segments := strings.SplitN(parts, ".", 3)
	if len(segments) < 2 {
		return check{"Go version", false, ver + " (cannot parse)"}
	}
	major, _ := strconv.Atoi(segments[0])
	minor, _ := strconv.Atoi(segments[1])
	if major < 1 || (major == 1 && minor < 21) {
		return check{"Go version", false, ver + " (need ≥ go1.21)"}
	}
	return check{"Go version", true, ver}
}

// checkEnv verifies an environment variable is set and non-empty.
// If required is false the check passes but notes when the var is absent.
func checkEnv(name string, required bool) check {
	val := os.Getenv(name)
	if val == "" {
		if required {
			return check{name, false, "not set (required)"}
		}
		return check{name, true, "not set (optional)"}
	}
	// Redact value — show only first 8 chars to confirm it is non-empty.
	preview := val
	if len(preview) > 8 {
		preview = preview[:8] + "…"
	}
	return check{name, true, "set (" + preview + ")"}
}

// checkPostgres attempts a TCP dial to the host extracted from DATABASE_URL.
func checkPostgres() check {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return check{"PostgreSQL reachable", false, "DATABASE_URL not set"}
	}
	host := extractHost(url, "5432")
	return dialCheck("PostgreSQL reachable", host, 3*time.Second)
}

// checkRedis attempts a TCP dial to the host extracted from REDIS_URL.
func checkRedis() check {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		return check{"Redis reachable", false, "REDIS_URL not set"}
	}
	host := extractHost(url, "6379")
	return dialCheck("Redis reachable", host, 3*time.Second)
}

// checkTemporal attempts a TCP dial to TEMPORAL_HOST (default port 7233).
func checkTemporal() check {
	host := os.Getenv("TEMPORAL_HOST")
	if host == "" {
		return check{"Temporal reachable", true, "TEMPORAL_HOST not set (optional)"}
	}
	if !strings.Contains(host, ":") {
		host = host + ":7233"
	}
	return dialCheck("Temporal reachable", host, 3*time.Second)
}

// checkMigrationDir verifies the migration directory exists.
func checkMigrationDir() check {
	dir := os.Getenv("AWO_MIGRATION_DIR")
	if dir == "" {
		dir = "db/migration"
	}
	if _, err := os.Stat(dir); err != nil {
		return check{"Migration dir", false, dir + " (not found)"}
	}
	// Count .up.sql files.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return check{"Migration dir", false, dir + ": " + err.Error()}
	}
	n := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			n++
		}
	}
	return check{"Migration dir", true, fmt.Sprintf("%s (%d migrations)", dir, n)}
}

// dialCheck attempts a TCP connection to addr within timeout.
func dialCheck(name, addr string, timeout time.Duration) check {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	d := &net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return check{name, false, addr + " — " + err.Error()}
	}
	conn.Close()
	return check{name, true, addr}
}

// extractHost parses a URL-like string and returns "host:port".
// Falls back to defaultPort when the URL contains no port.
// Handles postgres://, redis://, plain host:port, and bare hostnames.
func extractHost(raw, defaultPort string) string {
	// Strip scheme.
	if idx := strings.Index(raw, "://"); idx >= 0 {
		raw = raw[idx+3:]
	}
	// Strip userinfo (user:pass@).
	if idx := strings.Index(raw, "@"); idx >= 0 {
		raw = raw[idx+1:]
	}
	// Strip path/query.
	if idx := strings.IndexAny(raw, "/?#"); idx >= 0 {
		raw = raw[:idx]
	}
	// raw is now host or host:port.
	if strings.Contains(raw, ":") {
		return raw
	}
	return raw + ":" + defaultPort
}

// checkBinary verifies that a binary exists in PATH (used internally by
// extended doctor checks; exported for testing).
func checkBinary(name string) check {
	path, err := exec.LookPath(name)
	if err != nil {
		return check{name + " in PATH", false, "not found"}
	}
	return check{name + " in PATH", true, path}
}
