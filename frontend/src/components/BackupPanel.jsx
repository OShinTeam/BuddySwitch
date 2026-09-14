import { useEffect, useState } from 'react'
import { Button, ConfirmDialog, Drawer, Icons, IconButton, Select, Spinner } from './ui'
import { formatBytes } from '../lib/util'
import { ReadBackup } from '../../wailsjs/go/service/App'

/** 备份与还原面板：列出快照、预览内容、一键还原或清空。 */
export default function BackupPanel({
  open,
  plugin,
  plugins = [],
  backups,
  busy,
  t,
  onClose,
  onSelectPlugin,
  onRestore,
  onClear,
  onOpenDir,
}) {
  const [preview, setPreview] = useState(null)
  const [loadingPreview, setLoadingPreview] = useState('')
  const [confirm, setConfirm] = useState(null)

  useEffect(() => {
    if (!open) {
      setPreview(null)
      setConfirm(null)
    }
  }, [open])

  const togglePreview = async (entry) => {
    if (preview?.name === entry.name) {
      setPreview(null)
      return
    }
    setLoadingPreview(entry.name)
    try {
      const content = await ReadBackup(plugin.id, entry.name)
      setPreview({ name: entry.name, content })
    } catch (err) {
      setPreview({ name: entry.name, content: `${t('backup_read_failed')}: ${err?.message || err}` })
    } finally {
      setLoadingPreview('')
    }
  }

  const runConfirm = async () => {
    if (!confirm) return
    const action = confirm
    setConfirm(null)
    if (action.type === 'clear') await onClear()
    else await onRestore(action.name)
  }

  const latest = backups[0]

  return (
    <>
      <Drawer
        open={open}
        onClose={onClose}
        width="w-[600px]"
        title={t('backup_title')}
        subtitle={plugin ? `${plugin.name} · ${plugin.source_file}` : undefined}
        footer={
          <>
            <Button variant="ghost" icon={Icons.folder} onClick={onOpenDir}>
              {t('btn_reveal')}
            </Button>
            <Button
              variant="danger"
              icon={Icons.trash}
              disabled={backups.length === 0 || busy}
              onClick={() => setConfirm({ type: 'clear' })}
            >
              {t('backup_clear')}
            </Button>
          </>
        }
      >
        <p className="mb-4 rounded-lg bg-slate-50 px-3 py-2.5 text-[11px] leading-relaxed text-slate-500">
          {t('backup_desc')}
        </p>

        {/* 在「全部模型」视图下没有天然的目标插件，这里让用户直接选一个 */}
        {plugins.length > 1 ? (
          <div className="mb-4">
            <span className="mb-1.5 block text-xs font-medium text-slate-600">
              {t('backup_which_agent')}
            </span>
            <Select value={plugin?.id || ''} onChange={(event) => onSelectPlugin(event.target.value)}>
              {plugins.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.name} · {item.backup_count ?? 0} {t('backup_count_unit')}
                  {item.source_ready ? '' : ` (${t('plugin_source_missing')})`}
                </option>
              ))}
            </Select>
          </div>
        ) : null}

        {latest ? (
          <Button
            variant="primary"
            icon={Icons.history}
            className="mb-4 w-full"
            disabled={busy}
            onClick={() => setConfirm({ type: 'restore', name: latest.name })}
          >
            {t('backup_restore_latest')}
          </Button>
        ) : null}

        {backups.length === 0 ? (
          <p className="py-8 text-center text-xs text-slate-400">{t('backup_empty')}</p>
        ) : (
          <ul className="space-y-2">
            {backups.map((entry) => (
              <li key={entry.name} className="rounded-lg ring-1 ring-slate-200">
                <div className="flex items-center gap-3 px-3 py-2.5">
                  <div className="min-w-0 flex-1">
                    <p className="text-[13px] font-medium text-slate-700">{entry.mod_time}</p>
                    <p className="mt-0.5 truncate font-mono text-[11px] text-slate-400">
                      {entry.source} · {formatBytes(entry.size)}
                    </p>
                  </div>
                  <IconButton
                    title={preview?.name === entry.name ? t('btn_close') : t('col_status')}
                    onClick={() => togglePreview(entry)}
                  >
                    {loadingPreview === entry.name ? (
                      <Spinner className="h-3.5 w-3.5" />
                    ) : (
                      <Icons.search className="h-3.5 w-3.5" />
                    )}
                  </IconButton>
                  <Button
                    size="sm"
                    disabled={busy}
                    onClick={() => setConfirm({ type: 'restore', name: entry.name })}
                  >
                    {t('backup_restore')}
                  </Button>
                </div>
                {preview?.name === entry.name ? (
                  <pre className="max-h-56 overflow-auto border-t border-slate-200 bg-slate-50 px-3 py-2 font-mono text-[11px] leading-relaxed text-slate-600">
                    {preview.content}
                  </pre>
                ) : null}
              </li>
            ))}
          </ul>
        )}
      </Drawer>

      <ConfirmDialog
        open={Boolean(confirm)}
        danger={confirm?.type === 'clear'}
        title={confirm?.type === 'clear' ? t('backup_clear') : t('backup_restore')}
        message={confirm?.type === 'clear' ? t('backup_confirm_clear') : t('backup_confirm_restore')}
        confirmLabel={confirm?.type === 'clear' ? t('btn_clear') : t('btn_restore')}
        cancelLabel={t('btn_cancel')}
        onConfirm={runConfirm}
        onCancel={() => setConfirm(null)}
      />
    </>
  )
}
