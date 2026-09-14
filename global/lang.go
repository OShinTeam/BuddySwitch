package global

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	Pa "path"
	"sync"
	"time"
)

var (
	pathLang = "lang/"
	LangFS   fs.FS

	// langMu 保护下面全部语言状态。语言可以随时被切换，而这些状态会被
	// 各个 goroutine（Wails 的每个绑定调用、后台探测）并发读取，
	// 所以不能只是「启动时写一次」就当它是只读的。
	langMu        sync.RWMutex
	useLangPath   = defaultLangDir
	allLangInfo   []LanguageInfo
	langCodeToDir map[string]string
	packCache     map[string]*cachedPack

	// mergedCache 缓存「当前语言 + 默认语言」合并后的文案，
	// mergedLang 记录它属于哪个语言目录；磁盘上的语言包一变就整体作废。
	mergedCache map[string]string
	mergedLang  string
)

// defaultLangDir 是默认语言目录名，同时作为文案缺失时的回落来源。
const defaultLangDir = "default"

type LanguageInfo struct {
	LanguageName        string `json:"language_name"`
	LanguageCode        string `json:"language_code"`
	TextmapPath         string `json:"textmap_path"`
	TranslationProgress string `json:"translation_progress"`
	Translator          string `json:"translator"`
	LastUpdated         string `json:"last_updated"`
	Version             string `json:"version"`
}

type LanguagePack struct {
	LanguageInfo
	Textmap map[string]string `json:"textmap"`
}

// cachedPack 是磁盘语言包的内存缓存，附带来源文件的修改时间。
// 只按目录名缓存、不看时间戳的话，用户改了 lang/ 下的文案就必须重启程序。
type cachedPack struct {
	pack *LanguagePack
	mod  time.Time
	// fromDisk 为 false 表示这份包来自内嵌资源，磁盘上没有对应目录，
	// 也就不需要（也无法）做时间戳校验。
	fromDisk bool
}

func init() {
	langCodeToDir = map[string]string{}
	packCache = map[string]*cachedPack{}
}

func InitLang() {
	langMu.Lock()
	defer langMu.Unlock()

	useLang := Config().Language
	allLangInfo = []LanguageInfo{}
	langCodeToDir = map[string]string{}

	// 磁盘目录优先，内嵌资源兜底：同 code 的语言只登记先扫到的那个。
	scanFileSystemLangs(langCodeToDir)
	scanEmbeddedLangs(langCodeToDir)

	if dir, ok := langCodeToDir[useLang]; ok {
		Log.Infof("找到匹配语言: %s -> %s", useLang, dir)
		useLangPath = dir
	} else {
		Log.Infof("未找到匹配语言 %s，使用默认语言", useLang)
		useLangPath = defaultLangDir
	}
}

func scanFileSystemLangs(langDirMap map[string]string) {
	entries, err := os.ReadDir(pathLang)
	if err != nil {
		// 首次启动时磁盘上还没有 lang/ 目录，属正常情况。
		Log.Debugf("未从文件系统读到 lang 目录（将只用内嵌语言包）: %v", err)
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		loadLangInfoFromFile(Pa.Join(pathLang, entry.Name()), entry.Name(), langDirMap, "[文件系统]")
	}
}

func scanEmbeddedLangs(langDirMap map[string]string) {
	if LangFS == nil {
		Log.Warn("内嵌语言资源未初始化，仅使用磁盘语言包")
		return
	}
	entries, err := fs.ReadDir(LangFS, "lang")
	if err != nil {
		Log.Warnf("读取内嵌 lang 目录失败: %v", err)
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := fs.ReadFile(LangFS, Pa.Join("lang", entry.Name(), "info.json"))
		if err != nil {
			Log.Warnf("读取内嵌语言信息失败 %s: %v", entry.Name(), err)
			continue
		}
		parseAndAddLangInfo(data, entry.Name(), langDirMap, "[嵌入FS]")
	}
}

func loadLangInfoFromFile(dir string, dirName string, langDirMap map[string]string, source string) {
	data, err := os.ReadFile(Pa.Join(dir, "info.json"))
	if err != nil {
		Log.Warnf("读取语言信息失败 %s: %v", dir, err)
		return
	}
	parseAndAddLangInfo(data, dirName, langDirMap, source)
}

