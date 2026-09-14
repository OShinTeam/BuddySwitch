<p align="center">
  <img src="./build/appicon.png" alt="BuddySwitch" width="300"/>
</p>

# BuddySwitch

**跨 Agent 的模型管理器**。它维护一份你自己的上游与模型库 —— 从上游直接拉取模型清单、
缓存到本地、测试可用性，再一键写入 WorkBuddy、CodeBuddy 或任何其它 agent 的配置。

一次配置，多处复用：同一个上游下的模型，不必在每个 agent 里重复录入一遍。

---

## 信息架构

侧栏分成两块，职责不重叠：

| 视图 | 数据来源 | 用来做什么 |
|------|----------|-----------|
| **本地模型** | `data/upstreams.json`（BuddySwitch 自己维护） | 主视图。新增上游、拉取模型清单、手动补充、测试、再推到任意 agent |
| **各 Agent** | 该 agent 自己的 `models.json` | 看它当前配了什么；编辑 / 删除会直接写回它的配置 |

「本地模型」是按**上游**分组的（同一个接入点 + 同一把密钥算一个上游），
默认全部收起以免页面过长；搜索时会自动展开。

> 为什么不做「所有 Agent 模型的汇总视图」：多个 agent 常共用同一个上游，
> 汇总只会让同一批模型重复出现。以 Agent 为轴没有意义，以**上游**为轴才有。

---

## 核心能力

| 能力 | 说明 |
|------|------|
| 向上游拉取 | 用上游的密钥直接问它有哪些模型（`GET /models`），而不是读本地配置 —— 所以还没被任何 agent 用上的新模型也能发现 |
| 本地模型缓存 | 拉回来的清单存进本地库，跨 agent 复用；可手工维护、可导出 |
| 手动补充 | 有些上游的模型清单接口返回不全，拉不到的可以直接填 id |
| 一键应用 | 把某个上游下的模型勾选后写入指定 agent，整批只写一次盘、只留一份快照 |
| 可用性测试 | 按定义构造探针请求，测出可用 / 密钥无效 / 不可达，并记录延迟 |
| 快照与还原 | 每次写入前自动留快照，按份数轮转，任意一次修改都能一键回退 |
| 插件化接入 | 新增一个 agent 只需要写一份 JSON 定义，**不需要改一行代码** |

---

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go + Wails v2（无边框窗口、原生对话框、事件推送） |
| 前端 | React 19 + Tailwind CSS 4 + Vite 7 |
| 国际化 | 内嵌多语言包，缺失文案自动回落到默认语言；后端错误与探测结论同样走语言包 |
| 日志 | Logrus，按启动时间归档，自动清理旧文件（日志文案固定中文，便于排障） |

---

## 项目结构

```
├── upstream/               # 本地模型库：上游 + 模型清单（data/upstreams.json）
├── plugin/                 # 插件体系（Agent 接入的核心）
│   ├── types.go            #   插件定义、统一模型、探测与清单规格
│   ├── document.go         #   按 schema 解析 / 写回 agent 原生配置
│   ├── probe.go            #   模板化探针请求与结果判定
│   ├── listing.go          #   向上游索要模型清单
│   ├── registry.go         #   内置 + 磁盘插件的加载与查询
│   ├── util.go             #   JSONC 去注释、路径变量展开
│   └── builtin/            #   内置插件定义（workbuddy / codebuddy）
├── backup/                 # 配置快照：留档、轮转、还原
├── store/                  # BuddySwitch 自身的状态（启停、探测结果）
├── service/                # Wails 绑定层，业务编排
├── api/                    # 不依赖 Wails 上下文的业务函数
├── global/                 # 配置、语言包、日志
├── lang/                   # 内嵌语言包（zh-CN / en-US / ja-JP / ko-KR）
├── scripts/                # 开发期脚本（语言包完整性校验等）
├── frontend/               # React + Tailwind 前端
├── main.go
└── wails.json
```

运行期会在工作目录产生：

```
config.json               # 应用设置
plugins/                  # 用户自定义或覆盖用的插件定义
data/upstreams.json       # 本地模型库（含上游密钥，注意保管）
data/state.json           # 启停状态与探测结果
data/backups/<插件id>/     # 配置快照
logs/                     # 日志
```

