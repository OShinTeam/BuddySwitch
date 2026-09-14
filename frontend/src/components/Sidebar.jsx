import { Icons, IconButton, Switch } from './ui'
import { pluginColor } from '../lib/util'

function NavRow({ icon: Icon, label, hint, accent, active, onClick }) {
  return (
    <div
      className={`flex cursor-pointer items-center gap-2.5 rounded-lg px-2.5 py-2 transition-colors ${
        active ? 'bg-indigo-50' : 'hover:bg-slate-50'
      }`}
      onClick={onClick}
    >
      <Icon className={`h-4 w-4 shrink-0 ${active ? accent : 'text-slate-400'}`} />
      <div className="min-w-0 flex-1">
        <p
          className={`truncate text-[13px] ${
            active ? `font-semibold ${accent}` : 'font-medium text-slate-700'
          }`}
        >
          {label}
        </p>
        {hint ? <p className="mt-0.5 truncate text-[11px] text-slate-400">{hint}</p> : null}
      </div>
    </div>
  )
}

function PluginRow({ plugin, active, onSelect, onToggle, modelsLabel, disabledHint }) {
  const color = pluginColor(plugin.color)
  const broken = !plugin.loaded
  return (
    <div
      className={`flex cursor-pointer items-center gap-2.5 rounded-lg px-2.5 py-2 transition-colors ${
        active ? 'bg-indigo-50' : 'hover:bg-slate-50'
      }`}
      onClick={() => onSelect(plugin.id)}
    >
      <span
        className="h-6 w-1 shrink-0 rounded-full"
        style={{ backgroundColor: broken ? '#cbd5e1' : color }}
      />
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5">
          <span
            className={`truncate text-[13px] ${
              active ? 'font-semibold text-indigo-700' : 'font-medium text-slate-700'
            }`}
          >
            {plugin.name}
          </span>
          {broken ? <Icons.alert className="h-3 w-3 shrink-0 text-amber-500" /> : null}
        </div>
        <p className="mt-0.5 truncate text-[11px] text-slate-400">
          {broken ? plugin.load_error || '' : `${plugin.model_count} ${modelsLabel}`}
        </p>
      </div>
      <div onClick={(event) => event.stopPropagation()}>
        <Switch
          checked={plugin.enabled}
          disabled={broken}
          title={disabledHint}
          onChange={(next) => onToggle(plugin.id, next)}
        />
      </div>
    </div>
  )
}

export default function Sidebar({
  t,
  plugins,
  active,
  cacheCount = 0,
  cacheModels = 0,
  onSelect,
  onTogglePlugin,
  onReload,
  onOpenPluginDir,
  onExportExample,
  onOpenUpstreams,
  onOpenSettings,
}) {
  const agentModels = plugins
    .filter((item) => item.enabled)
    .reduce((sum, item) => sum + item.model_count, 0)

  return (
    <aside className="flex w-[232px] shrink-0 flex-col border-r border-slate-200 bg-white">
      <div className="flex h-11 items-center justify-between px-3">
        <span className="text-[11px] font-semibold uppercase tracking-wide text-slate-400">
          {t('sidebar_scope')}
        </span>
        <IconButton title={t('sidebar_refresh')} onClick={onReload}>
          <Icons.refresh className="h-4 w-4" />
        </IconButton>
      </div>

      <div className="flex-1 space-y-0.5 overflow-y-auto px-2 pb-2">
        <NavRow
          icon={Icons.layers}
          label={t('sidebar_cache')}
          hint={`${cacheModels} ${t('plugin_models_suffix')} · ${cacheCount} ${t('upstream_count')}`}
          accent="text-indigo-700"
          active={active === 'cache'}
          onClick={() => onSelect('cache')}
        />

        <div className="px-2.5 pb-1 pt-3 text-[11px] font-semibold uppercase tracking-wide text-slate-400">
          {t('sidebar_agents')}
        </div>

        {plugins.map((plugin) => (
          <PluginRow
            key={plugin.id}
            plugin={plugin}
            active={active === plugin.id}
            onSelect={onSelect}
            onToggle={onTogglePlugin}
            modelsLabel={t('plugin_models_suffix')}
            disabledHint={t('sidebar_agents_hint')}
          />
        ))}

        {plugins.length === 0 ? (
          <p className="px-2.5 py-3 text-[11px] leading-relaxed text-slate-400">
            {t('plugin_source_missing')}
          </p>
        ) : null}
      </div>

      <div className="space-y-1 border-t border-slate-200 p-2">
        <button
          type="button"
          onClick={onOpenUpstreams}
          className="flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-[13px] font-medium text-slate-600 transition-colors hover:bg-slate-50"
        >
          <Icons.server className="h-4 w-4 text-slate-400" />
          {t('upstream_manage')}
        </button>
        <button
          type="button"
          onClick={onOpenSettings}
          className="flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-[13px] font-medium text-slate-600 transition-colors hover:bg-slate-50"
        >
          <Icons.sliders className="h-4 w-4 text-slate-400" />
          {t('btn_settings')}
        </button>
        <button
          type="button"
          onClick={onOpenPluginDir}
          className="flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-[13px] font-medium text-slate-600 transition-colors hover:bg-slate-50"
        >
          <Icons.folder className="h-4 w-4 text-slate-400" />
          {t('sidebar_open_dir')}
        </button>
        <button
          type="button"
          onClick={onExportExample}
          className="flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-[13px] font-medium text-slate-600 transition-colors hover:bg-slate-50"
        >
          <Icons.download className="h-4 w-4 text-slate-400" />
          {t('sidebar_export')}
        </button>
      </div>
    </aside>
  )
}
