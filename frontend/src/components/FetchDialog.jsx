import { useEffect, useMemo, useState } from 'react'
import { Button, Chip, Drawer, LAYERS, Select, Spinner } from './ui'
import { maskSecret } from '../lib/util'

/**
 * 拉取向导：向某个上游索要它的模型清单，并让用户挑选要写进哪个 Agent。
 *
 * 与「读取本地配置文件」不同，这里是用上游的密钥直接问上游有哪些模型，
 * 因此还没被任何 Agent 用上的新模型也能被发现。
 */
export default function FetchDialog({
  open,
  source,
  plugins,
  defaultTarget,
  intent = 'deploy',
  loading,
  error,
  remote,
  t,
  onClose,
  onRetry,
  onConfirm,
}) {
  const candidates = useMemo(() => plugins.filter((p) => p.loaded), [plugins])
  const existing = useMemo(() => new Set(source?.existingIds || []), [source])
  const isCache = intent === 'cache'

  const [picked, setPicked] = useState(() => new Set())
  const [target, setTarget] = useState('')
  const [busy, setBusy] = useState(false)
  const [applyError, setApplyError] = useState('')

  // 默认只勾「上游有、这个 Agent 还没有」的模型——那才是拉取要解决的问题。
  useEffect(() => {
    if (!open) return
    setApplyError('')
    setBusy(false)
    setTarget(defaultTarget || candidates[0]?.id || '')
  }, [open, defaultTarget, candidates])

  useEffect(() => {
    if (!open) return
    setPicked(new Set((remote || []).filter((m) => !existing.has(m.id)).map((m) => m.id)))
  }, [open, remote, existing])

  const allIDs = useMemo(() => (remote || []).map((m) => m.id), [remote])
  const freshIDs = useMemo(() => allIDs.filter((id) => !existing.has(id)), [allIDs, existing])
  const allPicked = allIDs.length > 0 && picked.size === allIDs.length

  const toggle = (id) =>
    setPicked((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })

  const submit = async () => {
    if (!isCache && !target) return
    if (picked.size === 0) return
    setBusy(true)
    setApplyError('')
    try {
      await onConfirm({
        pluginID: isCache ? '' : target,
        models: remote
          .filter((m) => picked.has(m.id))
          .map((m) => ({
            id: m.id,
            display_name: m.display_name || m.id,
            base_url: source.url,
            api_key: source.apiKey,
            capabilities: m.capabilities,
          })),
      })
    } catch (err) {
      setApplyError(err?.message || String(err))
      setBusy(false)
    }
  }

  return (
    <Drawer
      open={open}
      onClose={onClose}
      width="w-[560px]"
      layer={LAYERS.raised}
      title={t('fetch_title')}
      subtitle={source?.label ? `${source.label} · ${source.url}` : undefined}
      footer={
        <>
          <Button variant="ghost" onClick={onClose} disabled={busy}>
            {t('btn_cancel')}
          </Button>
          <Button
            variant="primary"
            onClick={submit}
            disabled={busy || loading || (!isCache && !target) || picked.size === 0}
          >
            {isCache ? t('fetch_cache') : t('fetch_write')} {picked.size ? `(${picked.size})` : ''}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <div className="rounded-lg bg-slate-50 px-3 py-2.5">
          <div className="flex items-center gap-2">
            <span className="shrink-0 text-[11px] text-slate-500">{t('model_base_url')}</span>
            <span className="min-w-0 flex-1 truncate font-mono text-[11px] text-slate-600">
              {source?.url}
            </span>
          </div>
          <div className="mt-1 flex items-center gap-2">
            <span className="shrink-0 text-[11px] text-slate-500">{t('model_api_key')}</span>
            <span className="font-mono text-[11px] text-slate-600">
              {source?.apiKey ? maskSecret(source.apiKey) : '—'}
            </span>
          </div>
        </div>

        {loading ? (
          <div className="flex items-center justify-center gap-2 py-10 text-xs text-slate-400">
            <Spinner className="h-4 w-4" />
            {t('fetch_loading')}
          </div>
        ) : error ? (
          <div className="space-y-3">
            <p className="rounded-md bg-rose-50 px-3 py-2.5 text-[12px] leading-relaxed text-rose-700 ring-1 ring-inset ring-rose-200">
              {error}
            </p>
            <Button icon={Spinner} onClick={onRetry} className="w-full">
              {t('fetch_retry')}
            </Button>
          </div>
        ) : (
          <>
            <div className="flex flex-wrap items-center gap-2 text-[11px] text-slate-500">
              <span>
                {t('fetch_provided')} <b className="font-semibold text-slate-700">{allIDs.length}</b>
              </span>
              <span>
                {t('fetch_new')} <b className="font-semibold text-emerald-600">{freshIDs.length}</b>
              </span>
              <span>
                {t('fetch_existing')} <b className="font-semibold text-slate-700">{existing.size}</b>
              </span>
              <div className="ml-auto flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => setPicked(new Set(freshIDs))}
                  className="font-medium text-indigo-600 hover:underline"
                >
                  {t('fetch_only_new')}
                </button>
                <button
                  type="button"
                  onClick={() => setPicked(allPicked ? new Set() : new Set(allIDs))}
                  className="font-medium text-indigo-600 hover:underline"
                >
                  {allPicked ? t('apply_select_none') : t('apply_select_all')}
                </button>
              </div>
            </div>

            {allIDs.length === 0 ? (
              <p className="py-8 text-center text-xs text-slate-400">{t('fetch_empty')}</p>
            ) : (
              <ul className="max-h-72 space-y-1 overflow-y-auto rounded-lg p-1.5 ring-1 ring-slate-200">
                {remote.map((model) => {
                  const held = existing.has(model.id)
                  return (
                    <li key={model.id}>
                      <label className="flex cursor-pointer items-center gap-2.5 rounded-md px-2 py-1.5 hover:bg-slate-50">
                        <input
                          type="checkbox"
                          checked={picked.has(model.id)}
                          onChange={() => toggle(model.id)}
                          className="h-3.5 w-3.5 shrink-0 accent-indigo-600"
                        />
                        <span className="min-w-0 flex-1">
                          <span className="block truncate text-[12px] text-slate-700">
                            {model.display_name || model.id}
                          </span>
                          <span className="block truncate font-mono text-[10px] text-slate-400">
                            {model.id}
                            {model.note ? ` · ${model.note}` : ''}
                          </span>
                        </span>
                        <Chip
                          className={
                            held
                              ? 'shrink-0 bg-slate-50 text-slate-500 ring-slate-200'
                              : 'shrink-0 bg-emerald-50 text-emerald-700 ring-emerald-200'
                          }
                        >
                          {held ? t('fetch_badge_held') : t('fetch_badge_new')}
                        </Chip>
                      </label>
                    </li>
                  )
                })}
              </ul>
            )}

            {isCache ? null : (
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
            )}

            <p className="text-[11px] leading-relaxed text-slate-400">
              {isCache ? t('fetch_hint_cache') : t('fetch_hint')}
            </p>
          </>
        )}

        {applyError ? (
          <p className="rounded-md bg-rose-50 px-3 py-2 text-[12px] leading-relaxed text-rose-700 ring-1 ring-inset ring-rose-200">
            {applyError}
          </p>
        ) : null}
      </div>
    </Drawer>
  )
}
