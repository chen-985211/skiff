package contract_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

func TestI18nPolicyRecognizesHardcodedCopy(t *testing.T) {
	const fixture = `package sample
import toolkit "github.com/egoist/mygo/ui"
import desktop "github.com/egoist/mygo"
func render(c *toolkit.Context) {
	// 中文注释
	toolkit.Text(c, "Pause download")
	toolkit.Text(c, tr(c, "task.pause"))
	toolkit.Textf(c, "%d", 3)
	toolkit.Text(c, "HTTPS")
	toolkit.ButtonBase(c.Key("stable-id")).Label("Clear search").Tooltip(tr(c, "search.clear"))
	_ = "\u6682\u505c"
	_ = desktop.MenuItem{Label: "New Download"}
	_ = desktop.MenuItem{Label: tr(c, "download.new")}
}
`
	issues, err := checkInterfaceCopy("fixture.go", []byte(fixture), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"fixture.go:6:", "fixture.go:10:", "fixture.go:11:", "fixture.go:12:"}
	if len(issues) != len(want) {
		t.Fatalf("issues = %v, want %v", issues, want)
	}
	for i, issue := range issues {
		if !strings.HasPrefix(issue, want[i]) {
			t.Errorf("issue %d = %s, want location %s", i, issue, want[i])
		}
	}
}

func checkInterfaceCopy(path string, source []byte, messages map[string]map[string]string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, 0)
	if err != nil {
		return nil, err
	}
	imports := map[string]string{}
	for _, spec := range file.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		name := filepath.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		imports[name] = path
	}
	visible := map[*ast.BasicLit]bool{}
	var mark func(ast.Expr)
	mark = func(expr ast.Expr) {
		switch node := expr.(type) {
		case *ast.BasicLit:
			visible[node] = true
		case *ast.BinaryExpr:
			if node.Op == token.ADD {
				mark(node.X)
				mark(node.Y)
			}
		case *ast.ParenExpr:
			mark(node.X)
		case *ast.CallExpr:
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok && packageOf(sel, imports) == "fmt" && sel.Sel.Name == "Sprintf" && len(node.Args) > 0 {
				mark(node.Args[0])
			}
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			sel, ok := node.Fun.(*ast.SelectorExpr)
			if !ok {
				break
			}
			if packageOf(sel, imports) == "github.com/egoist/mygo/ui" {
				switch sel.Sel.Name {
				case "Text", "Textf", "Button", "Label", "Checkbox", "Switch":
					if len(node.Args) > 1 {
						mark(node.Args[1])
					}
				case "AlertDialog":
					for _, arg := range node.Args[min(2, len(node.Args)):] {
						mark(arg)
					}
				}
			} else {
				switch sel.Sel.Name {
				case "Label", "Tooltip", "Placeholder", "Description", "Item":
					if len(node.Args) > 0 {
						mark(node.Args[0])
					}
				}
			}
		case *ast.CompositeLit:
			sel, ok := node.Type.(*ast.SelectorExpr)
			if !ok || packageOf(sel, imports) != "github.com/egoist/mygo" {
				break
			}
			for _, element := range node.Elts {
				if field, ok := element.(*ast.KeyValueExpr); ok {
					if key, ok := field.Key.(*ast.Ident); ok {
						switch key.Name {
						case "Title", "Message", "Label", "ButtonLabel":
							mark(field.Value)
						case "Name":
							if sel.Sel.Name == "FileFilter" {
								mark(field.Value)
							}
						}
					}
				}
			}
		}
		return true
	})
	var issues []string
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && messages != nil {
			if message := checkMessageCall(call, imports, messages); message != "" {
				issues = append(issues, fmt.Sprintf("%s: %s", fset.Position(call.Pos()), message))
			}
		}
		literal, ok := n.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		value, _ := strconv.Unquote(literal.Value)
		if strings.ContainsFunc(value, func(r rune) bool {
			return unicode.Is(unicode.Han, r) || strings.ContainsRune("。，；：！？（）", r)
		}) || visible[literal] && isProse(value) {
			issues = append(issues, fmt.Sprintf("%s: move user-facing text to the i18n catalogs", fset.Position(literal.Pos())))
		}
		return true
	})
	return issues, nil
}

