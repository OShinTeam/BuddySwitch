package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"buddyswitch/global"
)

// Placeholders 返回探测模板中可用的占位符及其取值。
func (m Model) Placeholders(pluginID string) map[string]string {
	return map[string]string{
		"id":           m.ID,
		"display_name": m.DisplayName,
		"displayName":  m.DisplayName,
		"provider":     m.Provider,
		"base_url":     strings.TrimRight(m.BaseURL, "/"),
		"baseUrl":      strings.TrimRight(m.BaseURL, "/"),
		"api_key":      m.APIKey,
		"apiKey":       m.APIKey,
		"plugin_id":    pluginID,
	}
}

// Render 用 {{field}} 占位符渲染字符串，未识别的占位符原样保留。
func Render(tpl string, vars map[string]string) string {
	if !strings.Contains(tpl, "{{") {
		return tpl
	}
	var b strings.Builder
	rest := tpl
	for {
		i := strings.Index(rest, "{{")
		if i < 0 {
			b.WriteString(rest)
			break
		}
		b.WriteString(rest[:i])
		rest = rest[i+2:]
		j := strings.Index(rest, "}}")
		if j < 0 {
			b.WriteString("{{")
			b.WriteString(rest)
			break
		}
		key := strings.TrimSpace(rest[:j])
		if v, ok := vars[key]; ok {
			b.WriteString(v)
		} else {
			b.WriteString("{{" + rest[:j] + "}}")
		}
		rest = rest[j+2:]
	}
	return b.String()
}

// RenderAny 递归渲染任意 JSON 结构中的字符串。
func RenderAny(v any, vars map[string]string) any {
	switch t := v.(type) {
	case string:
		return Render(t, vars)
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, item := range t {
			out[k] = RenderAny(item, vars)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = RenderAny(item, vars)
		}
		return out
	default:
		return v
	}
}

// Client 是探测使用的 HTTP 客户端，可由上层替换（例如注入代理或测试桩）。
var Client = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        16,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     60 * time.Second,
	},
}

// DefaultProbe 是 OpenAI 兼容的聊天接口探测规格。
//
// 上游缓存是程序级资产，不该依赖某个 Agent 插件才能测，所以给一份兜底：
// 插件没定义 probe（甚至一个插件都没有）时也能测缓存里的模型。
func DefaultProbe() ProbeSpec {
	return ProbeSpec{
		Type:       "openai",
		Endpoint:   "{{base_url}}",
		EnsurePath: "/chat/completions",
		Method:     http.MethodPost,
		Headers:    map[string]string{"Authorization": "Bearer {{api_key}}"},
		Body: map[string]any{
			"model":      "{{id}}",
			"messages":   []any{map[string]any{"role": "user", "content": "ping"}},
			"max_tokens": 1,
			"stream":     false,
		},
		TimeoutMs: 15000,
	}
}

// ProbeFor 返回插件自定义的 probe 规格，没定义就给 OpenAI 兼容的兜底。
func (def *Definition) ProbeFor() ProbeSpec {
	if strings.TrimSpace(def.Probe.Endpoint) != "" {
		return def.Probe
	}
	return DefaultProbe()
}

// RunProbe 按插件定义探测一个模型。
func (def *Definition) RunProbe(ctx context.Context, m Model) ProbeResult {
	return RunProbeSpec(ctx, def.ProbeFor(), def.ID, m)
}

