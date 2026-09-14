package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPlaceholdersAndRender(t *testing.T) {
	m := Model{ID: "demo-model", DisplayName: "演示", BaseURL: "https://api.example.invalid/v1/", APIKey: "sk-demo"}
	vars := m.Placeholders("demo-agent")

	if vars["base_url"] != "https://api.example.invalid/v1" {
		t.Fatalf("base_url 末尾斜杠应当被去掉: %q", vars["base_url"])
	}
	if vars["plugin_id"] != "demo-agent" {
		t.Fatalf("plugin_id 占位符缺失: %q", vars["plugin_id"])
	}

	cases := map[string]string{
		"{{id}}":       "demo-model",
		"{{base_url}}": "https://api.example.invalid/v1",
		"{{unknown}}":  "{{unknown}}",
		"没有占位符":        "没有占位符",
		"a{{id}}b":     "ademo-modelb",
		"{{ id }}":     "demo-model",
		"{{id":         "{{id",
		"{{}}":         "{{}}",
		"{{id}}{{id}}": "demo-modeldemo-model",
	}
	for tpl, want := range cases {
		if got := Render(tpl, vars); got != want {
			t.Errorf("Render(%q) = %q, 期望 %q", tpl, got, want)
		}
	}
}

func TestRenderAnyWalksNested(t *testing.T) {
	vars := map[string]string{"id": "m1"}
	in := map[string]any{
		"model":    "{{id}}",
		"messages": []any{map[string]any{"role": "user", "content": "{{id}}"}},
		"stream":   false,
		"max":      1,
	}
	out := RenderAny(in, vars).(map[string]any)
	if out["model"] != "m1" {
		t.Fatalf("顶层字符串没渲染: %v", out["model"])
	}
	if out["stream"] != false || out["max"] != 1 {
		t.Fatalf("非字符串值不该被改动: %v", out)
	}
	msg := out["messages"].([]any)[0].(map[string]any)
	if msg["content"] != "m1" {
		t.Fatalf("嵌套字符串没渲染: %v", msg)
	}
}

func TestExpectedStatusAndSummarize(t *testing.T) {
	if !expectedStatus(ProbeSpec{}, 200) || !expectedStatus(ProbeSpec{}, 299) {
		t.Fatal("默认 2xx 视为成功")
	}
	if expectedStatus(ProbeSpec{}, 301) || expectedStatus(ProbeSpec{}, 199) {
		t.Fatal("2xx 以外不该视为成功")
	}
	if !expectedStatus(ProbeSpec{ExpectStatus: []int{200, 204}}, 204) {
		t.Fatal("自定义区间没生效")
	}
	if expectedStatus(ProbeSpec{ExpectStatus: []int{200, 204}}, 205) {
		t.Fatal("自定义区间上界没生效")
	}

	if got := summarize(nil); got == "" {
		t.Fatal("空响应体应当给出一句兜底说明")
	}
	if got := summarize([]byte(`{"error":{"message":"密钥不对"}}`)); got != "密钥不对" {
		t.Fatalf("应当抽取嵌套 message，得到 %q", got)
	}
	if got := summarize([]byte(`{"message":"  多余   空格  "}`)); got != "多余 空格" {
		t.Fatalf("空白应当被压平，得到 %q", got)
	}
	long := strings.Repeat("字", 300)
	if got := summarize([]byte(long)); len([]rune(got)) > 201 {
		t.Fatalf("过长的响应体应当被截断，得到 %d 个字符", len([]rune(got)))
	}
}

func TestExtractMessage(t *testing.T) {
	cases := map[string]string{
		`{"message":"a"}`:           "a",
		`{"msg":"b"}`:               "b",
		`{"error":"c"}`:             "c",
		`{"error_description":"d"}`: "d",
		`{"error":{"message":"e"}}`: "e",
		`{"error":{"code":"f"}}`:    "",
		`不是 JSON`:                   "",
		`{"message":123}`:           "",
	}
	for raw, want := range cases {
		if got := extractMessage([]byte(raw)); got != want {
			t.Errorf("extractMessage(%s) = %q, 期望 %q", raw, got, want)
		}
	}
}