func parseAndAddLangInfo(data []byte, dirName string, langDirMap map[string]string, source string) {
	var info LanguageInfo
	if err := json.Unmarshal(data, &info); err != nil {
		Log.Warnf("解析语言信息失败 %s: %v", dirName, err)
		return
	}
	if info.LanguageCode == "" || info.TextmapPath == "" {
		Log.Warnf("语言信息不完整: %s", dirName)
		return
	}
	if containsLang(allLangInfo, info.LanguageCode) {
		return
	}
	allLangInfo = append(allLangInfo, info)
	langDirMap[info.LanguageCode] = dirName
	Log.Debugf("%s 识别到语言: %s (%s) -> %s", source, info.LanguageName, info.LanguageCode, dirName)
}

func containsLang(slice []LanguageInfo, code string) bool {
	for _, s := range slice {
		if s.LanguageCode == code {
			return true
		}
	}
	return false
}

// ClearLangCache 丢弃全部语言包与合并文案缓存，下次读取时重新落盘加载。
func ClearLangCache() {
	langMu.Lock()
	defer langMu.Unlock()
	resetLangCacheLocked()
}

func resetLangCacheLocked() {
	packCache = map[string]*cachedPack{}
	mergedCache = nil
	mergedLang = ""
}

// langFilesChangedLocked 判断当前语言涉及的磁盘文件是否变过。
//
// 只 stat 两三个文件，代价可以忽略；换来的是改完语言包不用重启程序。
// 调用前必须持有 langMu。
func langFilesChangedLocked() bool {
	for _, dir := range []string{defaultLangDir, useLangPath} {
		key := Pa.Join(pathLang, dir)
		cached, ok := packCache[key]
		if !ok {
			return true
		}
		if cached.fromDisk && !cached.mod.Equal(newestModTime(packFiles(key))) {
			return true
		}
	}
	return false
}

// GetLangInfoList 返回全部可用语言。
func GetLangInfoList() []LanguageInfo {
	langMu.RLock()
	defer langMu.RUnlock()
	return append([]LanguageInfo(nil), allLangInfo...)
}

// UpdateCurrentLangPath 依据当前配置重新定位语言目录。
func UpdateCurrentLangPath() {
	langMu.Lock()
	defer langMu.Unlock()
	useLang := Config().Language
	if dir, ok := langCodeToDir[useLang]; ok {
		Log.Infof("更新语言路径: %s -> %s", useLang, dir)
		useLangPath = dir
	} else {
		Log.Infof("未找到匹配语言 %s，使用默认语言", useLang)
		useLangPath = defaultLangDir
	}
}

// CurrentLangDir 返回当前语言所在的目录名。
func CurrentLangDir() string {
	langMu.RLock()
	defer langMu.RUnlock()
	return useLangPath
}

// T 按当前语言取一条文案，供后端生成用户可见的错误与提示使用。
//
// 找不到 key 时直接返回 key 本身——宁可在界面上看到一个显眼的键名，
// 也不要返回空字符串让用户面对一句「无内容的错误」。
// 带 args 时按 fmt.Sprintf 处理，用于「在 %s 中未找到模型 %s」这类句子。
func T(key string, args ...any) string {
	text := mergedTextMap()[key]
	if text == "" {
		text = key
	}
	if len(args) == 0 {
		return text
	}
	return fmt.Sprintf(text, args...)
}

// GetLangTextMap 返回当前语言的文案，已与默认语言合并。
//
// 当前语言缺失的键自动回落到默认语言，这样只翻译了一部分的语言包
// 也不会在界面上暴露出裸键名。返回的是副本，调用方改它不会污染缓存。
func GetLangTextMap() map[string]string {
	merged := mergedTextMap()
	out := make(map[string]string, len(merged))
	for k, v := range merged {
		out[k] = v
	}
	return out
}

// mergedTextMap 返回缓存中的合并文案（调用方只读，不要修改）。
func mergedTextMap() map[string]string {
	langMu.RLock()
	if mergedCache != nil && mergedLang == useLangPath && !langFilesChangedLocked() {
		cached := mergedCache
		langMu.RUnlock()
		return cached
	}
	langMu.RUnlock()

	langMu.Lock()
	defer langMu.Unlock()
	if mergedCache != nil && mergedLang == useLangPath && !langFilesChangedLocked() {
		return mergedCache
	}

	merged := map[string]string{}
	if pack, err := loadPackByDirLocked(defaultLangDir); err == nil {
		for k, v := range pack.Textmap {
			merged[k] = v
		}
	}
	if useLangPath != defaultLangDir {
		if pack, err := loadPackByDirLocked(useLangPath); err == nil {
			for k, v := range pack.Textmap {
				merged[k] = v
			}
		}
	}
	mergedCache = merged
	mergedLang = useLangPath
	return merged
}