func packageOf(sel *ast.SelectorExpr, imports map[string]string) string {
	if ident, ok := sel.X.(*ast.Ident); ok && ident.Obj == nil {
		return imports[ident.Name]
	}
	return ""
}

var formatVerb = regexp.MustCompile(`%(?:\[[0-9]+\])?[-+#0 .0-9]*[%vTtbcdoOqxXUeEfFgGspw]`)

func isProse(value string) bool {
	switch value {
	case "HTTP", "HTTPS", "FTP", "DNS":
		return false
	}
	return strings.ContainsFunc(formatVerb.ReplaceAllString(value, ""), unicode.IsLetter)
}

func TestI18nPresentationCopy(t *testing.T) {
	root := i18nRepositoryRoot()
	messages := readMessages(t, filepath.Join(root, "internal", "contexts", "transfer", "presentation", "i18n", "catalogs", "zh-CN.json"))
	for _, dir := range []string{"internal/contexts", "internal/platform/desktop"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rpath, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			relative := filepath.ToSlash(rpath)
			if dir == "internal/contexts" && !strings.Contains(relative, "/presentation/") {
				return nil
			}
			issues, err := checkInterfaceCopy(relative, data, messages)
			for _, issue := range issues {
				t.Error(issue)
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestI18nPolicyChecksStaticMessageReferences(t *testing.T) {
	const fixture = `package sample
import locale "skiff/internal/contexts/transfer/presentation/i18n"
func render(c any, key string) {
	_ = tr(c, "missing.text")
	_ = trf(c, "known.format", locale.Args{"wrong": 3})
	_ = tr(c, "known.format")
	_ = trf(c, "known.format", locale.Args{"count": 3})
	_ = tr(c, key)
}
`
	issues, err := checkInterfaceCopy("fixture.go", []byte(fixture), map[string]map[string]string{
		"known.format": {"count": "plural"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 3 {
		t.Fatalf("got %d violations, want 3: %v", len(issues), issues)
	}
}

func checkMessageCall(call *ast.CallExpr, imports map[string]string, messages map[string]map[string]string) string {
	keyIndex, argsIndex := 1, -1
	switch function := call.Fun.(type) {
	case *ast.Ident:
		if function.Obj != nil {
			return ""
		}
		switch function.Name {
		case "tr":
		case "trf":
			argsIndex = 2
		case "copyError":
			keyIndex, argsIndex = 0, 1
		default:
			return ""
		}
	case *ast.SelectorExpr:
		if packageOf(function, imports) != "skiff/internal/contexts/transfer/presentation/i18n" {
			return ""
		}
		switch function.Sel.Name {
		case "Text":
		case "Format":
			argsIndex = 2
		default:
			return ""
		}
	default:
		return ""
	}
	if len(call.Args) <= keyIndex {
		return ""
	}
	key, static := stringLiteral(call.Args[keyIndex])
	if !static {
		return ""
	}
	required, found := messages[key]
	if !found {
		return fmt.Sprintf("unknown i18n message key %q", key)
	}
	provided := map[string]bool{}
	if argsIndex >= 0 && len(call.Args) > argsIndex {
		switch args := call.Args[argsIndex].(type) {
		case *ast.CompositeLit:
			for _, expr := range args.Elts {
				pair, ok := expr.(*ast.KeyValueExpr)
				if !ok {
					return ""
				}
				name, ok := stringLiteral(pair.Key)
				if !ok {
					return ""
				}
				provided[name] = true
			}
		case *ast.Ident:
			if args.Name != "nil" {
				return ""
			}
		default:
			return ""
		}
	}
	for name := range required {
		if !provided[name] {
			return fmt.Sprintf("i18n message %q is missing parameter %q", key, name)
		}
	}
	for name := range provided {
		if _, ok := required[name]; !ok {
			return fmt.Sprintf("i18n message %q has unknown parameter %q", key, name)
		}
	}
	return ""
}

func stringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	return value, err == nil
}

func hasChineseCopy(value string) bool {
	return strings.ContainsFunc(value, func(r rune) bool {
		return unicode.Is(unicode.Han, r) || strings.ContainsRune("。，；：！？（）", r)
	})
}
