package application

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"skiff/internal/contexts/transfer/domain"
)

type TorrentFile struct {
	Path string
	Size int64
}
type Torrent struct {
	Hash, Name string
	Files      []TorrentFile
	Multi      bool
	Total      int64
}
type TorrentLibrary interface {
	PrepareTorrent(context.Context, string) (Torrent, error)
	CachedTorrent(string) (Torrent, error)
}

func (s *Service) PrepareTorrent(ctx context.Context, source string) (Torrent, error) {
	lib, ok := s.engine.(TorrentLibrary)
	if !ok {
		return Torrent{}, errors.New("当前引擎不支持 BT")
	}
	return lib.PrepareTorrent(ctx, strings.TrimSpace(source))
}
func (s *Service) AddTorrent(ctx context.Context, hash, dir string, files []int) (string, error) {
	lib, ok := s.engine.(TorrentLibrary)
	if !ok {
		return "", errors.New("当前引擎不支持 BT")
	}
	meta, err := lib.CachedTorrent(hash)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(dir) || len(files) == 0 {
		return "", errors.New("请选择保存目录和至少一个文件")
	}
	dir = filepath.Clean(dir)
	selected := slices.Clone(files)
	slices.Sort(selected)
	selected = slices.Compact(selected)
	parts := make([]string, 0, len(selected))
	var size int64
	for _, i := range selected {
		if i < 1 || i > len(meta.Files) {
			return "", errors.New("所选文件已失效")
		}
		parts = append(parts, strconv.Itoa(i))
		size += meta.Files[i-1].Size
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.tasks {
		r := t.Record()
		if r.TorrentHash == hash && r.Directory == dir {
			return "", errors.New("此种子已在该目录的任务列表中")
		}
	}
	id := s.nextID()
	rec := domain.Record{ID: id, URL: "magnet:?xt=urn:btih:" + hash, Directory: dir, Path: filepath.Join(dir, meta.Name), Name: meta.Name, TorrentHash: hash, SelectedFiles: strings.Join(parts, ","), Total: size, Status: domain.Waiting}
	s.tasks = append(s.tasks, domain.Restore(rec))
	if err = s.persist(ctx); err != nil {
		s.tasks = s.tasks[:len(s.tasks)-1]
		return "", err
	}
	if err = s.engine.Enqueue(ctx, transferSpec(rec, false)); err != nil {
		s.tasks[len(s.tasks)-1].Mark(domain.Failed, err.Error())
		return id, errors.Join(err, s.persist(ctx))
	}
	return id, nil
}
