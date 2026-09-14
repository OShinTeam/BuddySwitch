package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClampKeep(t *testing.T) {
	cases := map[int]int{
		0:    DefaultKeep,
		-3:   DefaultKeep,
		1:    1,
		7:    7,
		100:  100,
		101:  MaxKeep,
		9999: MaxKeep,
	}
	for in, want := range cases {
		if got := ClampKeep(in); got != want {
			t.Errorf("ClampKeep(%d) = %d, 期望 %d", in, got, want)
		}
	}
}

func TestDir(t *testing.T) {
	if got := Dir("data/backups", "demo-agent"); got != filepath.Join("data/backups", "demo-agent") {
		t.Fatalf("Dir = %q", got)
	}
}

func TestSourceOf(t *testing.T) {
	cases := map[string]string{
		"models.json.2026-09-14_22-05-31.417.bak": "models.json",
		"a.b.c.2020-01-01_00-00-00.000.bak":       "a.b.c",
		"models.json.2020-01-01_00-00-00.000":     "models.json",
		"single.bak":                              "single",
		"weird.bak":                               "weird",
	}
	for in, want := range cases {
		if got := sourceOf(in); got != want {
			t.Errorf("sourceOf(%q) = %q, 期望 %q", in, got, want)
		}
	}
}

func TestCaptureSkipsMissingAndEmpty(t *testing.T) {
	root := t.TempDir()
	dir := t.TempDir()

	missing := filepath.Join(dir, "not-there.json")
	path, err := Capture(missing, root, "demo-agent", 5)
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Fatalf("源文件不存在时不该产生快照，得到 %q", path)
	}

	empty := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(empty, []byte("   \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, err = Capture(empty, root, "demo-agent", 5)
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Fatalf("空文件不该产生快照，得到 %q", path)
	}
}

func TestPruneKeepsNewest(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(t.TempDir(), "models.json")
	if err := os.WriteFile(src, []byte(`[{"id":"m1"}]`), 0o644); err != nil {
		t.Fatal(err)
	}

	// 连续留 6 份，保留 3 份。
	for i := 0; i < 6; i++ {
		if _, err := Capture(src, root, "demo-agent", 3); err != nil {
			t.Fatal(err)
		}
		// 让修改时间拉开，Prune 是按时间倒序保留最新的。
		time.Sleep(12 * time.Millisecond)
		os.Chtimes(src, time.Now(), time.Now())
	}

	items, err := List(Dir(root, "demo-agent"))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("应当只保留 3 份，实际 %d 份: %+v", len(items), items)
	}
	for i := 1; i < len(items); i++ {
		if items[i-1].ModTime < items[i].ModTime {
			t.Fatalf("列表应当按时间倒序: %+v", items)
		}
	}
	if items[0].Source != "models.json" {
		t.Fatalf("应当能还原出原始文件名，得到 %q", items[0].Source)
	}
	if items[0].Size == 0 {
		t.Fatal("快照不该是空文件")
	}
}

func TestRestoreIsUndoable(t *testing.T) {
	root := t.TempDir()
	dir := t.TempDir()
	dest := filepath.Join(dir, "models.json")

	original := `[{"id":"m1"}]`
	if err := os.WriteFile(dest, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	snap, err := Capture(dest, root, "demo-agent", 10)
	if err != nil {
		t.Fatal(err)
	}
	if snap == "" {
		t.Fatal("应当留下快照")
	}

	// 之后内容被改坏。
	if err := os.WriteFile(dest, []byte(`[{"id":"坏掉了"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Restore(snap, dest, root, "demo-agent", 10); err != nil {
		t.Fatal(err)
	}
	if got := read(t, dest); got != original {
		t.Fatalf("还原后内容不对: %s", got)
	}

	// 还原本身也要留痕：此时应当有 2 份快照，能再回退到「坏掉」那一版。
	items, err := List(Dir(root, "demo-agent"))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("还原应当先存一份当前内容，快照数 %d", len(items))
	}
}

func TestRestoreRejectsNonSnapshot(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(t.TempDir(), "models.json")
	if err := Restore(filepath.Join(root, "not-a-snapshot.json"), dest, root, "demo-agent", 5); err == nil {
		t.Fatal("非 .bak 文件应当被拒绝")
	}
}

func TestClearAndListOnEmpty(t *testing.T) {
	root := t.TempDir()

	items, err := List(Dir(root, "demo-agent"))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("目录不存在时应当返回空列表，得到 %+v", items)
	}
	removed, err := Clear(Dir(root, "demo-agent"))
	if err != nil {
		t.Fatal(err)
	}
	if removed != 0 {
		t.Fatalf("目录不存在时应当删除 0 份，得到 %d", removed)
	}

	src := filepath.Join(t.TempDir(), "models.json")
	if err := os.WriteFile(src, []byte(`[{"id":"m1"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Capture(src, root, "demo-agent", 10); err != nil {
		t.Fatal(err)
	}
	removed, err = Clear(Dir(root, "demo-agent"))
	if err != nil || removed != 1 {
		t.Fatalf("清空失败: removed=%d err=%v", removed, err)
	}
	items, _ = List(Dir(root, "demo-agent"))
	if len(items) != 0 {
		t.Fatalf("清空后不该还有快照: %+v", items)
	}
}

func TestRead(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.bak")
	content := `{"models":[]}`
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Read(file)
	if err != nil {
		t.Fatal(err)
	}
	if got != content {
		t.Fatalf("Read = %q", got)
	}

	if _, err := Read(filepath.Join(dir, "nope.bak")); err == nil {
		t.Fatal("文件不存在时应当报错")
	}

	big := filepath.Join(dir, "big.bak")
	if err := os.WriteFile(big, []byte(strings.Repeat("x", 600*1024)), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = Read(big)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 512*1024 {
		t.Fatalf("预览应当被限制在 512KB，实际 %d", len(got))
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
