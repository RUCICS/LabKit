package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTempDirManagerCreateWorkspaceCreatesMissingRoot(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "missing", "workspace-root")
	manager := TempDirManager{Root: root}

	workspace, cleanup, err := manager.CreateWorkspace()
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}
	defer func() {
		if cleanup != nil {
			_ = cleanup()
		}
	}()

	if _, err := os.Stat(root); err != nil {
		t.Fatalf("workspace root stat error = %v, want existing root", err)
	}
	if _, err := os.Stat(workspace); err != nil {
		t.Fatalf("workspace stat error = %v, want existing workspace", err)
	}
}
