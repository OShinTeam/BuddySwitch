# wails-template

Wails v2 + Vue 3 + Element Plus 通用桌面应用开发模板

## 技术栈

| 层级 | 技术 | 说明 |
|------|------|------|
| 后端 | Go + Wails v2 | 桌面应用框架，提供原生 API 调用能力 |
| 前端 | Vue 3 + Element Plus | Composition API + 企业级 UI 组件库 |
| 国际化 | i18n | 内嵌多语言包，支持运行时切换 |
| 日志 | Logrus | 彩色格式化日志，按启动时间归档 |

## 项目结构

```
├── api/                        # API 层
├── build/                      # 构建资源
├── frontend/                   # Vue 3 前端
│   ├── src/                    
│   │   ├── components/         # Vue 组件
│   │   │   ├── HeaderBar.vue   # 自定义标题栏
│   │   │   └── Test.vue        # 功能演示页面
│   │   ├── composables/        # Vue 组合式函数
│   │   │   └── useI18n.js      # i18n
│   │   ├── App.vue             # 根组件
│   │   └── main.js             # 入口文件
│   └── wailsjs/                # Wails 自动生成的 JS 绑定
├── global/                     # 全局配置
│   ├── init.go                 # 初始化入口
│   ├── config.go               # 配置缓存
│   ├── lang.go                 # 语言包加载
│   └── logger.go               # 日志系统
├── lang/                       # 语言包目录
│   ├── default/                # 简体中文（默认）
│   │   ├── info.json           # 语言元信息
│   │   └── textmap.json        # 翻译文本
│   └── en-US/                  # English
│       ├── info.json
│       └── textmap.json
├── logger/                     # 日志系统
├── service/                    # 业务逻辑层
│   ├── router.go               # Wails 绑定方法
│   └── temp.go                 # 示例函数实现
├── main.go                     # 程序入口
├── go.mod                    
├── go.sum
└── wails.json                  # Wails 配置
```

## 快速开始

### 环境要求

- [Go](https://go.dev/dl/) >= 1.21
- [Node.js](https://nodejs.org/) >= 18
- [Wails CLI](https://wails.io/docs/gettingstarted/installation) v2

```bash
# 安装 Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### 运行项目

```bash
# 安装前端依赖
cd frontend && npm install && cd ..

# 开发模式（热重载）
wails dev -tags debug

# 构建生产版本
wails build
```

## 使用模板开发

### 1. 修改程序名称

需要修改以下文件中的程序名：

| 文件 | 修改项 |
|------|--------|
| `wails.json` | `name` 和 `outputfilename` |
| `main.go` | `options.App.Title` |
| `lang/*/textmap.json` | `app_name` 的值 |

### 2. 添加后端功能

**第一步**：在 `service/` 目录下新建 Go 文件，编写业务逻辑：

```go
// service/myfeature.go
package service

import "fmt"

func calculate(a, b int) int {
    return a + b
}
```

**第二步**：在 `service/router.go` 中注册为 Wails 绑定方法：

```go
// 在 router.go 中添加
func (a *App) Calculate(a, b int) int {
    return calculate(a, b)
}
```

**第三步**：重新生成前端绑定（或手动添加到 `frontend/wailsjs/go/service/App.js` 和 `App.d.ts`）：

```bash
wails generate module
```

### 3. 前端调用后端

```vue
<script setup>
import { ref } from 'vue'
import { Calculate } from '../../wailsjs/go/service/App'

const result = ref(0)

async function doCalculate() {
  result.value = await Calculate(1, 2)
}
</script>

<template>
  <div>计算结果: {{ result }}</div>
  <el-button @click="doCalculate">计算</el-button>
</template>
```

### 4. 后端向前端推送数据（事件系统）

```go
// 后端发送事件
import "github.com/wailsapp/wails/v2/pkg/runtime"

func (a *App) SendUpdate() {
    runtime.EventsEmit(a.ctx, "data-update", map[string]string{
        "status": "ok",
    })
}
```

```js
// 前端监听事件
import { EventsOn } from '../../wailsjs/runtime'

EventsOn('data-update', (data) => {
  console.log('收到更新:', data.status)
})
```

### 5. 添加新的语言

在 `lang/` 目录下创建新文件夹（如 `ja-JP`），包含两个文件：

**`lang/ja-JP/info.json`**：
```json
{
  "language_name": "日本語",
  "language_code": "ja-JP",
  "textmap_path": "textmap.json",
  "translation_progress": "0%",
  "translator": "",
  "last_updated": "",
  "version": "1.0.0"
}
```

**`lang/ja-JP/textmap.json`**：
```json
{
  "app_name": "アプリ名",
  "menu_main": "ホーム"
}
```

语言包会被自动扫描并添加到语言切换列表中。应用目录下的语言包优先于内嵌的语言包。

### 6. 使用日志系统

```go
import "wails-temp/global"

// 不同等级的日志
global.Log.Debug("调试信息")
global.Log.Info("一般信息")
global.Log.Warn("警告信息")
global.Log.Error("错误信息")

// 格式化输出
global.Log.Infof("用户 %s 登录", username)
```

日志文件保存在 `logs/` 目录下，按启动时间命名。

## 模板内置功能

| 功能 | 说明 | 演示位置 |
|------|------|----------|
| 自定义标题栏 | 无边框窗口 | 顶部标题栏 |
| 前后端数据交互 | Wails 绑定调用 Go 函数 | Greet 演示 |
| 事件系统 | 后端实时推送数据到前端 | 实时时间显示 |
| JSON 数据处理 | 后端生成 JSON，前端表格展示 | JSON 演示 |
| 语言切换 | 运行时切换多语言 | 语言切换面板 |
| 日志等级设置 | 动态调整日志输出级别 | 日志等级面板 |
| 日志文件查看 | 读取并展示日志文件内容 | 日志查看器 |
| 系统信息 | 获取 OS、架构、CPU 等信息 | 系统信息面板 |
| 文件对话框 | 系统原生打开/保存/目录选择 | 文件对话框面板 |
| 文件读写 | 读取和写入本地文件 | 文件读写面板 |
| 事件通知 | 后端向前端发送自定义通知 | 事件通知面板 |

## 兼容性信息

- [Wails](https://github.com/wailsapp/wails) v2.12.0
- Go >= 1.21
- Node.js >= 18

## License

MIT license
