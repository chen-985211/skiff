package aria2

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"skiff/internal/contexts/transfer/application"
	"strings"
	"sync"
	"time"
)

// aria2 applies custom headers to every redirect. For requests with private
// fields, resolve redirects here and stream the final response back to aria2.
// Ordinary downloads go directly through the engine.
type downloadRoutes struct {
	mu      sync.Mutex
	server  *http.Server
	base    string
	byID    map[string]string
	byToken map[string]application.TransferSpec
}

const redirectLimitQuery = "redirect-limit"

func (g *downloadRoutes) register(s application.TransferSpec) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.server == nil {
		l, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			return "", err
		}
		g.base = "http://" + l.Addr().String()
		g.server = &http.Server{Handler: http.HandlerFunc(g.serve), ReadHeaderTimeout: 5 * time.Second}
		g.byID = make(map[string]string)
		g.byToken = make(map[string]application.TransferSpec)
		go g.server.Serve(l)
	}
	delete(g.byToken, g.byID[s.ID])
	token := rand.Text()
	g.byID[s.ID] = token
	g.byToken[token] = s
	return g.base + "/_skiff_transfer/" + token, nil
}
func (g *downloadRoutes) remove(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.byToken, g.byID[id])
	delete(g.byID, id)
}
func (g *downloadRoutes) close() {
	g.mu.Lock()
	server := g.server
	g.mu.Unlock()
	if server != nil {
		server.Close()
	}
}
func applyHeaders(r *http.Request, block string) {
	for line := range strings.Lines(block) {
		k, v, ok := strings.Cut(line, ":")
		if ok {
			r.Header.Add(k, strings.TrimSpace(v))
		}
	}
}
func sourceClient(s application.TransferSpec) *http.Client {
	return &http.Client{Transport: networkTransport{s.Network}, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return application.ErrTooManyRedirects
		}
		for _, prev := range via {
			if !strings.EqualFold(prev.URL.Host, req.URL.Host) || req.URL.Scheme != prev.URL.Scheme {
				for line := range strings.Lines(s.Headers) {
					k, _, _ := strings.Cut(line, ":")
					req.Header.Del(k)
				}
				req.Header.Del("Referer")
				break
			}
		}
		return nil
	}}
}
func (g *downloadRoutes) serve(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.URL.Path, "/_skiff_transfer/")
	g.mu.Lock()
	s, ok := g.byToken[token]
	g.mu.Unlock()
	if !ok || r.Method != "GET" && r.Method != "HEAD" {
		http.NotFound(w, r)
		return
	}
	if r.URL.RawQuery == redirectLimitQuery {
		redirectAtLimit(w, r)
		return
	}
	upstream, err := http.NewRequestWithContext(r.Context(), r.Method, s.URL, nil)
	if err != nil {
		http.Error(w, "invalid source", 400)
		return
	}
	for _, k := range []string{"Range", "If-Range", "If-Match", "If-Unmodified-Since"} {
		if v := r.Header.Get(k); v != "" {
			upstream.Header.Set(k, v)
		}
	}
	upstream.Header.Set("Accept-Encoding", "identity")
	applyHeaders(upstream, s.Headers)
	resp, err := sourceClient(s).Do(upstream)
	if err != nil {
		if r.Context().Err() == nil {
			if errors.Is(err, application.ErrTooManyRedirects) {
				redirectAtLimit(w, r)
			} else {
				http.Error(w, "source unavailable", 502)
			}
		}
		return
	}
	defer resp.Body.Close()
	for _, k := range []string{"Content-Disposition", "Content-Length", "Content-Range", "Content-Type", "Accept-Ranges", "Etag", "Last-Modified", "Content-Encoding", "Retry-After"} {
		if v := resp.Header.Get(k); v != "" {
			w.Header().Set(k, v)
		}
	}
	if w.Header().Get("Content-Disposition") == "" {
		name := path.Base(resp.Request.URL.Path)
		if name == "." || name == "/" {
			u, _ := url.Parse(s.URL)
			name = u.Hostname()
		}
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	}
	w.WriteHeader(resp.StatusCode)
	if r.Method != "HEAD" {
		if _, err := io.Copy(w, resp.Body); err != nil && r.Context().Err() != context.Canceled {
			panic(http.ErrAbortHandler)
		}
	}
}

func redirectAtLimit(w http.ResponseWriter, r *http.Request) {
	// Skiff handles the remaining redirects locally so aria2 reports its own redirect-limit code.
	w.Header().Set("Location", r.URL.Path+"?"+redirectLimitQuery)
	w.WriteHeader(http.StatusFound)
}
