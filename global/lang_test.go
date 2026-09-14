package global

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// TestMain 装一个丢弃输出的日志器：InitLogger 会往工作目录写日志文件，
// 单测里不需要那份副作用，但语言加载路径会调用 Log。
func TestMain(m *testing.M) {
	Log = logrus.New()
	Log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// withTempLang 把语言目录指到一个临时目录，避免测试碰到仓库里的真实语言包。
func withTempLang(t *testing.T) string {
	t.Helper()
	oldPath := pathLang
	oldLang := Config().Language
	oldUse := useLangPath

	dir := t.TempDir()
	pathLang = dir
	langMu.Lock()
	resetLangCacheLocked()
	langMu.Unlock()

	t.Cleanup(func() {
		pathLang = oldPath
		langMu.Lock()
		resetLangCacheLocked()
		langMu.Unlock()
		_ = Update(func(cfg *GConfig) { cfg.Language = oldLang })
		_ = oldUse
	})
	return dir
}

func writeLang(t *testing.T, root, name string, code string, textmap map[string]string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	info := LanguageInfo{
		LanguageName: name,
		LanguageCode: code,
		TextmapPath:  "textmap.json",
	}
	infoData, _ := json.Marshal(info)
	if err := os.WriteFile(filepath.Join(dir, "info.json"), infoData, 0o644); err != nil {
		t.Fatal(err)
	}
	mapData, _ := json.Marshal(textmap)
	if err := os.WriteFile(filepath.Join(dir, "textmap.json"), mapData, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInitLangListsAvailableLanguages(t *testing.T) {
	root := withTempLang(t)
	writeLang(t, root, "default", "zh-CN", map[string]string{"a": "甲"})
	writeLang(t, root, "en-US", "en-US", map[string]string{"a": "A"})

	if err := Update(func(cfg *GConfig) { cfg.Language = "en-US" }); err != nil {
		t.Fatal(err)
	}
	InitLang()

	if got := CurrentLangDir(); got != "en-US" {
		t.Fatalf("应当切到 en-US 目录，得到 %q", got)
	}
	list := GetLangInfoList()
	if len(list) != 2 {
		t.Fatalf("应当识别 2 种语言，得到 %d: %+v", len(list), list)
	}
}

func TestUnknownLanguageFallsBackToDefault(t *testing.T) {
	root := withTempLang(t)
	writeLang(t, root, "default", "zh-CN", map[string]string{"a": "甲"})

	if err := Update(func(cfg *GConfig) { cfg.Language = "xx-XX" }); err != nil {
		t.Fatal(err)
	}
	InitLang()

	if got := CurrentLangDir(); got != defaultLangDir {
		t.Fatalf("未知语言应当回落到 default，得到 %q", got)
	}
}

func TestTAndMergedTextMap(t *testing.T) {
	root := withTempLang(t)
	writeLang(t, root, "default", "zh-CN", map[string]string{
		"only_default": "%s 只在默认包里",
		"positional":   "在 %s 中未找到模型 %s",
	})
	writeLang(t, root, "en-US", "en-US", map[string]string{
		"only_default": "translated",
		"positional":   "Model %[2]s not found in %[1]s",
	})

	if err := Update(func(cfg *GConfig) { cfg.Language = "en-US" }); err != nil {
		t.Fatal(err)
	}
	InitLang()

	if got := T("only_default"); got != "translated" {
		t.Fatalf("应当取到当前语言的文案，得到 %q", got)
	}
	// 缺失的键返回键名本身——宁可显眼，也不要给用户一个空白错误。
	if got := T("no_such_key"); got != "no_such_key" {
		t.Fatalf("缺键时应当回显键名，得到 %q", got)
	}
	// 显式下标的写法必须真的能渲染（语序不同的语言就靠它）。
	if got := T("positional", "config.json", "demo-model"); got != "Model demo-model not found in config.json" {
		t.Fatalf("带参数渲染结果不对: %q", got)
	}

	merged := GetLangTextMap()
	if merged["only_default"] != "translated" {
		t.Fatalf("合并后的映射里应当是当前语言的值: %q", merged["only_default"])
	}

	// 返回的必须是副本，调用方改它不该污染缓存。
	merged["only_default"] = "被改坏了"
	if got := T("only_default"); got != "translated" {
		t.Fatal("GetLangTextMap 应当返回副本")
	}

	// 切到默认语言后空缺键仍然有值（回落链路）。
	if err := Update(func(cfg *GConfig) { cfg.Language = "zh-CN" }); err != nil {
		t.Fatal(err)
	}
	UpdateCurrentLangPath()
	if got := T("only_default"); got != "%s 只在默认包里" {
		t.Fatalf("切回默认语言后取值不对: %q", got)
	}
}

func TestLangCacheInvalidatedByFileChange(t *testing.T) {
	root := withTempLang(t)
	writeLang(t, root, "default", "zh-CN", map[string]string{"k": "旧值"})
	if err := Update(func(cfg *GConfig) { cfg.Language = "zh-CN" }); err != nil {
		t.Fatal(err)
	}
	InitLang()

	if got := T("k"); got != "旧值" {
		t.Fatalf("首次读取不对: %q", got)
	}

	// 改文案并把时间戳推后：缓存必须失效，不需要重启程序。
	mapPath := filepath.Join(root, "default", "textmap.json")
	data, _ := json.Marshal(map[string]string{"k": "新值"})
	if err := os.WriteFile(mapPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(mapPath, future, future); err != nil {
		t.Fatal(err)
	}

	if got := T("k"); got != "新值" {
		t.Fatalf("磁盘语言包变化后应当重新加载，得到 %q", got)
	}
}

func TestGetLangPackIsCopy(t *testing.T) {
	root := withTempLang(t)
	writeLang(t, root, "default", "zh-CN", map[string]string{"k": "值"})
	if err := Update(func(cfg *GConfig) { cfg.Language = "zh-CN" }); err != nil {
		t.Fatal(err)
	}
	InitLang()

	pack, err := GetLangPack()
	if err != nil {
		t.Fatal(err)
	}
	if pack.Textmap["k"] != "值" {
		t.Fatalf("语言包内容不对: %+v", pack.Textmap)
	}
	pack.Textmap["k"] = "被改坏了"
	again, _ := GetLangPack()
	if again.Textmap["k"] != "值" {
		t.Fatal("GetLangPack 应当返回副本，不能让人改到缓存")
	}
}

// TestLangAccessIsRaceFree 覆盖「语言随时可切、读取到处都在」的并发场景。
func TestLangAccessIsRaceFree(t *testing.T) {
	root := withTempLang(t)
	writeLang(t, root, "default", "zh-CN", map[string]string{"k": "值"})
	writeLang(t, root, "en-US", "en-US", map[string]string{"k": "value"})
	InitLang()

	done := make(chan struct{})
	for i := 0; i < 4; i++ {
		go func(i int) {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 30; j++ {
				if i%2 == 0 {
					_ = T("k")
					GetLangTextMap()
					GetLangInfoList()
					CurrentLangDir()
					continue
				}
				code := "en-US"
				if j%2 == 0 {
					code = "zh-CN"
				}
				_ = Update(func(cfg *GConfig) { cfg.Language = code })
				ClearLangCache()
				UpdateCurrentLangPath()
			}
		}(i)
	}
	for i := 0; i < 4; i++ {
		<-done
	}
}
