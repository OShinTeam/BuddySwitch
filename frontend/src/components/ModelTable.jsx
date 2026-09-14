import { useMemo, useState } from 'react'
import { Chip, Icons, IconButton, Spinner, Switch } from './ui'
import { activeCapabilities, capLabel, maskSecret, pluginColor, probeMeta, shortenEndpoint } from '../lib/util'

function StatusCell({ model, probing, onProbe, t }) {
  const meta = probeMeta(model.probe?.status)
  if (probing) {
    return (
      <span className="inline-flex items-center gap-1.5 text-[11px] text-indigo-600">
        <Spinner className="h-3 w-3" />
        {t('probe_running')}
      </span>
    )
  }
  return (
    <button
      type="button"
      onClick={() => onProbe(model)}
      title={model.probe?.message || t('btn_test_single')}
      className="inline-flex items-center gap-1.5 rounded px-1 py-0.5 transition-colors hover:bg-slate-100"
    >
      <span className={`h-2 w-2 shrink-0 rounded-full ${meta.dot}`} />
      <span className={`text-[11px] ${meta.text}`}>{t(meta.key)}</span>
      {model.probe?.latency_ms ? (
        <span className="text-[11px] text-slate-400">{model.probe.latency_ms}ms</span>
      ) : null}
    </button>
  )
}

/** 上游分组标题行：一眼看出「这是谁家的、用哪把密钥、下面挂了几个模型」。 */
function GroupHeader({ group, open, mode, showPlugin, pluginMap, t, onToggle, onFetch, onApply, onAddModel }) {
  const origins = showPlugin
    ? [...new Set((group.models || []).map((m) => pluginMap.get(m.plugin_id)?.name || m.plugin_id))]
    : []

  return (
    <tr>
      <td colSpan={6} className="border-b border-slate-200 bg-slate-50/80 px-2 py-1.5">
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={onToggle}
            className="flex h-5 w-5 shrink-0 items-center justify-center rounded text-slate-400 transition-colors hover:bg-slate-200 hover:text-slate-600"
          >
            <Icons.chevron className={`h-3.5 w-3.5 transition-transform ${open ? 'rotate-90' : ''}`} />
          </button>

          <Icons.server className="h-3.5 w-3.5 shrink-0 text-slate-400" />

          <span className="shrink-0 text-[12px] font-semibold text-slate-700">{group.label}</span>

          <Chip className="shrink-0 bg-white text-slate-500 ring-slate-200">
            {(group.models || []).length} {t('plugin_models_suffix')}
          </Chip>

          <span className="min-w-0 truncate font-mono text-[11px] text-slate-400" title={group.url}>
            {group.url}
          </span>

          {group.apiKey ? (
            <Chip className="shrink-0 bg-white text-slate-400 ring-slate-200">
              {maskSecret(group.apiKey)}
            </Chip>
          ) : null}

          {origins.length > 0 ? (
            <span className="shrink-0 text-[11px] text-slate-400">{origins.join(' · ')}</span>
          ) : null}

          <div className="ml-auto flex shrink-0 items-center gap-1">
            {mode === 'catalog' ? (
              <button
                type="button"
                onClick={onAddModel}
                title={t('manual_add_why')}
                className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[11px] font-medium text-slate-600 transition-colors hover:bg-white hover:text-indigo-600"
              >
                <Icons.plus className="h-3.5 w-3.5" />
                {t('manual_add_action')}
              </button>
            ) : null}
            <button
              type="button"
              onClick={onFetch}
              className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[11px] font-medium text-slate-600 transition-colors hover:bg-white hover:text-indigo-600"
            >
              <Icons.download className="h-3.5 w-3.5" />
              {t('fetch_action')}
            </button>
            <button
              type="button"
              onClick={onApply}
              className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[11px] font-medium text-indigo-600 transition-colors hover:bg-indigo-50"
            >
              <Icons.forward className="h-3.5 w-3.5" />
              {t('btn_apply_to_agent')}
            </button>
          </div>
        </div>
      </td>
    </tr>
  )
}

/**
 * 模型表格。
 *
 * 分组由调用方给：缓存视图给的是「上游目录」，Agent 视图给的是「该 Agent 配置里
 * 按接入点分组的模型」。两种视图共用这张表，差别只在某些列/按钮是否有意义。
 */
