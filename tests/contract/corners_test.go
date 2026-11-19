package contract_test

import (
	"encoding/xml"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"image"
	"image/color"
	"image/png"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

const cornersOwner = "internal/contexts/transfer/presentation/corners.go"

// Inline artwork can be assembled from SVG fragments instead of a complete document.
var inlineRoundedRect = regexp.MustCompile(`<rect\b[^<>]*\sr[xy]\s*=`)

// The toolkit switch affects every renderer and must be set before the first
// window. Reject other references too, including saving the setter in a variable.
func cornerSetup(path string, source []byte) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, source, 0)
	if err != nil {
		return nil, err
	}
	var findings []string
	names := map[string]bool{}
	for _, imp := range f.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		if p == "github.com/egoist/mygo/ui" {
			name := "ui"
			if imp.Name != nil {
				name = imp.Name.Name
			}
			if name == "." {
				findings = append(findings, path+": use a named UI import")
			}
			names[name] = true
		}
	}
	enabled := false
	ast.PreorderStack(f, nil, func(node ast.Node, parents []ast.Node) bool {
		if lit, ok := node.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			value, _ := strconv.Unquote(lit.Value)
			if inlineRoundedRect.MatchString(value) {
				findings = append(findings, fmt.Sprintf("%s: draw rounded SVG art with a continuous path", fset.Position(lit.Pos())))
			}
		}
		sel, ok := node.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "SetContinuousCorners" {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || !names[pkg.Name] {
			return true
		}
		// Accept exactly one direct call in the top-level body of init.
		if path == cornersOwner && !enabled && len(parents) == 5 {
			fn, isFunc := parents[1].(*ast.FuncDecl)
			stmt, isStmt := parents[3].(*ast.ExprStmt)
			call, isCall := parents[4].(*ast.CallExpr)
			if isFunc && fn.Recv == nil && fn.Name.Name == "init" && isStmt && isCall && stmt.X == call && call.Fun == sel && len(call.Args) == 1 {
				if arg, ok := call.Args[0].(*ast.Ident); ok && arg.Name == "true" {
					enabled = true
					return true
				}
			}
		}
		findings = append(findings, fmt.Sprintf("%s: configure continuous corners once in %s", fset.Position(sel.Pos()), cornersOwner))
		return true
	})
	if path == cornersOwner && !enabled {
		findings = append(findings, path+": enable continuous corners unconditionally in init")
	}
	return findings, nil
}

// SVG rectangle radii are elliptical arcs. Artwork with rounded corners uses
// paths; stroke caps and joins on line icons are independent of the silhouette.
func svgCornerFindings(path string, r io.Reader) ([]string, error) {
	d := xml.NewDecoder(r)
	var found []string
	for {
		t, err := d.Token()
		if err == io.EOF {
			return found, nil
		}
		if err != nil {
			return nil, err
		}
		start, ok := t.(xml.StartElement)
		if !ok || start.Name.Local != "rect" {
			continue
		}
		for _, a := range start.Attr {
			if a.Name.Local == "rx" || a.Name.Local == "ry" {
				line, _ := d.InputPos()
				found = append(found, fmt.Sprintf("%s:%d: use a continuous outline path for rounded artwork", path, line))
				break
			}
		}
	}
}

func TestInterfacePolicyContinuousCorners(t *testing.T) {
	root := filepath.Join("..", "..")
	sawSetup := false
	for _, dir := range []string{"cmd", "internal", "resources"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			var found []string
			switch filepath.Ext(path) {
			case ".go":
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				sawSetup = sawSetup || rel == cornersOwner
				found, err = cornerSetup(rel, data)
				if err != nil {
					return err
				}
			case ".svg":
				f, err := os.Open(path)
				if err != nil {
					return err
				}
				defer f.Close()
				found, err = svgCornerFindings(rel, f)
				if err != nil {
					return err
				}
			}
			for _, msg := range found {
				t.Error(msg)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if !sawSetup {
		t.Error("missing app-wide continuous corner setup")
	}
}

func TestInterfacePolicyCornerRegressions(t *testing.T) {
	const head = "package view\nimport kit \"github.com/egoist/mygo/ui\"\n"
	for _, tc := range []struct {
		path, body string
		bad        bool
	}{
		{cornersOwner, "func init(){kit.SetContinuousCorners(true)}", false},
		{cornersOwner, "func init(){if enabled {kit.SetContinuousCorners(true)}}", true},
		{cornersOwner, "func init(){kit.SetContinuousCorners(false)}", true},
		{"other.go", "var reset=kit.SetContinuousCorners", true},
		{"other.go", "func render(){kit.SetContinuousCorners(false)}", true},
		{"icons.go", "const badge = `<svg><rect width='20' rx='4'/></svg>`", true},
		{"icons.go", "const mask = `<rect width='20' height='20'/>`", false},
	} {
		got, err := cornerSetup(tc.path, []byte(head+tc.body))
		if err != nil || (len(got) > 0) != tc.bad {
			t.Errorf("%s: %v, %v", tc.body, got, err)
		}
	}
	for _, tc := range []struct {
		svg string
		bad bool
	}{
		{`<svg><rect width="12" rx="3"/></svg>`, true},
		{`<svg><rect height="12" ry="4"/></svg>`, true},
		{`<svg><rect width="12"/><path stroke-linecap="round" d="M1 2L3 4"/></svg>`, false},
	} {
		got, err := svgCornerFindings("mark.svg", strings.NewReader(tc.svg))
		if err != nil || (len(got) > 0) != tc.bad {
			t.Errorf("%s: %v, %v", tc.svg, got, err)
		}
	}
}

func TestInterfacePolicyLogoShoulders(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "..", "resources", "icon.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	icon, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join("..", "..", "internal", "contexts", "transfer", "presentation", "art", "skiff.svg"))
	if err != nil {
		t.Fatal(err)
	}
	logo, err := ui.ParseSVG(source)
	if err != nil {
		t.Fatal(err)
	}
	for name, im := range map[string]image.Image{"app icon": icon, "sidebar SVG": logo.Image(1024, 1024, color.Black)} {
		t.Run(name, func(t *testing.T) { checkLogoShoulders(t, im) })
	}
}

func checkLogoShoulders(t *testing.T, im image.Image) {
	t.Helper()
	if im.Bounds() != image.Rect(0, 0, 1024, 1024) {
		t.Fatal(im.Bounds())
	}
	// The 50%-alpha contour of Calculator on macOS 26.5.2, scaled from
	// its 824px body to Skiff's 928px body. Coordinates exclude the padding.
	const inset = 48
	profile := []image.Point{{X: 70, Y: 70}, {X: 96, Y: 48}, {X: 128, Y: 29}, {X: 160, Y: 16}, {X: 192, Y: 8}, {X: 224, Y: 4}, {X: 256, Y: 1}, {X: 464, Y: 0}}
	check := func(x, y int, inside bool) {
		t.Helper()
		for range 4 {
			_, _, _, a := im.At(x, y).RGBA()
			if (a >= 0x8000) != inside {
				t.Fatalf("corner diverges from the native contour at (%d,%d): alpha %d", x, y, a)
			}
			x, y = 1023-y, x // Rotate the probe to the next corner.
		}
	}
	for _, p := range profile {
		for _, x := range []int{inset + p.X, 1023 - inset - p.X} {
			// Tolerate a pixel of variation between SVG rasterizers.
			check(x, inset+p.Y-2, false)
			check(x, inset+p.Y+1, true)
		}
	}
}
