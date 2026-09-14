package upstream

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func newCatalog(t *testing.T) *Catalog {
	t.Helper()
	return Open(filepath.Join(t.TempDir(), "upstreams.json"))
}

func TestIDForIsStableAndKeySensitive(t *testing.T) {
	a := IDFor("https://api.example.invalid/v1", "key-a")
	b := IDFor("https://api.example.invalid/v1", "key-a")
	c := IDFor("https://api.example.invalid/v2", "key-a")
	d := IDFor("https://api.example.invalid/v1", "key-b")

	if a != b {
		t.Fatal("同一接入点与密钥应当派生出同一个 id")
	}
	if a == c || a == d {
		t.Fatal("接入点或密钥不同时 id 必须不同")
	}
	if len(a) == 0 || a[len(a)-1] == '-' {
		t.Fatalf("id 不该是空的或以下划线/横线收尾: %q", a)
	}
	if got := IDFor("   ", ""); got == "" {
		t.Fatal("接入点为空也应当得到一个可用的 id")
	}
}

func TestNameForAndSlugify(t *testing.T) {
	cases := map[string]string{
		"https://api.example.invalid/v1":  "api.example.invalid/v1",
		"https://api.example.invalid/v1/": "api.example.invalid/v1",
		"https://api.example.invalid":     "api.example.invalid",
		"https://api.example.invalid/":    "api.example.invalid",
		"":                                "",
	}
	for in, want := range cases {
		if got := NameFor(in); got != want {
			t.Errorf("NameFor(%q) = %q, 期望 %q", in, got, want)
		}
	}

	slugCases := map[string]string{
		"API.Example.INVALID": "api-example-invalid",
		"a b":                 "a-b",
		"---a---":             "a",
		"中文":                  "",
	}
	for in, want := range slugCases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, 期望 %q", in, got, want)
		}
	}
}

