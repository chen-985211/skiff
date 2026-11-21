package domain

import (
	"errors"
	"net/url"
	"strings"
)

type NetworkSettings struct{ Mode, Proxy, User, Password, Bypass, DNS string }

func (n NetworkSettings) Validate() error {
	switch n.Mode {
	case "", "system", "direct", "manual":
	default:
		return errors.New("未知的代理模式")
	}
	if n.DNS != "" && n.DNS != "system" {
		return errors.New("未知的 DNS 模式")
	}
	if n.Mode == "manual" {
		u, e := url.Parse(n.Proxy)
		if e != nil || u.Scheme != "http" || u.Hostname() == "" || u.User != nil || u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
			return errors.New("请输入 HTTP 代理地址，账号和密码单独填写")
		}
	}
	if strings.ContainsAny(n.User+n.Password+n.Bypass, "\x00\r\n") {
		return errors.New("代理配置包含无效字符")
	}
	return nil
}
func (t *Task) SetNetwork(n NetworkSettings) { t.record.Network = n }