func TestRunProbeSpec(t *testing.T) {
	m := Model{ID: "demo-model", BaseURL: "https://api.example.invalid/v1/chat/completions", APIKey: "sk-demo"}

	t.Run("缺接入点", func(t *testing.T) {
		res := RunProbeSpec(context.Background(), DefaultProbe(), "demo", Model{ID: "x"})
		if res.Status != ProbeError {
			t.Fatalf("应当归为本地错误，得到 %s", res.Status)
		}
	})

	t.Run("缺探测规格", func(t *testing.T) {
		res := RunProbeSpec(context.Background(), ProbeSpec{}, "demo", m)
		if res.Status != ProbeError {
			t.Fatalf("没有 endpoint 时应当报本地错误，得到 %s", res.Status)
		}
	})

	t.Run("非法地址", func(t *testing.T) {
		bad := m
		bad.BaseURL = "not-a-url"
		res := RunProbeSpec(context.Background(), DefaultProbe(), "demo", bad)
		if res.Status != ProbeError {
			t.Fatalf("非法地址应当报本地错误，得到 %s (%s)", res.Status, res.Message)
		}
	})

	t.Run("鉴权失败单独识别", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()
		res := RunProbeSpec(context.Background(), DefaultProbe(), "demo", withBaseURL(m, srv.URL))
		if res.Status != ProbeAuth {
			t.Fatalf("401 应当归为密钥无效，得到 %s", res.Status)
		}
		if res.LatencyMs < 0 {
			t.Fatal("延迟不该是负数")
		}
	})

	t.Run("404 提示端点不存在", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()
		res := RunProbeSpec(context.Background(), DefaultProbe(), "demo", withBaseURL(m, srv.URL))
		if res.Status != ProbeFail {
			t.Fatalf("404 应当归为不可用，得到 %s", res.Status)
		}
	})

	t.Run("成功时地址与请求体符合规格", func(t *testing.T) {
		var body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
				t.Errorf("应当补上 ensure_path，实际 %s", r.URL.Path)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer sk-demo" {
				t.Errorf("Authorization 头不对: %q", got)
			}
			buf := make([]byte, r.ContentLength)
			r.Body.Read(buf)
			body = string(buf)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		res := RunProbeSpec(context.Background(), DefaultProbe(), "demo", withBaseURL(m, srv.URL))
		if res.Status != ProbeOK {
			t.Fatalf("应当可用，得到 %s (%s)", res.Status, res.Message)
		}
		if !strings.Contains(body, `"model":"demo-model"`) {
			t.Fatalf("请求体里的 model 占位符没渲染: %s", body)
		}
		if res.CheckedAt == "" {
			t.Fatal("应当记录探测时间")
		}
	})

	t.Run("超时归为本地错误", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(300 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		spec := DefaultProbe()
		spec.TimeoutMs = 30
		res := RunProbeSpec(context.Background(), spec, "demo", withBaseURL(m, srv.URL))
		if res.Status != ProbeError {
			t.Fatalf("超时应当归为本地错误，得到 %s", res.Status)
		}
	})

	t.Run("连接不上归为本地错误", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		url := srv.URL
		srv.Close() // 立刻关掉，制造连接失败
		res := RunProbeSpec(context.Background(), DefaultProbe(), "demo", withBaseURL(m, url))
		if res.Status != ProbeError {
			t.Fatalf("连不上应当归为本地错误，得到 %s", res.Status)
		}
	})
}

// TestRunProbeSpecIsSafeUnderConcurrency 保证并发探测不会共享可变状态。
func TestRunProbeSpecIsSafeUnderConcurrency(t *testing.T) {
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	spec := DefaultProbe()
	base := withBaseURL(Model{ID: "demo", APIKey: "sk-demo"}, srv.URL)

	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 5; j++ {
				if res := RunProbeSpec(context.Background(), spec, "demo", base); res.Status != ProbeOK {
					t.Errorf("并发探测失败: %s %s", res.Status, res.Message)
					return
				}
			}
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}
	if atomic.LoadInt64(&hits) != 40 {
		t.Fatalf("期望 40 次请求，实际 %d", hits)
	}
}

func withBaseURL(m Model, url string) Model {
	m.BaseURL = url
	if m.APIKey == "" {
		m.APIKey = "sk-demo"
	}
	return m
}
