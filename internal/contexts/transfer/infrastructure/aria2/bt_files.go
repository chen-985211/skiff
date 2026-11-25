package aria2

import (
	"bytes"
	"context"
	"errors"
	"github.com/anacrolix/torrent/metainfo"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"skiff/internal/contexts/transfer/application"
)

type cancellableFile struct {
	file *os.File
	ctx  context.Context
}

func (f cancellableFile) Read(buf []byte) (int, error) {
	if err := f.ctx.Err(); err != nil {
		return 0, err
	}
	return f.file.Read(buf)
}
func (f cancellableFile) Close() error { return f.file.Close() }
func openPayload(root *os.Root, name string, size int64) (*os.File, os.FileInfo, error) {
	st, err := root.Lstat(name)
	if err != nil {
		return nil, nil, payloadOpenError(name, err)
	}
	if !st.Mode().IsRegular() || st.Size() != size {
		return nil, nil, application.NamedError{Code: application.ErrTorrentFileMismatch, Name: filepath.ToSlash(name)}
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, nil, payloadOpenError(name, err)
	}
	info, err := file.Stat()
	if err != nil || !os.SameFile(st, info) {
		file.Close()
		return nil, nil, application.NamedError{Code: application.ErrTorrentFileChanged, Name: filepath.ToSlash(name)}
	}
	return file, info, nil
}
func payloadOpenError(name string, err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return application.NamedError{Code: application.ErrTorrentFileMissing, Name: filepath.ToSlash(name), Cause: err}
	}
	return err
}

func generatePayloadPieces(info *metainfo.Info, open func(metainfo.FileInfo) (io.ReadCloser, error)) error {
	// The metainfo writer turns open failures into text on its worker goroutine.
	// Keep the original cause so callers can read the code and relative name.
	openFailure := make(chan error, 1)
	err := info.GeneratePieces(func(entry metainfo.FileInfo) (io.ReadCloser, error) {
		reader, err := open(entry)
		if err != nil {
			openFailure <- err
		}
		return reader, err
	})
	select {
	case cause := <-openFailure:
		return cause
	default:
		return err
	}
}

func (c *Client) VerifyFiles(ctx context.Context, hash, dir string) error {
	path, err := c.metadataFile(hash)
	if err != nil {
		return err
	}
	data, meta, err := readTorrent(path)
	if err != nil {
		return err
	}
	if meta.Hash != hash {
		return application.ErrTorrentHashChanged
	}
	m, err := metainfo.Load(bytes.NewReader(data))
	if err != nil {
		return err
	}
	info, err := m.UnmarshalInfo()
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	expected := info.Pieces
	err = generatePayloadPieces(&info, func(fi metainfo.FileInfo) (io.ReadCloser, error) {
		name := meta.Name
		if meta.Multi {
			name = filepath.Join(meta.Name, filepath.FromSlash(fi.DisplayPath(&info)))
		}
		file, _, err := openPayload(root, name, fi.Length)
		if err != nil {
			return nil, err
		}
		return cancellableFile{file, ctx}, nil
	})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return err
	}
	if !bytes.Equal(expected, info.Pieces) {
		return application.ErrTorrentFiles
	}
	return ctx.Err()
}

type copiedFile struct {
	name     string
	src, dst os.FileInfo
}
type fileMove struct {
	from, to *os.Root
	files    []copiedFile
}

func (m *fileMove) Finish(commit bool) error {
	defer m.from.Close()
	defer m.to.Close()
	var errs []error
	for _, f := range m.files {
		root, expected := m.to, f.dst
		if commit {
			saved, e := m.to.Lstat(f.name)
			if e != nil || !os.SameFile(saved, f.dst) || saved.Size() != f.dst.Size() || !saved.ModTime().Equal(f.dst.ModTime()) {
				errs = append(errs, application.NamedError{Code: application.ErrTorrentDestinationChanged, Name: filepath.ToSlash(f.name)})
				continue
			}
			root, expected = m.from, f.src
		}
		st, err := root.Lstat(f.name)
		if err != nil || !os.SameFile(st, expected) || (commit && (st.Size() != expected.Size() || !st.ModTime().Equal(expected.ModTime()))) {
			errs = append(errs, application.NamedError{Code: application.ErrTorrentFileChanged, Name: filepath.ToSlash(f.name)})
			continue
		}
		errs = append(errs, root.Remove(f.name))
	}
	return errors.Join(errs...)
}
func (c *Client) CopyFiles(ctx context.Context, hash, src, dst string) (_ application.TorrentMove, err error) {
	meta, err := c.CachedTorrent(hash)
	if err != nil {
		return nil, err
	}
	from, err := os.OpenRoot(src)
	if err != nil {
		return nil, err
	}
	to, err := os.OpenRoot(dst)
	if err != nil {
		from.Close()
		return nil, err
	}
	m := &fileMove{from: from, to: to}
	defer func() {
		if err != nil {
			err = errors.Join(err, m.Finish(false))
		}
	}()
	for _, entry := range meta.Files {
		name := meta.Name
		if meta.Multi {
			name = filepath.Join(name, filepath.FromSlash(entry.Path))
		}
		if err := to.MkdirAll(filepath.Dir(name), 0755); err != nil {
			return nil, err
		}
		if err := m.copyOne(ctx, name, entry.Size); err != nil {
			return nil, err
		}
	}
	return m, nil
}
func (m *fileMove) copyOne(ctx context.Context, name string, size int64) error {
	in, src, err := openPayload(m.from, name, size)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := m.to.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	dst, err := out.Stat()
	if err != nil {
		out.Close()
		return err
	}
	m.files = append(m.files, copiedFile{name, src, dst})
	_, err = io.Copy(out, cancellableFile{in, ctx})
	if err == nil {
		err = out.Sync()
	}
	if err == nil {
		m.files[len(m.files)-1].dst, err = out.Stat()
		if err != nil {
			m.files[len(m.files)-1].dst = dst
		}
	}
	return errors.Join(err, out.Close())
}
