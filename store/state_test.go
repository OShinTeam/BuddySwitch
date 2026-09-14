package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"buddyswitch/plugin"
)

func newStore(t *testing.T) (*Store, string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "state.json")
	return Open(file), file
}

func TestModelKey(t *testing.T) {
	if got := ModelKey("demo-agent", "demo-model"); got != "demo-agent/demo-model" {
		t.Fatalf("ModelKey 与前端 probeKey 必须保持一致，得到 %q", got)
	}
}

func TestOpenMissingAndCorrupt(t *testing.T) {
	file := filepath.Join(t.TempDir(), "state.json")
	s := Open(file)
	if s.PluginEnabled("demo-agent", true) != true {
		t.Fatal("没有记录时应当采用定义里的缺省值")
	}
	if _, ok := s.ModelEnabled("demo-agent/m1"); ok {
		t.Fatal("空状态不该有模型覆盖")
	}
	if s.Probe("demo-agent/m1") != nil {
		t.Fatal("空状态不该有探测结果")
	}

	if err := os.WriteFile(file, []byte("{ 坏掉的 JSON"), 0o644); err != nil {
		t.Fatal(err)
	}
	bad := Open(file)
	// 状态只是辅助信息，读坏了也不该阻断启动。
	if err := bad.SetPluginEnabled("demo-agent", false); err != nil {
		t.Fatalf("损坏状态文件后仍然应该可以写: %v", err)
	}
}

func TestPluginAndModelEnabledRoundTrip(t *testing.T) {
	s, file := newStore(t)

	if err := s.SetPluginEnabled("demo-agent", false); err != nil {
		t.Fatal(err)
	}
	if s.PluginEnabled("demo-agent", true) {
		t.Fatal("停用状态没生效")
	}
	if err := s.SetModelEnabled(ModelKey("demo-agent", "m1"), true); err != nil {
		t.Fatal(err)
	}
	if v, ok := s.ModelEnabled("demo-agent/m1"); !ok || !v {
		t.Fatal("模型启停没生效")
	}

	// 重新打开应当读回同样的内容。
	again := Open(file)
	if again.PluginEnabled("demo-agent", true) {
		t.Fatal("插件状态没有落盘")
	}
	if v, ok := again.ModelEnabled("demo-agent/m1"); !ok || !v {
		t.Fatal("模型状态没有落盘")
	}
}

func TestProbeResults(t *testing.T) {
	s, file := newStore(t)

	want := plugin.ProbeResult{Status: plugin.ProbeOK, StatusCode: 200, LatencyMs: 42, Message: "ok"}
	if err := s.SetProbe("demo-agent/m1", want); err != nil {
		t.Fatal(err)
	}
	got := s.Probe("demo-agent/m1")
	if got == nil || got.Status != plugin.ProbeOK || got.LatencyMs != 42 {
		t.Fatalf("探测结果没读回来: %+v", got)
	}

	// 返回的必须是副本，改它不该影响存储。
	got.Status = "被改坏了"
	if again := s.Probe("demo-agent/m1"); again.Status != plugin.ProbeOK {
		t.Fatal("Probe 应当返回副本")
	}

	again := Open(file)
	if r := again.Probe("demo-agent/m1"); r == nil || r.Status != plugin.ProbeOK {
		t.Fatal("探测结果没有落盘")
	}
}

func TestSetProbesWritesOnce(t *testing.T) {
	s, file := newStore(t)

	batch := map[string]plugin.ProbeResult{
		"demo-agent/m1": {Status: plugin.ProbeOK},
		"demo-agent/m2": {Status: plugin.ProbeFail},
		"demo-agent/m3": {Status: plugin.ProbeAuth},
	}
	if err := s.SetProbes(batch); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var parsed Data
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("状态文件应当是合法 JSON: %v", err)
	}
	if len(parsed.Probes) != 3 {
		t.Fatalf("三条结果应当一次写完，实际 %d 条", len(parsed.Probes))
	}

	// 空批次不该产生写盘动作。
	before, _ := os.Stat(file)
	if err := s.SetProbes(nil); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(file)
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("空批次不该写盘")
	}
}

// TestConcurrentSetProbes 是并发崩溃缺陷的回归测试。
//
// 之前的 Save 会在锁外序列化共享的 map，批量探测的多个 worker 一起写结果时
// 会让 Go 以 fatal error 结束进程。这个用例在修复前必崩。
func TestConcurrentSetProbes(t *testing.T) {
	s, file := newStore(t)

	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				key := ModelKey("demo-agent", string(rune('a'+w))+string(rune('0'+i%10)))
				if err := s.SetProbe(key, plugin.ProbeResult{Status: plugin.ProbeOK, LatencyMs: int64(i)}); err != nil {
					t.Errorf("并发写探测结果失败: %v", err)
					return
				}
			}
		}(w)
	}
	// 同时混入读操作，没有任何一对读写应当互相踩到。
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				s.Probe("demo-agent/m1")
				s.ModelEnabled("demo-agent/m1")
				s.PluginEnabled("demo-agent", true)
			}
		}()
	}
	wg.Wait()

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var parsed Data
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("并发写入后状态文件必须是完整的 JSON（不能是半截）: %v", err)
	}
	if len(parsed.Probes) == 0 {
		t.Fatal("并发写入的结果不该丢光")
	}
}

// TestConcurrentSettersKeepAllKeys 保证不同键的并发写入不会互相覆盖。
func TestConcurrentSettersKeepAllKeys(t *testing.T) {
	s, file := newStore(t)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				if err := s.SetModelEnabled(ModelKey("demo-agent", string(rune('a'+i))+string(rune('0'+j))), true); err != nil {
					t.Errorf("写失败: %v", err)
					return
				}
			}
		}(i)
	}
	wg.Wait()

	again := Open(file)
	if len(again.data.Models) != 8*10 {
		t.Fatalf("期望 80 条模型状态，实际 %d 条", len(again.data.Models))
	}
}

func TestSaveCreatesParentDir(t *testing.T) {
	file := filepath.Join(t.TempDir(), "nested", "deeper", "state.json")
	s := Open(file)
	if err := s.SetPluginEnabled("demo-agent", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("应当自动创建父目录: %v", err)
	}
}
