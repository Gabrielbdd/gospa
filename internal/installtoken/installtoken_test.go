package installtoken_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Gabrielbdd/gospa/internal/installtoken"
)

func TestLoad_LiteralEnvWins(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "tok")
	if err := os.WriteFile(file, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv(installtoken.EnvLiteral, "from-env")
	t.Setenv(installtoken.EnvFile, file)

	got, src, err := installtoken.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "from-env" {
		t.Errorf("token = %q; want from-env", got)
	}
	if src != installtoken.SourceLiteralEnv {
		t.Errorf("source = %v; want SourceLiteralEnv", src)
	}
}

func TestLoad_FileWhenNoLiteral(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "tok")
	if err := os.WriteFile(file, []byte("  from-file  \n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv(installtoken.EnvLiteral, "")
	t.Setenv(installtoken.EnvFile, file)

	got, src, err := installtoken.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "from-file" {
		t.Errorf("token = %q; want from-file (whitespace trimmed)", got)
	}
	if src != installtoken.SourceFile {
		t.Errorf("source = %v; want SourceFile", src)
	}
}

func TestLoad_MissingFileReturnsActionableError(t *testing.T) {
	t.Setenv(installtoken.EnvLiteral, "")
	t.Setenv(installtoken.EnvFile, "/nonexistent/install-token")

	_, _, err := installtoken.Load()
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	msg := err.Error()
	if !strings.Contains(msg, "reading install token") || !strings.Contains(msg, installtoken.EnvLiteral) {
		t.Errorf("error message lacks actionable hint: %v", err)
	}
}

func TestLoad_EmptyFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "tok")
	if err := os.WriteFile(file, []byte("\n   \n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv(installtoken.EnvLiteral, "")
	t.Setenv(installtoken.EnvFile, file)

	_, _, err := installtoken.Load()
	if err == nil {
		t.Fatal("expected error for empty file")
	}
}

func TestLoad_GeneratesWhenUnconfigured(t *testing.T) {
	t.Setenv(installtoken.EnvLiteral, "")
	t.Setenv(installtoken.EnvFile, "")

	got, src, err := installtoken.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src != installtoken.SourceGenerated {
		t.Errorf("source = %v; want SourceGenerated", src)
	}
	if len(got) != 32 {
		t.Errorf("token length = %d; want 32 hex chars", len(got))
	}
}

func TestEqual(t *testing.T) {
	tests := []struct {
		name              string
		provided, expected string
		want              bool
	}{
		{"matching", "abc", "abc", true},
		{"mismatching", "abc", "def", false},
		{"empty provided", "", "abc", false},
		{"empty expected", "abc", "", false},
		{"both empty", "", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := installtoken.Equal(tc.provided, tc.expected); got != tc.want {
				t.Errorf("Equal(%q, %q) = %v; want %v", tc.provided, tc.expected, got, tc.want)
			}
		})
	}
}
