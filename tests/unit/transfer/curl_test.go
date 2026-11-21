package transfer_test

import (
	"github.com/egoist/mygo/ui"
	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/presentation"
	"strings"
	"testing"
)

func TestBrowserCurlImport(t *testing.T) {
	command := "curl 'https://example.test/file?x=1&y=2' \\\n -H 'Authorization: Bearer token' -H 'accept-encoding: gzip, br' --cookie 'a=b' -A 'test browser' --compressed"
	input, ok, err := application.ImportCurl(command)
	if err != nil || !ok || input.URL != "https://example.test/file?x=1&y=2" || !strings.Contains(input.Headers, "Cookie: a=b") || !strings.Contains(input.Headers, "Authorization: Bearer token") || strings.Contains(input.Headers, "gzip") {
		t.Fatal(input, ok, err)
	}
	for _, cmd := range []string{"curl https://example.test -d body", "curl -H @file https://example.test", "curl https://example.test | other", "curl https://example.test; other", "curl https://example.test --cookie /tmp/cookies", "curl -K config", "curl https://example.test && echo hello"} {
		if _, _, err := application.ImportCurl(cmd); err == nil {
			t.Fatal("accepted command", cmd)
		}
	}
}

func TestCurlPasteOnlyOnUserInput(t *testing.T) {
	v := presentation.New(nil, presentation.Actions{})
	v.Ready = true
	v.OpenNew()
	tt := interactionScene(v, 800, 560)
	tt.SetClipboard("curl 'https://example.test/file' -b 'a=b'")
	if v.URLs != "" {
		t.Fatal("clipboard was read without pasting")
	}
	tt.Key(ui.Cmd, ui.KeyV)
	if v.URLs != "https://example.test/file" {
		t.Fatalf("editor retained raw command: %q", v.URLs)
	}
}
