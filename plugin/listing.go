package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"buddyswitch/global"
)

// buildEndpoint 把模板地址规范化。
//
// 顺序很重要：先去掉旧后缀再补新后缀，这样同一个 {{base_url}} 既能拼出
// 聊天接口，也能拼出模型清单接口——哪怕上游的 url 写的是完整路径
// （.../v1/chat/completions）还是只有 base（.../v1）。
func buildEndpoint(endpoint, stripSuffix, ensurePath string, vars map[string]string) string {
	url := strings.TrimRight(Render(endpoint, vars), "/")
	if s := strings.TrimSpace(stripSuffix); s != "" {
		s = "/" + strings.Trim(s, "/")
		if strings.HasSuffix(url, s) {
			url = strings.TrimSuffix(url, s)
		}
	}
	if s := strings.TrimSpace(ensurePath); s != "" {
		s = "/" + strings.Trim(s, "/")
		if !strings.HasSuffix(url, s) {
			url += s
		}
	}
	return url
}

// DefaultListing 是 OpenAI 兼容的模型清单接口。
//
// 上游缓存是程序级资产，不该依赖某个 Agent 插件才能拉取，所以这里给一份兜底：
// 插件没定义 listing（甚至一个插件都没有）时，「新增上游」照样能拉清单。
func DefaultListing() ListingSpec {
	return ListingSpec{
		Endpoint:    "{{base_url}}",
		StripSuffix: "/chat/completions",
		EnsurePath:  "/models",
		Method:      http.MethodGet,
		Headers:     map[string]string{"Authorization": "Bearer {{api_key}}"},
		TimeoutMs:   20000,
		ArrayPath:   "data",
		IDField:     "id",
		NameField:   "id",
		NoteField:   "owned_by",
	}
}

// ListingFor 返回插件自定义的 listing 规格，没定义就给 OpenAI 兼容的兜底。
func (def *Definition) ListingFor() ListingSpec {
	if strings.TrimSpace(def.Listing.Endpoint) != "" {
		return def.Listing
	}
	return DefaultListing()
}

// FetchRemoteModels 拿着接入点与密钥，按插件定义向上游索要它的模型清单。
func (def *Definition) FetchRemoteModels(ctx context.Context, rawURL, apiKey string) ([]RemoteModel, error) {
	return FetchListingModels(ctx, def.ListingFor(), def.ID, rawURL, apiKey)
}

// FetchListingModels 按给定规格向上游索要模型清单。
//
// 与读取本地配置文件是两件不同的事：这里问的是「上游提供哪些模型」，
// 因此一个新模型即使还没写进任何 agent 的配置，也能被发现。
func FetchListingModels(ctx context.Context, spec ListingSpec, pluginID, rawURL, apiKey string) ([]RemoteModel, error) {
	if strings.TrimSpace(spec.Endpoint) == "" {
		return nil, fmt.Errorf("%s", global.T("err_no_listing_spec"))
	}
	if strings.TrimSpace(rawURL) == "" {
		return nil, fmt.Errorf("%s", global.T("err_listing_url_empty"))
	}

	// 复用模型占位符：只用到 base_url 与 api_key，其余留空即可。
	vars := Model{BaseURL: rawURL, APIKey: apiKey, PluginID: pluginID}.Placeholders(pluginID)
	url := buildEndpoint(spec.Endpoint, spec.StripSuffix, spec.EnsurePath, vars)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, fmt.Errorf("%s", global.T("err_upstream_url_invalid", url))
	}

	method := strings.ToUpper(strings.TrimSpace(spec.Method))
	if method == "" {
		method = http.MethodGet
	}
	timeout := time.Duration(spec.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", global.T("err_build_request_failed"), err)
	}
	for k, v := range spec.Headers {
		req.Header.Set(k, Render(v, vars))
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "BuddySwitch/1.0")

	resp, err := Client.Do(req)
	if err != nil {
		if reqCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("%s", global.T("err_listing_timeout", timeout))
		}
		return nil, fmt.Errorf("%s: %w", global.T("err_upstream_request_failed"), err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))

	switch {
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf("%s", global.T("err_upstream_auth", resp.StatusCode))
	case resp.StatusCode == http.StatusNotFound:
		return nil, fmt.Errorf("%s", global.T("err_upstream_no_listing", url))
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return nil, fmt.Errorf("%s", global.T("err_upstream_status", resp.StatusCode, summarize(raw)))
	}

	models, err := parseRemoteModels(raw, spec)
	if err != nil {
		return nil, fmt.Errorf("%w（%s）", err, url)
	}
	return models, nil
}

// parseRemoteModels 按 listing 的字段映射把响应解析成模型清单。
func parseRemoteModels(raw []byte, spec ListingSpec) ([]RemoteModel, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var root any
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("%s", global.T("err_parse_upstream_failed"))
	}

	node := root
	if p := strings.TrimSpace(spec.ArrayPath); p != "" {
		value, ok := getByPath(root, p)
		if !ok {
			return nil, fmt.Errorf("%s", global.T("err_response_no_array", p))
		}
		node = value
	}
	arr, ok := node.([]any)
	if !ok {
		return nil, fmt.Errorf("%s", global.T("err_response_array_type"))
	}

	idField := orDefault(spec.IDField, "id")
	nameField := orDefault(spec.NameField, "id")
	noteField := strings.TrimSpace(spec.NoteField)

	out := make([]RemoteModel, 0, len(arr))
	seen := make(map[string]bool, len(arr))
	for _, item := range arr {
		var id, name, note string
		switch entry := item.(type) {
		case string:
			// 少数上游直接返回 ["gpt-4o", "gpt-4o-mini"]
			id = entry
		case map[string]any:
			if v, ok := getByPath(entry, idField); ok {
				id = toString(v)
			}
			if nameField != "" {
				if v, ok := getByPath(entry, nameField); ok {
					name = toString(v)
				}
			}
			if noteField != "" {
				if v, ok := getByPath(entry, noteField); ok {
					note = toString(v)
				}
			}
		}
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, RemoteModel{ID: id, DisplayName: strings.TrimSpace(name), Note: note})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func orDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