---

## 快速开始

环境要求：Go ≥ 1.21、Node.js ≥ 18、[Wails CLI](https://wails.io/docs/gettingstarted/installation) v2。

```bash
# 安装 Wails CLI（如果还没有）
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# 安装前端依赖
cd frontend && npm install && cd ..

# 开发模式（热重载）
wails dev

# 构建生产版本
wails build
```

> 本项目使用 Wails 的构建链路，不通过 `go build` 单独编译。

---

## 插件设计

### 一句话概括

**一个插件 = 一份「该 Agent 的模型配置格式定义」。**

它描述的不是行为，而是格式，分四段：

| 段 | 回答的问题 |
|----|-----------|
| `source` | 配置文件**放在哪** |
| `schema` | 文件**长什么样**、怎么读写 |
| `probe` | 怎么判断这个模型**还能不能用** |
| `listing` | 怎么向上游**索要模型清单**（可选，缺失时用内置的 OpenAI 兼容兜底） |

内置插件随二进制分发（`plugin/builtin/*.json`）；磁盘上的 `plugins/*.json` 可以
**覆盖**同 id 的内置定义，也可以**新增**插件。因此修正一个字段映射不需要重新编译。

> 本地模型库本身**不依赖任何插件**：`listing` / `probe` 都有 OpenAI 兼容的兜底规格
> （`plugin.DefaultListing()` / `plugin.DefaultProbe()`），所以哪怕一个插件都没有，
> 也能新增上游、拉清单、测模型。

### 完整示例

```jsonc
{
  "id": "my-agent",                    // 唯一标识，同时是快照目录名
  "name": "My Agent",                  // 界面显示名
  "description": "一句话说明",
  "vendor": "Your Company",
  "color": "#8B5CF6",                  // 界面上的标识色
  "version": "1.0.0",
  "order": 100,                        // 侧栏排序，越小越靠前
  "enabled": true,                     // 缺省启用状态

  // ① 去哪找配置文件
  "source": {
    "format": "json",                  // json | jsonc（jsonc 会先剥离注释）
    "paths": {
      "windows": ["%APPDATA%\\MyAgent\\models.json", "%USERPROFILE%\\.my-agent\\models.json"],
      "darwin":  ["~/Library/Application Support/MyAgent/models.json"],
      "linux":   ["~/.config/my-agent/models.json"],
      "default": []
    }
  },

  // ② 文件结构是什么样、怎么读写
  "schema": {
    // 模型数组的候选位置，按顺序尝试；"" 表示文件根节点本身就是数组。
    // 官方文档同时存在这两种结构，所以这里给一个列表最稳妥。
    "container": ["", "models"],
    "fields": {                        // 统一字段 -> 原生字段路径（支持 a.b.c 嵌套）
      "id": "id",
      "display_name": "name",
      "provider": "vendor",
      "base_url": "url",
      "api_key": "apiKey",
      "enabled": "enabled",
      "description": "description",
      "tags": "tags"
    },
    // 能力标记：键名自定义，值为原生布尔字段，界面按能力名展示标签
    "capabilities": {
      "tool_call": "supportsToolCall",
      "images": "supportsImages",
      "reasoning": "supportsReasoning"
    },
    // 可选的「可见模型清单」：文件里已存在该字段时，
    // 新增模型会把 id 追加进去、删除模型会摘掉，避免「加了却在 agent 里看不见」
    "sync_list": "availableModels",
    "defaults": { "vendor": "Custom" }, // 新增模型时一并写入的固定字段
    "indent": "  "
  },

  // ③ 怎么判断模型是否可用
  "probe": {
    "type": "openai",                  // openai | anthropic | custom，仅作标注
    "endpoint": "{{base_url}}",
    // 渲染后的地址若不以该后缀结尾就补上——用来兼容
    // 「url 有时是完整接口、有时只是 base」的配置
    "ensure_path": "/chat/completions",
    "method": "POST",
    "headers": { "Authorization": "Bearer {{api_key}}" },
    "body": {
      "model": "{{id}}",
      "messages": [{ "role": "user", "content": "ping" }],
      "max_tokens": 1,
      "stream": false
    },
    "timeout_ms": 15000,
    "expect_status": [200, 299]        // 可选；留空默认 2xx 算成功
  },

  // ④ 怎么向上游要模型清单（可选）
  "listing": {
    "endpoint": "{{base_url}}",
    // 先把地址尾部的旧后缀摘掉，再补上新后缀。
    // 因为 {{base_url}} 可能是 .../v1/chat/completions，也可能是 .../v1，
    // 而清单接口通常要的是 .../v1/models。
    "strip_suffix": "/chat/completions",
    "ensure_path": "/models",
    "method": "GET",
    "headers": { "Authorization": "Bearer {{api_key}}" },
    "timeout_ms": 20000,
    "array_path": "data",              // OpenAI 的 {"data":[...]}；留空表示根节点即数组
    "id_field": "id",
    "name_field": "id",
    "note_field": "owned_by"
  }
}
```

### 字段说明

**`source.paths`** —— 键为平台名（`windows` / `darwin` / `linux`），值为按优先级排列的
候选路径。路径支持三种写法，会在加载时展开：

- `%APPDATA%` / `%USERPROFILE%` 这类 Windows 变量
- `$HOME` / `${HOME}` 这类 shell 变量
- 开头的 `~`

同一插件下第一个真实存在的路径会被采用；都不存在时取首选路径，用于首次创建。
用户也可以在「设置」里为某个插件单独指定路径，覆盖这里的默认值。

**`schema.container`** —— 模型数组的**候选**位置，按顺序尝试，命中第一个能取出数组的路径：

- `""`（空串）表示「文件根节点本身就是数组」；
- `"models"` 取 `{"models": [...]}`；
- `"a.b"` 取 `{"a": {"b": [...]}}`。

写成字符串或数组都可以：`"container": "models"` 等价于 `"container": ["models"]`。
之所以支持候选列表，是因为现实里同一个 agent 往往有**多种被官方文档承认的结构**——
WorkBuddy 就同时支持根节点数组和 `{"models": [...]}`。

**`schema.fields`** —— 统一字段到原生字段的映射。`id` 是唯一必须映射的字段，
它决定了两边如何配对。值为空字符串表示该字段在原生文件里不存在。

| 统一字段 | 含义 |
|----------|------|
| `id` | 模型标识（必需）。**是发给上游的标识，不是给人看的名字** |
| `display_name` | 展示名称，缺省回落到 `id`；agent 下拉框里显示的就是它 |
| `provider` | 提供商 |
| `base_url` | 接入点 |
| `api_key` | 密钥 |
| `enabled` | 启用开关；不映射时状态只存在 BuddySwitch 本地 |
| `description` | 备注 |
| `tags` | 标签数组 |

> `description` 与 `tags` 是**按需出现**的：只有插件真的映射了它们，编辑器才会显示
> 对应的输入框。没映射却给出输入框，等于让用户填了也没地方存——那两个内置 agent
> 的配置里都没有这两个字段，所以你在它们的编辑器里看不到这两栏。

写入语义分两种，取决于调用方掌握多少信息：

- **编辑器保存**（含单独切换启用开关）：映射到的字段按你填的内容整体覆写，
  把标签清空就是真的清空；
- **从缓存把模型搬进某个 agent**：只写这次确实带过去的字段。搬运方并不知道目标
  原本的备注、标签，所以不会顺手把它们抹掉。

**`schema.capabilities`** —— 能力标记映射。键名由插件自定义（界面内置了
`tool_call` / `images` / `reasoning` 三个的中文文案，其它键名直接回显），值为原生布尔字段路径。

**`schema.sync_list`** —— 可选的「可见模型清单」。有些 agent 用单独一个数组控制
下拉框里显示哪些模型（WorkBuddy 的 `availableModels` 就是）。声明它之后，
新增模型会把 id 追加进去、删除模型会摘掉，避免出现「在 BuddySwitch 里加了模型，
agent 里却看不见」。**只在文件原本就存在该字段时才动它**，且不做全量对齐——
你可能故意把某个模型排除在外。

**`probe`** —— 探针。`endpoint` / `headers` / `body` 里的 `{{...}}` 占位符会被替换为
当前模型的字段值，可用变量：`id`、`display_name`、`provider`、`base_url`、`api_key`、`plugin_id`。

判定规则：

| 结果 | 触发条件 |
|------|----------|
| 可用 | 状态码落在 `expect_status`（默认 2xx） |
| 密钥无效 | 401 / 403 |
| 不可用 | 其它非预期状态码，会带上响应体里的 `message` |
| 连接失败 | 网络错误、超时、地址非法 |

**`listing`** —— 模型清单接口。`strip_suffix` 与 `ensure_path` 一前一后地规范化地址，
这是为了兼容「同一个 `{{base_url}}`，聊天接口要 `/chat/completions`、清单接口要 `/models`」
这种实际情况。解析时按 `array_path` 找到数组，再按 `id_field` / `name_field` / `note_field`
取字段；数组元素也可以直接是字符串。

### 接入一个新 Agent

1. 在界面左下角点「导出示例插件定义」，得到 `plugins/example-agent.json`；
2. 照着改成目标 agent 的真实路径与字段映射；
3. 点「重新加载插件」，新插件立即出现在侧栏。

整个过程不需要动代码，也不需要重新编译。

---

## 内置插件

| 插件 | 配置文件位置 | 结构 |
|------|--------------|------|
| **WorkBuddy** | `~/.workbuddy/models.json`（Windows 下 `%USERPROFILE%\.workbuddy\models.json`） | `container: ["", "models"]`，同时兼容根数组与 `{"models": [...]}` |
| **CodeBuddy** | `~/.codebuddy/models.json` | `container: ["models", ""]` |

两者的字段映射相同，都按官方文档来：

| 统一字段 | 原生字段 | 说明 |
|----------|----------|------|
| `id` | `id` | 模型标识，也是请求体里的 `model` |
| `display_name` | `name` | 下拉框里显示的就是它 |
| `provider` | `vendor` | 如 `Custom` / `OpenAI` |
| `base_url` | `url` | **完整接口地址**，一般以 `/chat/completions` 结尾 |
| `api_key` | `apiKey` | 密钥 |
| 能力 | `supportsToolCall` / `supportsImages` / `supportsReasoning` | 见 `capabilities` |
| — | `useCustomProtocol` | 关闭时 agent 会校验并补全 `url`，所以 `url` 也可能是 base；探针用 `ensure_path` 对齐这个行为 |

启用开关（`enabled`）**没有映射**——这两个 agent 的配置里本来就没有这个字段，
所以勾选状态只保存在 BuddySwitch 本地，不会往对方配置里塞一个它不认识的键。

---

## 备份与还原

写入 agent 配置是一件有风险的事，所以每次**写入之前**都会先留一份快照：

```
data/backups/workbuddy/models.json.2026-09-14_22-05-31.417.bak
```

- 快照目录按插件隔离，文件名内嵌毫秒级时间戳；
- 保留份数在「设置」里可调（默认 10，范围 1–100），超出后自动清理最旧的；
- 界面上可以查看快照列表、预览内容、还原到任意版本；在「本地模型」视图下也能打开，
  面板里切换要查看哪个 agent；
- **还原本身也会先备份当前内容**，所以还原动作同样可以撤销。

写入采用「先写临时文件再改名」的方式，中途失败不会破坏原配置；
未被插件映射到的字段会被完整保留，BuddySwitch 只碰它认识的那几个字段。

---

## 安全说明

- 所有操作都在本机进行，不发送任何遥测；
- `api_key` 会出现在两处：agent 自己的配置，以及 BuddySwitch 的 `data/upstreams.json`。
  后者是本地模型库为了跨 agent 复用而必须保存的，**是明文**，请和 agent 配置同等对待：
  别把它放进同步目录或版本库，共享机器上注意该文件的读写权限；
- 密钥仅用于构造请求头，日志里不会输出其内容；界面上以打码形式展示；
- 路径类接口都会校验，拒绝 `..` 与路径分隔符，防止越权读写。

---

## License

MIT
