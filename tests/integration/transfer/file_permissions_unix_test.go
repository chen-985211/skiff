//go:build integration && unix

package transfer_test

import (
	"os"
	"testing"
)

func assertPrivateTaskFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Errorf("task file exposes private URLs: %v", info.Mode())
	}
}
