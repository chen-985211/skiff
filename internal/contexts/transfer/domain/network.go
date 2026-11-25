package domain

import (
	"net/url"
	"strings"
)

type NetworkSettings struct{ Mode, Proxy, User, Password, Bypass, DNS string }

func (n NetworkSettings) Validate() error {
	switch n.Mode {
	case "", "system", "direct", "manual":
	default:
		return ErrUnknownProxyMode
	}
	if n.DNS != "" && n.DNS != "system" {
		return ErrUnknownDNSMode
	}
	if n.Mode == "manual" {
		u, e := url.Parse(n.Proxy)
		if e != nil || u.Scheme != "http" || u.Hostname() == "" || u.User != nil || u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
			return ErrInvalidProxyAddress
		}
	}
	if strings.ContainsAny(n.User+n.Password+n.Bypass, "\x00\r\n") {
		return ErrInvalidProxyValue
	}
	return nil
}
func (t *Task) SetNetwork(n NetworkSettings) { t.record.Network = n }
