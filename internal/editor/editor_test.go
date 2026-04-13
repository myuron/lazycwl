package editor

import (
	"os"
	"strings"
	"testing"
)

func TestWriteTempFile(t *testing.T) {
	content := "[2024-01-15T09:30:00.000Z] test log message\n"

	path, cleanup, err := WriteTempFile(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	if !strings.HasPrefix(path, os.TempDir()) {
		t.Errorf("expected temp file in %s, got %s", os.TempDir(), path)
	}
	if !strings.Contains(path, "lazycwl-") {
		t.Errorf("expected file name to contain lazycwl-, got %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}
	if string(data) != content {
		t.Errorf("expected content %q, got %q", content, string(data))
	}
}

func TestWriteTempFile_Cleanup(t *testing.T) {
	content := "test\n"

	path, cleanup, err := WriteTempFile(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cleanup()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected temp file to be deleted after cleanup")
	}
}

func TestEditorCommand(t *testing.T) {
	// Test with $EDITOR set
	t.Setenv("EDITOR", "nvim")
	cmd := EditorCommand()
	if cmd != "nvim" {
		t.Errorf("expected nvim, got %s", cmd)
	}

	// Test fallback to vim
	t.Setenv("EDITOR", "")
	cmd = EditorCommand()
	if cmd != "vim" {
		t.Errorf("expected vim fallback, got %s", cmd)
	}
}
