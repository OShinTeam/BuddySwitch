// Package plugin 定义 BuddySwitch 的 Agent 插件体系。
//
// 设计核心：一个插件 = 一份「模型写入格式定义」(Definition)。
// 只要提供这样一份 JSON 文件，BuddySwitch 就能对该 Agent 实现完整的智能管理：
//
//  1. source —— 定位该 Agent 在本机的模型配置文件（按平台给出候选路径）；
//  2. schema —— 把原生配置文件解析成统一模型列表，并把修改安全写回；
//  3. probe  —— 对该 Agent 下的模型发起可用性探测（OpenAI / Anthropic 风格）。
//
// 因此「接入一个新 Agent」= 新增一份 plugins/<id>.json，无需改动任何代码。
// 内置插件随二进制一同分发，磁盘上的 plugins/*.json 可覆盖内置、也可新增。
package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"buddyswitch/global"
)

// ---------------------------------------------------------------------------
// 统一模型字段
// ---------------------------------------------------------------------------

// schema.fields 的固定键名。插件定义把这些键映射到原生配置文件里的字段路径。
const (
	FieldID          = "id"
	FieldDisplayName = "display_name"
	FieldProvider    = "provider"
	FieldBaseURL     = "base_url"
	FieldAPIKey      = "api_key"
	FieldEnabled     = "enabled"
	FieldDescription = "description"
	FieldTags        = "tags"
)

// UnifiedFields 是 schema.fields 允许出现的全部键，用于校验插件定义。
var UnifiedFields = []string{
	FieldID, FieldDisplayName, FieldProvider, FieldBaseURL,
	FieldAPIKey, FieldEnabled, FieldDescription, FieldTags,
}

// RequiredFields 是插件定义必须映射的字段，缺少则无法正常管理。
var RequiredFields = []string{FieldID}

// ---------------------------------------------------------------------------
// 插件定义
// ---------------------------------------------------------------------------

