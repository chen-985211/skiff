package aria2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"

	"skiff/internal/contexts/transfer/application"
)

// Enumerate and hash from the same root, with a deterministic v1 file order.
func (c *Client) WriteTorrent(ctx context.Context, source, dest string, trackers []string) (_ application.Torrent, err error) {
	var empty application.Torrent
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	st, err := os.Lstat(source)
	if err != nil {
		return empty, err
	}
	if !st.IsDir() && !st.Mode().IsRegular() {
		return empty, errors.New("请选择普通文件或文件夹")
	}
	if !strings.EqualFold(filepath.Ext(dest), ".torrent") {
		return empty, errors.New("种子文件需要 .torrent 扩展名")
	}
	destParent, err := filepath.EvalSymlinks(filepath.Dir(dest))
	if err != nil {
		return empty, err
	}
	dest = filepath.Join(destParent, filepath.Base(dest))
	actual, err := filepath.EvalSymlinks(source)
	if err != nil {
		return empty, err
	}
	if actual == dest {
		return empty, errors.New("请选择与原文件不同的种子保存位置")
	}
	prior, statErr := os.Lstat(dest)
	if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return empty, statErr
	}
	if prior != nil {
		rel, _ := filepath.Rel(actual, dest)
		if os.SameFile(st, prior) || (st.IsDir() && filepath.IsLocal(rel)) {
			return empty, errors.New("保存会覆盖所选内容中的文件，请另选位置")
		}
		if !prior.Mode().IsRegular() {
			return empty, errors.New("种子保存目标不是普通文件")
		}
		if _, _, err = readTorrent(dest); err != nil {
			return empty, errors.New("同名文件不是有效种子，请另选名称")
		}
	}
	dir := actual
	if !st.IsDir() {
		dir = filepath.Dir(actual)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return empty, err
	}
	defer root.Close()
	info := metainfo.Info{Name: filepath.Base(source)}
	stats := map[string]os.FileInfo{}
	if st.IsDir() {
		err = fs.WalkDir(root.FS(), ".", func(name string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if err = ctx.Err(); err != nil {
				return err
			}
			if !d.Type().IsRegular() {
				return nil
			}
			stat, err := d.Info()
			if err != nil {
				return err
			}
			stats[name] = stat
			info.Files = append(info.Files, metainfo.FileInfo{Path: strings.Split(name, "/"), Length: stat.Size()})
			return nil
		})
		if err != nil {
			return empty, err
		}
	} else {
		info.Length = st.Size()
		stats[info.Name] = st
	}
	if info.TotalLength() <= 0 {
		return empty, errors.New("所选内容没有可制种的数据")
	}
	info.PieceLength = metainfo.ChoosePieceLength(info.TotalLength())
	err = info.GeneratePieces(func(f metainfo.FileInfo) (io.ReadCloser, error) {
		name := f.DisplayPath(&info)
		file, _, err := openPayload(root, filepath.FromSlash(name), f.Length)
		if err != nil {
			return nil, err
		}
		return cancellableFile{file, ctx}, nil
	})
	if ctx.Err() != nil {
		return empty, ctx.Err()
	}
	if err != nil {
		return empty, err
	}
	for path, before := range stats {
		if err = ctx.Err(); err != nil {
			return empty, err
		}
		after, e := root.Lstat(filepath.FromSlash(path))
		if e != nil || !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
			return empty, fmt.Errorf("原文件已变化，请重试：%s", path)
		}
	}
	raw, err := bencode.Marshal(info)
	if err != nil {
		return empty, err
	}
	m := metainfo.MetaInfo{InfoBytes: raw, CreatedBy: "Skiff"}
	if len(trackers) > 0 {
		m.Announce = trackers[0]
		for _, tr := range trackers {
			m.AnnounceList = append(m.AnnounceList, []string{tr})
		}
	}
	temp, err := os.CreateTemp(destParent, ".skiff-torrent-*")
	if err != nil {
		return empty, err
	}
	defer os.Remove(temp.Name())
	err = errors.Join(m.Write(temp), temp.Sync(), temp.Close())
	if err != nil {
		return empty, err
	}
	meta, err := c.PrepareTorrent(ctx, temp.Name())
	if err != nil {
		return empty, err
	}
	if err = ctx.Err(); err != nil {
		return empty, err
	}
	if prior == nil {
		// Publish only if the name is still free. Use an exclusive copy on filesystems without hard links.
		err = os.Link(temp.Name(), dest)
		if err != nil && !errors.Is(err, fs.ErrExist) {
			err = copyNewTorrent(temp.Name(), dest)
		}
	} else {
		now, e := os.Lstat(dest)
		if e != nil || !os.SameFile(prior, now) || !prior.ModTime().Equal(now.ModTime()) {
			return empty, errors.New("目标文件已变化，请重新选择")
		}
		err = os.Rename(temp.Name(), dest)
	}
	return meta, err
}
func copyNewTorrent(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	err = errors.Join(err, out.Sync(), out.Close())
	if err != nil {
		_ = os.Remove(dest)
	}
	return err
}
