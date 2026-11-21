package aria2

import (
	"context"
	"github.com/mattn/go-ieproxy"
	"golang.org/x/net/http/httpproxy"
	"net"
	"net/http"
	"net/url"
	"os"
	"skiff/internal/contexts/transfer/application"
	"strings"
	"sync"
	"time"
)

var systemProxy = sync.OnceValue(ieproxy.GetProxyFunc)
var systemHasProxy = sync.OnceValue(func() bool {
	conf := ieproxy.GetConf()
	if conf.Static.Active || conf.Automatic.Active {
		return true
	}
	for _, k := range []string{"http_proxy", "https_proxy", "HTTP_PROXY", "HTTPS_PROXY", "all_proxy", "ALL_PROXY"} {
		if os.Getenv(k) != "" {
			return true
		}
	}
	return false
})

func bypassHost(u *url.URL, list string) bool {
	host := strings.ToLower(u.Hostname())
	ip := net.ParseIP(host)
	for entry := range strings.FieldsFuncSeq(list, func(r rune) bool { return r == ',' || r == ';' || r == ' ' }) {
		entry = strings.ToLower(entry)
		if entry == "*" {
			return true
		}
		if _, network, err := net.ParseCIDR(entry); err == nil {
			if network.Contains(ip) {
				return true
			}
			continue
		}
		if h, port, e := net.SplitHostPort(entry); e == nil {
			target := u.Port()
			if target == "" {
				target = map[string]string{"http": "80", "https": "443", "ftp": "21"}[u.Scheme]
			}
			if port != target {
				continue
			}
			entry = h
		}
		entry = strings.Trim(strings.TrimPrefix(strings.TrimPrefix(entry, "*."), "."), "[]")
		if host == entry || strings.HasSuffix(host, "."+entry) {
			return true
		}
	}
	return false
}
func proxyFor(n application.NetworkSettings, r *http.Request) (*url.URL, error) {
	if n.Mode == "direct" || bypassHost(r.URL, n.Bypass) {
		return nil, nil
	}
	if n.Mode == "manual" {
		u, err := url.Parse(n.Proxy)
		if err != nil {
			return nil, err
		}
		if n.User != "" {
			u.User = url.UserPassword(n.User, n.Password)
		}
		return u, nil
	}
	if r.URL.Scheme == "ftp" {
		proxy := firstEnvironment("ftp_proxy", "FTP_PROXY", "all_proxy", "ALL_PROXY")
		if proxy == "" {
			proxy = ieproxy.GetConf().Static.Protocols["ftp"]
		}
		return environmentRoute(r.URL, proxy)
	}
	proxy, err := systemProxy()(r)
	if err != nil {
		return nil, err
	}
	if proxy != nil && proxy.Scheme == "" {
		p := *proxy
		p.Scheme = "http"
		return &p, nil
	}
	if proxy == nil {
		return environmentRoute(r.URL, firstEnvironment("all_proxy", "ALL_PROXY"))
	}
	return proxy, nil
}
func firstEnvironment(keys ...string) string {
	for _, key := range keys {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}
func environmentRoute(target *url.URL, address string) (*url.URL, error) {
	if address == "" {
		return nil, nil
	}
	cfg := httpproxy.Config{HTTPProxy: address, HTTPSProxy: address, NoProxy: firstEnvironment("no_proxy", "NO_PROXY")}
	u := *target
	if u.Scheme == "ftp" {
		u.Scheme = "http"
	}
	return cfg.ProxyFunc()(&u)
}

type networkKey struct{}

var automaticTransport = sync.OnceValue(func() *http.Transport { return sourceTransport(false) })
var systemDNSTransport = sync.OnceValue(func() *http.Transport { return sourceTransport(true) })

func sourceTransport(system bool) *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	if system {
		dialer.Resolver = net.DefaultResolver
	} else {
		dialer.Resolver = &net.Resolver{PreferGo: true}
	}
	tr.DialContext = dialer.DialContext
	tr.Proxy = func(r *http.Request) (*url.URL, error) {
		n, _ := r.Context().Value(networkKey{}).(application.NetworkSettings)
		return proxyFor(n, r)
	}
	return tr
}

type networkTransport struct{ network application.NetworkSettings }

func (t networkTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	transport := automaticTransport()
	if t.network.DNS == "system" {
		transport = systemDNSTransport()
	}
	return transport.RoundTrip(r.WithContext(context.WithValue(r.Context(), networkKey{}, t.network)))
}
func needsLocalRoute(s application.TransferSpec) bool {
	if strings.HasPrefix(s.URL, "ftp://") {
		return false
	}
	return s.Headers != "" || s.Network.Mode == "manual" || s.Network.Mode != "direct" && systemHasProxy()
}
func engineNetwork(s application.TransferSpec, options map[string]string) error {
	options["async-dns"] = "true"
	if s.Network.DNS == "system" {
		options["async-dns"] = "false"
	}
	if !strings.HasPrefix(s.URL, "ftp://") {
		return nil
	}
	req, err := http.NewRequest("GET", s.URL, nil)
	if err != nil {
		return err
	}
	proxy, err := proxyFor(s.Network, req)
	if err != nil {
		return err
	}
	if proxy != nil {
		p := *proxy
		if p.User != nil {
			options["ftp-proxy-user"] = p.User.Username()
			options["ftp-proxy-passwd"], _ = p.User.Password()
			p.User = nil
		}
		options["ftp-proxy"] = p.String()
	}
	return nil
}
