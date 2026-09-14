package service

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"buddyswitch/global"
	"buddyswitch/plugin"
	"buddyswitch/upstream"
)

// DiscoverUpstreams 从各 Agent 的现有模型里派生出上游清单（按「接入点 + 密钥」分组）。
// 只做派生、不落盘，用于界面上「按上游分组」和导入前的预览。
func (a *App) DiscoverUpstreams() []upstream.Upstream {
	type bucket struct {
		item    upstream.Upstream
		origins map[string]bool
		seen    map[string]bool
	}
	groups := map[string]*bucket{}

	for _, def := range a.reg.List() {
		if !def.Loaded {
			continue
		}
		models, err := a.PullModels(def.ID)
		if err != nil {
			continue
		}
		for _, m := range models {
			if strings.TrimSpace(m.BaseURL) == "" {
				continue
			}
			id := upstream.IDFor(m.BaseURL, m.APIKey)
			g, ok := groups[id]
			if !ok {
				g = &bucket{
					item: upstream.Upstream{
						ID:     id,
						Name:   upstream.NameFor(m.BaseURL),
						Vendor: m.Provider,
						URL:    m.BaseURL,
						APIKey: m.APIKey,
					},
					origins: map[string]bool{},
					seen:    map[string]bool{},
				}
				groups[id] = g
			}
			g.origins[def.ID] = true
			if g.item.Vendor == "" {
				g.item.Vendor = m.Provider
			}
			if g.seen[m.ID] {
				continue
			}
			g.seen[m.ID] = true
			g.item.Models = append(g.item.Models, upstream.Model{
				ID:           m.ID,
				DisplayName:  m.DisplayName,
				Capabilities: m.Capabilities,
			})
		}
	}

	out := make([]upstream.Upstream, 0, len(groups))
	for _, g := range groups {
		for name := range g.origins {
			g.item.Origins = append(g.item.Origins, name)
		}
		sort.Strings(g.item.Origins)
		sort.Slice(g.item.Models, func(i, j int) bool { return g.item.Models[i].ID < g.item.Models[j].ID })
		out = append(out, g.item)
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].Models) != len(out[j].Models) {
			return len(out[i].Models) > len(out[j].Models)
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// FetchModels 用上游自己的密钥去问它「你提供哪些模型」。
//
// 与 PullModels 的区别：PullModels 读的是本地配置文件里已有的模型，
// 这里问的是上游自己——所以还没被任何 agent 用上的新模型也能被发现。
//
// pluginID 只是用来挑一个可选的清单接口定义：传空、或该插件没定义 listing 时，
// 会退回到 OpenAI 兼容的兜底规格。上游缓存因此完全不依赖 Agent 插件。
func (a *App) FetchModels(pluginID string, url string, apiKey string) ([]plugin.RemoteModel, error) {
	spec := plugin.DefaultListing()
	if def, err := a.reg.Get(pluginID); err == nil && def.Loaded {
		spec = def.ListingFor()
	}

	models, err := plugin.FetchListingModels(a.probeContext(), spec, pluginID, url, apiKey)
	if err != nil {
		global.Log.Warnf("向上游 %s 索要模型清单失败: %v", url, err)
		return nil, err
	}
	global.Log.Infof("从上游 %s 索要到 %d 个模型", url, len(models))
	return models, nil
}

// specForPlugin 返回该插件适用的 probe 规格，插件缺失或无效时用兜底。
func (a *App) specForPlugin(pluginID string) plugin.ProbeSpec {
	if def, err := a.reg.Get(pluginID); err == nil && def.Loaded {
		return def.ProbeFor()
	}
	return plugin.DefaultProbe()
}

// ProbeRemote 对一条「不在任何 Agent 配置里」的模型发起探测。
//
// 上游缓存里的模型正是这种情况：它们只存在于 BuddySwitch 自己的目录中。
// pluginID 只用来挑探测规格，缺失时退回 OpenAI 兼容的兜底。
func (a *App) ProbeRemote(pluginID string, rawURL string, apiKey string, modelID string) (plugin.ProbeResult, error) {
	if strings.TrimSpace(modelID) == "" {
		return plugin.ProbeResult{}, fmt.Errorf("%s", global.T("err_model_id_empty"))
	}
	m := plugin.Model{
		ID:      modelID,
		BaseURL: rawURL,
		APIKey:  apiKey,
		Enabled: true,
	}
	res := plugin.RunProbeSpec(a.probeContext(), a.specForPlugin(pluginID), pluginID, m)
	global.Log.Infof("探测缓存模型 %s @ %s -> %s (%dms)", modelID, rawURL, res.Status, res.LatencyMs)
	return res, nil
}

// ProbeUpstream 探测某个缓存上游下的全部模型，返回键为模型 id 的结果表。
func (a *App) ProbeUpstream(pluginID string, upstreamID string) (map[string]plugin.ProbeResult, error) {
	item, ok := a.upstreams.Get(upstreamID)
	if !ok {
		return nil, fmt.Errorf("%s", global.T("err_upstream_not_found", upstreamID))
	}
	spec := a.specForPlugin(pluginID)

	targets := make([]plugin.Model, 0, len(item.Models))
	for _, um := range item.Models {
		name := um.DisplayName
		if strings.TrimSpace(name) == "" {
			name = um.ID
		}
		targets = append(targets, plugin.Model{
			ID:           um.ID,
			DisplayName:  name,
			Provider:     item.Vendor,
			BaseURL:      item.URL,
			APIKey:       item.APIKey,
			Enabled:      true,
			Capabilities: um.Capabilities,
		})
	}

	results := make(map[string]plugin.ProbeResult, len(targets))
	if len(targets) == 0 {
		return results, nil
	}

	ctx := a.probeContext()
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxProbeWorkers)
	for _, m := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func(m plugin.Model) {
			defer wg.Done()
			defer func() { <-sem }()
			res := plugin.RunProbeSpec(ctx, spec, pluginID, m)
			mu.Lock()
			results[m.ID] = res
			mu.Unlock()
		}(m)
	}
	wg.Wait()
	global.Log.Infof("已探测上游 %s 的 %d 个缓存模型", item.Name, len(results))
	return results, nil
}

