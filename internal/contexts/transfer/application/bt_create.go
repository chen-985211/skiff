package application

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

type TorrentWriter interface {
	WriteTorrent(context.Context, string, string, []string) (Torrent, error)
}

func (s *Service) CreateTorrent(ctx context.Context, source, output string, trackers []string, seed bool) (Torrent, error) {
	lib, ok := s.engine.(TorrentWriter)
	if !ok {
		return Torrent{}, errors.New("当前引擎不支持创建种子")
	}
	if !filepath.IsAbs(source) || !filepath.IsAbs(output) {
		return Torrent{}, errors.New("请选择原文件和种子保存路径")
	}
	var announce []string
	for _, line := range trackers {
		tr := strings.TrimSpace(line)
		if tr == "" {
			continue
		}
		u, err := url.Parse(tr)
		if err != nil || u.Hostname() == "" || strings.ContainsAny(tr, " \r\t\n") || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "udp") {
			return Torrent{}, fmt.Errorf("Tracker 地址无效：%s", tr)
		}
		announce = append(announce, tr)
	}
	source = filepath.Clean(source)
	meta, err := lib.WriteTorrent(ctx, source, output, unique(announce))
	if err != nil || !seed {
		return meta, err
	}
	dir := filepath.Dir(source)
	var existing TaskDTO
	for _, t := range s.Snapshot().Tasks {
		if t.TorrentHash == meta.Hash && t.Directory == dir {
			existing = t
			break
		}
	}
	if existing.Seeding {
		return meta, nil
	}
	if existing.ID != "" {
		err = s.UseTorrentFiles(ctx, existing.ID, dir, false, true)
	} else {
		selected := make([]int, len(meta.Files))
		for i := range selected {
			selected[i] = i + 1
		}
		_, err = s.addTorrent(ctx, meta.Hash, dir, selected, true)
	}
	if err != nil {
		return meta, fmt.Errorf("种子已保存，启动做种失败：%w", err)
	}
	return meta, nil
}
