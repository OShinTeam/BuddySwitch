package service

import (
	"fmt"
	"sort"
	"strings"

	"buddyswitch/global"
	"buddyswitch/plugin"
	"buddyswitch/store"
)

// PullModels 从某个 Agent 的原生配置文件拉取其全部模型。
func (a *App) PullModels(pluginID string) ([]plugin.Model, error) {
	def, file, err := a.requireSource(pluginID)
	if err != nil {
		return nil, err
	}
	doc, err := plugin.LoadDocument(def, file)
	if err != nil {
		global.Log.Errorf("读取 %s 的模型配置失败: %v", pluginID, err)
		return nil, err
	}
	models, err := doc.Models()
	if err != nil {
		global.Log.Errorf("解析 %s 的模型配置失败: %v", pluginID, err)
		return nil, err
	}
	for i := range models {
		models[i] = a.decorate(models[i])
	}
	sort.SliceStable(models, func(i, j int) bool {
		return models[i].DisplayName < models[j].DisplayName
	})
	global.Log.Infof("已从 %s 拉取 %d 个模型", pluginID, len(models))
	return models, nil
}

// PullAllModels 汇总所有已启用插件的模型，用于「全部」视图与批量测试。
// 单个插件读取失败不会中断整体，只记日志。
func (a *App) PullAllModels() []plugin.Model {
	var out []plugin.Model
	for _, def := range a.reg.List() {
		if !a.state.PluginEnabled(def.ID, def.DefaultEnabled()) {
			continue
		}
		models, err := a.PullModels(def.ID)
		if err != nil {
			global.Log.Warnf("拉取 %s 的模型失败，已跳过: %v", def.ID, err)
			continue
		}
		out = append(out, models...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].PluginID != out[j].PluginID {
			return out[i].PluginID < out[j].PluginID
		}
		return out[i].DisplayName < out[j].DisplayName
	})
	return out
}

// SaveModel 新增或更新一个模型，并写回 Agent 的原生配置。
func (a *App) SaveModel(pluginID string, m plugin.Model) (plugin.Model, error) {
	def, file, err := a.requireSource(pluginID)
	if err != nil {
		return plugin.Model{}, err
	}
	m.ID = strings.TrimSpace(m.ID)
	m.BaseURL = strings.TrimSpace(m.BaseURL)
	m.Provider = strings.TrimSpace(m.Provider)
	m.DisplayName = strings.TrimSpace(m.DisplayName)
	if m.ID == "" {
		return plugin.Model{}, fmt.Errorf("%s", global.T("err_model_id_empty"))
	}
	if m.DisplayName == "" {
		m.DisplayName = m.ID
	}

	doc, err := plugin.LoadDocument(def, file)
	if err != nil {
		return plugin.Model{}, err
	}
	if err := doc.Upsert(m, true); err != nil {
		return plugin.Model{}, err
	}
	a.captureBackup(pluginID, file)
	if err := doc.Save(); err != nil {
		return plugin.Model{}, err
	}
	if !def.CanPersistEnabled() {
		// 原生配置里没有启用开关，启停只能记在 BuddySwitch 自己的状态文件里。
		_ = a.state.SetModelEnabled(store.ModelKey(pluginID, m.ID), m.Enabled)
	}
	global.Log.Infof("模型 %s/%s 已写入 %s", pluginID, m.ID, file)

	saved := m
	saved.PluginID = pluginID
	saved.SourceFile = file
	saved.NativeEnabled = def.CanPersistEnabled()
	return a.decorate(saved), nil
}

// DeleteModel 从 Agent 的原生配置中移除一个模型。
func (a *App) DeleteModel(pluginID string, modelID string) error {
	def, file, err := a.requireSource(pluginID)
	if err != nil {
		return err
	}
	doc, err := plugin.LoadDocument(def, file)
	if err != nil {
		return err
	}
	removed, err := doc.Remove(modelID)
	if err != nil {
		return err
	}
	if !removed {
		return fmt.Errorf("%s", global.T("err_model_not_found_in_file", file, modelID))
	}
	a.captureBackup(pluginID, file)
	if err := doc.Save(); err != nil {
		return err
	}
	global.Log.Infof("模型 %s/%s 已从配置中删除", pluginID, modelID)
	return nil
}

// SetModelEnabled 切换模型的启用状态。
//
// 若插件定义了原生启用字段，则写回 Agent 配置文件；否则只记录在 BuddySwitch
// 自己的状态文件里，不污染对方的配置。
func (a *App) SetModelEnabled(pluginID string, modelID string, enabled bool) error {
	def, file, err := a.requireSource(pluginID)
	if err != nil {
		return err
	}
	key := store.ModelKey(pluginID, modelID)
	if !def.CanPersistEnabled() {
		return a.state.SetModelEnabled(key, enabled)
	}

	doc, err := plugin.LoadDocument(def, file)
	if err != nil {
		return err
	}
	models, err := doc.Models()
	if err != nil {
		return err
	}
	var target *plugin.Model
	for i := range models {
		if models[i].ID == modelID {
			target = &models[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("%s", global.T("err_model_not_found_in_file", file, modelID))
	}
	target.Enabled = enabled
	// 这是「改一个开关」，不是重写整条记录：走合并语义，
	// 不碰用户没在这里表达意见的字段。
	if err := doc.Upsert(*target, false); err != nil {
		return err
	}
	a.captureBackup(pluginID, file)
	if err := doc.Save(); err != nil {
		return err
	}
	return nil
}

// decorate 把本地状态（启停覆盖、探测结果）合并进模型。
func (a *App) decorate(m plugin.Model) plugin.Model {
	key := store.ModelKey(m.PluginID, m.ID)
	if !m.NativeEnabled {
		if v, ok := a.state.ModelEnabled(key); ok {
			m.Enabled = v
		}
	}
	m.Probe = a.state.Probe(key)
	return m
}
