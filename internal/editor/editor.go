package editor

import (
	"fmt"
	"os"
	"os/exec"
)

// WriteTempFile writes content to a temporary file and returns its path and a cleanup function.
func WriteTempFile(content string) (string, func(), error) {
	f, err := os.CreateTemp("", "lazycwl-*.log")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp file: %w", err)
	}

	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, fmt.Errorf("writing temp file: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", nil, fmt.Errorf("closing temp file: %w", err)
	}

	cleanup := func() {
		os.Remove(f.Name())
	}

	return f.Name(), cleanup, nil
}

// EditorCommand returns the editor command from $EDITOR, falling back to vim.
func EditorCommand() string {
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	return "vim"
}

// Cmd returns an *exec.Cmd to open the given file in the user's editor.
func Cmd(filePath string) *exec.Cmd {
	return exec.Command(EditorCommand(), filePath)
}

// Open opens the given file in the user's editor.
func Open(filePath string) error {
	cmd := Cmd(filePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running editor: %w", err)
	}
	return nil
}
