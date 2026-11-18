package contract_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestCleanArchitectureDependencyDirection(t *testing.T) {
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "../.."))
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relative, _ := filepath.Rel(root, path)
		relative = filepath.ToSlash(relative)
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			dependency, _ := strconv.Unquote(spec.Path.Value)
			forbidden := false
			if strings.Contains(relative, "/contexts/") && strings.Contains(dependency, "/platform/") {
				forbidden = true
			}
			if strings.Contains(relative, "/domain/") {
				forbidden = forbidden || strings.Contains(dependency, "/application") || strings.Contains(dependency, "/infrastructure") || strings.Contains(dependency, "/presentation") || strings.Contains(dependency, "mygo") || dependency == "os" || dependency == "net/http" || dependency == "os/exec"
			}
			if strings.Contains(relative, "/application/") {
				forbidden = forbidden || strings.Contains(dependency, "/infrastructure") || strings.Contains(dependency, "/presentation") || strings.Contains(dependency, "mygo") || dependency == "os" || dependency == "net/http" || dependency == "os/exec"
			}
			if strings.Contains(relative, "/presentation/") {
				forbidden = forbidden || strings.Contains(dependency, "/infrastructure") || strings.Contains(dependency, "/domain") || dependency == "os" || dependency == "net/http" || dependency == "os/exec"
			}
			if strings.Contains(relative, "/infrastructure/") && strings.Contains(dependency, "/presentation") {
				forbidden = true
			}
			if forbidden {
				t.Errorf("%s imports forbidden dependency %s", relative, dependency)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
