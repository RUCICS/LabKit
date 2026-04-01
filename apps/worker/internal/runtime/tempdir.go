package runtime

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const defaultWorkspacePrefix = "labkit-worker-"

type TempDirManager struct {
	Root   string
	Prefix string
}

func (m TempDirManager) CreateWorkspace() (string, func() error, error) {
	prefix := m.Prefix
	if prefix == "" {
		prefix = defaultWorkspacePrefix
	}

	if m.Root != "" {
		if err := os.MkdirAll(m.Root, 0o755); err != nil {
			return "", nil, fmt.Errorf("runtime: create workspace root: %w", err)
		}
	}

	dir, err := os.MkdirTemp(m.Root, prefix)
	if err != nil {
		return "", nil, fmt.Errorf("runtime: create temp workspace: %w", err)
	}

	cleanup := func() error {
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("runtime: remove temp workspace: %w", err)
		}
		return nil
	}

	return dir, cleanup, nil
}

func EnsureSubmissionDir(workspace string) (string, error) {
	dir := filepath.Join(workspace, "submission")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("runtime: create submission directory: %w", err)
	}
	return dir, nil
}

func (m TempDirManager) StageSubmissionDir(sourceDir string, declaredFiles []string) (string, func() error, error) {
	workspace, cleanup, err := m.CreateWorkspace()
	if err != nil {
		return "", nil, err
	}

	stagedDir, err := EnsureSubmissionDir(workspace)
	if err != nil {
		_ = cleanup()
		return "", nil, err
	}

	files, err := listSubmissionFiles(sourceDir, declaredFiles)
	if err != nil {
		_ = cleanup()
		return "", nil, err
	}
	for _, file := range files {
		target := filepath.Join(stagedDir, file.Name())
		if err := copyFile(file.Path(), target); err != nil {
			_ = cleanup()
			return "", nil, err
		}
	}

	return stagedDir, cleanup, nil
}

type submissionFile struct {
	name string
	path string
}

func (f submissionFile) Name() string {
	return f.name
}

func (f submissionFile) Path() string {
	return f.path
}

func listSubmissionFiles(dir string, declaredFiles []string) ([]submissionFile, error) {
	if len(declaredFiles) == 0 {
		return nil, fmt.Errorf("runtime: manifest submit.files is required")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("runtime: read submission directory: %w", err)
	}

	entryByName := make(map[string]os.DirEntry, len(entries))
	for _, entry := range entries {
		entryByName[entry.Name()] = entry
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("runtime: submission file %q must not be a symlink", entry.Name())
		}
		if !contains(declaredFiles, entry.Name()) {
			return nil, fmt.Errorf("runtime: unexpected submission file %q", entry.Name())
		}
	}

	files := make([]submissionFile, 0, len(declaredFiles))
	for _, declared := range declaredFiles {
		entry, ok := entryByName[declared]
		if !ok {
			return nil, fmt.Errorf("runtime: missing submission file %q", declared)
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("runtime: stat submission file %q: %w", declared, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("runtime: submission file %q must be a regular file", declared)
		}
		files = append(files, submissionFile{
			name: declared,
			path: filepath.Join(dir, declared),
		})
	}
	return files, nil
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("runtime: open source file: %w", err)
	}
	defer in.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("runtime: create staged file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("runtime: copy staged file: %w", err)
	}
	return nil
}