// RemoveUpstreamModel 从上游目录里摘掉一个模型（不影响任何 Agent 的配置）。
func (a *App) RemoveUpstreamModel(upstreamID string, modelID string) error {
	item, ok := a.upstreams.Get(upstreamID)
	if !ok {
		return fmt.Errorf("%s", global.T("err_upstream_not_found", upstreamID))
	}
	kept := make([]upstream.Model, 0, len(item.Models))
	removed := false
	for _, m := range item.Models {
		if m.ID == modelID {
			removed = true
			continue
		}
		kept = append(kept, m)
	}
	if !removed {
		return fmt.Errorf("%s", global.T("err_upstream_model_missing", item.Name, modelID))
	}
	item.Models = kept
	if _, err := a.upstreams.Put(item); err != nil {
		return err
	}
	global.Log.Infof("已从上游 %s 的缓存中移除模型 %s", item.Name, modelID)
	return nil
}

// AddUpstreamModels 把一批模型追加进某个上游的缓存（已存在则跳过）。
func (a *App) AddUpstreamModels(upstreamID string, models []plugin.Model) (int, error) {
	item, ok := a.upstreams.Get(upstreamID)
	if !ok {
		return 0, fmt.Errorf("%s", global.T("err_upstream_not_found", upstreamID))
	}
	existing := make(map[string]bool, len(item.Models))
	for _, m := range item.Models {
		existing[m.ID] = true
	}
	added := 0
	for _, m := range models {
		id := strings.TrimSpace(m.ID)
		if id == "" || existing[id] {
			continue
		}
		existing[id] = true
		name := strings.TrimSpace(m.DisplayName)
		if name == "" {
			name = id
		}
		item.Models = append(item.Models, upstream.Model{
			ID:           id,
			DisplayName:  name,
			Capabilities: m.Capabilities,
		})
		added++
	}
	if added == 0 {
		return 0, nil
	}
	if _, err := a.upstreams.Put(item); err != nil {
		return 0, err
	}
	global.Log.Infof("已向缓存上游 %s 追加 %d 个模型", item.Name, added)
	return added, nil
}

// ListUpstreams 返回程序级上游目录。
func (a *App) ListUpstreams() []upstream.Upstream {
	return a.upstreams.List()
}

// ImportResult 汇报一次导入合并的结果。
type ImportResult struct {
	Added   int `json:"added"`
	Updated int `json:"updated"`
	Total   int `json:"total"`
}

// ImportRequest 描述一次「选择性拉取」：只把用户勾中的上游并入目录。
type ImportRequest struct {
	// UpstreamIDs 是选中的上游 id。为空表示什么都不导入——
	// 拉取与否必须由用户决定，不做全量灌入。
	UpstreamIDs []string `json:"upstream_ids"`
	// ModelIDs 可选，键为上游 id，值为该上游下只导入的模型 id；缺省或为空表示该上游全部模型。
	ModelIDs map[string][]string `json:"model_ids,omitempty"`
}

