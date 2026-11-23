package aria2

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"skiff/internal/contexts/transfer/application"
)

type EngineRelease struct {
	URL, ArchiveSHA256, BinarySHA256 string
}

type Installation struct {
	directory string
	Release   EngineRelease
	Client    *http.Client
}

// Archive digests come from the release's SHA256SUMS. Binary digests
// also verify the cached executable on subsequent launches.
const releasePrefix = "https://github.com/motrixapp/aria2/releases/download/v1.37.0-motrix.24/aria2c-1.37.0-motrix.24-"

var engineReleases = map[string]EngineRelease{
	"darwin/arm64": {
		URL:           releasePrefix + "darwin-arm64.tar.gz",
		ArchiveSHA256: "ea67b6d91f275358f61fd5880db777854b519e16cb3dc5acf163e20c2a8af018",
		BinarySHA256:  "b5beffed4f4b56dd9bf155ca48c6fbca1cc69598917982fad9b5ebba65738047",
	},
	"darwin/amd64": {
		URL:           releasePrefix + "darwin-x64.tar.gz",
		ArchiveSHA256: "e31f15ede41c01f817f2b197ba08f2daa544d346ca2b54529c8563e74072bd58",
		BinarySHA256:  "d814bd512f8ce95e2e57549fe3d654fbe62c01ba15de52084b12923d3a8b2a49",
	},
	"linux/arm64": {
		URL:           releasePrefix + "linux-arm64.tar.gz",
		ArchiveSHA256: "adb72cbbeb263eac67953923093e16d4c9251fef3a3648aa794b615b605279bb",
		BinarySHA256:  "1a7ba9f87a6b68842e3d71cfe917c0fcb729b13274c3572bd7baa74af1a33c24",
	},
	"linux/amd64": {
		URL:           releasePrefix + "linux-x64.tar.gz",
		ArchiveSHA256: "f947ee872bfa294d958913503dc470140c247c97cc2e83b75d196e7a6bad155e",
		BinarySHA256:  "333a694fdaae0fba86e678e0c6893e909a556c2916ad6c5188bed09a99b1245f",
	},
	"windows/arm64": {
		URL:           releasePrefix + "win32-arm64.zip",
		ArchiveSHA256: "6cac43afc1e543f50951db49ef4bce9e73614d48ba2bd0aee8dc7d59ba54f29c",
		BinarySHA256:  "e883993c910efb8f096155676d950d8715561f15769c895fe8bdaedc0c0d3522",
	},
	"windows/amd64": {
		URL:           releasePrefix + "win32-x64.zip",
		ArchiveSHA256: "3305bc54c4b03da7d702848a189c8f40c85a23cce9c432e8ecf7c03d7b2dd11e",
		BinarySHA256:  "8138d93127fb5885a8100480a260d8383559b598e7e3e6ceedbc1d9f5959c849",
	},
}

func NewInstallation(dataDir string) *Installation {
	return &Installation{
		directory: filepath.Join(dataDir, "engine"),
		Release:   engineReleases[runtime.GOOS+"/"+runtime.GOARCH],
		// Installation precedes loading task preferences. Resolve each release
		// request, including redirects, with the system proxy and DNS settings.
		Client: &http.Client{Transport: systemDNSTransport()},
	}
}

func (r EngineRelease) binaryName() string {
	if strings.HasSuffix(r.URL, ".zip") {
		return "aria2c.exe"
	}
	return "aria2c"
}

const maxEngineBytes = 32 << 20

func (s *Installation) Find() (string, error) {
	path := filepath.Join(s.directory, s.Release.binaryName())
	if os.Getenv("SKIFF_ARIA2_BINARY") == "" && s.Release.URL != "" && s.validBinary(path) {
		return path, nil
	}
	path, err := FindBinary()
	if errors.Is(err, application.ErrEngineMissing) && s.Release.URL == "" {
		return "", errors.New("请为当前平台安装 aria2 后重新打开 Skiff")
	}
	return path, err
}

func (s *Installation) validBinary(path string) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0100 == 0) || info.Size() > maxEngineBytes {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	hash := sha256.New()
	_, err = io.Copy(hash, io.LimitReader(f, maxEngineBytes+1))
	return err == nil && hex.EncodeToString(hash.Sum(nil)) == s.Release.BinarySHA256
}

func (s *Installation) Install(ctx context.Context) (string, error) {
	if s.Release.URL == "" {
		return "", errors.New("当前平台没有可下载的引擎")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.Release.URL, nil)
	if err != nil {
		return "", err
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("无法下载引擎：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("引擎下载失败（HTTP %d）", resp.StatusCode)
	}
	// Verify the bounded archive before parsing it. Extract only the expected
	// regular file; no archive-controlled path or mode is used on disk.
	archive, err := io.ReadAll(io.LimitReader(resp.Body, maxEngineBytes+1))
	if canceled := ctx.Err(); canceled != nil {
		return "", canceled
	}
	if err != nil {
		return "", fmt.Errorf("引擎下载中断：%w", err)
	}
	if len(archive) > maxEngineBytes || fmt.Sprintf("%x", sha256.Sum256(archive)) != s.Release.ArchiveSHA256 {
		return "", errors.New("引擎安装包校验失败，请重新下载")
	}
	if err = os.MkdirAll(s.directory, 0700); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(s.directory, ".aria2c-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	hash := sha256.New()
	if err = s.Release.extract(archive, io.MultiWriter(tmp, hash)); err != nil {
		return "", err
	}
	if hex.EncodeToString(hash.Sum(nil)) != s.Release.BinarySHA256 {
		return "", errors.New("引擎文件校验失败，请重新下载")
	}
	if err = tmp.Chmod(0700); err != nil {
		return "", err
	}
	if err = tmp.Close(); err != nil {
		return "", err
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	path := filepath.Join(s.directory, s.Release.binaryName())
	if err = os.Rename(tmp.Name(), path); err != nil {
		return "", err
	}
	return path, nil
}

// Both published formats contain a root-level executable. The output path and
// permissions belong to the installer, regardless of archive metadata.
func (r EngineRelease) extract(archive []byte, dst io.Writer) error {
	if strings.HasSuffix(r.URL, ".zip") {
		z, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return fmt.Errorf("无法读取引擎安装包：%w", err)
		}
		if len(z.File) != 1 {
			return errors.New("引擎安装包内容无效")
		}
		f := z.File[0]
		if f.Name != r.binaryName() || !f.Mode().IsRegular() || f.UncompressedSize64 == 0 || f.UncompressedSize64 > maxEngineBytes {
			return errors.New("引擎安装包内容无效")
		}
		reader, err := f.Open()
		if err != nil {
			return err
		}
		defer reader.Close()
		_, err = io.Copy(dst, reader)
		return err
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return fmt.Errorf("无法读取引擎安装包：%w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	member, err := tr.Next()
	if err != nil || member.Name != r.binaryName() || member.Typeflag != tar.TypeReg || member.Size <= 0 || member.Size > maxEngineBytes {
		return errors.New("引擎安装包内容无效")
	}
	_, err = io.Copy(dst, tr)
	return err
}