func TestPutGetDelete(t *testing.T) {
	c := newCatalog(t)

	if _, err := c.Put(Upstream{URL: "   "}); err == nil {
		t.Fatal("接入点为空应当被拒绝")
	}

	saved, err := c.Put(Upstream{
		URL:    "https://api.example.invalid/v1",
		APIKey: "sk-demo",
		Models: []Model{{ID: "m1", DisplayName: "一号"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == "" || saved.Name == "" || saved.UpdatedAt == "" {
		t.Fatalf("Put 应当补全 id / name / 时间戳: %+v", saved)
	}

	got, ok := c.Get(saved.ID)
	if !ok || got.ID != saved.ID {
		t.Fatalf("写入后应当能读回: %+v %v", got, ok)
	}

	// 用户手动起名 + 备注，重新 Put 时要保留。
	got.Name = "我自己的名字"
	got.Notes = "备注"
	if _, err := c.Put(got); err != nil {
		t.Fatal(err)
	}
	again, _ := c.Get(saved.ID)
	if again.Name != "我自己的名字" || again.Notes != "备注" {
		t.Fatalf("用户的命名与备注不该被冲掉: %+v", again)
	}
	if n := len(c.List()); n != 1 {
		t.Fatalf("同一个 id 更新不该产生第二条，实际 %d 条", n)
	}

	removed, err := c.Delete(saved.ID)
	if err != nil || !removed {
		t.Fatalf("删除失败: %v %v", removed, err)
	}
	if removed, _ = c.Delete(saved.ID); removed {
		t.Fatal("重复删除不该报告成功")
	}
}

// TestGetAndListReturnDeepCopies 保证调用方改不到目录内部的数据。
func TestGetAndListReturnDeepCopies(t *testing.T) {
	c := newCatalog(t)
	if _, err := c.Put(Upstream{
		URL:    "https://api.example.invalid/v1",
		APIKey: "sk-demo",
		Models: []Model{{ID: "m1", Capabilities: map[string]bool{"tool_call": true}}},
	}); err != nil {
		t.Fatal(err)
	}

	item := c.List()[0]
	item.Models[0].ID = "被改坏了"
	item.Models[0].Capabilities["tool_call"] = false

	got, _ := c.Get(item.ID)
	if got.Models[0].ID != "m1" {
		t.Fatalf("List 返回的切片不该与目录共享底层数组: %+v", got.Models)
	}
	if !got.Models[0].Capabilities["tool_call"] {
		t.Fatalf("List 返回的能力映射也不该被共享: %+v", got.Models[0].Capabilities)
	}
}

func TestMerge(t *testing.T) {
	c := newCatalog(t)
	discovered := []Upstream{{
		ID:      "up-demo",
		Name:    "自动发现的名字",
		Vendor:  "Demo",
		URL:     "https://api.example.invalid/v1",
		APIKey:  "sk-demo",
		Origins: []string{"demo-agent"},
		Models:  []Model{{ID: "m1"}, {ID: "m2"}},
	}}

	added, updated, err := c.Merge(discovered)
	if err != nil {
		t.Fatal(err)
	}
	if added != 1 || updated != 0 {
		t.Fatalf("首次合并应当新增 1、更新 0，得到 %d/%d", added, updated)
	}

	// 再合一次：内容没变，就不该报告「更新」。
	added, updated, err = c.Merge(discovered)
	if err != nil {
		t.Fatal(err)
	}
	if added != 0 || updated != 0 {
		t.Fatalf("内容未变时不该报告变更，得到 %d/%d", added, updated)
	}

	// 清单里多了一个模型：这才是真的更新。
	more := discovered
	more[0].Models = []Model{{ID: "m1"}, {ID: "m2"}, {ID: "m3"}}
	added, updated, err = c.Merge(more)
	if err != nil {
		t.Fatal(err)
	}
	if added != 0 || updated != 1 {
		t.Fatalf("清单变化时应当更新 1，得到 %d/%d", added, updated)
	}

	item, _ := c.Get("up-demo")
	if len(item.Models) != 3 {
		t.Fatalf("模型应当合并而不是替换: %+v", item.Models)
	}
	if item.Name != "自动发现的名字" {
		t.Fatalf("首次合并用的是发现到的名字: %q", item.Name)
	}

	// 合并时以人工命名为准。
	if _, err := c.Put(Upstream{ID: "up-demo", Name: "人工命名", URL: item.URL, APIKey: item.APIKey, Models: item.Models}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.Merge(discovered); err != nil {
		t.Fatal(err)
	}
	after, _ := c.Get("up-demo")
	if after.Name != "人工命名" {
		t.Fatalf("自动同步不该冲掉人工起的名字: %q", after.Name)
	}
}

func TestMergeModelsKeepsExistingDetail(t *testing.T) {
	existing := []Model{{ID: "m1", DisplayName: "人工写的显示名", Capabilities: map[string]bool{"images": true}}}
	incoming := []Model{
		{ID: "m1", DisplayName: "上游给的名字", Capabilities: map[string]bool{"tool_call": true}},
		{ID: "m2"},
		{ID: ""},
	}
	out := mergeModels(existing, incoming)
	if len(out) != 2 {
		t.Fatalf("空 id 应当被跳过，得到 %+v", out)
	}
	if out[0].DisplayName != "人工写的显示名" {
		t.Fatalf("已有的显示名不该被覆盖: %+v", out[0])
	}
	if !out[0].Capabilities["images"] {
		t.Fatalf("已有的能力标记不该被覆盖: %+v", out[0])
	}
	if out[1].ID != "m2" {
		t.Fatalf("新模型应当追加: %+v", out)
	}
}

func TestMergeStrings(t *testing.T) {
	got := mergeStrings([]string{"b", "a"}, []string{"a", "c", ""})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("mergeStrings = %v, 期望 %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("mergeStrings = %v, 期望 %v（应当去重且排序）", got, want)
		}
	}
}

// TestConcurrentAccessIsSafe 是并发缺陷的回归测试：
// 之前 Save 会在锁外序列化共享的 entry 切片，与 Put/List 撞在一起。
func TestConcurrentAccessIsSafe(t *testing.T) {
	c := newCatalog(t)
	if _, err := c.Put(Upstream{ID: "up-demo", URL: "https://api.example.invalid/v1", APIKey: "k"}); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				item, ok := c.Get("up-demo")
				if !ok {
					t.Errorf("读不到条目")
					return
				}
				item.Models = append(item.Models, Model{ID: "m"})
				if _, err := c.Put(item); err != nil {
					t.Errorf("Put 失败: %v", err)
					return
				}
				c.List()
				c.Merge([]Upstream{{ID: "up-demo", URL: item.URL, APIKey: item.APIKey, Models: []Model{{ID: "x"}}}})
				c.Save()
			}
		}(i)
	}
	wg.Wait()
}

func TestCorruptFileDegrades(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "upstreams.json")
	if err := os.WriteFile(file, []byte("{ 这不是合法 JSON"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := Open(file)
	if len(c.List()) != 0 {
		t.Fatal("文件损坏时应当退化成空目录，而不是崩掉")
	}
}
