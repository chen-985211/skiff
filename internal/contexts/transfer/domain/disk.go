package domain

import "errors"

type DiskSettings struct {
	CacheMiB, SplitMiB int
	DisableCache       bool
	Allocation         string
}

func (d DiskSettings) Validate() error {
	if d.CacheMiB < 0 || d.CacheMiB > 4096 || d.SplitMiB < 0 || d.SplitMiB > 1024 {
		return errors.New("缓存支持 0–4096 MiB，最小分段支持 1–1024 MiB")
	}
	switch d.Allocation {
	case "", "none", "prealloc":
		return nil
	}
	return errors.New("未知的预分配选项")
}
func (d DiskSettings) CacheBytes() int64 {
	if d.DisableCache {
		return 0
	}
	if d.CacheMiB == 0 {
		return 16 << 20
	}
	return int64(d.CacheMiB) << 20
}
func (d DiskSettings) SplitBytes() int64 {
	if d.SplitMiB == 0 {
		return MinSplitBytes
	}
	return int64(d.SplitMiB) << 20
}
func (t *Task) SetDisk(d DiskSettings) { t.record.Disk = d }
