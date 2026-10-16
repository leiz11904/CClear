package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const CloseTimeoutMs = 5000

// AllowedNames are the only files an install may create or replace.
func AllowedNames() []string {
	return []string{"cclear.exe", "Uninstall.exe"}
}

func AppStillRunningError() error {
	return errors.New("CClear 仍在运行，安装已停止，没有改写已安装的文件。请先关闭 CClear，然后重试。")
}

func ReplaceBusyError(name string) error {
	return fmt.Errorf("无法更新 %s。请先关闭 CClear，然后重试。安装已撤销，没有留下半份文件。", name)
}

func allowedName(name string) error {
	for _, item := range AllowedNames() {
		if name == item {
			return nil
		}
	}
	return errors.New("安装程序不会写入未登记的文件")
}

// Txn stages CClear's own files and can roll them back.
// It never deletes files it did not create or replace.
type Txn struct {
	dir        string
	staging    string
	backup     string
	createdDir bool
	created    []string
	replaced   []string
	committed  bool
	rolled     bool
	finished   bool
}

func Begin(dir string) (*Txn, error) {
	if dir == "" {
		return nil, errors.New("请选择安装文件夹")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, errors.New("请选择一个有效的文件夹")
	}
	createdDir := false
	info, err := os.Stat(abs)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, errors.New("无法检查安装文件夹")
		}
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return nil, errors.New("无法创建安装文件夹")
		}
		createdDir = true
	} else if !info.IsDir() {
		return nil, errors.New("安装位置不是文件夹")
	}
	staging := filepath.Join(abs, ".cclear-new")
	backup := filepath.Join(abs, ".cclear-bak")
	if err := recoverInterrupted(abs, staging, backup); err != nil {
		if createdDir {
			_ = os.Remove(abs)
		}
		return nil, err
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		if createdDir {
			_ = os.Remove(abs)
		}
		return nil, errors.New("无法准备安装临时文件")
	}
	return &Txn{dir: abs, staging: staging, backup: backup, createdDir: createdDir}, nil
}

// recoverInterrupted finishes an interrupted replacement. A backup is first
// restored whenever its destination is missing, so a second crash cannot leave
// an installed file absent. Remaining staged files are then moved into place.
func recoverInterrupted(dir, staging, backup string) error {
	for _, name := range AllowedNames() {
		dest := filepath.Join(dir, name)
		bak := filepath.Join(backup, name)
		if _, err := os.Stat(dest); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return errors.New("无法检查已安装的程序文件")
		}
		if _, err := os.Stat(bak); err == nil {
			if err := os.Rename(bak, dest); err != nil {
				return ReplaceBusyError(name)
			}
		} else if !os.IsNotExist(err) {
			return errors.New("无法检查安装备份")
		}
	}

	for _, name := range AllowedNames() {
		src := filepath.Join(staging, name)
		if _, err := os.Stat(src); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return errors.New("无法检查安装临时文件")
		}
		dest := filepath.Join(dir, name)
		bak := filepath.Join(backup, name)
		if _, err := os.Stat(bak); err == nil {
			// A destination plus its backup means this item was already
			// replaced. Only discard the owned duplicate staging file.
			if _, err := os.Stat(dest); err == nil {
				_ = os.Remove(src)
				continue
			}
		} else if !os.IsNotExist(err) {
			return errors.New("无法检查安装备份")
		}
		if _, err := os.Stat(dest); err == nil {
			if err := os.MkdirAll(backup, 0o755); err != nil {
				return errors.New("无法准备安装备份")
			}
			if err := os.Rename(dest, bak); err != nil {
				return ReplaceBusyError(name)
			}
		} else if !os.IsNotExist(err) {
			return ReplaceBusyError(name)
		}
		if err := os.Rename(src, dest); err != nil {
			if _, statErr := os.Stat(dest); os.IsNotExist(statErr) {
				_ = os.Rename(bak, dest)
			}
			return fmt.Errorf("无法恢复 %s 的安装事务：%w", name, err)
		}
	}

	for _, name := range AllowedNames() {
		dest := filepath.Join(dir, name)
		bak := filepath.Join(backup, name)
		if _, err := os.Stat(dest); err == nil {
			_ = os.Remove(bak)
		}
	}
	_ = clearOwned(staging)
	_ = clearOwned(backup)
	return nil
}

func (t *Txn) Dir() string {
	if t == nil {
		return ""
	}
	return t.dir
}

func (t *Txn) Stage(name string, data []byte) error {
	if t == nil || t.rolled || t.finished {
		return errors.New("安装已经结束")
	}
	if err := allowedName(name); err != nil {
		return err
	}
	if len(data) == 0 {
		return errors.New("程序文件是空的")
	}
	return os.WriteFile(filepath.Join(t.staging, name), data, 0o755)
}

// Commit moves staged files into place. Existing copies are backed up first.
func (t *Txn) Commit() error {
	if t == nil || t.rolled || t.finished {
		return errors.New("安装已经结束")
	}
	if t.committed {
		return nil
	}
	var staged []string
	for _, name := range AllowedNames() {
		if _, err := os.Stat(filepath.Join(t.staging, name)); err == nil {
			staged = append(staged, name)
		}
	}
	if len(staged) == 0 {
		return errors.New("没有可安装的程序文件")
	}
	if err := os.MkdirAll(t.backup, 0o755); err != nil {
		return errors.New("无法准备安装备份")
	}
	for _, name := range staged {
		if err := t.swap(name); err != nil {
			_ = t.Rollback()
			return err
		}
	}
	t.committed = true
	return nil
}

func (t *Txn) swap(name string) error {
	dest := filepath.Join(t.dir, name)
	bak := filepath.Join(t.backup, name)
	src := filepath.Join(t.staging, name)
	if _, err := os.Stat(dest); err == nil {
		if err := os.Rename(dest, bak); err != nil {
			return ReplaceBusyError(name)
		}
		t.replaced = append(t.replaced, name)
	} else if !os.IsNotExist(err) {
		return ReplaceBusyError(name)
	} else {
		t.created = append(t.created, name)
	}
	if err := os.Rename(src, dest); err != nil {
		return fmt.Errorf("无法写入 %s，安装已撤销：%w", name, err)
	}
	return nil
}

// Rollback restores replaced files and removes files this attempt created.
func (t *Txn) Rollback() error {
	if t == nil || t.rolled || t.finished {
		return nil
	}
	t.rolled = true
	for _, name := range t.replaced {
		if err := allowedName(name); err != nil {
			continue
		}
		dest := filepath.Join(t.dir, name)
		bak := filepath.Join(t.backup, name)
		_ = os.Remove(dest)
		if _, err := os.Stat(bak); err == nil {
			_ = os.Rename(bak, dest)
		}
	}
	for _, name := range t.created {
		if err := allowedName(name); err != nil {
			continue
		}
		_ = os.Remove(filepath.Join(t.dir, name))
	}
	_ = clearOwned(t.staging)
	_ = clearOwned(t.backup)
	if t.createdDir {
		_ = os.Remove(t.dir)
	}
	return nil
}

// Cleanup drops backups after a finished install.
func (t *Txn) Cleanup() {
	if t == nil || t.rolled {
		return
	}
	t.finished = true
	_ = clearOwned(t.staging)
	_ = clearOwned(t.backup)
}

func clearOwned(dir string) error {
	if dir == "" {
		return nil
	}
	for _, name := range AllowedNames() {
		_ = os.Remove(filepath.Join(dir, name))
	}
	return os.Remove(dir)
}
