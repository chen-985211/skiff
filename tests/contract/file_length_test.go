package contract_test

import (
	"bytes"
	"io/fs"
	"os"
	"strings"
	"testing"
)

func TestGoFileLineLimit(t *testing.T) {
	const limit = 800
	root := os.DirFS("../..")
	err := fs.WalkDir(root, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch path {
			case ".git", ".mygo", "build", "release", "test-results", "vendor":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		source, err := fs.ReadFile(root, path)
		if err != nil {
			return err
		}
		lines := 0
		for range bytes.Lines(source) {
			lines++
		}
		if lines > limit {
			t.Errorf("%s:%d: %d lines (limit %d); split by responsibility", path, limit+1, lines, limit)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
