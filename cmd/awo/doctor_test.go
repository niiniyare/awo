package main

import (
	"net"
	"testing"
	"time"
)

func TestExtractHost(t *testing.T) {
	cases := []struct {
		raw, defaultPort, want string
	}{
		{"postgres://user:pass@localhost:5432/mydb", "5432", "localhost:5432"},
		{"postgres://localhost/mydb", "5432", "localhost:5432"},
		{"redis://localhost:6379", "6379", "localhost:6379"},
		{"redis://localhost", "6379", "localhost:6379"},
		{"localhost:9999", "5432", "localhost:9999"},
		{"localhost", "5432", "localhost:5432"},
		{"postgres://user:pass@db.example.com/mydb?sslmode=disable", "5432", "db.example.com:5432"},
	}
	for _, c := range cases {
		got := extractHost(c.raw, c.defaultPort)
		if got != c.want {
			t.Errorf("extractHost(%q, %q) = %q; want %q", c.raw, c.defaultPort, got, c.want)
		}
	}
}

func TestDialCheck_LocalListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	addr := ln.Addr().String()
	c := dialCheck("test", addr, 2*time.Second)
	if !c.ok {
		t.Errorf("expected ok=true for listening addr; detail=%s", c.detail)
	}
}

func TestDialCheck_Unreachable(t *testing.T) {
	// Port 1 is almost certainly not listening.
	c := dialCheck("test", "127.0.0.1:1", 2*time.Second)
	if c.ok {
		t.Error("expected ok=false for unreachable port")
	}
}

func TestCheckGoVersion_Valid(t *testing.T) {
	c := checkGoVersion()
	// We are running this test with Go, so it must pass.
	if !c.ok {
		t.Errorf("checkGoVersion failed on the Go version running the test: %s", c.detail)
	}
}

func TestCheckEnv_Set(t *testing.T) {
	t.Setenv("__AWO_TEST_VAR__", "testvalue")
	c := checkEnv("__AWO_TEST_VAR__", true)
	if !c.ok {
		t.Errorf("expected ok=true for set env var, got: %s", c.detail)
	}
}

func TestCheckEnv_Missing_Required(t *testing.T) {
	// Ensure var is absent by unsetting it (t.Setenv to empty is equivalent to unset for our check).
	t.Setenv("__AWO_TEST_MISSING__", "")
	c := checkEnv("__AWO_TEST_MISSING__", true)
	if c.ok {
		t.Error("expected ok=false for missing required var")
	}
}

func TestCheckEnv_Missing_Optional(t *testing.T) {
	t.Setenv("__AWO_TEST_OPT__", "")
	c := checkEnv("__AWO_TEST_OPT__", false)
	if !c.ok {
		t.Error("expected ok=true for missing optional var")
	}
}

func TestCheckMigrationDir_NotExist(t *testing.T) {
	t.Setenv("AWO_MIGRATION_DIR", "/nonexistent/path/xyz_doctor_test")
	c := checkMigrationDir()
	if c.ok {
		t.Error("expected ok=false for non-existent migration dir")
	}
}

func TestCheckMigrationDir_Exists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AWO_MIGRATION_DIR", dir)
	c := checkMigrationDir()
	if !c.ok {
		t.Errorf("expected ok=true for existing dir: %s", c.detail)
	}
}