// RunProbeSpec 按给定规格探测单个模型。
//
// 网络层错误一律收敛为 ProbeResult，不向上返回 error，方便批量测试时逐个汇报。
func RunProbeSpec(ctx context.Context, spec ProbeSpec, pluginID string, m Model) ProbeResult {
	started := time.Now()
	fail := func(status string, code int, msg string) ProbeResult {
		return ProbeResult{
			Status:     status,
			StatusCode: code,
			LatencyMs:  time.Since(started).Milliseconds(),
			Message:    msg,
			CheckedAt:  time.Now().Format(time.RFC3339),
		}
	}

	if strings.TrimSpace(m.BaseURL) == "" {
		return fail(ProbeError, 0, global.T("probe_msg_no_base_url"))
	}
	endpoint := strings.TrimSpace(spec.Endpoint)
	if endpoint == "" {
		return fail(ProbeError, 0, global.T("probe_msg_no_endpoint"))
	}

	vars := m.Placeholders(pluginID)
	url := buildEndpoint(endpoint, "", spec.EnsurePath, vars)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fail(ProbeError, 0, global.T("probe_msg_bad_url", url))
	}

	method := strings.ToUpper(strings.TrimSpace(spec.Method))
	if method == "" {
		if spec.Body != nil {
			method = http.MethodPost
		} else {
			method = http.MethodGet
		}
	}

	var payload []byte
	if spec.Body != nil {
		rendered := RenderAny(spec.Body, vars)
		data, err := json.Marshal(rendered)
		if err != nil {
			return fail(ProbeError, 0, global.T("probe_msg_body_failed", err))
		}
		payload = data
	}

	timeout := time.Duration(spec.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var bodyReader io.Reader
	if payload != nil {
		bodyReader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(reqCtx, method, url, bodyReader)
	if err != nil {
		return fail(ProbeError, 0, global.T("probe_msg_request_build_failed", err))
	}
	for k, v := range spec.Headers {
		req.Header.Set(k, Render(v, vars))
	}
	if payload != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "BuddySwitch/1.0")

	resp, err := Client.Do(req)
	if err != nil {
		if reqCtx.Err() == context.DeadlineExceeded {
			return fail(ProbeError, 0, global.T("probe_msg_timeout", timeout))
		}
		return fail(ProbeError, 0, global.T("probe_msg_request_failed", err))
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	elapsed := time.Since(started).Milliseconds()

	if expectedStatus(spec, resp.StatusCode) {
		return ProbeResult{
			Status:     ProbeOK,
			StatusCode: resp.StatusCode,
			LatencyMs:  elapsed,
			Message:    global.T("probe_msg_ok"),
			CheckedAt:  time.Now().Format(time.RFC3339),
		}
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return ProbeResult{
			Status:     ProbeAuth,
			StatusCode: resp.StatusCode,
			LatencyMs:  elapsed,
			Message:    global.T("probe_msg_auth"),
			CheckedAt:  time.Now().Format(time.RFC3339),
		}
	case http.StatusNotFound:
		return ProbeResult{
			Status:     ProbeFail,
			StatusCode: resp.StatusCode,
			LatencyMs:  elapsed,
			Message:    global.T("probe_msg_not_found"),
			CheckedAt:  time.Now().Format(time.RFC3339),
		}
	}
	return ProbeResult{
		Status:     ProbeFail,
		StatusCode: resp.StatusCode,
		LatencyMs:  elapsed,
		Message:    summarize(raw),
		CheckedAt:  time.Now().Format(time.RFC3339),
	}
}

func expectedStatus(spec ProbeSpec, code int) bool {
	if len(spec.ExpectStatus) >= 2 {
		return code >= spec.ExpectStatus[0] && code <= spec.ExpectStatus[1]
	}
	return code >= 200 && code < 300
}

// summarize 从响应体里抽出一句可读的失败原因。
func summarize(raw []byte) string {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return global.T("probe_msg_unexpected")
	}
	if extracted := extractMessage(raw); extracted != "" {
		text = extracted
	}
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) > 200 {
		text = string(runes[:200]) + "…"
	}
	return text
}

// extractMessage 尝试从常见错误响应结构中取出 message 字段。
func extractMessage(raw []byte) string {
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return ""
	}
	for _, key := range []string{"message", "msg", "error", "error_description"} {
		switch t := parsed[key].(type) {
		case string:
			return t
		case map[string]any:
			if msg, ok := t["message"].(string); ok {
				return msg
			}
		}
	}
	return ""
}
