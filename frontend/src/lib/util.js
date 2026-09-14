// 一些与界面无关的小工具函数。

/** 模型在探测结果里的唯一键，与后端 store.ModelKey 保持一致。 */
export function probeKey(pluginId, modelId) {
  return `${pluginId}/${modelId}`
}

/** 把 API Key 打码展示，真实值仍保留在内存里，保存时原样回传。 */
export function maskSecret(value) {
  if (!value) return '—'
  if (value.length <= 12) return '••••••'
  return `${value.slice(0, 5)}••••${value.slice(-4)}`
}

export function formatBytes(size) {
  if (!size) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let value = size
  let i = 0
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024
    i += 1
  }
  return `${value.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

/** 端点太长时省略中间部分，保证表格不被撑破。 */
export function shortenEndpoint(url, max = 42) {
  if (!url) return '—'
  if (url.length <= max) return url
  const head = url.slice(0, Math.floor(max * 0.6))
  const tail = url.slice(-Math.floor(max * 0.3))
  return `${head}…${tail}`
}

/** 从错误对象里取出一句可读的提示。 */
export function errorText(err) {
  if (!err) return ''
  if (typeof err === 'string') return err
  return err.message || String(err)
}

/** 探测状态 -> 展示信息。 */
export function probeMeta(status) {
  switch (status) {
    case 'ok':
      return { key: 'probe_ok', dot: 'bg-emerald-500', text: 'text-emerald-700', chip: 'bg-emerald-50 text-emerald-700 ring-emerald-200' }
    case 'auth':
      return { key: 'probe_auth', dot: 'bg-amber-500', text: 'text-amber-700', chip: 'bg-amber-50 text-amber-700 ring-amber-200' }
    case 'fail':
      return { key: 'probe_fail', dot: 'bg-rose-500', text: 'text-rose-700', chip: 'bg-rose-50 text-rose-700 ring-rose-200' }
    case 'error':
      return { key: 'probe_error', dot: 'bg-rose-500', text: 'text-rose-700', chip: 'bg-rose-50 text-rose-700 ring-rose-200' }
    default:
      return { key: 'probe_unknown', dot: 'bg-slate-300', text: 'text-slate-500', chip: 'bg-slate-50 text-slate-500 ring-slate-200' }
  }
}

/** 插件主色：允许插件定义里写任意 hex，取不到时回落到靛蓝。 */
export function pluginColor(color) {
  return /^#[0-9a-fA-F]{3,8}$/.test(color || '') ? color : '#6366F1'
}

/** 已知能力键的文案映射；未知键直接回显原键名，插件自定义的能力也能显示。 */
const CAPABILITY_LABELS = {
  tool_call: 'cap_tool_call',
  images: 'cap_images',
  reasoning: 'cap_reasoning',
}

export function capLabel(key, t) {
  const labelKey = CAPABILITY_LABELS[key]
  return labelKey ? t(labelKey) : key
}

/** 取出一个模型上已开启的能力键。 */
export function activeCapabilities(model) {
  const caps = model?.capabilities
  if (!caps) return []
  return Object.keys(caps).filter((key) => caps[key])
}

/**
 * 上游的展示名：主机名 + 路径。
 * “同一个上游”指的是同一套接入方式——同一个 url 加同一把密钥。
 * 没有接入点时返回一句本地化提示，所以需要传入 t。
 */
export function upstreamName(url, t) {
  if (!url) return t ? t('upstream_endpoint_missing') : ''
  try {
    const parsed = new URL(url)
    const path = parsed.pathname.replace(/\/+$/, '')
    return parsed.host + path
  } catch {
    return url
  }
}

/** 判断两条模型是否属于同一个上游（同一个接入点 + 同一把密钥）。 */
export function upstreamKey(model) {
  return `${model?.base_url || ''}\u0000${model?.api_key || ''}`
}

/**
 * 按上游把模型分组。
 *
 * 分组结果按「模型数量降序」排列，同数量时按名称——用得多、模型多的上游排在前面。
 * t 用于给「没有接入点」的分组起一个本地化的名字。
 */
export function groupByUpstream(models, t) {
  const groups = new Map()
  for (const model of models) {
    const key = upstreamKey(model)
    let group = groups.get(key)
    if (!group) {
      group = {
        key,
        url: model.base_url || '',
        apiKey: model.api_key || '',
        label: upstreamName(model.base_url, t),
        models: [],
      }
      groups.set(key, group)
    }
    group.models.push(model)
  }
  return [...groups.values()].sort(
    (a, b) => b.models.length - a.models.length || a.label.localeCompare(b.label),
  )
}
