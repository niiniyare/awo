package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeIdent(t *testing.T) {
	cases := []struct{ in, want string }{
		{"finance", "finance"},
		{"my-module", "my_module"},
		{"MyModule", "mymodule"},
		{"foo bar", "foo_bar"},
		{"123abc", "123abc"},
	}
	for _, c := range cases {
		got := sanitizeIdent(c.in)
		if got != c.want {
			t.Errorf("sanitizeIdent(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestToPascal(t *testing.T) {
	cases := []struct{ in, want string }{
		{"invoice", "Invoice"},
		{"invoice_line", "InvoiceLine"},
		{"my-entity", "MyEntity"},
		{"foo bar baz", "FooBarBaz"},
		{"already", "Already"},
	}
	for _, c := range cases {
		got := toPascal(c.in)
		if got != c.want {
			t.Errorf("toPascal(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestScaffoldModule_FileCount(t *testing.T) {
	files := scaffoldModule("crm")
	if len(files) != 5 {
		t.Errorf("scaffoldModule returned %d files; want 5", len(files))
	}
	for path, content := range files {
		if content == "" {
			t.Errorf("file %s has empty content", path)
		}
	}
}

func TestScaffoldModule_ContainsPackage(t *testing.T) {
	files := scaffoldModule("finance")
	for path, content := range files {
		if !strings.Contains(content, "package finance") {
			t.Errorf("%s does not contain 'package finance'", path)
		}
	}
}

func TestScaffoldEntity_SingleFile(t *testing.T) {
	files := scaffoldEntity("crm", "contact")
	if len(files) != 1 {
		t.Errorf("scaffoldEntity returned %d files; want 1", len(files))
	}
	for path, content := range files {
		if !strings.Contains(path, "entity_contact.go") {
			t.Errorf("unexpected path %s", path)
		}
		if !strings.Contains(content, "crm_contact") {
			t.Errorf("entity def does not reference crm_contact in %s", path)
		}
	}
}

func TestScaffoldWorkflow_TwoFiles(t *testing.T) {
	files := scaffoldWorkflow("approval")
	if len(files) != 2 {
		t.Errorf("scaffoldWorkflow returned %d files; want 2", len(files))
	}
	hasWorkflow, hasTest := false, false
	for path := range files {
		if strings.HasSuffix(path, "_workflow.go") && !strings.HasSuffix(path, "_test.go") {
			hasWorkflow = true
		}
		if strings.HasSuffix(path, "_workflow_test.go") {
			hasTest = true
		}
	}
	if !hasWorkflow {
		t.Error("missing workflow stub file")
	}
	if !hasTest {
		t.Error("missing workflow test file")
	}
}

func TestWriteScaffold_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "file.go")
	if err := writeScaffold(path, "package main\n"); err != nil {
		t.Fatalf("writeScaffold: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "package main\n" {
		t.Errorf("content mismatch: %q", data)
	}
}

func TestWriteScaffold_RejectsExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exists.go")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := writeScaffold(path, "new")
	if err == nil {
		t.Fatal("expected error for existing file, got nil")
	}
}
