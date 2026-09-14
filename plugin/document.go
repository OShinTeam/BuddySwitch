package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"buddyswitch/global"
)

// Document 表示一份被加载的 Agent 原生模型配置文件。
type Document struct {
	def    *Definition
	file   string
	root   any // 整个文件的解析结果（map 或数组）
	indent string

	// fresh 表示文件不存在或内容为空，此时允许在首选容器路径上直接建出来。
	fresh bool
	// containerPath 是解析命中的模型数组路径（"" 表示根节点即数组）。
	// 一旦确定就固定下来，保证同一次编辑里 Entries 与 setEntries 选到同一个位置。
	containerPath string
	resolved      bool
}

// LoadDocument 读取并解析 file，按 def 的 schema 定位模型数组。
// 文件不存在时返回一个空文档（仍然可以 Upsert 后 Save 创建出来）。
func LoadDocument(def *Definition, file string) (*Document, error) {
	doc := &Document{def: def, file: file, indent: "  ", fresh: true}
	if def.Schema.Indent != "" {
		doc.indent = def.Schema.Indent
	}

	data, err := os.ReadFile(file)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		doc.root = def.freshRoot()
		return doc, nil
	}

	if format(def.Source.Format) == "jsonc" {
		data = stripJSONC(data)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		doc.root = def.freshRoot()
		return doc, nil
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var root any
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("%s: %w", global.T("err_parse_file_failed", file), err)
	}
	doc.root = root
	doc.fresh = false
	return doc, nil
}

// File 返回文档对应的文件路径。
func (d *Document) File() string { return d.file }

// container 解析并缓存命中的模型数组路径。
//
// 按 schema.container 的候选顺序逐个尝试：空字符串表示「根节点即数组」，
// 其余按 a.b.c 取值并要求结果是数组。全部落空时，若是新建文件则采用首选路径，
// 否则报错——那意味着插件定义和实际配置对不上，需要用户修正映射。
func (d *Document) container() (string, error) {
	if d.resolved {
		return d.containerPath, nil
	}

	candidates := d.def.Schema.Container
	if len(candidates) == 0 {
		candidates = ContainerList{""}
	}

	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			if _, ok := d.root.([]any); ok {
				d.containerPath, d.resolved = "", true
				return "", nil
			}
			continue
		}
		if node, ok := getByPath(d.root, c); ok {
			if _, isArray := node.([]any); isArray {
				d.containerPath, d.resolved = c, true
				return c, nil
			}
		}
	}

	if d.fresh {
		first := ""
		if len(candidates) > 0 {
			first = strings.TrimSpace(candidates[0])
		}
		d.containerPath, d.resolved = first, true
		return first, nil
	}
	return "", fmt.Errorf("%s", global.T("err_container_not_found", d.file, []string(candidates)))
}

// Entries 返回模型数组的原始切片（与文档内部共享引用，可直接改写元素）。
func (d *Document) Entries() ([]any, error) {
	path, err := d.container()
	if err != nil {
		return nil, err
	}
	if path == "" {
		arr, ok := d.root.([]any)
		if !ok {
			return nil, fmt.Errorf("%s", global.T("err_root_not_array"))
		}
		return arr, nil
	}
	node, ok := getByPath(d.root, path)
	if !ok || node == nil {
		return nil, nil // 容器尚不存在，第一次写入时创建
	}
	arr, ok := node.([]any)
	if !ok {
		return nil, fmt.Errorf("%s", global.T("err_container_not_array", path))
	}
	return arr, nil
}

// setEntries 把改写后的数组写回文档结构中。
func (d *Document) setEntries(entries []any) error {
	path, err := d.container()
	if err != nil {
		return err
	}
	if path == "" {
		d.root = entries
		d.fresh = false
		return nil
	}
	root, ok := d.root.(map[string]any)
	if !ok {
		return fmt.Errorf("%s", global.T("err_container_needs_object", path))
	}
	setByPath(root, path, entries)
	d.fresh = false
	return nil
}

