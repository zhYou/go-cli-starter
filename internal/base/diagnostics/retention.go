package diagnostics

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/gofrs/flock"
)

var artifactPattern = regexp.MustCompile(`^(events|trace)(\.[0-9]{6,})?\.jsonl$`)

type candidate struct {
	name     string
	size     int64
	last     time.Time
	identity os.FileInfo
}

// Cleanup 非阻塞取得维护锁，只清理有有效标记且未被写入者锁定的运行目录。
func Cleanup(base string, config Config) error {
	if config.RetentionDays == 0 && config.MaxTotalSizeMiB == 0 {
		return nil
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	if info, err := root.Lstat(maintenanceName); err == nil && !info.Mode().IsRegular() {
		return invalidmaintenancelock.New()
	}
	maintenance := flock.New(filepath.Join(base, maintenanceName), flock.SetPermissions(0600))
	locked, err := maintenance.TryLock()
	if err != nil {
		return err
	}
	defer func() { _ = maintenance.Close() }()
	if !locked {
		return nil
	}
	entries, err := readEntries(root)
	if err != nil {
		return err
	}
	var total int64
	var candidates []candidate
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		c, err := inspect(root, entry.Name())
		if err != nil {
			continue
		}
		total += c.size
		candidates = append(candidates, c)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].last.Before(candidates[j].last) })
	cutoff := time.Now().Add(-time.Duration(config.RetentionDays) * 24 * time.Hour)
	for _, c := range candidates {
		expired := config.RetentionDays > 0 && c.last.Before(cutoff)
		oversized := config.MaxTotalSizeMiB > 0 && total > int64(config.MaxTotalSizeMiB)*mebibyte
		if !expired && !oversized {
			continue
		}
		removed, err := removeCandidate(root, base, c)
		if err != nil {
			return err
		}
		if removed {
			total -= c.size
		}
	}
	return nil
}
func inspect(root *os.Root, name string) (candidate, error) {
	c := candidate{name: name}
	info, err := root.Lstat(name)
	if err != nil {
		return c, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return c, invaliddirectory.New()
	}
	c.identity = info
	child, err := root.OpenRoot(name)
	if err != nil {
		return c, err
	}
	defer func() { _ = child.Close() }()
	entries, err := readEntries(child)
	if err != nil {
		return c, err
	}
	for _, e := range entries {
		if !e.Type().IsRegular() || (e.Name() != metadataName && e.Name() != metadataTemporaryName && e.Name() != leaseName && !artifactPattern.MatchString(e.Name())) {
			return c, unknowncontent.New()
		}
		info, err := e.Info()
		if err != nil {
			return c, err
		}
		c.size += info.Size()
		if info.ModTime().After(c.last) {
			c.last = info.ModTime()
		}
	}
	mi, err := child.Lstat(metadataName)
	if err != nil || !mi.Mode().IsRegular() {
		return c, invalidmetadata.New()
	}
	data, err := child.ReadFile(metadataName)
	if err != nil {
		return c, err
	}
	var meta metadata
	if err = json.Unmarshal(data, &meta); err != nil {
		return c, err
	}
	if meta.Version != metadataVersion || meta.ID != name || meta.Started.IsZero() {
		return c, unrecognizedmetadata.New()
	}
	if !meta.Finished.IsZero() {
		c.last = meta.Finished
	}
	return c, nil
}
func removeCandidate(root *os.Root, base string, c candidate) (bool, error) {
	leaseInfo, err := root.Lstat(filepath.Join(c.name, leaseName))
	if err != nil || !leaseInfo.Mode().IsRegular() {
		return false, nil
	}
	lease := flock.New(filepath.Join(base, c.name, leaseName), flock.SetFlag(os.O_RDWR))
	locked, err := lease.TryLock()
	if err != nil {
		return false, nil
	}
	defer func() { _ = lease.Close() }()
	if !locked {
		return false, nil
	}
	held, err := lease.Stat()
	if err != nil || !os.SameFile(leaseInfo, held) {
		return false, nil
	}
	fresh, err := inspect(root, c.name)
	if err != nil || !os.SameFile(c.identity, fresh.identity) || !fresh.last.Equal(c.last) {
		return false, nil
	}
	// 逐项移除已登记文件，未知内容会使目录保留；不对整个路径做递归删除。
	child, err := root.OpenRoot(c.name)
	if err != nil {
		return false, err
	}
	entries, err := readEntries(child)
	if err != nil {
		_ = child.Close()
		return false, err
	}
	for _, e := range entries {
		if e.Name() == leaseName || e.Name() == metadataName {
			continue
		}
		if err = child.Remove(e.Name()); err != nil && !errors.Is(err, fs.ErrNotExist) {
			_ = child.Close()
			return false, err
		}
	}
	// 数据清理全部成功后再移除标记，进程中断或单个删除失败时仍可被下次清理识别。
	if err = child.Remove(metadataName); err != nil && !errors.Is(err, fs.ErrNotExist) {
		_ = child.Close()
		return false, err
	}
	_ = child.Close()
	// Windows 不允许删除仍打开的锁文件，释放后再次核对目录身份；发布后的目录不会复用。
	if err = lease.Close(); err != nil {
		return false, err
	}
	if err = root.Remove(filepath.Join(c.name, leaseName)); err != nil {
		return false, err
	}
	return true, root.Remove(c.name)
}
