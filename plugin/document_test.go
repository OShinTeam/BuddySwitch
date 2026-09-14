package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testDef 是一份覆盖尽可能多映射的演示定义。
// 它用的是 demo 名字，与任何真实 agent 无关。
func testDef(t *testing.T) *Definition {
	t.Helper()
	def := &Definition{
		ID:   "demo-agent",
		Name: "Demo Agent",
		Source: SourceSpec{
			Format: "json",
			Paths:  map[string][]string{"default": {"models.json"}},
		},
		Schema: SchemaSpec{
			Container: ContainerList{"", "models"},
			Fields: map[string]string{
				FieldID:          "id",
				FieldDisplayName: "name",
				FieldProvider:    "vendor",
				FieldBaseURL:     "url",
				FieldAPIKey:      "apiKey",
				FieldEnabled:     "enabled",
				FieldDescription: "description",
				FieldTags:        "tags",
			},
			Capabilities: map[string]string{"tool_call": "supportsToolCall"},
			SyncList:     "availableModels",
		},
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("演示定义本身应当合法: %v", err)
	}
	return def
}

func newDoc(t *testing.T, content string) (*Definition, *Document, string) {
	t.Helper()
	def := testDef(t)
	file := filepath.Join(t.TempDir(), "models.json")
	if content != "" {
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	doc, err := LoadDocument(def, file)
	if err != nil {
		t.Fatalf("加载文档失败: %v", err)
	}
	return def, doc, file
}

func TestSplitPath(t *testing.T) {
	cases := map[string][]string{
		"":            nil,
		"id":          {"id"},
		"a.b.c":       {"a", "b", "c"},
		" a . b ":     {"a", "b"},
		"a..b":        {"a", "b"},
		"reasoning.x": {"reasoning", "x"},
	}
	for in, want := range cases {
		got := splitPath(in)
		if len(got) != len(want) {
			t.Fatalf("splitPath(%q) = %v, 期望 %v", in, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("splitPath(%q) = %v, 期望 %v", in, got, want)
			}
		}
	}
}

func TestGetSetByPath(t *testing.T) {
	root := map[string]any{}
	setByPath(root, "reasoning.effort", "high")
	setByPath(root, "id", "m1")

	if v, ok := getByPath(root, "reasoning.effort"); !ok || toString(v) != "high" {
		t.Fatalf("嵌套写入后读不回来: %v %v", v, ok)
	}
	if _, ok := getByPath(root, "reasoning.missing"); ok {
		t.Fatal("不存在的路径不该报告存在")
	}
	if _, ok := getByPath(root, "id.deeper"); ok {
		t.Fatal("穿过非对象节点应当返回 false")
	}
}

func TestStripJSONC(t *testing.T) {
	in := `{
  // 行注释
  "url": "https://api.example.invalid/v1", /* 块注释 */
  "note": "文本里的 // 和 /* 都不该被当注释",
  "n": 1
}`
	out := string(stripJSONC([]byte(in)))
	if strings.Contains(out, "行注释") || strings.Contains(out, "块注释") {
		t.Fatalf("注释没被剥干净:\n%s", out)
	}
	if !strings.Contains(out, "https://api.example.invalid/v1") {
		t.Fatalf("字符串里的 // 被误删:\n%s", out)
	}
	if !strings.Contains(out, "文本里的 // 和 /* 都不该被当注释") {
		t.Fatalf("字符串里的注释符号被误删:\n%s", out)
	}
}

func TestExpandPath(t *testing.T) {
	t.Setenv("DEMO_HOME", "/tmp/demo")
	cases := map[string]string{
		"%DEMO_HOME%/models.json": "/tmp/demo/models.json",
		"$DEMO_HOME/models.json":  "/tmp/demo/models.json",
		"${DEMO_HOME}/a.json":     "/tmp/demo/a.json",
		"%NOT_DEFINED%/a.json":    "%NOT_DEFINED%/a.json", // 未定义变量原样保留
		"":                        "",
	}
	for in, want := range cases {
		if got := ExpandPath(in); got != want {
			t.Errorf("ExpandPath(%q) = %q, 期望 %q", in, got, want)
		}
	}
}

func TestContainerCandidates(t *testing.T) {
	t.Run("根节点即数组", func(t *testing.T) {
		_, doc, _ := newDoc(t, `[{"id":"m1"}]`)
		models, err := doc.Models()
		if err != nil {
			t.Fatal(err)
		}
		if len(models) != 1 || models[0].ID != "m1" {
			t.Fatalf("解析结果不对: %+v", models)
		}
	})

	t.Run("models 键", func(t *testing.T) {
		_, doc, _ := newDoc(t, `{"models":[{"id":"m2","name":"二号"}]}`)
		models, err := doc.Models()
		if err != nil {
			t.Fatal(err)
		}
		if len(models) != 1 || models[0].ID != "m2" || models[0].DisplayName != "二号" {
			t.Fatalf("解析结果不对: %+v", models)
		}
	})

	t.Run("两种结构都不匹配时应当报错", func(t *testing.T) {
		_, doc, _ := newDoc(t, `{"other":[{"id":"m3"}]}`)
		if _, err := doc.Models(); err == nil {
			t.Fatal("结构对不上时应当报错，而不是静默返回空列表")
		}
	})

	t.Run("文件不存在时按首选结构新建", func(t *testing.T) {
		_, doc, file := newDoc(t, "")
		if err := doc.Upsert(Model{ID: "m4"}, true); err != nil {
			t.Fatal(err)
		}
		if err := doc.Save(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(strings.TrimSpace(string(data)), "[") {
			t.Fatalf("首选容器是根数组，应当写出数组: %s", data)
		}
	})
}

// TestUpsertReplaceSemantics 覆盖「清空标签保存无效」这个曾经的真实缺陷。
func TestUpsertReplaceSemantics(t *testing.T) {
	seed := `{"models":[{"id":"m1","name":"一号","tags":["old"],"description":"旧备注","vendor":"Demo"}]}`

	t.Run("编辑器保存：空标签就是清空", func(t *testing.T) {
		_, doc, file := newDoc(t, seed)
		if err := doc.Upsert(Model{ID: "m1", DisplayName: "一号", Tags: []string{}}, true); err != nil {
			t.Fatal(err)
		}
		if err := doc.Save(); err != nil {
			t.Fatal(err)
		}
		raw := readFile(t, file)
		if !strings.Contains(raw, `"tags": []`) {
			t.Fatalf("编辑器清空标签后应当写成空数组:\n%s", raw)
		}
	})

	t.Run("从缓存搬运：不碰对方的标签与备注", func(t *testing.T) {
		_, doc, file := newDoc(t, seed)
		// 搬运方只带 id / name，对 tags、description 没有意见。
		if err := doc.Upsert(Model{ID: "m1", DisplayName: "一号"}, false); err != nil {
			t.Fatal(err)
		}
		if err := doc.Save(); err != nil {
			t.Fatal(err)
		}
		raw := readFile(t, file)
		if !strings.Contains(raw, `"old"`) {
			t.Fatalf("合并语义不该抹掉原有标签:\n%s", raw)
		}
		if !strings.Contains(raw, `"旧备注"`) {
			t.Fatalf("合并语义不该抹掉原有备注:\n%s", raw)
		}
	})

	t.Run("合并语义不写入空字符串", func(t *testing.T) {
		_, doc, file := newDoc(t, seed)
		if err := doc.Upsert(Model{ID: "m1", DisplayName: "一号", Provider: ""}, false); err != nil {
			t.Fatal(err)
		}
		if err := doc.Save(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(readFile(t, file), `"vendor": "Demo"`) {
			t.Fatalf("没有意见的字段不该被清空:\n%s", readFile(t, file))
		}
	})
}

func TestUpsertAndRemoveKeepSyncList(t *testing.T) {
	_, doc, file := newDoc(t, `{"models":[],"availableModels":["keep-me"]}`)

	if err := doc.Upsert(Model{ID: "m1", DisplayName: "一号"}, true); err != nil {
		t.Fatal(err)
	}
	if err := doc.Save(); err != nil {
		t.Fatal(err)
	}
	raw := readFile(t, file)
	for _, want := range []string{`"keep-me"`, `"m1"`} {
		if !strings.Contains(raw, want) {
			t.Fatalf("新增模型后 availableModels 应当同时保留旧值与新 id:\n%s", raw)
		}
	}

	doc2, err := LoadDocument(testDef(t), file)
	if err != nil {
		t.Fatal(err)
	}
	removed, err := doc2.Remove("m1")
	if err != nil || !removed {
		t.Fatalf("删除失败: removed=%v err=%v", removed, err)
	}
	if err := doc2.Save(); err != nil {
		t.Fatal(err)
	}
	raw = readFile(t, file)
	if strings.Contains(raw, `"m1"`) {
		t.Fatalf("删除模型后应当把 id 从 availableModels 摘掉:\n%s", raw)
	}
	if !strings.Contains(raw, `"keep-me"`) {
		t.Fatalf("只该摘掉被删的 id，不该清空整个清单:\n%s", raw)
	}
}

func TestUpsertKeepsUnknownFields(t *testing.T) {
	_, doc, file := newDoc(t, `{"models":[{"id":"m1","name":"一号","reasoning":{"effort":"high"},"maxInputTokens":8192}]}`)
	if err := doc.Upsert(Model{ID: "m1", DisplayName: "改名了"}, true); err != nil {
		t.Fatal(err)
	}
	if err := doc.Save(); err != nil {
		t.Fatal(err)
	}
	raw := readFile(t, file)
	for _, want := range []string{`"maxInputTokens": 8192`, `"effort": "high"`, `"改名了"`} {
		if !strings.Contains(raw, want) {
			t.Fatalf("未被映射的字段必须原样保留（缺 %s）:\n%s", want, raw)
		}
	}
}

func TestRemoveMissing(t *testing.T) {
	_, doc, _ := newDoc(t, `[{"id":"m1"}]`)
	removed, err := doc.Remove("nope")
	if err != nil {
		t.Fatal(err)
	}
	if removed {
		t.Fatal("删一个不存在的模型不该报告删除成功")
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name string
		def  Definition
		ok   bool
	}{
		{"缺 id", Definition{Name: "x"}, false},
		{"缺 name", Definition{ID: "x"}, false},
		{"缺 source.paths", Definition{ID: "x", Name: "x"}, false},
		{"缺 schema.fields", Definition{ID: "x", Name: "x", Source: SourceSpec{Paths: map[string][]string{"default": {"a"}}}}, false},
		{"缺必需字段 id", Definition{ID: "x", Name: "x", Source: SourceSpec{Paths: map[string][]string{"default": {"a"}}}, Schema: SchemaSpec{Fields: map[string]string{"provider": "p"}}}, false},
		{"含未知字段", Definition{ID: "x", Name: "x", Source: SourceSpec{Paths: map[string][]string{"default": {"a"}}}, Schema: SchemaSpec{Fields: map[string]string{"id": "id", "whatever": "w"}}}, false},
		{"格式不支持", Definition{ID: "x", Name: "x", Source: SourceSpec{Paths: map[string][]string{"default": {"a"}}, Format: "yaml"}, Schema: SchemaSpec{Fields: map[string]string{"id": "id"}}}, false},
		{"合法", Definition{ID: "x", Name: "x", Source: SourceSpec{Paths: map[string][]string{"default": {"a"}}}, Schema: SchemaSpec{Fields: map[string]string{"id": "id"}}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.def.Validate()
			if c.ok && err != nil {
				t.Fatalf("应当通过，却报 %v", err)
			}
			if !c.ok && err == nil {
				t.Fatal("应当报错，却通过了")
			}
		})
	}
}

func TestMappedFieldKeysSkipsEmptyPath(t *testing.T) {
	def := &Definition{Schema: SchemaSpec{Fields: map[string]string{
		FieldID: "id", FieldTags: "tags", FieldDescription: "",
	}}}
	got := def.MappedFieldKeys()
	want := []string{FieldID, FieldTags}
	if len(got) != len(want) {
		t.Fatalf("MappedFieldKeys() = %v, 期望 %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("MappedFieldKeys() = %v, 期望 %v", got, want)
		}
	}
}

func TestContainerListJSONShape(t *testing.T) {
	var one ContainerList
	if err := one.UnmarshalJSON([]byte(`"models"`)); err != nil {
		t.Fatal(err)
	}
	if len(one) != 1 || one[0] != "models" {
		t.Fatalf("字符串写法应当解析成单元素列表: %v", one)
	}

	var many ContainerList
	if err := many.UnmarshalJSON([]byte(`["", "models"]`)); err != nil {
		t.Fatal(err)
	}
	if len(many) != 2 {
		t.Fatalf("数组写法解析错误: %v", many)
	}

	out, err := many.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `["","models"]` {
		t.Fatalf("多元素应当写回数组，得到 %s", out)
	}
	if out, err = one.MarshalJSON(); err != nil || string(out) != `"models"` {
		t.Fatalf("单元素应当写回字符串，得到 %s (%v)", out, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
