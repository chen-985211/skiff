package application

import (
	"context"
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
		return Torrent{}, ErrTorrentCreationUnsupported
	}
	if !filepath.IsAbs(source) || !filepath.IsAbs(output) {
		return Torrent{}, ErrTorrentPathsRequired
	}
	var announce []string
	for _, line := range trackers {
		tr := strings.TrimSpace(line)
		if tr == "" {
			continue
		}
		u, err := url.Parse(tr)
		if err != nil || u.Hostname() == "" || strings.ContainsAny(tr, " \r\t\n") || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "udp") {
			return Torrent{}, ErrInvalidTracker
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
		return meta, Wrap(ErrTorrentSeed, err)
	}
	return meta, nil
}
