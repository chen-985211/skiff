package application

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

type TorrentWriter interface {
	WriteTorrent(context.Context, string, string, []string) (Torrent, error)
}

// TorrentSeedError reports a failure to start seeding after saving the torrent.
type TorrentSeedError struct {
	Cause error
}

func (e *TorrentSeedError) Error() string {
	return fmt.Sprintf("%s: %v", ErrTorrentSeed, e.Cause)
}

func (e *TorrentSeedError) Unwrap() []error {
	if e.Cause != nil {
		return []error{ErrTorrentSeed, e.Cause}
	}
	return []error{ErrTorrentSeed}
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
	for i, line := range trackers {
		tr := strings.TrimSpace(line)
		if tr == "" {
			continue
		}
		u, err := url.Parse(tr)
		if err != nil || u.Hostname() == "" || strings.ContainsAny(tr, " \r\t\n") || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "udp") {
			return Torrent{}, NamedError{Code: ErrInvalidTracker, Name: strconv.Itoa(i + 1)}
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
		return meta, &TorrentSeedError{Cause: err}
	}
	return meta, nil
}
