// Package store 负责 BuddySwitch 自身的状态持久化。
//
// 与「Agent 原生配置」明确区分：本包只写 data/state.json，
// 记录插件的启停、没有原生开关的模型的启停，以及最近一次探测结果。
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"buddyswitch/plugin"

	"buddyswitch/global"
)

// Data 是状态文件的结构。
type Data struct {
	Plugins map[string]bool               `json:"plugins,omitempty"`
	Models  map[string]bool               `json:"models,omitempty"`
	Probes  map[string]plugin.ProbeResult `json:"probes,omitempty"`
}

// Store 是状态文件的内存镜像，所有修改会立即落盘。
type Store struct {
	mu   sync.Mutex
	file string
	data Data
}

// Open 读取状态文件。文件缺失或损坏时返回一个空状态——状态只是辅助信息，
// 不应因为读不到而阻塞应用启动。
func Open(file string) *Store {
	s := &Store{file: file, data: Data{
		Plugins: map[string]bool{},
		Models:  map[string]bool{},
		Probes:  map[string]plugin.ProbeResult{},
	}}
	if data, err := os.ReadFile(file); err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &s.data)
	}
	s.normalize()
	return s
}

func (s *Store) normalize() {
	if s.data.Plugins == nil {
		s.data.Plugins = map[string]bool{}
	}
	if s.data.Models == nil {
		s.data.Models = map[string]bool{}
	}
	if s.data.Probes == nil {
		s.data.Probes = map[string]plugin.ProbeResult{}
	}
}

// File 返回状态文件路径。
func (s *Store) File() string { return s.file }

// ModelKey 生成模型状态键。
func ModelKey(pluginID, modelID string) string {
	return pluginID + "/" + modelID
}

// PluginEnabled 返回插件的生效启停状态，未记录时使用 def 作为缺省值。
func (s *Store) PluginEnabled(pluginID string, def bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.data.Plugins[pluginID]; ok {
		return v
	}
	return def
}

// SetPluginEnabled 记录插件启停状态。
func (s *Store) SetPluginEnabled(pluginID string, on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Plugins[pluginID] = on
	return s.saveLocked()
}

// ModelEnabled 返回模型是否有本地启停覆盖。
func (s *Store) ModelEnabled(key string) (bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data.Models[key]
	return v, ok
}

// SetModelEnabled 记录模型启停状态。
func (s *Store) SetModelEnabled(key string, on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Models[key] = on
	return s.saveLocked()
}

// Probe 返回最近一次探测结果。
func (s *Store) Probe(key string) *plugin.ProbeResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.data.Probes[key]; ok {
		copied := r
		return &copied
	}
	return nil
}

// SetProbe 记录探测结果。
func (s *Store) SetProbe(key string, r plugin.ProbeResult) error {
	return s.SetProbes(map[string]plugin.ProbeResult{key: r})
}

// SetProbes 一次性记录一批探测结果，整批只写一次盘。
//
// 批量测试会并发产出几十上百条结果，逐条落盘既慢（第 k 次写盘要重新序列化
// 前 k-1 条）又没必要——探测结果丢一次无所谓，重新测就是了。
func (s *Store) SetProbes(results map[string]plugin.ProbeResult) error {
	if len(results) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, r := range results {
		s.data.Probes[key] = r
	}
	return s.saveLocked()
}

// Save 把状态写回磁盘。
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

// saveLocked 在持有锁的前提下写盘。
//
// 序列化必须留在锁内：`snapshot := s.data` 只是浅拷贝，map 字段拿到的仍是
// 同一份底层数据，出锁再 Marshal 就等于让 json 去读别的 goroutine 正在写的
// map——Go 会直接以 fatal error 结束进程（不可 recover）。
// 写盘是本地小文件，串行的代价远小于崩溃。
func (s *Store) saveLocked() error {
	file := s.file

	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o644); err != nil {
		return fmt.Errorf("%s: %w", global.T("err_write_state_failed"), err)
	}
	return os.Rename(tmp, file)
}
