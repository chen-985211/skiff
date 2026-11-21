package application

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"skiff/internal/contexts/transfer/domain"
	"strings"
	"unicode"
)

type ImportedRequest struct{ URL, Headers string }

// Tokenize browser-generated cURL as text. Never expand variables, read
// argument files, or invoke a shell.
func curlWords(input string) ([]string, error) {
	input = strings.NewReplacer("\\\r\n", "", "\\\n", "", "^\r\n", "", "^\n", "").Replace(input)
	var args []string
	var word strings.Builder
	var quote rune
	started, escape := false, false
	for _, r := range input {
		if escape {
			word.WriteRune(r)
			started = true
			escape = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escape = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			continue
		}
		switch {
		case r == '\'' || r == '"':
			quote = r
			started = true
		case unicode.IsSpace(r):
			if started {
				args = append(args, word.String())
				word.Reset()
				started = false
			}
		case strings.ContainsRune("|;&<>()`$", r):
			return nil, errors.New("请粘贴单条浏览器下载请求")
		default:
			word.WriteRune(r)
			started = true
		}
	}
	if escape || quote != 0 {
		return nil, errors.New("cURL 引号或转义不完整")
	}
	if started {
		args = append(args, word.String())
	}
	return args, nil
}
func ImportCurl(text string) (ImportedRequest, bool, error) {
	var out ImportedRequest
	text = strings.TrimSpace(text)
	first := strings.Fields(text)
	if len(first) == 0 || first[0] != "curl" && first[0] != "curl.exe" {
		return out, false, nil
	}
	args, err := curlWords(text)
	if err != nil {
		return out, true, err
	}
	var headers []string
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
			if out.URL != "" {
				return out, true, errors.New("每条 cURL 只能包含一个地址")
			}
			out.URL = arg
			continue
		}
		switch arg {
		case "--compressed", "--location", "-L", "-g", "--globoff", "--http2", "--http1.1", "--silent", "-s", "-S", "--show-error":
			continue
		}
		option, value, inline := strings.Cut(arg, "=")
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && len(arg) > 2 {
			option, value, inline = arg[:2], arg[2:], true
		}
		switch option {
		case "-H", "--header", "-A", "--user-agent", "-b", "--cookie", "-e", "--referer", "-u", "--user", "--url", "-X", "--request":
		default:
			return out, true, errors.New("该 cURL 包含普通文件下载不支持的操作")
		}
		if !inline {
			if i+1 == len(args) {
				return out, true, errors.New("cURL 参数缺少值")
			}
			i++
			value = args[i]
		}
		switch option {
		case "--url":
			if out.URL != "" {
				return out, true, errors.New("每条 cURL 只能包含一个地址")
			}
			out.URL = value
		case "-X", "--request":
			if !strings.EqualFold(value, "GET") {
				return out, true, errors.New("仅支持 GET 下载请求")
			}
		case "-H", "--header":
			key, _, ok := strings.Cut(value, ":")
			if !ok || strings.HasPrefix(value, "@") {
				return out, true, errors.New("请粘贴请求头内容")
			}
			switch strings.ToLower(key) {
			case "accept-encoding", "content-length", "range", "connection", "host":
				continue
			}
			headers = append(headers, value)
		case "-A", "--user-agent":
			headers = append(headers, "User-Agent: "+value)
		case "-b", "--cookie":
			if !strings.Contains(value, "=") || strings.HasPrefix(value, "@") {
				return out, true, errors.New("请粘贴 Cookie 值")
			}
			headers = append(headers, "Cookie: "+value)
		case "-e", "--referer":
			headers = append(headers, "Referer: "+value)
		case "-u", "--user":
			if !strings.Contains(value, ":") {
				return out, true, errors.New("cURL 未包含完整登录信息")
			}
			headers = append(headers, "Authorization: Basic "+base64.StdEncoding.EncodeToString([]byte(value)))
		}
	}
	u, err := url.Parse(out.URL)
	if err != nil || u.Hostname() == "" || u.Scheme != "https" && u.Scheme != "http" {
		return out, true, fmt.Errorf("cURL 缺少有效 HTTP 文件地址")
	}
	out.Headers, err = domain.NormalizeHeaders(strings.Join(headers, "\n"))
	return out, true, err
}
