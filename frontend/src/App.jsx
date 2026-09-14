import { useCallback, useEffect, useMemo, useState } from 'react'
import TitleBar from './components/TitleBar'
import Sidebar from './components/Sidebar'
import ModelTable from './components/ModelTable'
import ModelEditor from './components/ModelEditor'
import BackupPanel from './components/BackupPanel'
import SettingsPanel from './components/SettingsPanel'
import UpstreamPanel from './components/UpstreamPanel'
import UpstreamEditor from './components/UpstreamEditor'
import ApplyDialog from './components/ApplyDialog'
import FetchDialog from './components/FetchDialog'
import AddModelDialog from './components/AddModelDialog'
import { Button, ConfirmDialog, EmptyState, Icons, Select, Spinner, TextInput } from './components/ui'
import { useToast } from './components/Toaster'
import { useI18n } from './lib/i18n'
import { errorText, groupByUpstream, probeKey } from './lib/util'
import * as api from '../wailsjs/go/service/App'

// 缓存视图里模型的 uid 前缀，和 Agent 视图的 "插件id/模型id" 区分开。
const CACHE_SCOPE = 'cache'

export default function App() {
  const { t, lang, langs, changeLang } = useI18n()
  const toast = useToast()

  const [plugins, setPlugins] = useState([])
  // active 为 'cache' 时看的是 BuddySwitch 自己的上游缓存，否则看某个 Agent 的配置。
  const [active, setActive] = useState(CACHE_SCOPE)

  const [models, setModels] = useState([])
  const [upstreams, setUpstreams] = useState([])
  const [loading, setLoading] = useState(true)

  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState('all')

  const [probeMap, setProbeMap] = useState({})
  const [probing, setProbing] = useState(() => new Set())
  const [testingAll, setTestingAll] = useState(false)

  const [editorOpen, setEditorOpen] = useState(false)
  const [editing, setEditing] = useState(null)
  const [pendingDelete, setPendingDelete] = useState(null)

  const [backupOpen, setBackupOpen] = useState(false)
  const [backupPluginID, setBackupPluginID] = useState('')
  const [backups, setBackups] = useState([])
  const [backupBusy, setBackupBusy] = useState(false)

  const [upstreamOpen, setUpstreamOpen] = useState(false)
  const [upstreamEditorOpen, setUpstreamEditorOpen] = useState(false)
  const [upstreamEditing, setUpstreamEditing] = useState(null)

  const [applySource, setApplySource] = useState(null)
  const [fetchState, setFetchState] = useState(null)
  const [manualAddGroup, setManualAddGroup] = useState(null)

  const [settings, setSettings] = useState(null)
  const [settingsOpen, setSettingsOpen] = useState(false)

  const isCatalog = active === CACHE_SCOPE
  const mode = isCatalog ? 'catalog' : 'agent'

  const activePlugin = useMemo(
    () => plugins.find((plugin) => plugin.id === active) || null,
    [plugins, active],
  )

  const backupPlugin = useMemo(
    () => plugins.find((plugin) => plugin.id === backupPluginID) || null,
    [plugins, backupPluginID],
  )

  // --- 数据加载 ---------------------------------------------------------

  const loadPlugins = useCallback(async () => {
    try {
      setPlugins((await api.ListPlugins()) || [])
    } catch (err) {
      toast.failure(t('toast_failed'), errorText(err))
    }
  }, [toast, t])

  const loadUpstreams = useCallback(async () => {
    try {
      setUpstreams((await api.ListUpstreams()) || [])
    } catch (err) {
      console.error(err)
    }
  }, [])

  const loadSettings = useCallback(async () => {
    try {
      setSettings(await api.GetSettings())
    } catch (err) {
      console.error(err)
    }
  }, [])

  const loadModels = useCallback(
    async (target) => {
      if (target === CACHE_SCOPE) {
        await loadUpstreams()
        return
      }
      setLoading(true)
      try {
        setModels((await api.PullModels(target)) || [])
      } catch (err) {
        setModels([])
        toast.failure(t('toast_failed'), errorText(err))
      } finally {
        setLoading(false)
      }
    },
    [loadUpstreams, toast, t],
  )

  useEffect(() => {
    setLoading(true)
    Promise.all([loadPlugins(), loadSettings(), loadUpstreams()]).finally(() => setLoading(false))
  }, [loadPlugins, loadSettings, loadUpstreams])

  useEffect(() => {
    loadModels(active)
  }, [active, loadModels])

  // --- 插件 -------------------------------------------------------------

  const handleReloadPlugins = async () => {
    try {
      setPlugins((await api.ReloadPlugins()) || [])
      await loadModels(active)
      toast.success(t('toast_reloaded'))
    } catch (err) {
      toast.failure(t('toast_failed'), errorText(err))
    }
  }

  const handleTogglePlugin = async (pluginId, next) => {
    setPlugins((prev) => prev.map((p) => (p.id === pluginId ? { ...p, enabled: next } : p)))
    try {
      await api.SetPluginEnabled(pluginId, next)
    } catch (err) {
      setPlugins((prev) => prev.map((p) => (p.id === pluginId ? { ...p, enabled: !next } : p)))
      toast.failure(t('toast_failed'), errorText(err))
    }
  }

  const handleExportExample = async () => {
    try {
      toast.success(t('toast_exported'), await api.ExportExamplePlugin())
    } catch (err) {
      toast.failure(t('toast_failed'), errorText(err))
    }
  }

  // --- 探测 -------------------------------------------------------------

  const handleProbe = async (model) => {
    setProbing((prev) => new Set(prev).add(model.uid))
    try {
      const result = isCatalog
        ? // 缓存里的模型不属于任何 Agent，插件 id 传空，后端用兜底规格
          await api.ProbeRemote('', model.base_url, model.api_key, model.id)
        : await api.ProbeModel(model.plugin_id, model.id)
      setProbeMap((prev) => ({ ...prev, [model.uid]: result }))
    } catch (err) {
      toast.failure(t('toast_failed'), errorText(err))
    } finally {
      setProbing((prev) => {
        const next = new Set(prev)
        next.delete(model.uid)
        return next
      })
    }
  }

  const handleTestAll = async () => {
    const targets = visibleGroups.flatMap((group) => group.models)
    if (targets.length === 0) return
    setTestingAll(true)
    setProbing(new Set(targets.map((m) => m.uid)))
    try {
      if (isCatalog) {
        const patch = {}
        for (const group of visibleGroups) {
          try {
            const results = (await api.ProbeUpstream('', group.upstreamId)) || {}
            Object.entries(results).forEach(([modelID, result]) => {
              patch[`${group.upstreamId}/${modelID}`] = result
            })
          } catch (err) {
            console.error(err)
          }
        }
        setProbeMap((prev) => ({ ...prev, ...patch }))
      } else {
        const results = (await api.ProbeModels(active, targets.map((m) => m.id))) || {}
        setProbeMap((prev) => ({ ...prev, ...results }))
      }
    } catch (err) {
      toast.failure(t('toast_failed'), errorText(err))
    } finally {
      setProbing(new Set())
      setTestingAll(false)
    }
  }

  // --- 模型（仅 Agent 视图）---------------------------------------------

  const handleToggleModel = async (model, next) => {
    const key = probeKey(model.plugin_id, model.id)
    setModels((prev) =>
      prev.map((item) => (probeKey(item.plugin_id, item.id) === key ? { ...item, enabled: next } : item)),
    )
    try {
      await api.SetModelEnabled(model.plugin_id, model.id, next)
    } catch (err) {
      setModels((prev) =>
        prev.map((item) =>
          probeKey(item.plugin_id, item.id) === key ? { ...item, enabled: model.enabled } : item,
        ),
      )
      toast.failure(t('toast_failed'), errorText(err))
    }
  }

  const openEditor = (model) => {
    setEditing(model)
    setEditorOpen(true)
  }

  const handleSubmitModel = async (payload) => {
    const pluginId = editing?.plugin_id || active
    if (!pluginId || pluginId === CACHE_SCOPE) throw new Error('no plugin selected')
    await api.SaveModel(pluginId, payload)
    setEditorOpen(false)
    setEditing(null)
    await Promise.all([loadModels(active), loadPlugins()])
    toast.success(t('toast_saved'))
  }

  // --- 删除（两种视图各自的语义）----------------------------------------

  const confirmDelete = async () => {
    const pending = pendingDelete
    setPendingDelete(null)
    if (!pending) return
    try {
      if (pending.kind === 'cache') {
        await api.RemoveUpstreamModel(pending.upstreamId, pending.model.id)
        await loadUpstreams()
      } else {
        await api.DeleteModel(pending.model.plugin_id, pending.model.id)
        await Promise.all([loadModels(active), loadPlugins()])
      }
      toast.success(t('toast_deleted'))
    } catch (err) {
      toast.failure(t('toast_failed'), errorText(err))
    }
  }

  // --- 拉取（向上游索要模型清单）----------------------------------------

  const runFetch = async (source, intent, ownerPluginID = '') => {
    setFetchState({ source, intent, owner: ownerPluginID, loading: true, error: '', remote: null })
    try {
      const remote = (await api.FetchModels(ownerPluginID, source.url, source.apiKey)) || []
      setFetchState({ source, intent, owner: ownerPluginID, loading: false, error: '', remote })
    } catch (err) {
      setFetchState({ source, intent, owner: ownerPluginID, loading: false, error: errorText(err), remote: null })
    }
  }

  const handleFetch = (group) => {
    // 分组里模型的所属插件只在 Agent 视图下有意义；缓存视图交给后端兜底。
    const owner = isCatalog ? '' : group.models[0]?.plugin_id || active
    runFetch(
      {
        label: group.label,
        url: group.url,
        apiKey: group.apiKey,
        existingIds: (group.models || []).map((m) => m.id),
        upstreamId: group.upstreamId,
      },
      isCatalog ? 'cache' : 'deploy',
      owner,
    )
  }

  const handleFetchConfirm = async ({ pluginID, models: payload }) => {
    const upstreamId = fetchState?.source?.upstreamId

    if (fetchState?.intent === 'cache') {
      // 缓存视图：把选中的模型并入该上游，不碰任何 Agent 的配置。
      if (!upstreamId) throw new Error(t('fetch_need_upstream'))
      const added = await api.AddUpstreamModels(upstreamId, payload)
      setFetchState(null)
      await loadUpstreams()
      toast.success(t('toast_cached'), `+${added}`)
      return
    }

    const result = await api.ApplyModels({ plugin_id: pluginID, models: payload })
    setFetchState(null)
    await Promise.all([loadModels(active), loadPlugins()])
    toast.success(t('toast_applied'), `${result.written} → ${result.file}`)
  }

  const handleManualAdd = async (payload) => {
    const upstreamId = manualAddGroup?.upstreamId
    if (!upstreamId) throw new Error(t('fetch_need_upstream'))
    const added = await api.AddUpstreamModels(upstreamId, [
      { id: payload.id, display_name: payload.display_name || payload.id },
    ])
    if (added === 0) throw new Error(t('manual_add_exists'))
    setManualAddGroup(null)
    await loadUpstreams()
    toast.success(t('manual_add_done'), payload.id)
  }

  // --- 应用到 Agent ------------------------------------------------------

  const handleApply = async ({ pluginID, models: payload }) => {
    const result = await api.ApplyModels({ plugin_id: pluginID, models: payload })
    setApplySource(null)
    await Promise.all([loadModels(active), loadPlugins()])
    toast.success(t('toast_applied'), `${result.written} → ${result.file}`)
  }

  const toApplySource = (group) => ({
    label: group.label,
    url: group.url,
    apiKey: group.apiKey,
    models: (group.models || []).map((m) => ({
      id: m.id,
      display_name: m.display_name,
      provider: m.provider,
      base_url: m.base_url,
      api_key: m.api_key,
      capabilities: m.capabilities,
    })),
  })

  // --- 上游目录 ---------------------------------------------------------

  const openUpstreamEditor = (item) => {
    setUpstreamEditing(item)
    setUpstreamEditorOpen(true)
  }

  const handleEditorFetch = async (url, apiKey) =>
    // 新增上游与任何 Agent 无关，交给后端的 OpenAI 兼容兜底规格即可。
    (await api.FetchModels('', url, apiKey)) || []

  const handleEditorSave = async (item) => {
    await api.SaveUpstream(item)
    setUpstreamEditorOpen(false)
    setUpstreamEditing(null)
    await loadUpstreams()
    toast.success(t('toast_saved'))
  }

  const handleImportUpstreams = async (request) => {
    try {
      const result = await api.ImportUpstreams(request)
      await loadUpstreams()
      toast.success(
        t('toast_upstream_imported'),
        `+${result.added} · ~${result.updated} · ${result.total}`,
      )
    } catch (err) {
      toast.failure(t('toast_failed'), errorText(err))
    }
  }

  const handleDeleteUpstream = async (id) => {
    try {
      await api.DeleteUpstream(id)
      await loadUpstreams()
      toast.success(t('toast_deleted'))
    } catch (err) {
      toast.failure(t('toast_failed'), errorText(err))
    }
  }

  // --- 备份 -------------------------------------------------------------

  const reloadBackups = useCallback(async (pluginID) => {
    if (!pluginID) return
    try {
      setBackups((await api.ListBackups(pluginID)) || [])
    } catch (err) {
      console.error(err)
    }
  }, [])

  const openBackups = async () => {
    const target = isCatalog ? plugins[0]?.id || '' : active
    if (!target) return
    setBackupPluginID(target)
    setBackupOpen(true)
    await reloadBackups(target)
  }

  const handleSelectBackupPlugin = async (pluginID) => {
    setBackupPluginID(pluginID)
    await reloadBackups(pluginID)
  }

  const afterConfigWrite = async () => {
    await Promise.all([reloadBackups(backupPluginID), loadModels(active), loadPlugins()])
  }

  const handleRestore = async (name) => {
    setBackupBusy(true)
    try {
      await api.RestoreBackup(backupPluginID, name)
      await afterConfigWrite()
      toast.success(t('toast_restored'))
    } catch (err) {
      toast.failure(t('toast_failed'), errorText(err))
    } finally {
      setBackupBusy(false)
    }
  }

  const handleClearBackups = async () => {
    setBackupBusy(true)
    try {
      await api.ClearBackups(backupPluginID)
      await afterConfigWrite()
    } catch (err) {
      toast.failure(t('toast_failed'), errorText(err))
    } finally {
      setBackupBusy(false)
    }
  }

  // --- 设置 -------------------------------------------------------------

  const handleUpdateSettings = async (patch) => {
    const merged = { ...(settings || {}), ...patch }
    setSettings(merged)
    try {
      await api.UpdateSettings(merged)
      if (patch.log_level) await loadSettings()
      if (patch.language) await changeLang(patch.language)
    } catch (err) {
      toast.failure(t('toast_failed'), errorText(err))
    }
  }

  const handleSetSourcePath = async (pluginId, path) => {
    try {
      await api.SetSourcePath(pluginId, path)
      await Promise.all([loadPlugins(), loadModels(active), loadSettings(), loadUpstreams()])
      toast.success(t('toast_saved'))
    } catch (err) {
      toast.failure(t('toast_failed'), errorText(err))
    }
  }

  // --- 视图：把两种数据源都归一成「上游分组」----------------------------

  const matches = useCallback(
    (model, url, provider) => {
      const needle = query.trim().toLowerCase()
      if (filter === 'enabled' && !model.enabled) return false
      if (filter === 'problem' && !['fail', 'auth', 'error'].includes(model.probe?.status)) return false
      if (!needle) return true
      return [model.id, model.display_name, provider, url]
        .filter(Boolean)
        .some((value) => String(value).toLowerCase().includes(needle))
    },
    [query, filter],
  )

  // 缓存视图：上游目录本身就是分组，模型补上接入信息与探测结果。
  // 空上游也要显示——刚建好、还没拉取的上游如果被过滤掉，连拉取入口都没有了；
  // 只有在用户主动搜索/筛选时才隐藏它，那时它确实没有可匹配的内容。
  const narrowing = query.trim() !== '' || filter !== 'all'

  const catalogGroups = useMemo(
    () =>
      upstreams
        .map((item) => {
          const models = (item.models || [])
            .map((m) => {
              const uid = `${item.id}/${m.id}`
              return {
                uid,
                id: m.id,
                display_name: m.display_name || m.id,
                provider: item.vendor,
                base_url: item.url,
                api_key: item.api_key,
                capabilities: m.capabilities,
                upstream_id: item.id,
                probe: probeMap[uid] || null,
              }
            })
            .filter((m) => matches(m, item.url, item.vendor))
          return {
            key: `up:${item.id}`,
            upstreamId: item.id,
            label: item.name,
            url: item.url,
            apiKey: item.api_key,
            models,
          }
        })
        .filter((group) => group.models.length > 0 || !narrowing),
    [upstreams, matches, probeMap, narrowing],
  )

  // Agent 视图：该 Agent 配置里的模型，按接入点分组。
  const agentGroups = useMemo(
    () =>
      groupByUpstream(models, t)
        .map((group) => ({
          ...group,
          upstreamId: '',
          models: group.models
            .map((m) => {
              const uid = probeKey(m.plugin_id, m.id)
              return { ...m, uid, probe: probeMap[uid] || m.probe }
            })
            .filter((m) => matches(m, group.url, m.provider)),
        }))
        .filter((group) => group.models.length > 0),
    [models, matches, probeMap, t],
  )

  const visibleGroups = isCatalog ? catalogGroups : agentGroups

  const cacheModelCount = useMemo(
    () => upstreams.reduce((sum, item) => sum + (item.models?.length || 0), 0),
    [upstreams],
  )

  const stats = useMemo(() => {
    const all = visibleGroups.flatMap((group) => group.models)
    return {
      total: all.length,
      enabled: all.filter((m) => m.enabled).length,
      healthy: all.filter((m) => m.probe?.status === 'ok').length,
    }
  }, [visibleGroups])

  const emptyState = isCatalog ? (
    <EmptyState
      title={t('cache_empty_title')}
      desc={t('cache_empty_desc')}
      action={
        <Button variant="primary" icon={Icons.plus} onClick={() => openUpstreamEditor(null)}>
          {t('upstream_add')}
        </Button>
      }
    />
  ) : (
    <EmptyState
      title={t('empty_title')}
      desc={t('empty_desc')}
      action={
        <Button variant="primary" icon={Icons.plus} onClick={() => setUpstreamOpen(true)}>
          {t('upstream_title')}
        </Button>
      }
    />
  )

  return (
    <div className="flex h-full flex-col bg-slate-50 text-slate-800">
      <TitleBar title={t('app_name')} subtitle={t('app_tagline')} />

      <div className="flex min-h-0 flex-1">
        <Sidebar
          t={t}
          plugins={plugins}
          active={active}
          cacheCount={upstreams.length}
          cacheModels={cacheModelCount}
          onSelect={setActive}
          onTogglePlugin={handleTogglePlugin}
          onReload={handleReloadPlugins}
          onOpenPluginDir={() => api.OpenPluginDir()}
          onExportExample={handleExportExample}
          onOpenUpstreams={() => setUpstreamOpen(true)}
          onOpenSettings={() => setSettingsOpen(true)}
        />

        <main className="flex min-w-0 flex-1 flex-col">
          {/* 工具栏：搜索可伸缩、按钮 shrink-0、整体允许换行，任何窗口宽度下都不会溢出。
              拉取挂在每个上游分组的标题行上——它是分组级动作。 */}
          <div className="flex min-h-12 shrink-0 flex-wrap items-center gap-2 border-b border-slate-200 bg-white px-3 py-2">
            <div className="relative min-w-[120px] max-w-[220px] flex-1">
              <Icons.search className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
              <TextInput
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder={t('search_placeholder')}
                className="pl-8"
              />
            </div>

            <Select
              value={filter}
              onChange={(event) => setFilter(event.target.value)}
              className="w-24 shrink-0"
            >
              <option value="all">{t('filter_all')}</option>
              {isCatalog ? null : <option value="enabled">{t('filter_enabled')}</option>}
              <option value="problem">{t('filter_problem')}</option>
            </Select>

            <div className="ml-auto flex shrink-0 items-center gap-2">
              <Button icon={Icons.refresh} onClick={() => loadModels(active)} disabled={loading}>
                {t('btn_reload')}
              </Button>
              <Button
                icon={testingAll ? Spinner : Icons.beaker}
                onClick={handleTestAll}
                disabled={testingAll || visibleGroups.length === 0}
              >
                {t('btn_test_all')}
              </Button>
              <Button
                icon={Icons.history}
                onClick={openBackups}
                disabled={plugins.length === 0}
                title={t('btn_backups_hint')}
              >
                {t('btn_backups')}
              </Button>
              <Button
                variant="primary"
                icon={Icons.plus}
                onClick={() => openUpstreamEditor(null)}
              >
                {t('upstream_add')}
              </Button>
            </div>
          </div>

          <div className="flex shrink-0 flex-wrap items-center gap-x-5 gap-y-1 border-b border-slate-200 bg-white px-4 py-1.5 text-[11px] text-slate-500">
            <span className="font-medium text-slate-600">
              {isCatalog ? t('sidebar_cache') : activePlugin?.name}
            </span>
            <span>
              {t('stat_total')} <b className="font-semibold text-slate-700">{stats.total}</b>
            </span>
            {isCatalog ? null : (
              <span>
                {t('stat_enabled')} <b className="font-semibold text-slate-700">{stats.enabled}</b>
              </span>
            )}
            <span>
              {t('stat_healthy')} <b className="font-semibold text-emerald-600">{stats.healthy}</b>
            </span>
            <span>
              {t('stat_upstreams')}{' '}
              <b className="font-semibold text-slate-700">{visibleGroups.length}</b>
            </span>
            {activePlugin && !activePlugin.source_ready ? (
              <span className="ml-auto text-amber-600">
                {t('plugin_source_missing')} · {activePlugin.source_file}
              </span>
            ) : null}
          </div>

          <div className="min-h-0 flex-1 overflow-auto">
            {loading ? (
              <div className="flex h-full items-center justify-center gap-2 text-xs text-slate-400">
                <Spinner className="h-4 w-4" />
                {t('btn_reload')}
              </div>
            ) : visibleGroups.length === 0 ? (
              emptyState
            ) : (
              <ModelTable
                t={t}
                mode={mode}
                groups={visibleGroups}
                plugins={plugins}
                probing={probing}
                showPlugin={!isCatalog}
                searching={query.trim() !== ''}
                onToggle={handleToggleModel}
                onProbe={handleProbe}
                onEdit={openEditor}
                onDelete={(group, model) =>
                  setPendingDelete(
                    isCatalog
                      ? { kind: 'cache', upstreamId: group.upstreamId, model }
                      : { kind: 'model', model },
                  )
                }
                onFetch={handleFetch}
                onApply={(group) => setApplySource(toApplySource(group))}
                onAddModel={setManualAddGroup}
              />
            )}
          </div>
        </main>
      </div>

      <ModelEditor
        open={editorOpen}
        model={editing}
        plugin={editing ? plugins.find((p) => p.id === editing.plugin_id) : activePlugin}
        t={t}
        onClose={() => {
          setEditorOpen(false)
          setEditing(null)
        }}
        onSubmit={handleSubmitModel}
      />

      <ApplyDialog
        open={Boolean(applySource)}
        source={applySource}
        plugins={plugins}
        t={t}
        onClose={() => setApplySource(null)}
        onConfirm={handleApply}
      />

      <FetchDialog
        open={Boolean(fetchState)}
        source={fetchState?.source}
        intent={fetchState?.intent}
        plugins={plugins}
        defaultTarget={isCatalog ? '' : active}
        loading={Boolean(fetchState?.loading)}
        error={fetchState?.error}
        remote={fetchState?.remote}
        t={t}
        onClose={() => setFetchState(null)}
        onRetry={() => fetchState && runFetch(fetchState.source, fetchState.intent, fetchState.owner)}
        onConfirm={handleFetchConfirm}
      />

      <AddModelDialog
        open={Boolean(manualAddGroup)}
        upstreamLabel={manualAddGroup?.label}
        upstreamUrl={manualAddGroup?.url}
        t={t}
        onClose={() => setManualAddGroup(null)}
        onSubmit={handleManualAdd}
      />

      <UpstreamEditor
        open={upstreamEditorOpen}
        initial={upstreamEditing}
        t={t}
        onClose={() => {
          setUpstreamEditorOpen(false)
          setUpstreamEditing(null)
        }}
        onFetch={handleEditorFetch}
        onSave={handleEditorSave}
      />

      <UpstreamPanel
        open={upstreamOpen}
        upstreams={upstreams}
        t={t}
        onClose={() => setUpstreamOpen(false)}
        onDiscover={() => api.DiscoverUpstreams()}
        onImport={handleImportUpstreams}
        onApply={setApplySource}
        onFetchUpstream={handleFetch}
        onOpenEditor={openUpstreamEditor}
        onDelete={handleDeleteUpstream}
        onReveal={() => settings?.data_dir && api.RevealPath(settings.data_dir)}
      />

      <BackupPanel
        open={backupOpen}
        plugin={backupPlugin}
        plugins={plugins}
        backups={backups}
        busy={backupBusy}
        t={t}
        onClose={() => setBackupOpen(false)}
        onSelectPlugin={handleSelectBackupPlugin}
        onRestore={handleRestore}
        onClear={handleClearBackups}
        onOpenDir={() => backupPluginID && api.OpenBackupDir(backupPluginID)}
      />

      <SettingsPanel
        open={settingsOpen}
        settings={settings}
        plugins={plugins}
        langs={langs}
        lang={lang}
        t={t}
        onClose={() => setSettingsOpen(false)}
        onUpdate={handleUpdateSettings}
        onChooseFile={async () => {
          try {
            return (await api.OpenFileSelect()) || ''
          } catch (err) {
            toast.failure(t('toast_failed'), errorText(err))
            return ''
          }
        }}
        onSetSourcePath={handleSetSourcePath}
        onReveal={(path) => path && api.RevealPath(path)}
      />

      <ConfirmDialog
        open={Boolean(pendingDelete)}
        danger
        title={t('btn_delete')}
        message={pendingDelete?.kind === 'cache' ? t('cache_remove_confirm') : t('confirm_delete')}
        confirmLabel={t('btn_delete')}
        cancelLabel={t('btn_cancel')}
        onConfirm={confirmDelete}
        onCancel={() => setPendingDelete(null)}
      />
    </div>
  )
}
