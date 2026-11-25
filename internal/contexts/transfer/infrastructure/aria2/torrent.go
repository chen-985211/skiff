package aria2

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anacrolix/torrent/metainfo"
	"skiff/internal/contexts/transfer/application"
	"skiff/internal/contexts/transfer/infrastructure/persistence"
)

const maxMetadata = 16 << 20

func readTorrent(file string) ([]byte, application.Torrent, error) {
	var t application.Torrent
	f, err := os.Open(file)
	if err != nil {
		return nil, t, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, t, err
	}
	if !st.Mode().IsRegular() || st.Size() > maxMetadata {
		return nil, t, application.ErrTorrentInvalid
	}
	data, err := io.ReadAll(io.LimitReader(f, maxMetadata+1))
	if err != nil || len(data) > maxMetadata {
		return nil, t, application.ErrTorrentRead
	}
	m, err := metainfo.Load(bytes.NewReader(data))
	if err != nil {
		return nil, t, application.Wrap(application.ErrTorrentRead, err)
	}
	info, err := m.UnmarshalInfo()
	if err != nil || info.MetaVersion > 1 || info.PieceLength <= 0 {
		return nil, t, application.ErrTorrentVersion
	}
	safe := func(part string) bool {
		return filepath.IsLocal(part) && part != "." && !strings.ContainsAny(part, "/\\:\x00")
	}
	if !safe(info.Name) || !safe(info.BestName()) {
		return nil, t, errors.New("种子文件名无效")
	}
	t.Private = info.Private != nil && *info.Private
	t.Name, t.Hash, t.Multi = info.BestName(), m.HashInfoBytes().HexString(), len(info.Files) > 0
	seen := map[string]bool{}
	for file := range info.UpvertedV1Files() {
		for _, path := range [][]string{file.Path, file.PathUtf8} {
			for _, part := range path {
				if !safe(part) {
					return nil, t, errors.New("种子包含无效路径")
				}
			}
		}
		name := file.DisplayPath(&info)
		if name == "" || seen[name] || file.Length < 0 || file.Length > math.MaxInt64-t.Total {
			return nil, t, errors.New("种子文件列表无效")
		}
		seen[name] = true
		t.Total += file.Length
		t.Files = append(t.Files, application.TorrentFile{Path: name, Size: file.Length})
	}
	if t.Total == 0 || int64(len(info.Pieces))/20 != (t.Total-1)/info.PieceLength+1 || len(info.Pieces)%20 != 0 {
		return nil, t, errors.New("种子分片信息无效")
	}
	return data, t, nil
}
func (c *Client) metadataFile(hash string) (string, error) {
	if b, err := hex.DecodeString(hash); err != nil || len(b) != 20 || c.metadataDir == "" {
		return "", errors.New("种子记录无效")
	}
	return filepath.Join(c.metadataDir, strings.ToLower(hash)+".torrent"), nil
}
func (c *Client) CachedTorrent(hash string) (application.Torrent, error) {
	f, err := c.metadataFile(hash)
	if err != nil {
		return application.Torrent{}, err
	}
	_, t, err := readTorrent(f)
	if err == nil && t.Hash != hash {
		err = errors.New("元数据与磁力指纹不一致")
	}
	return t, err
}
func (c *Client) PrepareTorrent(ctx context.Context, source string) (application.Torrent, error) {
	var t application.Torrent
	expected := ""
	if strings.HasPrefix(strings.ToLower(source), "magnet:") {
		mag, err := metainfo.ParseMagnetUri(source)
		if err != nil {
			return t, application.Wrap(application.ErrMagnetInvalid, err)
		}
		expected = mag.InfoHash.HexString()
		if t, err := c.CachedTorrent(expected); err == nil {
			return t, nil
		}
		if err = os.MkdirAll(c.metadataDir, 0700); err != nil {
			return t, err
		}
		temp, err := os.MkdirTemp(c.metadataDir, "lookup-")
		if err != nil {
			return t, err
		}
		defer os.RemoveAll(temp)
		var gid string
		opts := map[string]string{"dir": temp, "bt-metadata-only": "true", "bt-save-metadata": "true", "pause": "false", "force-save": "false"}
		if err = c.call(ctx, "aria2.addUri", []any{[]string{source}, opts}, &gid); err != nil {
			return t, err
		}
		defer func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = c.Execute(cleanup, gid, application.Remove)
		}()
		ticker := time.NewTicker(350 * time.Millisecond)
		defer ticker.Stop()
		for {
			var status struct{ Status, ErrorMessage string }
			if err = c.call(ctx, "aria2.tellStatus", []any{gid, []string{"status", "errorMessage"}}, &status); err != nil {
				return t, err
			}
			if status.Status == "complete" {
				break
			}
			if status.Status == "error" || status.Status == "removed" {
				return t, application.ErrMagnetMetadata
			}
			select {
			case <-ctx.Done():
				return t, ctx.Err()
			case <-ticker.C:
			}
		}
		source = filepath.Join(temp, expected+".torrent")
	}
	data, t, err := readTorrent(source)
	if err != nil {
		return t, err
	}
	if expected != "" && t.Hash != expected {
		return t, application.ErrMagnetMetadata
	}
	target, err := c.metadataFile(t.Hash)
	if err != nil {
		return t, err
	}
	return t, persistence.WriteAtomic(target, data)
}
func (c *Client) enqueueTorrent(ctx context.Context, s application.TransferSpec) error {
	path, err := c.metadataFile(s.TorrentHash)
	if err != nil {
		return err
	}
	data, meta, err := readTorrent(path)
	if err != nil {
		return err
	}
	if meta.Hash != s.TorrentHash {
		return errors.New("种子记录与文件不符")
	}
	opts := map[string]string{"gid": s.ID, "dir": s.Directory, "select-file": s.SelectedFiles, "pause": fmt.Sprint(s.Paused), "check-integrity": "true", "allow-overwrite": "false", "rpc-save-upload-metadata": "true"}
	if !s.Seed.Enabled {
		opts["seed-time"] = "0"
	} else {
		opts["bt-hash-check-seed"] = "true"
		opts["seed-ratio"] = fmt.Sprint(s.Seed.Ratio)
		if s.Seed.Minutes > 0 {
			opts["seed-time"] = fmt.Sprint(s.Seed.Minutes)
		}
	}
	if s.Trackers != "" && !meta.Private {
		opts["bt-tracker"] = s.Trackers
	}
	if s.BT.MaxPeers > 0 {
		opts["bt-max-peers"] = fmt.Sprint(s.BT.MaxPeers)
	}
	return c.call(ctx, "aria2.addTorrent", []any{base64.StdEncoding.EncodeToString(data), []string{}, opts}, nil)
}
