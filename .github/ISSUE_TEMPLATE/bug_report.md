---
name: Bug 报告
about: 报告 BuddySwitch 中的问题以帮助我们改进
title: "[Bug] "
labels: bug
assignees: ''

---

## 问题描述

清晰简明地描述这个 Bug 是什么。

## 复现步骤

1. 打开视图 '...'（本地模型 / 各 Agent / 设置）
2. 执行操作 '...'（新增上游 / 拉取清单 / 测试 / 应用到 Agent / 快照还原 ...）
3. 出现错误

## 期望行为

你预期会发生什么。

## 实际行为

实际发生了什么（包括界面报错、异常截图等）。

## 环境信息

- **操作系统**: [例如 Windows 10 22H2 / Windows 11 / macOS 14]
- **BuddySwitch 版本**: [release 版本号，或 commit SHA]
- **安装方式**: [构建版 / 开发模式 wails dev]
- **涉及的 Agent**: [WorkBuddy / CodeBuddy / 自定义插件，不涉及可写「无」]

## 日志

`logs/` 目录下按启动时间归档，请附上对应时间段的日志文件（或关键报错片段）。

## 涉及的配置文件（注意脱敏）

如问题与写入 Agent 配置、上游拉取有关，请附上相关文件内容：

- `data/upstreams.json`（本地模型库）⚠️ **含密钥，粘贴前务必删除 `apiKey`**
- `data/state.json`（启停与探测结果）
- `data/backups/<插件id>/`（出问题前后的配置快照）
- 目标 Agent 的 `models.json` ⚠️ **同样注意删除密钥**

## 附加信息

- [ ] 我已删除日志与配置中所有密钥 / token 等敏感信息
- [ ] 我已搜索过现有 Issues，未发现重复

任何其他上下文、截图或录屏。
