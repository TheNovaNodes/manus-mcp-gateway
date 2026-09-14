package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env.test")
	content := `
# Comment
TEST_KEY_SIMPLE=simple_value
TEST_KEY_QUOTED="quoted value"
TEST_KEY_MULTILINE="line1
line2"
`
	if err := os.WriteFile(envPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test env: %v", err)
	}

	loadDotEnv(envPath)

	if os.Getenv("TEST_KEY_SIMPLE") != "simple_value" {
		t.Errorf("expected simple_value, got %s", os.Getenv("TEST_KEY_SIMPLE"))
	}
	if os.Getenv("TEST_KEY_QUOTED") != "quoted value" {
		t.Errorf("expected 'quoted value', got %s", os.Getenv("TEST_KEY_QUOTED"))
	}
	if os.Getenv("TEST_KEY_MULTILINE") != "line1\nline2" {
		t.Errorf("expected multiline value, got %q", os.Getenv("TEST_KEY_MULTILINE"))
	}
}
