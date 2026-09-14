// Package upstream 维护 BuddySwitch 自己的「上游目录」。
//
// 上游 = 一组模型共用的接入点与凭据（url + apiKey）。同一个上游下的模型，
// 通常只是模型名不同，接入方式完全一致。
//
// 这个目录是程序级资产：它不依附于某个 agent，因此可以在 agent 之间复用——
// 把 WorkBuddy 里攒好的一批模型，一键写到 CodeBuddy 的配置里。
// 目录持久化在 data/upstreams.json，与各 agent 的原生配置互不干扰。
package upstream

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"buddyswitch/global"
)

// Model 是上游目录里记录的模型条目。
//
// 只保存「跨 agent 通用」的元信息：id、显示名、能力标记。
// 具体的启用状态、备注等属于各 agent 的本地状态，不放进目录。
type Model struct {
	ID           string          `json:"id"`
	DisplayName  string          `json:"display_name,omitempty"`
	Capabilities map[string]bool `json:"capabilities,omitempty"`
}

// Upstream 是一个上游条目。
type Upstream struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Vendor string  `json:"vendor,omitempty"`
	URL    string  `json:"url"`
	APIKey string  `json:"api_key"`
	Notes  string  `json:"notes,omitempty"`
	Models []Model `json:"models"`

	// Origins 记录这个上游最初是从哪些插件发现的，纯信息用途。
	Origins   []string `json:"origins,omitempty"`
	UpdatedAt string   `json:"updated_at,omitempty"`
}

// Catalog 是上游目录的内存镜像，修改后立即落盘。
type Catalog struct {
	mu      sync.Mutex
	file    string
	entries []Upstream
}

type diskFormat struct {
	Version   int        `json:"version"`
	Upstreams []Upstream `json:"upstreams"`
}

// Open 读取目录文件，缺失或损坏时返回空目录。
func Open(file string) *Catalog {
	c := &Catalog{file: file}
	if data, err := os.ReadFile(file); err == nil && len(data) > 0 {
		var parsed diskFormat
		if json.Unmarshal(data, &parsed) == nil {
			c.entries = parsed.Upstreams
		}
	}
	return c
}

// File 返回目录文件路径。
func (c *Catalog) File() string { return c.file }

// List 返回按名称排序的上游条目副本。
//
// 返回的是深拷贝：调用方拿到的切片与目录内部不共享底层数组，
// 于是「改一改再 Put 回去」这类用法不会在锁外碰到目录正在用的数据。
func (c *Catalog) List() []Upstream {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Upstream, len(c.entries))
	for i, u := range c.entries {
		out[i] = cloneUpstream(u)
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].Models) != len(out[j].Models) {
			return len(out[i].Models) > len(out[j].Models)
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Get 按 id 查找，返回深拷贝。
func (c *Catalog) Get(id string) (Upstream, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, u := range c.entries {
		if u.ID == id {
			return cloneUpstream(u), true
		}
	}
	return Upstream{}, false
}

// Put 写入或更新一个上游条目，返回补全后的结果。
func (c *Catalog) Put(u Upstream) (Upstream, error) {
	if strings.TrimSpace(u.URL) == "" {
		return Upstream{}, fmt.Errorf("%s", global.T("err_upstream_url_empty"))
	}
	if u.ID == "" {
		u.ID = IDFor(u.URL, u.APIKey)
	}
	if strings.TrimSpace(u.Name) == "" {
		u.Name = NameFor(u.URL)
	}
	u.UpdatedAt = time.Now().Format(time.RFC3339)

	c.mu.Lock()
	defer c.mu.Unlock()
	replaced := false
	for i := range c.entries {
		if c.entries[i].ID == u.ID {
			c.entries[i] = u
			replaced = true
			break
		}
	}
	if !replaced {
		c.entries = append(c.entries, u)
	}

	return u, c.saveLocked()
}

// Delete 删除一个上游条目。
func (c *Catalog) Delete(id string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	kept := make([]Upstream, 0, len(c.entries))
	removed := false
	for _, u := range c.entries {
		if u.ID == id {
			removed = true
			continue
		}
		kept = append(kept, u)
	}
	if !removed {
		return false, nil
	}
	c.entries = kept
	return true, c.saveLocked()
}

// Merge 把发现到的上游合并进目录，并在同一次加锁里落盘。
//
// 合并规则：按 id（接入点 + 密钥）配对。已存在时只补全模型清单与来源，
// 保留用户自己起的名字和备注——那些是人工投入，不该被自动同步冲掉。
//
// updated 统计的是**真的发生了变化**的条目数，不是配对成功的条目数——
// 否则反复导入同一批上游会一直提示「更新 N 条」，让人以为有东西在变。
func (c *Catalog) Merge(discovered []Upstream) (added int, updated int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, d := range discovered {
		found := false
		for i := range c.entries {
			if c.entries[i].ID != d.ID {
				continue
			}
			found = true
			entry := &c.entries[i]
			beforeModels := len(entry.Models)
			beforeOrigins := len(entry.Origins)
			beforeVendor := entry.Vendor

			entry.Models = mergeModels(entry.Models, d.Models)
			entry.Origins = mergeStrings(entry.Origins, d.Origins)
			if entry.Vendor == "" {
				entry.Vendor = d.Vendor
			}
			if beforeModels != len(entry.Models) ||
				beforeOrigins != len(entry.Origins) ||
				beforeVendor != entry.Vendor {
				entry.UpdatedAt = time.Now().Format(time.RFC3339)
				updated++
			}
			break
		}
		if !found {
			d.UpdatedAt = time.Now().Format(time.RFC3339)
			c.entries = append(c.entries, d)
			added++
		}
	}

	if added == 0 && updated == 0 {
		return 0, 0, nil
	}
	return added, updated, c.saveLocked()
}

// Save 把目录写回磁盘。
func (c *Catalog) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.saveLocked()
}

