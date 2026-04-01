package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
)

type workFile struct {
	Use []struct {
		DiskPath string
	} `json:"Use"`
}

func TestWorkspaceModules(t *testing.T) {
	rootDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	work, err := loadWorkFile(rootDir)
	if err != nil {
		t.Fatalf("load go.work: %v", err)
	}

	moduleDirs := make([]string, 0, len(work.Use))
	for _, use := range work.Use {
		dir := filepath.Clean(use.DiskPath)
		if dir == "." {
			continue
		}
		moduleDirs = append(moduleDirs, dir)
	}
	sort.Strings(moduleDirs)

	for _, moduleDir := range moduleDirs {
		moduleDir := moduleDir
		t.Run(moduleDir, func(t *testing.T) {
			cmd := exec.Command("go", "test", "./...")
			cmd.Dir = filepath.Join(rootDir, moduleDir)
			cmd.Env = os.Environ()

			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go test ./... in %s failed: %v\n%s", moduleDir, err, output)
			}
		})
	}
}

func loadWorkFile(rootDir string) (workFile, error) {
	cmd := exec.Command("go", "work", "edit", "-json")
	cmd.Dir = rootDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return workFile{}, fmt.Errorf("go work edit -json: %w\n%s", err, output)
	}

	var work workFile
	if err := json.Unmarshal(output, &work); err != nil {
		return workFile{}, fmt.Errorf("decode go.work json: %w", err)
	}
	return work, nil
}