// ImportUpstreams 把选中的上游合并进目录。
func (a *App) ImportUpstreams(req ImportRequest) ImportResult {
	total := len(a.upstreams.List())
	if len(req.UpstreamIDs) == 0 {
		global.Log.Info("上游导入：未选择任何条目，已跳过")
		return ImportResult{Total: total}
	}

	want := make(map[string]bool, len(req.UpstreamIDs))
	for _, id := range req.UpstreamIDs {
		want[id] = true
	}

	selected := make([]upstream.Upstream, 0, len(req.UpstreamIDs))
	for _, item := range a.DiscoverUpstreams() {
		if !want[item.ID] {
			continue
		}
		// 允许再细到模型级别：用户可能只想从某个上游里挑几个模型。
		if ids, ok := req.ModelIDs[item.ID]; ok && len(ids) > 0 {
			keep := make(map[string]bool, len(ids))
			for _, id := range ids {
				keep[id] = true
			}
			filtered := make([]upstream.Model, 0, len(ids))
			for _, m := range item.Models {
				if keep[m.ID] {
					filtered = append(filtered, m)
				}
			}
			item.Models = filtered
		}
		selected = append(selected, item)
	}

	added, updated, err := a.upstreams.Merge(selected)
	if err != nil {
		global.Log.Warnf("保存上游目录失败: %v", err)
	}
	global.Log.Infof("上游目录已同步：选中 %d 个、新增 %d、更新 %d", len(selected), added, updated)
	return ImportResult{Added: added, Updated: updated, Total: len(a.upstreams.List())}
}

// SaveUpstream 新增或更新一个上游条目。
func (a *App) SaveUpstream(item upstream.Upstream) (upstream.Upstream, error) {
	saved, err := a.upstreams.Put(item)
	if err != nil {
		return upstream.Upstream{}, err
	}
	global.Log.Infof("上游 %s 已保存", saved.Name)
	return saved, nil
}

// DeleteUpstream 从目录中移除一个上游。
func (a *App) DeleteUpstream(id string) error {
	removed, err := a.upstreams.Delete(id)
	if err != nil {
		return err
	}
	if !removed {
		return fmt.Errorf("%s", global.T("err_upstream_not_found", id))
	}
	global.Log.Infof("上游 %s 已从目录中移除", id)
	return nil
}

// ApplyModelsRequest 描述「把一批模型写入某个 Agent」的请求。
//
// 模型列表里自带 url / apiKey，所以调用方既可以直接搬用上游目录的条目，
// 也可以直接搬用模型列表里某个上游分组——走的是同一条路径。
type ApplyModelsRequest struct {
	PluginID string         `json:"plugin_id"`
	Models   []plugin.Model `json:"models"`
}

// ApplyModelsResult 汇报一次写入的结果。
type ApplyModelsResult struct {
	Written int            `json:"written"`
	File    string         `json:"file"`
	Models  []plugin.Model `json:"models"`
}

// ApplyModels 把一批模型写入目标 Agent 的原生配置文件。
//
// 整批只写一次盘、只留一份快照——批量导入不该把备份目录刷满。
func (a *App) ApplyModels(req ApplyModelsRequest) (ApplyModelsResult, error) {
	if len(req.Models) == 0 {
		return ApplyModelsResult{}, fmt.Errorf("%s", global.T("err_no_model_selected"))
	}
	def, file, err := a.requireSource(req.PluginID)
	if err != nil {
		return ApplyModelsResult{}, err
	}

	doc, err := plugin.LoadDocument(def, file)
	if err != nil {
		return ApplyModelsResult{}, err
	}

	written := 0
	for _, incoming := range req.Models {
		id := strings.TrimSpace(incoming.ID)
		if id == "" {
			continue
		}
		name := strings.TrimSpace(incoming.DisplayName)
		if name == "" {
			name = id
		}
		m := plugin.Model{
			ID:           id,
			DisplayName:  name,
			Provider:     incoming.Provider,
			BaseURL:      strings.TrimSpace(incoming.BaseURL),
			APIKey:       incoming.APIKey,
			Enabled:      true,
			Description:  incoming.Description,
			Tags:         incoming.Tags,
			Capabilities: incoming.Capabilities,
		}
		// 搬运语义：上游缓存里并不知道目标 Agent 原有的备注、标签，
		// 所以只写这次确实带过来的字段，不顺手清空别的。
		if err := doc.Upsert(m, false); err != nil {
			return ApplyModelsResult{}, err
		}
		written++
	}
	if written == 0 {
		return ApplyModelsResult{}, fmt.Errorf("%s", global.T("err_no_writable_model"))
	}

	a.captureBackup(req.PluginID, file)
	if err := doc.Save(); err != nil {
		return ApplyModelsResult{}, err
	}
	global.Log.Infof("%d 个模型已写入 %s", written, file)

	models, err := doc.Models()
	if err != nil {
		return ApplyModelsResult{Written: written, File: file}, nil
	}
	out := make([]plugin.Model, 0, len(models))
	for _, m := range models {
		out = append(out, a.decorate(m))
	}
	return ApplyModelsResult{Written: written, File: file, Models: out}, nil
}
