package handlers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveWorkspacePathRejectsMalformedWorkspaceIDs(t *testing.T) {
	root := t.TempDir()

	validWorkspace := filepath.Join(root, "workspace-123")
	if err := os.Mkdir(validWorkspace, 0o755); err != nil {
		t.Fatalf("create valid workspace: %v", err)
	}

	for _, workspaceID := range []string{
		".",
		"..",
		"nested/child",
		"nested\\child",
		"./child",
		"child/..",
		"..\\child",
		"child\\..",
	} {
		t.Run(workspaceID, func(t *testing.T) {
			if _, err := resolveWorkspacePath(root, workspaceID); err == nil {
				t.Fatalf("expected malformed workspace ID %q to be rejected", workspaceID)
			}
		})
	}
}
