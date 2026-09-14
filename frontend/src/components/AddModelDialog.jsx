import { useEffect, useState } from 'react'
import { Button, Drawer, Field, Icons, LAYERS, TextInput } from './ui'

/**
 * 手动往某个上游里补一个模型。
 *
 * 有些上游的 /models 接口返回不全（只有部分模型对外列出），所以光靠拉取
 * 会产生盲区，必须允许直接手填 id。
 */
export default function AddModelDialog({ open, upstreamLabel, upstreamUrl, t, onClose, onSubmit }) {
  const [id, setId] = useState('')
  const [name, setName] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!open) return
    setId('')
    setName('')
    setBusy(false)
    setError('')
  }, [open])

  const canSubmit = id.trim() !== '' && !busy

  const submit = async () => {
    if (!canSubmit) return
    setBusy(true)
    setError('')
    try {
      await onSubmit({ id: id.trim(), display_name: name.trim() })
    } catch (err) {
      setError(err?.message || String(err))
      setBusy(false)
    }
  }

  return (
    <Drawer
      open={open}
      onClose={onClose}
      width="w-[440px]"
      layer={LAYERS.raised}
      title={t('manual_add_title')}
      subtitle={upstreamLabel ? `${upstreamLabel} · ${upstreamUrl}` : undefined}
      footer={
        <>
          <Button variant="ghost" onClick={onClose} disabled={busy}>
            {t('btn_cancel')}
          </Button>
          <Button variant="primary" icon={Icons.plus} onClick={submit} disabled={!canSubmit}>
            {t('manual_add_confirm')}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <Field label={t('model_id')} hint={t('manual_add_hint')}>
          <TextInput
            value={id}
            onChange={(event) => setId(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter') submit()
            }}
            spellCheck={false}
            autoFocus
            className="font-mono"
            placeholder="gpt-4o-mini"
          />
        </Field>

        <Field label={t('model_name')}>
          <TextInput
            value={name}
            onChange={(event) => setName(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter') submit()
            }}
            placeholder={id.trim()}
          />
        </Field>

        <p className="rounded-lg bg-slate-50 px-3 py-2.5 text-[11px] leading-relaxed text-slate-500">
          {t('manual_add_why')}
        </p>

        {error ? (
          <p className="rounded-md bg-rose-50 px-3 py-2 text-[12px] leading-relaxed text-rose-700 ring-1 ring-inset ring-rose-200">
            {error}
          </p>
        ) : null}
      </div>
    </Drawer>
  )
}
