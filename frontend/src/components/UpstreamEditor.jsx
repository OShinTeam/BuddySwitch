import { useEffect, useState } from 'react'
import { Button, Chip, Drawer, Field, Icons, LAYERS, Spinner, TextArea, TextInput } from './ui'

const EMPTY = { id: '', name: '', vendor: '', url: '', api_key: '', notes: '' }

/**
 * 解析手动补充的模型清单。每行一个，支持「id」与「id | 显示名」。
 *
 * 存在的理由：部分上游的 /models 接口返回不全，只靠拉取会有盲区，
 * 必须允许把拉不到的那些直接写进来。
 */
function parseManual(text) {
  const seen = new Set()
  const out = []
  for (const line of text.split('\n')) {
    const trimmed = line.trim()
    if (!trimmed) continue
    const [id, name] = trimmed.split('|').map((part) => part.trim())
    if (!id || seen.has(id)) continue
    seen.add(id)
    out.push({ id, display_name: name || '' })
  }
  return out
}

/**
 * 新增 / 编辑上游。
 *
 * 「新增上游」取代了原来的「新增模型」：模型不再手敲，而是填好接入点与密钥后
 * 直接向上游拉取清单，再勾选要记录哪些——这个顺序才是真实的配置流程。
 */
export default function UpstreamEditor({ open, initial, t, onClose, onFetch, onSave }) {
  const [form, setForm] = useState(EMPTY)
  const [remote, setRemote] = useState(null)
  const [picked, setPicked] = useState(() => new Set())
  const [loading, setLoading] = useState(false)
  const [fetchError, setFetchError] = useState('')
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState('')
  const [touched, setTouched] = useState(false)
  const [manualText, setManualText] = useState('')

  const isEdit = Boolean(initial?.id)

  useEffect(() => {
    if (!open) return
    const base = initial
      ? { ...EMPTY, ...initial, models: undefined }
      : { ...EMPTY }
    setForm(base)
    setRemote(null)
    setFetchError('')
    setSaveError('')
    setSaving(false)
    setTouched(false)
    setManualText('')
    // 编辑时先把已有的模型清单当成「已有」，拉取后可以对照增删
    setPicked(new Set((initial?.models || []).map((m) => m.id)))
  }, [open, initial])

  const set = (key) => (event) => {
    setTouched(true)
    setForm((prev) => ({ ...prev, [key]: event.target.value }))
  }

  const canFetch = form.url.trim() !== '' && !loading
  const canSave = form.url.trim() !== '' && !saving

  const runFetch = async () => {
    setLoading(true)
    setFetchError('')
    try {
      const list = await onFetch(form.url.trim(), form.api_key)
      setRemote(list || [])
      // 首次拉取时默认勾上上游返回的全部；编辑过就保留已选，避免覆盖用户的选择
      setPicked((prev) => (touched && prev.size > 0 ? prev : new Set((list || []).map((m) => m.id))))
    } catch (err) {
      setRemote(null)
      setFetchError(err?.message || String(err))
    } finally {
      setLoading(false)
    }
  }

  const toggle = (id) =>
    setPicked((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })

  const submit = async () => {
    if (!canSave) return
    setSaving(true)
    setSaveError('')
    try {
      // 清单 = 拉取结果里勾中的 ∪ 手动补充的。
      // 没拉取过就沿用编辑前的旧清单，避免因为没点拉取而清空。
      const source = remote ?? initial?.models ?? []
      const pickedFromSource = source
        .filter((m) => picked.has(m.id))
        .map((m) => ({ id: m.id, display_name: m.display_name || '', capabilities: m.capabilities }))
      const manual = parseManual(manualText)
      const manualIds = new Set(manual.map((m) => m.id))
      // 手填的优先，避免同一 id 出现两条
      const models = [...pickedFromSource.filter((m) => !manualIds.has(m.id)), ...manual]

      await onSave({
        id: form.id,
        name: form.name.trim(),
        vendor: form.vendor.trim(),
        url: form.url.trim(),
        api_key: form.api_key,
        notes: form.notes,
        models,
      })
    } catch (err) {
      setSaveError(err?.message || String(err))
    } finally {
      setSaving(false)
    }
  }

  const allIDs = remote ? remote.map((m) => m.id) : []
  const allPicked = allIDs.length > 0 && picked.size === allIDs.length

  return (
    <Drawer
      open={open}
      onClose={onClose}
      width="w-[540px]"
      layer={LAYERS.raised}
      title={isEdit ? t('upstream_edit') : t('upstream_add')}
      subtitle={t('upstream_editor_desc')}
      footer={
        <>
          <Button variant="ghost" onClick={onClose} disabled={saving}>
            {t('btn_cancel')}
          </Button>
          <Button variant="primary" onClick={submit} disabled={!canSave}>
            {t('btn_save')}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <Field label={t('upstream_name')} hint={t('upstream_name_hint')}>
          <TextInput value={form.name} onChange={set('name')} placeholder="OpenAI" />
        </Field>

        <Field label={t('model_base_url')}>
          <TextInput
            value={form.url}
            onChange={set('url')}
            spellCheck={false}
            className="font-mono"
            placeholder="https://api.example.com/v1/chat/completions"
          />
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <Field label={t('model_api_key')}>
            <TextInput
              value={form.api_key}
              onChange={set('api_key')}
              spellCheck={false}
              autoComplete="off"
              className="font-mono"
              placeholder="sk-..."
            />
          </Field>
          <Field label={t('model_provider')}>
            <TextInput value={form.vendor} onChange={set('vendor')} placeholder="Custom" />
          </Field>
        </div>

        <Field label={t('upstream_notes')}>
          <TextInput value={form.notes} onChange={set('notes')} />
        </Field>

        <div className="rounded-lg p-3 ring-1 ring-slate-200">
          <div className="flex items-center justify-between gap-2">
            <span className="text-xs font-medium text-slate-600">{t('upstream_models')}</span>
            <Button
              size="sm"
              icon={loading ? Spinner : Icons.download}
              onClick={runFetch}
              disabled={!canFetch}
            >
              {t('fetch_action')}
            </Button>
          </div>

          {loading ? (
            <p className="mt-3 text-center text-[11px] text-slate-400">{t('fetch_loading')}</p>
          ) : fetchError ? (
            <p className="mt-3 rounded-md bg-rose-50 px-3 py-2 text-[11px] leading-relaxed text-rose-700">
              {fetchError}
            </p>
          ) : remote ? (
            <>
              <div className="mt-3 flex items-center justify-between text-[11px] text-slate-500">
                <span>
                  {remote.length} {t('plugin_models_suffix')} · {t('fetch_selected')} {picked.size}
                </span>
                <button
                  type="button"
                  onClick={() => setPicked(allPicked ? new Set() : new Set(allIDs))}
                  className="font-medium text-indigo-600 hover:underline"
                >
                  {allPicked ? t('apply_select_none') : t('apply_select_all')}
                </button>
              </div>
              <ul className="mt-1.5 max-h-56 space-y-0.5 overflow-y-auto">
                {remote.map((model) => (
                  <li key={model.id}>
                    <label className="flex cursor-pointer items-center gap-2 rounded-md px-1.5 py-1 hover:bg-slate-50">
                      <input
                        type="checkbox"
                        checked={picked.has(model.id)}
                        onChange={() => toggle(model.id)}
                        className="h-3.5 w-3.5 shrink-0 accent-indigo-600"
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
            </>
          ) : (
            <p className="mt-3 text-[11px] leading-relaxed text-slate-400">
              {t('upstream_fetch_hint')}
            </p>
          )}
        </div>

        <div className="rounded-lg p-3 ring-1 ring-slate-200">
          <span className="text-xs font-medium text-slate-600">{t('upstream_manual_title')}</span>
          <p className="mt-1 text-[11px] leading-relaxed text-slate-400">
            {t('upstream_manual_hint')}
          </p>
          <TextArea
            rows={3}
            value={manualText}
            onChange={(event) => setManualText(event.target.value)}
            spellCheck={false}
            className="mt-2 font-mono text-[11px]"
            placeholder={'gpt-4o-mini\nclaude-sonnet-4-5 | Claude Sonnet 4.5'}
          />
        </div>

        {isEdit ? (
          <p className="flex items-center gap-1.5 text-[11px] text-slate-400">
            <Chip className="bg-slate-50 text-slate-500 ring-slate-200">
              {(initial?.models || []).length} {t('plugin_models_suffix')}
            </Chip>
            {t('upstream_edit_saved_hint')}
          </p>
        ) : null}

        {saveError ? (
          <p className="rounded-md bg-rose-50 px-3 py-2 text-[12px] leading-relaxed text-rose-700 ring-1 ring-inset ring-rose-200">
            {saveError}
          </p>
        ) : null}
      </div>
    </Drawer>
  )
}
