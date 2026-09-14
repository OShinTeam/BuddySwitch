import { useEffect, useMemo, useState } from 'react'
import { Button, Drawer, Field, TextArea, TextInput } from './ui'
import { capLabel } from '../lib/util'

const EMPTY = {
  id: '',
  display_name: '',
  provider: '',
  base_url: '',
  api_key: '',
  description: '',
  tags: [],
  enabled: true,
}

/** 新增 / 编辑模型的抽屉表单。 */
export default function ModelEditor({ open, model, plugin, t, onClose, onSubmit }) {
  const [form, setForm] = useState(EMPTY)
  const [tagsText, setTagsText] = useState('')
  const [caps, setCaps] = useState({})
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const isEdit = Boolean(model)

  // 能力键取自插件声明，也兼顾该模型已有的字段——这样新增模型时也能勾选能力。
  const capabilityKeys = useMemo(() => {
    const keys = new Set(plugin?.capabilities || [])
    Object.keys(model?.capabilities || {}).forEach((key) => keys.add(key))
    return [...keys]
  }, [plugin, model])

  // 备注与标签只在插件真的映射了它们时才展示：给出一个写不进去的输入框，
  // 等于骗用户白填一遍。已经带着值的模型例外，那是既存数据，得让人看得到。
  const mappedFields = useMemo(() => new Set(plugin?.mapped_fields || []), [plugin])
  const showTags = mappedFields.has('tags') || (model?.tags || []).length > 0
  const showDescription = mappedFields.has('description') || Boolean(model?.description)

  useEffect(() => {
    if (!open) return
    const next = model ? { ...EMPTY, ...model, tags: model.tags || [] } : { ...EMPTY }
    setForm(next)
    setTagsText((next.tags || []).join(', '))
    setCaps({ ...(model?.capabilities || {}) })
    setError('')
    setBusy(false)
  }, [open, model])

  const set = (key) => (event) => setForm((prev) => ({ ...prev, [key]: event.target.value }))

  const canSubmit = useMemo(() => form.id.trim() !== '' && !busy, [form.id, busy])

  const submit = async () => {
    if (!canSubmit) return
    setBusy(true)
    setError('')
    try {
      await onSubmit({
        ...form,
        id: form.id.trim(),
        display_name: form.display_name.trim() || form.id.trim(),
        tags: tagsText
          .split(',')
          .map((tag) => tag.trim())
          .filter(Boolean),
        capabilities: caps,
      })
    } catch (err) {
      setError(err?.message || String(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Drawer
      open={open}
      onClose={onClose}
      width="w-[520px]"
      title={isEdit ? t('model_title_edit') : t('model_title_add')}
      subtitle={plugin ? `${plugin.name} · ${plugin.source_file}` : undefined}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            {t('btn_cancel')}
          </Button>
          <Button variant="primary" disabled={!canSubmit} onClick={submit}>
            {t('btn_save')}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <Field label={t('model_id')} hint={t('model_id_hint')}>
          <TextInput
            value={form.id}
            onChange={set('id')}
            disabled={isEdit}
            spellCheck={false}
            className="font-mono"
            placeholder="gpt-4o-mini"
          />
        </Field>

        <Field label={t('model_name')}>
          <TextInput value={form.display_name} onChange={set('display_name')} placeholder={form.id} />
        </Field>

        <div className={`grid gap-3 ${showTags ? 'grid-cols-2' : 'grid-cols-1'}`}>
          <Field label={t('model_provider')}>
            <TextInput value={form.provider} onChange={set('provider')} placeholder="openai" />
          </Field>
          {showTags ? (
            <Field label={t('model_tags')} hint={t('model_tags_hint')}>
              <TextInput
                value={tagsText}
                onChange={(e) => setTagsText(e.target.value)}
                placeholder="chat, fast"
              />
            </Field>
          ) : null}
        </div>

        <Field label={t('model_base_url')}>
          <TextInput
            value={form.base_url}
            onChange={set('base_url')}
            spellCheck={false}
            className="font-mono"
            placeholder="https://api.example.com"
          />
        </Field>

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

        {showDescription ? (
          <Field label={t('model_desc')}>
            <TextArea rows={3} value={form.description} onChange={set('description')} />
          </Field>
        ) : null}

        <label className="flex cursor-pointer items-center gap-2.5 rounded-lg bg-slate-50 px-3 py-2.5">
          <input
            type="checkbox"
            checked={Boolean(form.enabled)}
            onChange={(event) => setForm((prev) => ({ ...prev, enabled: event.target.checked }))}
            className="h-4 w-4 accent-indigo-600"
          />
          <span className="text-[13px] text-slate-700">{t('model_enabled')}</span>
        </label>

        {capabilityKeys.length > 0 ? (
          <div>
            <span className="mb-1.5 block text-xs font-medium text-slate-600">
              {t('model_capabilities')}
            </span>
            <div className="flex flex-wrap gap-2">
              {capabilityKeys.map((key) => (
                <label
                  key={key}
                  className="flex cursor-pointer items-center gap-1.5 rounded-md bg-slate-50 px-2.5 py-1.5 text-[12px] text-slate-700"
                >
                  <input
                    type="checkbox"
                    checked={Boolean(caps[key])}
                    onChange={(event) =>
                      setCaps((prev) => ({ ...prev, [key]: event.target.checked }))
                    }
                    className="h-3.5 w-3.5 accent-indigo-600"
                  />
                  {capLabel(key, t)}
                </label>
              ))}
            </div>
          </div>
        ) : null}

        {plugin && !plugin.can_toggle_enabled ? (
          <p className="text-[11px] leading-relaxed text-slate-400">{t('model_enabled_hint')}</p>
        ) : null}

        {error ? (
          <p className="rounded-md bg-rose-50 px-3 py-2 text-[12px] leading-relaxed text-rose-700 ring-1 ring-inset ring-rose-200">
            {error}
          </p>
        ) : null}
      </div>
    </Drawer>
  )
}