export default function ModelTable({
  t,
  mode = 'agent',
  groups,
  plugins,
  probing,
  showPlugin,
  onToggle,
  onProbe,
  onEdit,
  onDelete,
  onFetch,
  onApply,
  onAddModel,
  searching = false,
}) {
  // 默认全部收起：上游一多、每个上游下又挂着十几个模型，全展开会把页面拉得很长。
  const [expanded, setExpanded] = useState(() => new Set())
  const pluginMap = useMemo(() => new Map(plugins.map((p) => [p.id, p])), [plugins])

  const toggleGroup = (key) =>
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })

  const isCatalog = mode === 'catalog'

  return (
    <div className="min-w-full">
      <table className="w-full border-separate border-spacing-0 text-left">
        <thead className="sticky top-0 z-10 bg-white">
          <tr className="text-[11px] font-medium uppercase tracking-wide text-slate-400">
            <th className="w-8 border-b border-slate-200 px-2 py-2" />
            <th className="w-[104px] border-b border-slate-200 px-2 py-2">{t('col_status')}</th>
            <th className="border-b border-slate-200 px-3 py-2">{t('col_model')}</th>
            <th className="w-[150px] border-b border-slate-200 px-3 py-2">{t('col_provider')}</th>
            <th className="w-[180px] border-b border-slate-200 px-3 py-2">{t('col_endpoint')}</th>
            <th className="w-[92px] border-b border-slate-200 px-3 py-2 text-right">
              {t('col_actions')}
            </th>
          </tr>
        </thead>

        {groups.map((group) => {
          // 搜索时强制展开，否则搜了半天看到的还是一排收起的标题；
          // 空分组也强制展开，因为那行提示本身就是「去拉取 / 手动添加」的入口。
          const empty = (group.models || []).length === 0
          const open = searching || empty || expanded.has(group.key)
          return (
            <tbody key={group.key}>
              <GroupHeader
                group={group}
                open={open}
                mode={mode}
                showPlugin={showPlugin}
                pluginMap={pluginMap}
                t={t}
                onToggle={() => toggleGroup(group.key)}
                onFetch={() => onFetch(group)}
                onApply={() => onApply(group)}
                onAddModel={() => onAddModel(group)}
              />
              {open && (group.models || []).length === 0 ? (
                <tr>
                  <td
                    colSpan={6}
                    className="border-b border-slate-100 bg-white px-3 py-3 text-center text-[11px] text-slate-400"
                  >
                    {t(mode === 'catalog' ? 'cache_group_empty' : 'agent_group_empty')}
                  </td>
                </tr>
              ) : null}
              {open
                ? (group.models || []).map((model) => {
                    const plugin = pluginMap.get(model.plugin_id)
                    const caps = activeCapabilities(model)
                    return (
                      <tr key={model.uid} className="group bg-white transition-colors hover:bg-slate-50/70">
                        <td className="border-b border-slate-100 px-2 py-1.5">
                          {isCatalog ? null : (
                            <Switch
                              checked={model.enabled}
                              onChange={(next) => onToggle(model, next)}
                              title={t('model_enabled')}
                            />
                          )}
                        </td>
                        <td className="border-b border-slate-100 px-2 py-1.5">
                          <StatusCell
                            model={model}
                            probing={probing.has(model.uid)}
                            onProbe={onProbe}
                            t={t}
                          />
                        </td>
                        <td className="border-b border-slate-100 px-3 py-1.5">
                          <div className="flex items-center gap-1.5">
                            {showPlugin ? (
                              <span
                                className="h-2 w-2 shrink-0 rounded-full"
                                style={{ backgroundColor: pluginColor(plugin?.color) }}
                                title={plugin?.name || model.plugin_id}
                              />
                            ) : null}
                            <span className="truncate text-[13px] font-medium text-slate-800">
                              {model.display_name || model.id}
                            </span>
                          </div>
                          <p className="mt-0.5 truncate font-mono text-[11px] text-slate-400">
                            {model.id}
                          </p>
                        </td>
                        <td className="border-b border-slate-100 px-3 py-1.5">
                          <div className="flex flex-wrap items-center gap-1">
                            {model.provider ? (
                              <Chip className="bg-slate-50 text-slate-600 ring-slate-200">
                                {model.provider}
                              </Chip>
                            ) : null}
                            {caps.map((cap) => (
                              <Chip key={cap} className="bg-indigo-50 text-indigo-600 ring-indigo-100">
                                {capLabel(cap, t)}
                              </Chip>
                            ))}
                          </div>
                        </td>
                        <td className="border-b border-slate-100 px-3 py-1.5">
                          <span className="font-mono text-[11px] text-slate-400" title={model.base_url}>
                            {shortenEndpoint(model.base_url, 26)}
                          </span>
                        </td>
                        <td className="border-b border-slate-100 px-3 py-1.5">
                          <div className="flex items-center justify-end gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
                            <IconButton title={t('btn_test_single')} onClick={() => onProbe(model)}>
                              <Icons.beaker className="h-3.5 w-3.5" />
                            </IconButton>
                            {isCatalog ? (
                              <IconButton
                                title={t('cache_remove')}
                                className="hover:bg-rose-50 hover:text-rose-600"
                                onClick={() => onDelete(group, model)}
                              >
                                <Icons.trash className="h-3.5 w-3.5" />
                              </IconButton>
                            ) : (
                              <>
                                <IconButton title={t('btn_edit')} onClick={() => onEdit(model)}>
                                  <Icons.pencil className="h-3.5 w-3.5" />
                                </IconButton>
                                <IconButton
                                  title={t('btn_delete')}
                                  className="hover:bg-rose-50 hover:text-rose-600"
                                  onClick={() => onDelete(group, model)}
                                >
                                  <Icons.trash className="h-3.5 w-3.5" />
                                </IconButton>
                              </>
                            )}
                          </div>
                        </td>
                      </tr>
                    )
                  })
                : null}
            </tbody>
          )
        })}
      </table>
    </div>
  )
}
