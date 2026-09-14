import { useEffect, useState } from 'react'
import { Button, Drawer, Field, Icons, IconButton, Select, TextInput } from './ui'

const LOG_LEVELS = ['DEBUG', 'INFO', 'WARN', 'ERROR']

function PathRow({ label, value, onReveal, t }) {
  return (
    <div className="flex items-center gap-2 py-1.5">
      <span className="w-28 shrink-0 text-xs text-slate-500">{label}</span>
      <span className="min-w-0 flex-1 truncate font-mono text-[11px] text-slate-600" title={value}>
        {value || '—'}
      </span>
      <IconButton title={t('btn_reveal')} onClick={() => onReveal(value)}>
        <Icons.reveal className="h-3.5 w-3.5" />
      </IconButton>
    </div>
  )
}

export default function SettingsPanel({
  open,
  settings,
  plugins,
  langs,
  lang,
  t,
  onClose,
  onUpdate,
  onChooseFile,
  onSetSourcePath,
  onReveal,
}) {
  const [keep, setKeep] = useState(settings?.backup_keep ?? 10)
  const [sourceDraft, setSourceDraft] = useState({})

  useEffect(() => {
    if (!open) return
    setKeep(settings?.backup_keep ?? 10)
    setSourceDraft({})
  }, [open, settings])

  const applyKeep = (value) => {
    const parsed = Number.parseInt(String(value), 10)
    if (Number.isNaN(parsed)) {
      setKeep(settings?.backup_keep ?? 10)
      return
    }
    const clamped = Math.min(100, Math.max(1, parsed))
    setKeep(clamped)
    onUpdate({ backup_keep: clamped })
  }

  return (
    <Drawer open={open} onClose={onClose} width="w-[560px]" title={t('setting_title')}>
      <div className="space-y-6">
        <section className="space-y-3">
          <Field label={t('setting_language')}>
            <Select value={lang} onChange={(event) => onUpdate({ language: event.target.value })}>
              {langs.map((item) => (
                <option key={item.language_code} value={item.language_code}>
                  {item.language_name}
                </option>
              ))}
            </Select>
          </Field>

          <Field label={t('setting_log_level')}>
            <Select
              value={settings?.log_level || 'INFO'}
              onChange={(event) => onUpdate({ log_level: event.target.value })}
            >
              {LOG_LEVELS.map((level) => (
                <option key={level} value={level}>
                  {level}
                </option>
              ))}
            </Select>
          </Field>

          <Field label={t('setting_backup_keep')} hint={t('setting_backup_keep_desc')}>
            <TextInput
              type="number"
              min={1}
              max={100}
              value={keep}
              onChange={(event) => setKeep(event.target.value)}
              onBlur={(event) => applyKeep(event.target.value)}
            />
          </Field>
        </section>

        <section>
          <h3 className="mb-1 text-[11px] font-semibold uppercase tracking-wide text-slate-400">
            {t('setting_paths')}
          </h3>
          <div className="divide-y divide-slate-100">
            <PathRow label={t('setting_work_dir')} value={settings?.work_dir} onReveal={onReveal} t={t} />
            <PathRow label={t('setting_data_dir')} value={settings?.data_dir} onReveal={onReveal} t={t} />
            <PathRow label={t('setting_backup_dir')} value={settings?.backup_dir} onReveal={onReveal} t={t} />
            <PathRow label={t('plugin_definition')} value={settings?.plugin_dir} onReveal={onReveal} t={t} />
          </div>
        </section>

        <section>
          <h3 className="mb-2 text-[11px] font-semibold uppercase tracking-wide text-slate-400">
            {t('plugin_source_path')}
          </h3>
          <div className="space-y-3">
            {plugins.map((plugin) => {
              const draft = sourceDraft[plugin.id] ?? settings?.source_overrides?.[plugin.id] ?? ''
              const ready = plugin.source_ready
              return (
                <div key={plugin.id} className="rounded-lg p-3 ring-1 ring-slate-200">
                  <div className="flex items-center gap-2">
                    <span className="text-[13px] font-medium text-slate-700">{plugin.name}</span>
                    <span
                      className={`inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-[10px] font-medium ring-1 ring-inset ${
                        ready
                          ? 'bg-emerald-50 text-emerald-700 ring-emerald-200'
                          : 'bg-slate-50 text-slate-500 ring-slate-200'
                      }`}
                    >
                      {ready ? t('plugin_source_ready') : t('plugin_source_missing')}
                    </span>
                  </div>
                  <p className="mt-1 break-all font-mono text-[11px] text-slate-500">
                    {plugin.source_file || '—'}
                  </p>
                  <div className="mt-2 flex items-center gap-2">
                    <TextInput
                      value={draft}
                      placeholder={t('plugin_source_hint')}
                      spellCheck={false}
                      className="font-mono text-xs"
                      onChange={(event) =>
                        setSourceDraft((prev) => ({ ...prev, [plugin.id]: event.target.value }))
                      }
                    />
                    <Button size="sm" onClick={() => onSetSourcePath(plugin.id, draft)}>
                      {t('btn_save')}
                    </Button>
                    <Button
                      size="sm"
                      icon={Icons.folder}
                      onClick={async () => {
                        const picked = await onChooseFile(plugin.id)
                        if (picked) setSourceDraft((prev) => ({ ...prev, [plugin.id]: picked }))
                      }}
                    >
                      {t('btn_choose_file')}
                    </Button>
                  </div>
                </div>
              )
            })}
          </div>
        </section>
      </div>
    </Drawer>
  )
}