// Definition 是一份完整的「模型写入格式定义」。
type Definition struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Vendor      string `json:"vendor,omitempty"`
	Color       string `json:"color,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Version     string `json:"version,omitempty"`
	Order       int    `json:"order,omitempty"`

	// Enabled 为定义的缺省开关。nil 视为启用；用户在界面上的切换结果
	// 保存在状态文件中，不写回插件定义本身（内置插件是只读的）。
	Enabled *bool `json:"enabled,omitempty"`

	Source SourceSpec `json:"source"`
	Schema SchemaSpec `json:"schema"`
	Probe  ProbeSpec  `json:"probe"`
	// Listing 描述如何向上游索要模型清单（拉取）。缺失时该插件只能读配置文件。
	Listing ListingSpec `json:"listing,omitempty"`

	// 以下为运行时信息，由注册表填充，不参与定义文件的读写。
	Builtin     bool     `json:"builtin"`
	DefPath     string   `json:"def_path,omitempty"`
	SourceFiles []string `json:"source_files,omitempty"` // 当前平台解析出的候选路径
	Loaded      bool     `json:"loaded"`
	LoadError   string   `json:"load_error,omitempty"`
}

// SourceSpec 描述到哪里找该 Agent 的模型配置文件。
type SourceSpec struct {
	// Paths 的键为平台名（windows / darwin / linux），值为候选路径列表，按优先级排列。
	// 路径支持 %APPDATA% 风格的 Windows 变量、$VAR / ${VAR} 以及开头的 ~。
	// 特殊的 "default" 键会作为所有平台的兜底。
	Paths map[string][]string `json:"paths"`
	// Format 目前支持 json / jsonc（jsonc 会先剥离注释）。
	Format string `json:"format,omitempty"`
}

// ContainerList 是模型数组的候选位置列表。
//
// 现实里同一个 agent 可能有多种被官方文档承认的结构——比如 WorkBuddy 既支持
// 根节点直接是数组，也支持 {"models": [...]}。因此这里按顺序逐个尝试，
// 命中第一个能取出数组的路径。
//
// 定义文件里既可以直接写字符串 "models"，也可以写数组 ["", "models"]。
type ContainerList []string

// UnmarshalJSON 同时接受字符串与字符串数组两种写法。
func (c *ContainerList) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var list []string
		if err := json.Unmarshal(data, &list); err != nil {
			return err
		}
		*c = list
		return nil
	}
	var one string
	if err := json.Unmarshal(data, &one); err != nil {
		return err
	}
	*c = []string{one}
	return nil
}

// MarshalJSON 单元素时写回字符串，多元素时写回数组，保持定义文件易读。
func (c ContainerList) MarshalJSON() ([]byte, error) {
	if len(c) == 1 {
		return json.Marshal(c[0])
	}
	return json.Marshal([]string(c))
}

// SchemaSpec 描述原生配置文件的内部结构，是读写的唯一依据。
type SchemaSpec struct {
	// Container 是模型数组在文件中的候选路径，如 ["", "models"]。
	// 空字符串表示「文件根节点本身就是数组」，会最先尝试。
	Container ContainerList `json:"container,omitempty"`
	// Fields 把统一字段映射到原生字段路径（同样支持 a.b.c 的嵌套写法）。
	// 值为空字符串表示该字段在原生文件中不存在。
	Fields map[string]string `json:"fields"`
	// Capabilities 把「能力标记」映射到原生布尔字段，例如
	// {"tool_call": "supportsToolCall"}。键名由插件自定义，界面按原样展示。
	Capabilities map[string]string `json:"capabilities,omitempty"`
	// SyncList 是可选的「可见模型清单」路径（如 availableModels）。
	// 文件里已存在该字段时，它的内容会与实际模型列表保持一致——
	// 否则会出现「BuddySwitch 里加了模型，agent 下拉框里却看不见」。
	SyncList string `json:"sync_list,omitempty"`
	// Defaults 是新增模型时写入的固定原生字段（如 "type": "openai"）。
	Defaults map[string]any `json:"defaults,omitempty"`
	// Indent 为写回时的缩进字符，默认两个空格。
	Indent string `json:"indent,omitempty"`
}

// ProbeSpec 描述如何探测一个模型是否可用。
//
// Endpoint / Headers / Body 中可以使用 {{field}} 占位符，field 取自统一模型字段
// （id / display_name / provider / base_url / api_key）以及 {{plugin_id}}。
type ProbeSpec struct {
	// Type 仅作展示与默认值提示：openai / anthropic / custom。
	Type     string `json:"type,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	// EnsurePath 是可选的路径补全后缀，例如 "/chat/completions"。
	// 渲染后的地址若不以它结尾就追加，用来兼容「url 有时是完整接口、
	// 有时是 base」的配置——WorkBuddy 的 useCustomProtocol 关闭时正是这种情形。
	EnsurePath string            `json:"ensure_path,omitempty"`
	Method     string            `json:"method,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       any               `json:"body,omitempty"`
	TimeoutMs  int               `json:"timeout_ms,omitempty"`
	// ExpectStatus 是视为成功的状态码区间列表，形如 [200, 299]；留空默认 2xx 成功，
	// 401/403 会被单独识别为「密钥无效」。
	ExpectStatus []int `json:"expect_status,omitempty"`
}

// ListingSpec 描述「怎么向上游要它的模型清单」。
//
// 这是与 probe 平行的另一件事：probe 问「这个模型还能不能用」，
// listing 问「这个上游到底提供哪些模型」。有了它，拉取就是拿密钥去问上游，
// 而不是把本地配置文件里已有的东西读出来。
type ListingSpec struct {
	// Endpoint 通常是 {{base_url}}，即复用上游的接入点。
	Endpoint string `json:"endpoint,omitempty"`
	// StripSuffix 先把地址尾部的这个后缀去掉，再补 EnsurePath。
	// 用来把 .../v1/chat/completions 还原成 .../v1，才能拼出 /v1/models。
	StripSuffix string `json:"strip_suffix,omitempty"`
	// EnsurePath 地址不以它结尾就补上，通常填 "/models"。
	EnsurePath string            `json:"ensure_path,omitempty"`
	Method     string            `json:"method,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	TimeoutMs  int               `json:"timeout_ms,omitempty"`
	// ArrayPath 是响应里模型数组的位置，"data" 对应 OpenAI 的 {"data":[...]}；
	// 留空表示响应根节点本身就是数组。
	ArrayPath string `json:"array_path,omitempty"`
	// IDField / NameField / NoteField 是数组元素里各字段的路径。
	// 留空时 IDField 与 NameField 都按 "id" 处理。
	IDField   string `json:"id_field,omitempty"`
	NameField string `json:"name_field,omitempty"`
	NoteField string `json:"note_field,omitempty"`
}

// RemoteModel 是从上游索要回来的一条模型记录。
type RemoteModel struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name,omitempty"`
	Note        string `json:"note,omitempty"`
}

// ---------------------------------------------------------------------------
// 统一模型
// ---------------------------------------------------------------------------

