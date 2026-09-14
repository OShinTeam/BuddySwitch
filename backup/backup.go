// Package backup 为 Agent 原生模型配置文件提供快照、轮转与还原能力。
//
// 每次写回原生配置之前都会先留一份快照，快照按时间戳命名并保存在
// data/backups/<插件id>/ 下，超出保留份数的旧快照会被自动清理。
// 用户因此随时可以一键回到任意一次修改之前的状态。
package backup

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"buddyswitch/global"
)

// 默认保留份数与允许的取值区间。
const (
	DefaultKeep = 10
	MinKeep     = 1
	MaxKeep     = 100
)

// Entry 描述一份快照。
type Entry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
	Source  string `json:"source"`
}

// ClampKeep 把用户设置的保留份数收敛到合法区间。
func ClampKeep(keep int) int {
	if keep <= 0 {
		return DefaultKeep
	}
	if keep < MinKeep {
		return MinKeep
	}
	if keep > MaxKeep {
		return MaxKeep
	}
	return keep
}

// Dir 返回某个插件的快照目录。
func Dir(root, pluginID string) string {
	return filepath.Join(root, pluginID)
}

// Capture 在写回之前为 srcFile 留一份快照，并按 keep 清理旧快照。
// 返回快照路径；源文件不存在或为空时不产生快照，返回空字符串。
func Capture(srcFile, root, pluginID string, keep int) (string, error) {
	data, err := os.ReadFile(srcFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return "", nil
	}

	dir := Dir(root, pluginID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("%s: %w", global.T("err_create_backup_dir"), err)
	}

	name := fmt.Sprintf("%s.%s.bak", filepath.Base(srcFile), time.Now().Format("2006-01-02_15-04-05.000"))
	dest := filepath.Join(dir, name)
	if err := writeFile(dest, data); err != nil {
		return "", err
	}

	if _, err := Prune(dir, keep); err != nil {
		// 清理旧快照失败不影响这一份的可用性，记一笔就够了。
		global.Log.Warnf("清理旧快照失败: %v", err)
	}
	return dest, nil
}

// writeFile 以原子方式写入文件。
func writeFile(dest string, data []byte) error {
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

// List 返回某个快照目录下的全部快照，最新的排在前面。
func List(dir string) ([]Entry, error) {
	items, err := snapshots(dir)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(items))
	for _, item := range items {
		out = append(out, Entry{
			Name:    item.name,
			Path:    filepath.Join(dir, item.name),
			Size:    item.size,
			ModTime: item.modTime.Format(time.DateTime),
			Source:  sourceOf(item.name),
		})
	}
	return out, nil
}

type snapshot struct {
	name    string
	size    int64
	modTime time.Time
}

// snapshots 读取目录内的快照并按修改时间倒序返回（最新在前）。
// 排序用修改时间而不是文件名，这样即便同一目录里混着不同源文件的快照也不会错乱。
func snapshots(dir string) ([]snapshot, error) {
	items, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []snapshot{}, nil
		}
		return nil, err
	}
	out := make([]snapshot, 0, len(items))
	for _, item := range items {
		if item.IsDir() || !strings.HasSuffix(item.Name(), ".bak") {
			continue
		}
		info, err := item.Info()
		if err != nil {
			continue
		}
		out = append(out, snapshot{name: item.Name(), size: info.Size(), modTime: info.ModTime()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].modTime.After(out[j].modTime) })
	return out, nil
}

// sourceOf 从快照文件名还原出它对应的原始配置文件名。
// 命名规则是 <原始文件名>.<时间戳>.bak，时间戳本身含毫秒点，所以要摘掉末尾两段。
func sourceOf(name string) string {
	name = strings.TrimSuffix(name, ".bak")
	parts := strings.Split(name, ".")
	if len(parts) <= 2 {
		return name
	}
	return strings.Join(parts[:len(parts)-2], ".")
}

// Prune 只保留最新的 keep 份快照，返回被删除的数量。
func Prune(dir string, keep int) (int, error) {
	keep = ClampKeep(keep)
	items, err := snapshots(dir)
	if err != nil {
		return 0, err
	}
	if len(items) <= keep {
		return 0, nil
	}

	removed := 0
	for _, item := range items[keep:] {
		if err := os.Remove(filepath.Join(dir, item.name)); err == nil {
			removed++
		}
	}
	return removed, nil
}

// Restore 用快照覆盖目标配置文件。覆盖前会先把当前内容再存一份快照，
// 因此「还原」这个动作本身也是可以撤销的。
func Restore(backupPath, destFile, root, pluginID string, keep int) error {
	if !strings.HasSuffix(backupPath, ".bak") {
		return fmt.Errorf("%s", global.T("err_invalid_backup_file", backupPath))
	}
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("%s: %w", global.T("err_read_backup_failed"), err)
	}
	if _, err := Capture(destFile, root, pluginID, keep); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destFile), 0o755); err != nil {
		return err
	}
	return writeFile(destFile, data)
}

// Clear 删除某个快照目录下的全部快照，返回删除数量。
func Clear(dir string) (int, error) {
	items, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	removed := 0
	for _, item := range items {
		if item.IsDir() {
			continue
		}
		if err := os.Remove(filepath.Join(dir, item.Name())); err == nil {
			removed++
		}
	}
	return removed, nil
}

// Read 读取快照内容，供界面预览。
func Read(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 512*1024))
	if err != nil {
		return "", err
	}
	return string(data), nil
}
