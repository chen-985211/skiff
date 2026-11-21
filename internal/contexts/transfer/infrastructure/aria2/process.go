package aria2

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/infrastructure/persistence"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Process struct {
	Client *Client
	cmd    *exec.Cmd
	done   chan struct{}
	mu     sync.Mutex
	closed bool
}

func FindBinary() (string, error) {
	if value := os.Getenv("SKIFF_ARIA2_BINARY"); value != "" {
		return exec.LookPath(value)
	}
	name := "aria2c"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if exe, err := os.Executable(); err == nil {
		for _, candidate := range []string{filepath.Join(filepath.Dir(exe), "..", "Resources", "engine", name), filepath.Join(filepath.Dir(exe), "engine", name)} {
			if _, err := exec.LookPath(candidate); err == nil {
				return filepath.Clean(candidate), nil
			}
		}
	}
	if binary, err := exec.LookPath(name); err == nil {
		return binary, nil
	}
	for _, candidate := range []string{"/opt/homebrew/bin/aria2c", "/usr/local/bin/aria2c"} {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", application.ErrEngineMissing
}
func Start(ctx context.Context, dataDir, binary string) (*Process, error) {
	return StartConfigured(ctx, dataDir, binary, application.Preferences{})
}
func StartConfigured(ctx context.Context, dataDir, binary string, prefs application.Preferences) (*Process, error) {
	if err := prefs.Disk.Validate(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	var secretBytes [24]byte
	if _, err := rand.Read(secretBytes[:]); err != nil {
		return nil, err
	}
	secret := hex.EncodeToString(secretBytes[:])
	session := filepath.Join(dataDir, "aria2.session")
	f, err := os.OpenFile(session, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	f.Close()
	config := strings.Join([]string{
		"disk-cache=" + strconv.FormatInt(prefs.Disk.CacheBytes(), 10), "rpc-max-request-size=32M", "enable-rpc=true", "rpc-listen-all=false", "rpc-allow-origin-all=false",
		"rpc-listen-port=" + strconv.Itoa(port), "rpc-secret=" + secret,
		"save-session=" + session, "input-file=" + session, "save-session-interval=5",
		"force-save=true", "continue=false", "pause=true", "max-concurrent-downloads=" + strconv.Itoa(application.DefaultConcurrency),
		"max-connection-per-server=4", "split=4", "min-split-size=4M",
		"auto-file-renaming=true", "allow-overwrite=false", "file-allocation=none",
		"follow-torrent=false", "follow-metalink=false", "enable-dht=false", "enable-dht6=false",
		"enable-peer-exchange=false", "bt-enable-lpd=false", "seed-time=0",
		"console-log-level=error", "summary-interval=0", "download-result=hide", "max-download-result=10000",
		"stop-with-process=" + strconv.Itoa(os.Getpid()),
	}, "\n") + "\n"
	conf := filepath.Join(dataDir, "aria2.conf")
	if err := persistence.WriteAtomic(conf, []byte(config)); err != nil {
		return nil, err
	}
	logFile, err := os.OpenFile(filepath.Join(dataDir, "engine.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return nil, err
	}
	p := &Process{cmd: exec.Command(binary, "--conf-path="+conf), done: make(chan struct{})}
	// All transfer proxy choices are explicit. Inherited engine environment
	// variables would otherwise override per-task direct and bypass modes.
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch strings.ToLower(key) {
		case "all_proxy", "http_proxy", "https_proxy", "ftp_proxy", "no_proxy":
			continue
		}
		p.cmd.Env = append(p.cmd.Env, entry)
	}
	p.cmd.Stdout, p.cmd.Stderr = logFile, logFile
	if err := p.cmd.Start(); err != nil {
		logFile.Close()
		return nil, err
	}
	go func() { _ = p.cmd.Wait(); logFile.Close(); close(p.done) }()
	p.Client = NewClient(fmt.Sprintf("http://127.0.0.1:%d/jsonrpc", port), secret)
	p.Client.metadataDir = filepath.Join(dataDir, "torrents")
	timer := time.NewTimer(8 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		probe, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
		err := p.Client.call(probe, "aria2.getVersion", nil, nil)
		cancel()
		if err == nil {
			return p, nil
		}
		select {
		case <-ctx.Done():
			p.kill()
			return nil, ctx.Err()
		case <-p.done:
			return nil, errors.New("aria2 启动失败，请查看数据目录内的 engine.log")
		case <-timer.C:
			p.kill()
			return nil, errors.New("下载引擎启动超时")
		case <-ticker.C:
		}
	}
}
func (p *Process) kill() { _ = p.cmd.Process.Kill(); <-p.done; p.Client.http.CloseIdleConnections() }
func (p *Process) Close(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	defer p.Client.http.CloseIdleConnections()
	defer p.Client.routes.close()
	select {
	case <-p.done:
		return errors.New("下载引擎已意外退出")
	default:
	}
	var failures []error
	if err := p.Client.call(ctx, "aria2.forcePauseAll", nil, nil); err != nil {
		failures = append(failures, err)
	}
	if err := p.Client.call(ctx, "aria2.saveSession", nil, nil); err != nil {
		failures = append(failures, err)
	}
	if err := p.Client.call(ctx, "aria2.shutdown", nil, nil); err != nil {
		failures = append(failures, err)
	}
	select {
	case <-p.done:
	case <-ctx.Done():
		p.kill()
		failures = append(failures, ctx.Err())
	}
	return errors.Join(failures...)
}
