//go:build darwin

package contract_test

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// The developer entry point must keep live reload from selecting a revoked
// identity that macOS may still list in its local keychains.
func TestDevTargetSelectsLocalSigning(t *testing.T) {
	cmd := exec.Command("make", "-s", "-n", "dev")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cannot resolve the dev target: %v\n%s", err, out)
	}
	if !slices.Equal(strings.Fields(string(out)), []string{"go", "tool", "mygo", "dev", "-sign", "-"}) {
		t.Fatalf("dev target must select local ad hoc signing: %s", out)
	}
}