// Model 是 BuddySwitch 内部使用的统一模型表示。
type Model struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"display_name"`
	Provider    string   `json:"provider"`
	BaseURL     string   `json:"base_url"`
	APIKey      string   `json:"api_key"`
	Enabled     bool     `json:"enabled"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`

	// PluginID / SourceFile 标记该模型的来源，前端据此分组。
	PluginID   string `json:"plugin_id"`
	SourceFile string `json:"source_file"`

	// NativeEnabled 表示原生配置文件里是否真的存在启用开关。
	NativeEnabled bool `json:"native_enabled"`

	// Capabilities 是插件在 schema.capabilities 里声明过的能力标记，
	// 键为插件自定义的名字（如 tool_call / images / reasoning）。
	Capabilities map[string]bool `json:"capabilities,omitempty"`

	// Probe 是最近一次探测结果，来自状态文件。
	Probe *ProbeResult `json:"probe,omitempty"`
}

// ---------------------------------------------------------------------------
// 探测结果
// ---------------------------------------------------------------------------

// 探测状态。
const (
	ProbeUnknown = "unknown" // 尚未探测
	ProbeOK      = "ok"      // 可用
	ProbeAuth    = "auth"    // 密钥无效或无权限
	ProbeFail    = "fail"    // 服务端返回非预期状态
	ProbeError   = "error"   // 网络/解析等本地错误
)

// ProbeResult 是一次探测的结论。
type ProbeResult struct {
	Status     string `json:"status"`
	StatusCode int    `json:"status_code"`
	LatencyMs  int64  `json:"latency_ms"`
	Message    string `json:"message,omitempty"`
	CheckedAt  string `json:"checked_at,omitempty"`
}

// ---------------------------------------------------------------------------
// 校验与辅助
// ---------------------------------------------------------------------------

// DefaultEnabled 返回插件定义声明的缺省启用状态。
func (d *Definition) DefaultEnabled() bool {
	return d.Enabled == nil || *d.Enabled
}

// CanPersistEnabled 表示该插件的原生配置文件里存在启用开关字段。
func (d *Definition) CanPersistEnabled() bool {
	p, ok := d.Schema.Fields[FieldEnabled]
	return ok && strings.TrimSpace(p) != ""
}

// idField 返回原生配置中作为唯一键的字段路径。
func (d *Definition) idField() string {
	if p, ok := d.Schema.Fields[FieldID]; ok && strings.TrimSpace(p) != "" {
		return p
	}
	return "id"
}

// freshRoot 返回「配置文件尚不存在」时应采用的空结构。
//
// 以候选路径的第一项为准：首选「根节点即数组」就返回空数组，
// 否则返回空对象，等第一次写入时按路径把容器建出来。
func (d *Definition) freshRoot() any {
	first := ""
	if len(d.Schema.Container) > 0 {
		first = strings.TrimSpace(d.Schema.Container[0])
	}
	if first == "" {
		return []any{}
	}
	return map[string]any{}
}

// CapabilityKeys 返回 schema.capabilities 中声明过的键，顺序稳定，便于比对与展示。
func (d *Definition) CapabilityKeys() []string {
	if len(d.Schema.Capabilities) == 0 {
		return nil
	}
	keys := make([]string, 0, len(d.Schema.Capabilities))
	for k := range d.Schema.Capabilities {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// MappedFieldKeys 返回 schema.fields 里真正生效的统一字段键。
//
// 「写成空字符串」是定义文件里表示「该字段在本 agent 上不存在」的写法，
// 这类键不算映射——界面不该为它渲染一个写不进去的输入框。
func (d *Definition) MappedFieldKeys() []string {
	if len(d.Schema.Fields) == 0 {
		return nil
	}
	keys := make([]string, 0, len(d.Schema.Fields))
	for k, path := range d.Schema.Fields {
		if strings.TrimSpace(path) == "" {
			continue
		}
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// Validate 检查一份插件定义是否可用。
func (d *Definition) Validate() error {
	if strings.TrimSpace(d.ID) == "" {
		return fmt.Errorf("%s", global.T("err_plugin_no_id"))
	}
	if strings.TrimSpace(d.Name) == "" {
		return fmt.Errorf("%s", global.T("err_plugin_no_name", d.ID))
	}
	if len(d.Source.Paths) == 0 {
		return fmt.Errorf("%s", global.T("err_plugin_no_paths", d.ID))
	}
	if len(d.Schema.Fields) == 0 {
		return fmt.Errorf("%s", global.T("err_plugin_no_fields", d.ID))
	}
	for _, f := range RequiredFields {
		if p, ok := d.Schema.Fields[f]; !ok || strings.TrimSpace(p) == "" {
			return fmt.Errorf("%s", global.T("err_plugin_missing_field", d.ID, f))
		}
	}
	for k := range d.Schema.Fields {
		if !slices.Contains(UnifiedFields, k) {
			return fmt.Errorf("%s", global.T("err_plugin_unknown_field", d.ID, k))
		}
	}
	switch format(d.Source.Format) {
	case "json", "jsonc":
	default:
		return fmt.Errorf("%s", global.T("err_plugin_bad_format", d.ID, d.Source.Format))
	}
	return nil
}

func format(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "json"
	}
	return s
}

// ResolvePaths 返回当前操作系统下该插件模型配置文件的候选绝对路径。
func (d *Definition) ResolvePaths(goos string) []string {
	raw := append([]string{}, d.Source.Paths[goos]...)
	raw = append(raw, d.Source.Paths["default"]...)

	out := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, p := range raw {
		p = ExpandPath(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// ExpandPath 展开路径中的 %VAR%（Windows 风格）、$VAR / ${VAR} 与开头的 ~。
func ExpandPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = expandWindowsVars(p)
	p = os.ExpandEnv(p)
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			p = home + p[1:]
		}
	}
	return p
}
