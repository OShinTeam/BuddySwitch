package service

import (
	"context"
	"fmt"
	"sync"

	"buddyswitch/global"
	"buddyswitch/plugin"
	"buddyswitch/store"
)

// maxProbeWorkers 限制并发探测数，避免一次性打满连接数或触发服务端限流。
const maxProbeWorkers = 4

type probeJob struct {
	pluginID string
	model    plugin.Model
}

// ProbeModel 对单个模型发起探测并返回结果。
func (a *App) ProbeModel(pluginID string, modelID string) (plugin.ProbeResult, error) {
	def, _, err := a.requireSource(pluginID)
	if err != nil {
		return plugin.ProbeResult{}, err
	}
	models, err := a.PullModels(pluginID)
	if err != nil {
		return plugin.ProbeResult{}, err
	}
	for _, m := range models {
		if m.ID != modelID {
			continue
		}
		res := def.RunProbe(a.probeContext(), m)
		key := store.ModelKey(pluginID, modelID)
		if err := a.state.SetProbe(key, res); err != nil {
			global.Log.Warnf("保存探测结果失败: %v", err)
		}
		global.Log.Infof("探测 %s/%s -> %s (%dms)", pluginID, modelID, res.Status, res.LatencyMs)
		return res, nil
	}
	return plugin.ProbeResult{}, fmt.Errorf("%s", global.T("err_model_not_found", pluginID, modelID))
}

// ProbeModels 并发探测某个插件下指定的一批模型，返回键为 "插件id/模型id" 的结果表。
func (a *App) ProbeModels(pluginID string, modelIDs []string) (map[string]plugin.ProbeResult, error) {
	if _, _, err := a.requireSource(pluginID); err != nil {
		return nil, err
	}
	models, err := a.PullModels(pluginID)
	if err != nil {
		return nil, err
	}
	want := make(map[string]bool, len(modelIDs))
	for _, id := range modelIDs {
		want[id] = true
	}
	jobs := make([]probeJob, 0, len(models))
	for _, m := range models {
		if len(want) == 0 || want[m.ID] {
			jobs = append(jobs, probeJob{pluginID: pluginID, model: m})
		}
	}
	return a.runProbes(jobs), nil
}

// ProbeAll 探测全部已启用插件下的所有模型。
func (a *App) ProbeAll() (map[string]plugin.ProbeResult, error) {
	var jobs []probeJob
	for _, def := range a.reg.List() {
		if !a.state.PluginEnabled(def.ID, def.DefaultEnabled()) {
			continue
		}
		models, err := a.PullModels(def.ID)
		if err != nil {
			global.Log.Warnf("拉取 %s 失败，跳过探测: %v", def.ID, err)
			continue
		}
		for _, m := range models {
			jobs = append(jobs, probeJob{pluginID: def.ID, model: m})
		}
	}
	return a.runProbes(jobs), nil
}

// runProbes 以固定并发度执行探测任务，结束后整批落盘一次。
func (a *App) runProbes(jobs []probeJob) map[string]plugin.ProbeResult {
	results := make(map[string]plugin.ProbeResult, len(jobs))
	if len(jobs) == 0 {
		return results
	}

	ctx := a.probeContext()
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxProbeWorkers)

	for _, job := range jobs {
		def, err := a.reg.Get(job.pluginID)
		if err != nil {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(def *plugin.Definition, job probeJob) {
			defer wg.Done()
			defer func() { <-sem }()

			res := def.RunProbe(ctx, job.model)

			mu.Lock()
			results[store.ModelKey(job.pluginID, job.model.ID)] = res
			mu.Unlock()
		}(def, job)
	}
	wg.Wait()

	// 整批只写一次盘。逐条落盘的话，第 k 条要重新序列化前 k-1 条结果，
	// 一次「测试全部」就是 O(n²) 的写入量；而这些结果丢一次也不可惜。
	if err := a.state.SetProbes(results); err != nil {
		global.Log.Warnf("保存探测结果失败: %v", err)
	}
	return results
}

func (a *App) probeContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}
