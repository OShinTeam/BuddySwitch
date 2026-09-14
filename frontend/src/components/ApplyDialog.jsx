import { useEffect, useMemo, useState } from 'react'
import { Button, Drawer, Icons, LAYERS, Select } from './ui'
import { activeCapabilities, capLabel, maskSecret } from '../lib/util'

/**
 * 把一批模型写入指定 Agent 的对话框。
 *
 * 模型列表里的「上游分组」和上游目录里的条目都复用它——
 * 两边要回答的是同一个问题：这些模型，搬到哪个 Agent 里去。
 */
export default function ApplyDialog({ open, source, plugins, t, onClose, onConfirm }) {
  const candidates = useMemo(() => plugins.filter((p) => p.loaded), [plugins])
  const models = source?.models || []

  const [target, setTarget] = useState('')
  const [selected, setSelected] = useState(() => new Set())
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!open) return
    setSelected(new Set(models.map((m) => m.id)))
    setError('')
    setBusy(false)
    // 默认选第一个有配置文件位置的 Agent，省掉一次点击
    const firstWithSource = candidates.find((p) => p.source_file) || candidates[0]
    setTarget(firstWithSource?.id || '')
  }, [open, source])

  const toggle = (id) =>
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })

  const allSelected = models.length > 0 && selected.size === models.length

  const submit = async () => {
    if (!target || selected.size === 0) return
    setBusy(true)
    setError('')
    try {
      await onConfirm({
        pluginID: target,
        models: models.filter((m) => selected.has(m.id)),
      })
    } catch (err) {
      setError(err?.message || String(err))
      setBusy(false)
    }
  }

  return (
    <Drawer
      open={open}
      onClose={onClose}
      width="w-[520px]"
      layer={LAYERS.raised}
      title={t('apply_title')}
      subtitle={source?.label ? `${source.label} · ${source.url}` : undefined}
      footer={
        <>
          <Button variant="ghost" onClick={onClose} disabled={busy}>
            {t('btn_cancel')}
          </Button>
          <Button
            variant="primary"
            icon={Icons.forward}
            onClick={submit}
            disabled={busy || !target || selected.size === 0}
          >
            {t('btn_apply')} {selected.size ? `(${selected.size})` : ''}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <div className="rounded-lg bg-slate-50 px-3 py-2.5">
          <div className="flex items-center gap-2">
            <span className="text-[11px] text-slate-500">{t('col_endpoint')}</span>
            <span className="min-w-0 flex-1 truncate font-mono text-[11px] text-slate-600">
              {source?.url}
            </span>
          </div>
          <div className="mt-1 flex items-center gap-2">
            <span className="text-[11px] text-slate-500">{t('model_api_key')}</span>
            <span className="font-mono text-[11px] text-slate-600">
              {source?.apiKey ? maskSecret(source.apiKey) : '—'}
            </span>
          </div>
        </div>

        <label className="block">
          <span className="mb-1.5 block text-xs font-medium text-slate-600">
            {t('apply_target_agent')}
          </span>
          <Select value={target} onChange={(event) => setTarget(event.target.value)}>
            {candidates.map((plugin) => (
              <option key={plugin.id} value={plugin.id}>
                {plugin.name}
                {plugin.source_ready ? '' : ` (${t('plugin_source_missing')})`}
              </option>
            ))}
          </Select>
        </label>

        <div>
          <div className="mb-2 flex items-center justify-between">
            <span className="text-xs font-medium text-slate-600">
              {t('apply_select_models')} · {selected.size}/{models.length}
            </span>
            <button
              type="button"
              onClick={() => setSelected(allSelected ? new Set() : new Set(models.map((m) => m.id)))}
              className="text-[11px] font-medium text-indigo-600 hover:underline"
            >
              {allSelected ? t('apply_select_none') : t('apply_select_all')}
            </button>
          </div>

          <ul className="max-h-64 space-y-1 overflow-y-auto rounded-lg p-1.5 ring-1 ring-slate-200">
            {models.map((model) => {
              const caps = activeCapabilities(model)
              return (
                <li key={model.id}>
                  <label className="flex cursor-pointer items-center gap-2.5 rounded-md px-2 py-1.5 hover:bg-slate-50">
                    <input
                      type="checkbox"
                      checked={selected.has(model.id)}
                      onChange={() => toggle(model.id)}
                      className="h-3.5 w-3.5 shrink-0 accent-indigo-600"
                    />
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-[12px] text-slate-700">
                        {model.display_name || model.id}
                      </span>
                      <span className="block truncate font-mono text-[10px] text-slate-400">
                        {model.id}
                      </span>
                    </span>
                    {caps.length > 0 ? (
                      <span className="shrink-0 text-[10px] text-slate-400">
                        {caps.map((cap) => capLabel(cap, t)).join(' · ')}
                      </span>
                    ) : null}
                  </label>
                </li>
              )
            })}
          </ul>
        </div>

        <p className="text-[11px] leading-relaxed text-slate-400">{t('apply_hint')}</p>

        {error ? (
          <p className="rounded-md bg-rose-50 px-3 py-2 text-[12px] leading-relaxed text-rose-700 ring-1 ring-inset ring-rose-200">
            {error}
          </p>
        ) : null}
      </div>
    </Drawer>
  )
}
