//go:build integration

package transfer_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/infrastructure/aria2"
)

func TestDownloadDirectoryChecksTheSpecificMacOSACLRight(t *testing.T) {
	client := aria2.NewClient("", "")
	for _, forbidden := range []string{"add_file", "add_subdirectory"} {
		t.Run(forbidden, func(t *testing.T) {
			root := t.TempDir()
			if output, err := exec.CommandContext(t.Context(), "/bin/chmod", "+a", "everyone deny "+forbidden, root).CombinedOutput(); err != nil {
				t.Fatalf("could not set fixture ACL: %v (%s)", err, output)
			}
			target := filepath.Join(root, "new", "child")
			allowed, denied := root, target
			if forbidden == "add_file" {
				allowed, denied = target, root
			}
			if _, err := client.ProtectCheckpoints(t.Context(), allowed, nil); err != nil {
				t.Errorf("ACL blocked an operation it allows: %v", err)
			}
			if _, err := client.ProtectCheckpoints(t.Context(), denied, nil); application.CodeOf(err) != application.ErrDirectoryUnavailable || !errors.Is(err, os.ErrPermission) {
				t.Errorf("ACL restriction was ignored or lost its cause: %v", err)
			}
			if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("directory preflight changed the filesystem: %v", err)
			}
		})
	}
}