// touchList 维护「可见模型清单」（如 WorkBuddy 的 availableModels）。
//
// 只做两件最小的事：新增模型时把 id 追加进去，删除模型时把 id 摘掉。
// 刻意不做全量对齐——用户可能故意把某个模型从清单里排除，全量覆盖会把这份意图冲掉。
// 并且只有文件里已经存在该字段时才动它，不使用它的配置不会凭空多出一个字段。
func (d *Document) touchList(addID, removeID string) {
	path := strings.TrimSpace(d.def.Schema.SyncList)
	if path == "" {
		return
	}
	root, ok := d.root.(map[string]any)
	if !ok {
		return
	}
	node, exists := getByPath(root, path)
	if !exists {
		return
	}
	arr, isArray := node.([]any)
	if !isArray {
		return
	}

	out := make([]any, 0, len(arr)+1)
	seen := make(map[string]bool, len(arr)+1)
	for _, item := range arr {
		id := toString(item)
		if id == "" || id == removeID || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, item)
	}
	if addID != "" && !seen[addID] {
		out = append(out, addID)
	}
	setByPath(root, path, out)
}

// Models 按 schema 把原生条目解析为统一模型列表。
func (d *Document) Models() ([]Model, error) {
	entries, err := d.Entries()
	if err != nil {
		return nil, err
	}
	out := make([]Model, 0, len(entries))
	for _, entry := range entries {
		obj, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		m := d.def.toModel(obj, d.file)
		if m.ID == "" {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

// toModel 把原生条目映射为统一模型。
func (def *Definition) toModel(entry map[string]any, file string) Model {
	pick := func(field string) (any, bool) {
		path := strings.TrimSpace(def.Schema.Fields[field])
		if path == "" {
			return nil, false
		}
		return getByPath(entry, path)
	}

	m := Model{PluginID: def.ID, SourceFile: file}
	if v, ok := pick(FieldID); ok {
		m.ID = toString(v)
	}
	if v, ok := pick(FieldDisplayName); ok {
		m.DisplayName = toString(v)
	}
	if v, ok := pick(FieldProvider); ok {
		m.Provider = toString(v)
	}
	if v, ok := pick(FieldBaseURL); ok {
		m.BaseURL = toString(v)
	}
	if v, ok := pick(FieldAPIKey); ok {
		m.APIKey = toString(v)
	}
	if v, ok := pick(FieldEnabled); ok {
		m.Enabled = toBool(v)
		m.NativeEnabled = true
	} else {
		m.Enabled = true
	}
	if v, ok := pick(FieldDescription); ok {
		m.Description = toString(v)
	}
	if v, ok := pick(FieldTags); ok {
		m.Tags = toStringSlice(v)
	}
	if keys := def.CapabilityKeys(); len(keys) > 0 {
		caps := make(map[string]bool, len(keys))
		for _, key := range keys {
			path := strings.TrimSpace(def.Schema.Capabilities[key])
			if path == "" {
				continue
			}
			if v, ok := getByPath(entry, path); ok {
				caps[key] = toBool(v)
			} else {
				caps[key] = false
			}
		}
		m.Capabilities = caps
	}
	if m.DisplayName == "" {
		m.DisplayName = m.ID
	}
	return m
}

// Upsert 按原生 id 更新或新增一条模型记录。
//
// replace 表示调用方是否持有完整信息：
//   - true（编辑器、单独改启用开关）：映射到的字段一律按传入值覆写，
//     空值就是「清空」——用户把标签删干净再保存，配置里就该真的没有标签；
//   - false（把缓存里的模型搬进某个 agent）：只写有值的字段。
//     搬运方并不掌握目标原有备注、标签这些信息，不该顺手抹掉它们。
func (d *Document) Upsert(m Model, replace bool) error {
	entries, err := d.Entries()
	if err != nil {
		return err
	}
	if entries == nil {
		entries = []any{}
	}

	idPath := d.def.idField()
	target := -1
	for i, entry := range entries {
		obj, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if v, ok := getByPath(obj, idPath); ok && toString(v) == m.ID {
			target = i
			break
		}
	}

	if target >= 0 {
		obj, ok := entries[target].(map[string]any)
		if !ok {
			return fmt.Errorf("%s", global.T("err_entry_not_object", entries[target]))
		}
		d.def.applyModel(obj, m, false, replace)
	} else {
		obj := map[string]any{}
		// 保留未被映射的字段：新建时用 defaults 打底。
		for k, v := range d.def.Schema.Defaults {
			obj[k] = v
		}
		d.def.applyModel(obj, m, true, replace)
		entries = append(entries, obj)
	}

	if err := d.setEntries(entries); err != nil {
		return err
	}
	if target < 0 {
		d.touchList(m.ID, "")
	}
	return nil
}

// Remove 按原生 id 删除一条模型记录，返回是否确实删除了内容。
func (d *Document) Remove(id string) (bool, error) {
	entries, err := d.Entries()
	if err != nil {
		return false, err
	}
	idPath := d.def.idField()
	kept := make([]any, 0, len(entries))
	removed := false
	for _, entry := range entries {
		obj, ok := entry.(map[string]any)
		if ok {
			if v, ok := getByPath(obj, idPath); ok && toString(v) == id {
				removed = true
				continue
			}
		}
		kept = append(kept, entry)
	}
	if !removed {
		return false, nil
	}
	if err := d.setEntries(kept); err != nil {
		return false, err
	}
	d.touchList("", id)
	return true, nil
}

// applyModel 把统一模型字段写入原生条目。
//
// create 表示这是新增的条目；replace 表示调用方持有完整信息（见 Upsert）。
func (def *Definition) applyModel(entry map[string]any, m Model, create bool, replace bool) {
	put := func(field string, value any) {
		path := strings.TrimSpace(def.Schema.Fields[field])
		if path == "" {
			return
		}
		setByPath(entry, path, value)
	}
	// replace 时按传入值写（空也算一种意见）；否则只写有值的，
	// 避免「从缓存搬模型」把目标原有的信息清成空字符串。
	putFilled := func(field string, value string) {
		if replace || strings.TrimSpace(value) != "" {
			put(field, value)
		}
	}

	put(FieldID, m.ID)
	putFilled(FieldDisplayName, m.DisplayName)
	putFilled(FieldProvider, m.Provider)
	putFilled(FieldBaseURL, m.BaseURL)
	putFilled(FieldAPIKey, m.APIKey)
	putFilled(FieldDescription, m.Description)

	if replace || len(m.Tags) > 0 || create {
		tags := make([]any, 0, len(m.Tags))
		for _, t := range m.Tags {
			tags = append(tags, t)
		}
		put(FieldTags, tags)
	}
	if def.CanPersistEnabled() {
		put(FieldEnabled, m.Enabled)
	}

	// 能力标记：编辑时按传入值写回；新建时把没给到的补成 false，
	// 避免 agent 因为字段缺失而拿不到默认能力。
	for _, key := range def.CapabilityKeys() {
		path := strings.TrimSpace(def.Schema.Capabilities[key])
		if path == "" {
			continue
		}
		if v, ok := m.Capabilities[key]; ok {
			setByPath(entry, path, v)
		} else if create {
			setByPath(entry, path, false)
		}
	}
}

// Save 把文档原子地写回磁盘。
//
// 这里不做备份——快照由 backup 包在调用方统一负责，因为「保留几份、放哪」
// 属于应用配置，不是格式定义该关心的事。
func (d *Document) Save() error {
	if err := os.MkdirAll(filepath.Dir(d.file), 0o755); err != nil {
		return err
	}

	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetIndent("", d.indent)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(d.root); err != nil {
		return err
	}

	// 原子写入：先写临时文件再改名，避免中途失败破坏配置。
	tmp := d.file + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, d.file)
}

// ---------------------------------------------------------------------------
// 路径工具
// ---------------------------------------------------------------------------

func splitPath(p string) []string {
	raw := strings.Split(strings.TrimSpace(p), ".")
	segs := make([]string, 0, len(raw))
	for _, s := range raw {
		if s = strings.TrimSpace(s); s != "" {
			segs = append(segs, s)
		}
	}
	return segs
}

// getByPath 从任意嵌套结构中按 a.b.c 取值。
func getByPath(node any, path string) (any, bool) {
	segs := splitPath(path)
	if len(segs) == 0 {
		return node, true
	}
	cur := node
	for _, seg := range segs {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = obj[seg]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// setByPath 向嵌套结构中按 a.b.c 写值，缺失的中间层会自动创建。
func setByPath(root map[string]any, path string, value any) {
	segs := splitPath(path)
	if len(segs) == 0 {
		return
	}
	cur := root
	for i := 0; i < len(segs)-1; i++ {
		next, ok := cur[segs[i]].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[segs[i]] = next
		}
		cur = next
	}
	cur[segs[len(segs)-1]] = value
}

// ---------------------------------------------------------------------------
// 值转换
// ---------------------------------------------------------------------------

func toString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case json.Number:
		return t.String()
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return fmt.Sprint(v)
	}
}

func toBool(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "1", "true", "yes", "on", "enabled":
			return true
		}
		return false
	case json.Number:
		f, err := t.Float64()
		return err == nil && f != 0
	case float64:
		return t != 0
	case int:
		return t != 0
	default:
		return false
	}
}

func toStringSlice(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case string:
		if strings.TrimSpace(t) == "" {
			return nil
		}
		parts := strings.Split(t, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s := toString(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return t
	default:
		return nil
	}
}