// saveLocked 在持有锁的前提下写盘。
//
// 与 store.Store 同理：`diskFormat{Upstreams: c.entries}` 复制的是切片头，
// 出锁再 Marshal 就会去读别的 goroutine 正在改的条目。序列化必须锁内完成。
func (c *Catalog) saveLocked() error {
	file := c.file

	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(diskFormat{Version: 1, Upstreams: c.entries}, "", "  ")
	if err != nil {
		return err
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, append(buf, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}

// cloneUpstream 深拷贝一个条目，让调用方拿到的东西与目录内部完全脱钩。
func cloneUpstream(u Upstream) Upstream {
	out := u
	if u.Models != nil {
		out.Models = make([]Model, len(u.Models))
		for i, m := range u.Models {
			m.Capabilities = maps.Clone(m.Capabilities)
			out.Models[i] = m
		}
	}
	out.Origins = append([]string(nil), u.Origins...)
	return out
}

// ---------------------------------------------------------------------------
// 派生与命名
// ---------------------------------------------------------------------------

// IDFor 由接入点与密钥派生稳定 id，同一个上游在任何 agent 上都会得到同一个 id。
func IDFor(rawURL, apiKey string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(rawURL) + "\n" + apiKey))
	slug := slugify(hostOf(rawURL))
	if slug == "" {
		slug = "upstream"
	}
	return fmt.Sprintf("%s-%s", slug, hex.EncodeToString(sum[:])[:6])
}

// NameFor 给出上游的默认名称：主机名 + 路径，便于一眼认出是谁。
func NameFor(rawURL string) string {
	host := hostOf(rawURL)
	if host == "" {
		return rawURL
	}
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return host
	}
	path := strings.TrimSuffix(parsed.Path, "/")
	if path == "" || path == "/" {
		return host
	}
	return host + path
}

func hostOf(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return ""
	}
	return parsed.Host
}

func slugify(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func mergeModels(existing []Model, incoming []Model) []Model {
	index := make(map[string]int, len(existing))
	out := make([]Model, len(existing))
	copy(out, existing)
	for i, m := range out {
		index[m.ID] = i
	}
	for _, m := range incoming {
		if m.ID == "" {
			continue
		}
		if i, ok := index[m.ID]; ok {
			if out[i].DisplayName == "" {
				out[i].DisplayName = m.DisplayName
			}
			if len(out[i].Capabilities) == 0 {
				out[i].Capabilities = m.Capabilities
			}
			continue
		}
		index[m.ID] = len(out)
		out = append(out, m)
	}
	return out
}

func mergeStrings(existing []string, incoming []string) []string {
	seen := make(map[string]bool, len(existing))
	out := make([]string, 0, len(existing)+len(incoming))
	for _, s := range append(append([]string{}, existing...), incoming...) {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
