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
	"strings"
)

const journalDir = ".template-init"

type transaction struct {
	RollbackComplete bool              `json:"rollback_complete,omitempty"`
	FormatVersion    int               `json:"format_version"`
	Module           string            `json:"module"`
	Name             string            `json:"name"`
	OriginalModule   string            `json:"original_module"`
	OriginalName     string            `json:"original_name"`
	Phase            string            `json:"phase"`
	Files            []transactionFile `json:"files"`
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
	if tx.RollbackComplete && tx.Phase != "prepared" {
		return errors.New("回滚终态与恢复阶段矛盾")
	}
	terminal := tx.Phase == "committed" || tx.RollbackComplete
	seen := map[string]bool{}
	for index, file := range tx.Files {
		if !file.Existed && (file.Path != ".template-origin.json" || file.Before != digest(nil)) {
			return errors.New("恢复记录的原文件存在性与路径或内容矛盾")
		}

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
			decoded, err := hex.DecodeString(want)
			if err != nil || len(decoded) != sha256.Size {
				return errors.New("恢复摘要格式无效")
			}
			backup, err := i.safePath(root, relative)
			if err != nil {
				return err
			}
			data, err := afero.ReadFile(i.FS, backup)
			if terminal && errors.Is(err, os.ErrNotExist) {
				continue
			}
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
	tx.Phase = "prepared"
	tx.RollbackComplete = true
	if err := i.save(root, tx); err != nil {
		return err
	}
	return i.cleanup(root, tx)
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
		if errors.Is(err, os.ErrNotExist) {
			entries, listErr := afero.ReadDir(i.FS, dir)
			if listErr == nil && len(entries) == 0 {
				return i.FS.Remove(dir)
			}
		}
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
	if tx.Phase == "committed" || tx.RollbackComplete {
		return i.cleanup(root, tx)
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
	if err := i.cleanup(root, tx); err != nil {
		return changed, fmt.Errorf("初始化已提交，清理恢复目录失败：%w", err)
	}
	return changed, nil
}

// UnmarshalJSON 拒绝必需字段缺失或 null，避免把损坏记录解释成合法零值。
func (file *transactionFile) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, key := range []string{"path", "mode", "existed", "before_sha256", "after_sha256", "backup", "staged"} {
		value, ok := fields[key]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("恢复文件记录缺少有效字段 %s", key)
		}
	}
	type plain transactionFile
	var decoded plain
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	*file = transactionFile(decoded)
	return nil
}

func (i *Initializer) cleanup(root string, tx transaction) error {
	// 终态日志保留到最后，备份允许已被此前的清理删除。
	for _, file := range tx.Files {
		for _, relative := range []string{file.Backup, file.Staged} {
			path, err := i.safePath(root, relative)
			if err != nil {
				return err
			}
			if err := i.FS.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	dir := filepath.Join(root, journalDir)
	entries, err := afero.ReadDir(i.FS, dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == "journal.json" {
			continue
		}
		// 崩溃可能留下写 journal 的临时文件；不删除其他未知内容。
		if !entry.Mode().IsRegular() || !strings.HasPrefix(entry.Name(), ".template-init-") {
			return fmt.Errorf("恢复目录包含未知内容，保留现场：%s", entry.Name())
		}
		if err := i.FS.Remove(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	if err := i.FS.Remove(filepath.Join(dir, "journal.json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// 此处若中断，只留下空目录；重试可安全移除。
	return i.FS.Remove(dir)
}
