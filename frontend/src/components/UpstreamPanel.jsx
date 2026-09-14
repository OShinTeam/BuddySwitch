import { useEffect, useMemo, useState } from 'react'
import { Button, Chip, ConfirmDialog, Drawer, Icons, IconButton, Spinner } from './ui'
import { maskSecret } from '../lib/util'

/**
 * 上游目录面板。
 *
 * 两个模式：浏览/维护目录，以及「从各 Agent 拉取上游」。
 * 拉取刻意做成勾选式——先发现、再由用户决定拿哪些上游、哪些模型，
 * 而不是把各 Agent 的配置全量灌进目录。
 */
export default function UpstreamPanel({
  open,
  upstreams,
  t,
  onClose,
  onDiscover,
  onImport,
  onApply,
  onFetchUpstream,
  onOpenEditor,
  onDelete,
  onReveal,
}) {
  const [mode, setMode] = useState('list')
  const [discovered, setDiscovered] = useState([])
  const [loadingDiscover, setLoadingDiscover] = useState(false)
  const [picked, setPicked] = useState(() => new Set())
  const [pickedModels, setPickedModels] = useState({})
  const [openGroups, setOpenGroups] = useState(() => new Set())
  const [pendingDelete, setPendingDelete] = useState(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!open) {
      setMode('list')
      setPendingDelete(null)
      setPicked(new Set())
      setPickedModels({})
      setOpenGroups(new Set())
    }
  }, [open])

  const totalModels = upstreams.reduce((sum, item) => sum + (item.models?.length || 0), 0)
  const known = useMemo(() => new Set(upstreams.map((item) => item.id)), [upstreams])

  const toggleOpenGroup = (id) =>
    setOpenGroups((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })

  const toSource = (item) => ({
    label: item.name,
    url: item.url,
    apiKey: item.api_key,
    models: (item.models || []).map((m) => ({
      id: m.id,
      display_name: m.display_name || m.id,
      provider: item.vendor,
      base_url: item.url,
      api_key: item.api_key,
      capabilities: m.capabilities,
    })),
  })

  // --- 拉取模式 ---------------------------------------------------------

  const enterImport = async () => {
    setMode('import')
    setLoadingDiscover(true)
    setPicked(new Set())
    setPickedModels({})
    try {
      setDiscovered((await onDiscover()) || [])
    } finally {
      setLoadingDiscover(false)
    }
  }

  const selectAll = () => {
    setPicked(new Set(discovered.map((item) => item.id)))
    setPickedModels({})
  }

  const selectNone = () => {
    setPicked(new Set())
    setPickedModels({})
  }

  const toggleUpstream = (id) =>
    setPicked((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })

  const toggleModel = (upstreamID, modelID, allModelIDs) =>
    setPickedModels((prev) => {
      const current = new Set(prev[upstreamID] ?? allModelIDs)
      if (current.has(modelID)) current.delete(modelID)
      else current.add(modelID)
      const next = { ...prev }
      // 全部选中时清掉该条覆盖，语义上等于「整条上游」
      if (current.size === allModelIDs.length) delete next[upstreamID]
      else next[upstreamID] = [...current]
      return next
    })

  const runImport = async () => {
    setBusy(true)
    try {
      await onImport({ upstream_ids: [...picked], model_ids: pickedModels })
      setMode('list')
    } finally {
      setBusy(false)
    }
  }

  const pickedModelCount = discovered
    .filter((item) => picked.has(item.id))
    .reduce((sum, item) => sum + (pickedModels[item.id]?.length ?? item.models.length), 0)

  // --- 渲染 -------------------------------------------------------------

  return (
    <>
      <Drawer
        open={open}
        onClose={onClose}
        width="w-[620px]"
        title={mode === 'import' ? t('upstream_import_title') : t('upstream_title')}
        subtitle={
          mode === 'import'
            ? t('upstream_import_desc')
            : `${upstreams.length} ${t('upstream_count')} · ${totalModels} ${t('plugin_models_suffix')}`
        }
        footer={
          mode === 'import' ? (
            <>
              <Button variant="ghost" onClick={selectAll} disabled={loadingDiscover}>
                {t('apply_select_all')}
              </Button>
              <Button variant="ghost" onClick={selectNone} disabled={loadingDiscover}>
                {t('apply_select_none')}
              </Button>
              <div className="flex-1" />
              <Button variant="ghost" onClick={() => setMode('list')} disabled={busy}>
                {t('btn_cancel')}
              </Button>
              <Button
                variant="primary"
                icon={Icons.download}
                onClick={runImport}
                disabled={busy || picked.size === 0}
              >
                {t('upstream_import_confirm')}
                {picked.size ? ` (${picked.size} · ${pickedModelCount})` : ''}
              </Button>
            </>
          ) : (
            <>
              <Button variant="ghost" icon={Icons.reveal} onClick={onReveal}>
                {t('btn_reveal')}
              </Button>
              <Button icon={Icons.download} onClick={enterImport} disabled={busy}>
                {t('upstream_import')}
              </Button>
              <Button variant="primary" icon={Icons.plus} onClick={() => onOpenEditor(null)}>
                {t('upstream_add')}
              </Button>
            </>
          )
        }
      >
        {mode === 'import' ? (
          <>
            <p className="mb-3 rounded-lg bg-indigo-50 px-3 py-2.5 text-[11px] leading-relaxed text-indigo-700">
              {t('upstream_import_hint')}
            </p>

            {loadingDiscover ? (
              <div className="flex items-center justify-center gap-2 py-10 text-xs text-slate-400">
                <Spinner className="h-4 w-4" />
                {t('upstream_scanning')}
              </div>
            ) : discovered.length === 0 ? (
              <p className="py-10 text-center text-xs text-slate-400">{t('upstream_nothing_found')}</p>
            ) : (
              <ul className="space-y-2">
                {discovered.map((item) => {
                  const isPicked = picked.has(item.id)
                  const modelIDs = item.models.map((m) => m.id)
                  const effective = pickedModels[item.id] ?? modelIDs
                  const expanded = openGroups.has(item.id)
                  return (
                    <li key={item.id} className="rounded-lg ring-1 ring-slate-200">
                      <div className="flex items-start gap-2.5 px-3 py-2.5">
                        <input
                          type="checkbox"
                          checked={isPicked}
                          onChange={() => toggleUpstream(item.id)}
                          className="mt-0.5 h-3.5 w-3.5 shrink-0 accent-indigo-600"
                        />
                        <div className="min-w-0 flex-1">
                          <div className="flex flex-wrap items-center gap-1.5">
                            <span className="truncate text-[13px] font-medium text-slate-800">
                              {item.name}
                            </span>
                            <Chip className="bg-slate-50 text-slate-500 ring-slate-200">
                              {item.models.length} {t('plugin_models_suffix')}
                            </Chip>
                            {known.has(item.id) ? (
                              <Chip className="bg-amber-50 text-amber-700 ring-amber-200">
                                {t('upstream_already_in_catalog')}
                              </Chip>
                            ) : (
                              <Chip className="bg-emerald-50 text-emerald-700 ring-emerald-200">
                                {t('upstream_new')}
                              </Chip>
                            )}
                          </div>
                          <p className="mt-0.5 truncate font-mono text-[11px] text-slate-500">
                            {item.url}
                          </p>
                          <p className="mt-0.5 font-mono text-[10px] text-slate-400">
                            {item.api_key ? maskSecret(item.api_key) : '—'}
                            {item.origins?.length ? ` · ${item.origins.join(' · ')}` : ''}
                          </p>
                        </div>
                        {item.models.length > 0 ? (
                          <IconButton
                            title={t('upstream_show_models')}
                            onClick={() => toggleOpenGroup(item.id)}
                          >
                            <Icons.chevron
                              className={`h-3.5 w-3.5 transition-transform ${expanded ? 'rotate-90' : ''}`}
                            />
                          </IconButton>
                        ) : null}
                      </div>

                      {expanded ? (
                        <ul className="space-y-0.5 border-t border-slate-100 px-3 py-2">
                          {item.models.map((model) => (
                            <li key={model.id}>
                              <label className="flex cursor-pointer items-center gap-2 rounded-md px-1.5 py-1 hover:bg-slate-50">
                                <input
                                  type="checkbox"
                                  checked={isPicked && effective.includes(model.id)}
                                  disabled={!isPicked}
                                  onChange={() => toggleModel(item.id, model.id, modelIDs)}
                                  className="h-3.5 w-3.5 shrink-0 accent-indigo-600 disabled:opacity-40"
                                />
                                <span className="truncate text-[12px] text-slate-600">
                                  {model.display_name || model.id}
                                </span>
                                <span className="truncate font-mono text-[10px] text-slate-400">
                                  {model.id}
                                </span>
                              </label>
                            </li>
                          ))}
                        </ul>
                      ) : null}
                    </li>
                  )
                })}
              </ul>
            )}
          </>
        ) : (
          <>
            <p className="mb-4 rounded-lg bg-slate-50 px-3 py-2.5 text-[11px] leading-relaxed text-slate-500">
              {t('upstream_desc')}
            </p>

            {upstreams.length === 0 ? (
              <p className="py-10 text-center text-xs leading-relaxed text-slate-400">
                {t('upstream_empty')}
              </p>
            ) : (
              <ul className="space-y-2.5">
                {upstreams.map((item) => {
                  const expanded = openGroups.has(item.id)
                  const models = item.models || []
                  return (
                    <li key={item.id} className="rounded-lg ring-1 ring-slate-200">
                      <div className="flex items-start gap-3 px-3 py-2.5">
                        <div className="min-w-0 flex-1">
                          <div className="flex flex-wrap items-center gap-1.5">
                            <span className="truncate text-[13px] font-medium text-slate-800">
                              {item.name}
                            </span>
                            {item.vendor ? (
                              <Chip className="bg-slate-50 text-slate-500 ring-slate-200">
                                {item.vendor}
                              </Chip>
                            ) : null}
                            <Chip className="bg-slate-50 text-slate-500 ring-slate-200">
                              {models.length} {t('plugin_models_suffix')}
                            </Chip>
                          </div>
                          <p
                            className="mt-0.5 truncate font-mono text-[11px] text-slate-500"
                            title={item.url}
                          >
                            {item.url}
                          </p>
                          <p className="mt-0.5 font-mono text-[10px] text-slate-400">
                            {item.api_key ? maskSecret(item.api_key) : '—'}
                            {item.origins?.length ? ` · ${item.origins.join(' · ')}` : ''}
                          </p>
                        </div>

                        <div className="flex shrink-0 items-center gap-1">
                          <Button
                            size="sm"
                            icon={Icons.download}
                            onClick={() => onFetchUpstream(item)}
                          >
                            {t('fetch_action')}
                          </Button>
                          <Button
                            size="sm"
                            icon={Icons.forward}
                            onClick={() => onApply(toSource(item))}
                            disabled={models.length === 0}
                          >
                            {t('btn_apply')}
                          </Button>
                          <IconButton title={t('btn_edit')} onClick={() => onOpenEditor(item)}>
                            <Icons.pencil className="h-3.5 w-3.5" />
                          </IconButton>
                          <IconButton
                            title={t('btn_delete')}
                            className="hover:bg-rose-50 hover:text-rose-600"
                            onClick={() => setPendingDelete(item)}
                          >
                            <Icons.trash className="h-3.5 w-3.5" />
                          </IconButton>
                        </div>
                      </div>

                      {models.length > 0 ? (
                        <button
                          type="button"
                          onClick={() => toggleOpenGroup(item.id)}
                          className="flex w-full items-center gap-1.5 border-t border-slate-100 px-3 py-1.5 text-[11px] text-slate-400 transition-colors hover:bg-slate-50"
                        >
                          <Icons.chevron
                            className={`h-3 w-3 transition-transform ${expanded ? 'rotate-90' : ''}`}
                          />
                          {expanded ? t('btn_close') : t('upstream_show_models')}
                        </button>
                      ) : null}

                      {expanded ? (
                        <ul className="space-y-0.5 border-t border-slate-100 px-3 py-2">
                          {models.map((model) => (
                            <li key={model.id} className="flex items-center gap-2">
                              <span className="truncate text-[12px] text-slate-600">
                                {model.display_name || model.id}
                              </span>
                              <span className="truncate font-mono text-[10px] text-slate-400">
                                {model.id}
                              </span>
                            </li>
                          ))}
                        </ul>
                      ) : null}
                    </li>
                  )
                })}
              </ul>
            )}
          </>
        )}
      </Drawer>

      <ConfirmDialog
        open={Boolean(pendingDelete)}
        danger
        title={t('btn_delete')}
        message={t('upstream_confirm_delete')}
        confirmLabel={t('btn_delete')}
        cancelLabel={t('btn_cancel')}
        onConfirm={async () => {
          const item = pendingDelete
          setPendingDelete(null)
          if (item) await onDelete(item.id)
        }}
        onCancel={() => setPendingDelete(null)}
      />
    </>
  )
}
