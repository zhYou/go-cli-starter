package scaffold

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/spf13/afero"
	"io"
	"os"
	"path/filepath"
	"slices"
)

const journalDir = ".template-init"

type transaction struct {
	FormatVersion  int               `json:"format_version"`
	Module         string            `json:"module"`
	Name           string            `json:"name"`
	OriginalModule string            `json:"original_module"`
	OriginalName   string            `json:"original_name"`
	Phase          string            `json:"phase"`
	Files          []transactionFile `json:"files"`
}
type transactionFile struct {
	Path    string      `json:"path"`
	Mode    os.FileMode `json:"mode"`
	Existed bool        `json:"existed"`
	Before  string      `json:"before_sha256"`
	After   string      `json:"after_sha256"`
	Backup  string      `json:"backup"`
	Staged  string      `json:"staged"`
}

func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func (i *Initializer) save(root string, tx transaction) error {
	data, err := json.MarshalIndent(tx, "", "  ")
	if err != nil {
		return err
	}
	return i.atomicWrite(change{filepath.Join(root, journalDir, "journal.json"), data, 0600})
}
func (i *Initializer) prepare(root, module, name string, changes []change) (transaction, error) {
	tx := transaction{FormatVersion: 1, Module: module, Name: name, OriginalModule: TemplateModule, OriginalName: TemplateName, Phase: "prepared"}
	if err := i.FS.Mkdir(filepath.Join(root, journalDir), 0700); err != nil {
		return tx, err
	}
	for index, item := range changes {
		relative, err := filepath.Rel(root, item.path)
		if err != nil {
			return tx, err
		}
		before, err := afero.ReadFile(i.FS, item.path)
		existed := err == nil
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return tx, err
		}
		file := transactionFile{Path: filepath.ToSlash(relative), Mode: item.mode, Existed: existed, Before: digest(before), After: digest(item.data), Backup: fmt.Sprintf("%s/%06d.before", journalDir, index), Staged: fmt.Sprintf("%s/%06d.after", journalDir, index)}
		for path, data := range map[string][]byte{file.Backup: before, file.Staged: item.data} {
			if err := i.atomicWrite(change{filepath.Join(root, filepath.FromSlash(path)), data, 0600}); err != nil {
				return tx, err
			}
		}
		tx.Files = append(tx.Files, file)
	}
	return tx, i.save(root, tx)
}
func (i *Initializer) validateTransaction(root string, tx transaction) error {
	if tx.FormatVersion != 1 || tx.OriginalModule != TemplateModule || tx.OriginalName != TemplateName || len(tx.Files) == 0 || !slices.Contains([]string{"prepared", "applying", "committed"}, tx.Phase) {
		return errors.New("恢复记录格式或阶段无效")
	}
	seen := map[string]bool{}
	for index, file := range tx.Files {
		if !slices.Contains(managedFiles, file.Path) && file.Path != ".template-origin.json" {
			return fmt.Errorf("恢复路径不在受管清单：%q", file.Path)
		}
		if seen[file.Path] || file.Mode&^os.FileMode(0777) != 0 {
			return errors.New("恢复记录包含重复路径或无效权限")
		}
		seen[file.Path] = true
		path, err := i.safePath(root, file.Path)
		if err != nil {
			return err
		}
		if info, err := i.FS.Stat(path); err == nil && !info.Mode().IsRegular() {
			return fmt.Errorf("恢复目标不是常规文件：%s", file.Path)
		}
		if file.Backup != fmt.Sprintf("%s/%06d.before", journalDir, index) || file.Staged != fmt.Sprintf("%s/%06d.after", journalDir, index) {
			return errors.New("恢复备份路径无效")
		}
		for relative, want := range map[string]string{file.Backup: file.Before, file.Staged: file.After} {
			backup, err := i.safePath(root, relative)
			if err != nil {
				return err
			}
			data, err := afero.ReadFile(i.FS, backup)
			if err != nil {
				return err
			}
			if len(want) != 64 || digest(data) != want {
				return fmt.Errorf("恢复数据摘要不符：%s", relative)
			}
		}
		data, err := afero.ReadFile(i.FS, path)
		exists := err == nil
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		before := exists == file.Existed && digest(data) == file.Before
		after := exists && digest(data) == file.After
		if !before && !after || tx.Phase == "prepared" && !before || tx.Phase == "committed" && !after {
			return fmt.Errorf("目标文件已被修改，保留恢复现场：%s", file.Path)
		}
	}
	return nil
}
func (i *Initializer) rollback(root string, tx transaction) error {
	if err := i.validateTransaction(root, tx); err != nil {
		return err
	}
	for index := len(tx.Files) - 1; index >= 0; index-- {
		file := tx.Files[index]
		path, err := i.safePath(root, file.Path)
		if err != nil {
			return err
		}
		current, err := afero.ReadFile(i.FS, path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if file.Existed {
			if digest(current) == file.Before {
				continue
			}
			data, err := afero.ReadFile(i.FS, filepath.Join(root, file.Backup))
			if err != nil {
				return err
			}
			if err := i.atomicWrite(change{path, data, file.Mode}); err != nil {
				return err
			}
		} else if err == nil {
			if err := i.FS.Remove(path); err != nil {
				return err
			}
		}
	}
	return i.FS.RemoveAll(filepath.Join(root, journalDir))
}
func (i *Initializer) recover(root, module, name string) error {
	dir, err := i.safePath(root, journalDir)
	if err != nil {
		return err
	}
	if _, err := i.FS.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	path, err := i.safePath(root, journalDir+"/journal.json")
	if err != nil {
		return err
	}
	data, err := afero.ReadFile(i.FS, path)
	if err != nil {
		return fmt.Errorf("恢复记录不完整，保留现场：%w", err)
	}
	var tx transaction
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&tx); err != nil {
		return fmt.Errorf("恢复记录损坏：%w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("恢复记录包含额外内容")
	}
	if tx.Module != module || tx.Name != name {
		return errors.New("恢复目标与本次参数冲突，保留现场")
	}
	if err := i.validateTransaction(root, tx); err != nil {
		return err
	}
	if tx.Phase == "committed" {
		return i.FS.RemoveAll(dir)
	}
	return i.rollback(root, tx)
}
func (i *Initializer) apply(ctx context.Context, root, module, name string, changes []change) ([]string, error) {
	tx, err := i.prepare(root, module, name, changes)
	if err != nil {
		return nil, errors.Join(err, i.FS.RemoveAll(filepath.Join(root, journalDir)))
	}
	tx.Phase = "applying"
	if err := i.save(root, tx); err != nil {
		return nil, errors.Join(err, i.rollback(root, tx))
	}
	fail := func(cause error) ([]string, error) { return nil, errors.Join(cause, i.rollback(root, tx)) }
	changed := []string{}
	for index, item := range changes {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if err := i.validateTransaction(root, tx); err != nil {
			return nil, err
		}
		if err := i.atomicWrite(item); err != nil {
			return fail(fmt.Errorf("初始化 %s 失败：%w", item.path, err))
		}
		changed = append(changed, tx.Files[index].Path)
	}
	tx.Phase = "committed"
	if err := i.save(root, tx); err != nil {
		tx.Phase = "applying"
		return fail(err)
	}
	if err := i.FS.RemoveAll(filepath.Join(root, journalDir)); err != nil {
		return changed, fmt.Errorf("初始化已提交，清理恢复目录失败：%w", err)
	}
	return changed, nil
}
