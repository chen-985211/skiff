//go:build integration

package transfer_test

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/mattn/go-ieproxy"

	"skiff/internal/contexts/transfer/infrastructure/aria2"
)

func TestEngineInstallUsesNativeHTTPSProxy(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		t.Skip("desktop proxy discovery is native to Windows and macOS")
	}
	for _, key := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy"} {
		if os.Getenv(key) != "" {
			t.Skip("environment settings take precedence over the desktop proxy")
		}
	}
	if ieproxy.GetConf().Automatic.Active {
		t.Skip("automatic discovery may need a network PAC service")
	}
	setup := aria2.NewInstallation(t.TempDir())
	req, err := http.NewRequest(http.MethodGet, setup.Release.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	want, err := ieproxy.GetProxyFunc()(req)
	if err != nil {
		t.Fatal(err)
	}
	if want == nil {
		t.Skip("no native proxy configured for this release URL")
	}
	tr := setup.Client.Transport
	if tr == nil {
		tr = http.DefaultTransport
	}
	got, err := tr.(*http.Transport).Proxy(req)
	if err != nil || got == nil || got.Host != want.Host {
		t.Fatal("the HTTPS release request did not select the desktop proxy")
	}
}

func TestEngineInstallUsesProxyAfterRedirect(t *testing.T) {
	// Both Go and the native proxy resolver cache settings on first use.
	if os.Getenv("SKIFF_TEST_ENGINE_PROXY") == "" {
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestEngineInstallUsesProxyAfterRedirect$")
		cmd.Env = append(os.Environ(), "SKIFF_TEST_ENGINE_PROXY=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, output)
		}
		return
	}

	payload := []byte("engine from the release proxy")
	archive := engineArchive(t, "aria2c.exe", payload)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !r.URL.IsAbs() || r.URL.Path != "/asset.zip" {
			http.Error(w, "expected a proxied asset request", http.StatusBadRequest)
			return
		}
		_, _ = w.Write(archive)
	}))
	defer proxy.Close()
	// The unspecified address stays local even if the client incorrectly dials
	// it directly. Unlike a loopback address, it is eligible for proxying.
	asset := strings.Replace(proxy.URL, "127.0.0.1", "0.0.0.0", 1) + "/asset.zip"
	release := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, asset, http.StatusFound)
	}))
	defer release.Close()
	for _, key := range []string{"HTTP_PROXY", "http_proxy", "NO_PROXY", "no_proxy"} {
		t.Setenv(key, "")
	}
	// An explicit HTTPS setting isolates the test from the host's desktop
	// settings. HTTP uses Skiff's ALL_PROXY fallback, except on loopback.
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy"} {
		t.Setenv(key, proxy.URL)
	}

	setup := aria2.NewInstallation(t.TempDir())
	setup.Release = aria2.EngineRelease{
		URL:           release.URL + "/engine.zip",
		ArchiveSHA256: fmt.Sprintf("%x", sha256.Sum256(archive)),
		BinarySHA256:  fmt.Sprintf("%x", sha256.Sum256(payload)),
	}
	if _, err := setup.Install(t.Context()); err != nil {
		t.Fatal(err)
	}
}