// GetLangPack 返回当前语言的完整语言包（不做默认语言合并）。
func GetLangPack() (*LanguagePack, error) {
	langMu.Lock()
	defer langMu.Unlock()
	pack, err := loadPackByDirLocked(useLangPath)
	if err != nil {
		return nil, err
	}
	// 缓存里那份要保持不变，这里复制一份给调用方。
	out := &LanguagePack{LanguageInfo: pack.LanguageInfo, Textmap: make(map[string]string, len(pack.Textmap))}
	for k, v := range pack.Textmap {
		out.Textmap[k] = v
	}
	return out, nil
}

// loadPackByDirLocked 按目录名加载语言包，优先磁盘目录，其次内嵌资源。
// 调用前必须持有 langMu。
func loadPackByDirLocked(dir string) (*LanguagePack, error) {
	key := Pa.Join(pathLang, dir)

	if cached, ok := packCache[key]; ok {
		if !cached.fromDisk || cached.mod.Equal(newestModTime(packFiles(key))) {
			return cached.pack, nil
		}
		Log.Infof("语言包 %s 已变化，重新加载", key)
	}

	if _, err := os.Stat(key); err == nil {
		pack, err := tryLoadLangPack(key)
		if err != nil {
			return nil, err
		}
		packCache[key] = &cachedPack{pack: pack, mod: newestModTime(packFiles(key)), fromDisk: true}
		return pack, nil
	}

	pack, err := tryLoadLangPackFromEmbed(Pa.Join("lang", dir))
	if err != nil {
		return nil, err
	}
	// 内嵌资源不会在运行期变化，无需时间戳校验；但仍然给它缓存。
	packCache[key] = &cachedPack{pack: pack, fromDisk: false}
	return pack, nil
}

func packFiles(dir string) []string {
	return []string{Pa.Join(dir, "info.json"), Pa.Join(dir, "textmap.json")}
}

// newestModTime 取一组文件里最新的修改时间；全部读不到时返回零值。
func newestModTime(paths []string) time.Time {
	var newest time.Time
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest
}

func tryLoadLangPack(langPath string) (*LanguagePack, error) {
	langInfo, err := readLangInfo(langPath)
	if err != nil {
		return nil, err
	}
	textmap, err := readTextmap(langPath)
	if err != nil {
		return nil, err
	}
	return &LanguagePack{LanguageInfo: langInfo, Textmap: textmap}, nil
}

func tryLoadLangPackFromEmbed(langPath string) (*LanguagePack, error) {
	if LangFS == nil {
		return nil, fmt.Errorf("嵌入的文件系统未初始化")
	}

	infoData, err := fs.ReadFile(LangFS, Pa.Join(langPath, "info.json"))
	if err != nil {
		return nil, err
	}
	var langInfo LanguageInfo
	if err := json.Unmarshal(infoData, &langInfo); err != nil {
		return nil, fmt.Errorf("解析内嵌 info.json 失败: %w", err)
	}

	textmapData, err := fs.ReadFile(LangFS, Pa.Join(langPath, "textmap.json"))
	if err != nil {
		return nil, err
	}
	var textmap map[string]string
	if err := json.Unmarshal(textmapData, &textmap); err != nil {
		return nil, fmt.Errorf("解析内嵌 textmap.json 失败: %w", err)
	}

	return &LanguagePack{LanguageInfo: langInfo, Textmap: textmap}, nil
}

func readLangInfo(langPath string) (LanguageInfo, error) {
	var info LanguageInfo
	data, err := os.ReadFile(Pa.Join(langPath, "info.json"))
	if err != nil {
		return info, err
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return info, fmt.Errorf("解析 info.json 失败: %w", err)
	}
	return info, nil
}

func readTextmap(langPath string) (map[string]string, error) {
	data, err := os.ReadFile(Pa.Join(langPath, "textmap.json"))
	if err != nil {
		return nil, err
	}
	var textmap map[string]string
	if err := json.Unmarshal(data, &textmap); err != nil {
		return nil, fmt.Errorf("解析 textmap.json 失败: %w", err)
	}
	return textmap, nil
}
