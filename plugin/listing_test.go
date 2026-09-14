package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestBuildEndpoint 覆盖「同一个 base_url 既要拼聊天接口、又要拼清单接口」。
func TestBuildEndpoint(t *testing.T) {
	cases := []struct {
		name        string
		endpoint    string
		stripSuffix string
		ensurePath  string
		vars        map[string]string
		want        string
	}{
		{
			name:        "base 结尾补 /models",
			endpoint:    "{{base_url}}",
			stripSuffix: "/chat/completions",
			ensurePath:  "/models",
			vars:        map[string]string{"base_url": "https://api.example.invalid/v1"},
			want:        "https://api.example.invalid/v1/models",
		},
		{
			name:        "完整接口先摘后缀再补",
			endpoint:    "{{base_url}}",
			stripSuffix: "/chat/completions",
			ensurePath:  "/models",
			vars:        map[string]string{"base_url": "https://api.example.invalid/v1/chat/completions"},
			want:        "https://api.example.invalid/v1/models",
		},
		{
			name:        "已经是对的目标地址就不重复补",
			endpoint:    "{{base_url}}",
			stripSuffix: "/chat/completions",
			ensurePath:  "/models",
			vars:        map[string]string{"base_url": "https://api.example.invalid/v1/models"},
			want:        "https://api.example.invalid/v1/models",
		},
		{
			name:        "尾部斜杠被规整",
			endpoint:    "{{base_url}}",
			stripSuffix: "/chat/completions",
			ensurePath:  "/models",
			vars:        map[string]string{"base_url": "https://api.example.invalid/v1/"},
			want:        "https://api.example.invalid/v1/models",
		},
		{
			name:        "不声明任何后缀时保持原样",
			endpoint:    "{{base_url}}",
			stripSuffix: "",
			ensurePath:  "",
			vars:        map[string]string{"base_url": "https://api.example.invalid/v1/"},
			want:        "https://api.example.invalid/v1",
		},
		{
			name:        "未识别的占位符原样保留",
			endpoint:    "{{base_url}}/{{unknown}}",
			stripSuffix: "",
			ensurePath:  "",
			vars:        map[string]string{"base_url": "https://api.example.invalid"},
			want:        "https://api.example.invalid/{{unknown}}",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := buildEndpoint(c.endpoint, c.stripSuffix, c.ensurePath, c.vars); got != c.want {
				t.Fatalf("得到 %q，期望 %q", got, c.want)
			}
		})
	}
}

func TestParseRemoteModels(t *testing.T) {
	t.Run("OpenAI 风格", func(t *testing.T) {
		raw := []byte(`{"object":"list","data":[{"id":"b-model","owned_by":"demo"},{"id":"a-model","owned_by":"demo"}]}`)
		models, err := parseRemoteModels(raw, DefaultListing())
		if err != nil {
			t.Fatal(err)
		}
		if len(models) != 2 {
			t.Fatalf("应当解析出 2 条，得到 %d", len(models))
		}
		if models[0].ID != "a-model" {
			t.Fatalf("结果应当按 id 排序，得到 %+v", models)
		}
		if models[0].Note != "demo" {
			t.Fatalf("note_field 没生效: %+v", models[0])
		}
	})

	t.Run("根节点即数组且元素是字符串", func(t *testing.T) {
		spec := DefaultListing()
		spec.ArrayPath = ""
		models, err := parseRemoteModels([]byte(`["m1","m2","m1"]`), spec)
		if err != nil {
			t.Fatal(err)
		}
		if len(models) != 2 {
			t.Fatalf("应当去重成 2 条，得到 %d", len(models))
		}
		if models[0].ID != "m1" || models[0].DisplayName != "" {
			t.Fatalf("字符串元素只该给出 id: %+v", models[0])
		}
	})

	t.Run("缺 id 的元素被跳过", func(t *testing.T) {
		raw := []byte(`{"data":[{"id":"ok"},{"name":"没有 id"},{"id":"   "}]}`)
		models, err := parseRemoteModels(raw, DefaultListing())
		if err != nil {
			t.Fatal(err)
		}
		if len(models) != 1 || models[0].ID != "ok" {
			t.Fatalf("应当只剩 1 条，得到 %+v", models)
		}
	})

	t.Run("array_path 指错时报错", func(t *testing.T) {
		spec := DefaultListing()
		spec.ArrayPath = "missing"
		if _, err := parseRemoteModels([]byte(`{"data":[]}`), spec); err == nil {
			t.Fatal("找不到数组时应当报错")
		}
	})

	t.Run("找到的节点不是数组时报错", func(t *testing.T) {
		if _, err := parseRemoteModels([]byte(`{"data":{"nope":1}}`), DefaultListing()); err == nil {
			t.Fatal("节点不是数组时应当报错")
		}
	})

	t.Run("自定义字段路径", func(t *testing.T) {
		spec := DefaultListing()
		spec.ArrayPath = "result.items"
		spec.IDField = "model_id"
		spec.NameField = "label"
		raw := []byte(`{"result":{"items":[{"model_id":"x1","label":"显示名"}]}}`)
		models, err := parseRemoteModels(raw, spec)
		if err != nil {
			t.Fatal(err)
		}
		if len(models) != 1 || models[0].ID != "x1" || models[0].DisplayName != "显示名" {
			t.Fatalf("自定义路径没生效: %+v", models)
		}
	})
}

func TestFetchListingModelsErrors(t *testing.T) {
	t.Run("空接入点", func(t *testing.T) {
		if _, err := FetchListingModels(context.Background(), DefaultListing(), "demo", "", "k"); err == nil {
			t.Fatal("接入点为空时应当报错")
		}
	})

	t.Run("非 http 地址", func(t *testing.T) {
		if _, err := FetchListingModels(context.Background(), DefaultListing(), "demo", "ftp://x", "k"); err == nil {
			t.Fatal("非 http 地址应当报错")
		}
	})

	t.Run("没有清单规格", func(t *testing.T) {
		spec := ListingSpec{}
		if _, err := FetchListingModels(context.Background(), spec, "demo", "https://api.example.invalid", "k"); err == nil {
			t.Fatal("没有 endpoint 时应当报错")
		}
	})

	t.Run("状态码分支", func(t *testing.T) {
		cases := []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError}
		for _, code := range cases {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("Authorization"); got != "Bearer demo-key" {
					t.Errorf("请求头没按规格渲染: %q", got)
				}
				w.WriteHeader(code)
				w.Write([]byte(`{"message":"服务端说不行"}`))
			}))
			if _, err := FetchListingModels(context.Background(), DefaultListing(), "demo", srv.URL, "demo-key"); err == nil {
				t.Fatalf("HTTP %d 应当报错", code)
			}
			srv.Close()
		}
	})

	t.Run("正常返回", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/models" {
				t.Errorf("路径不对: %s", r.URL.Path)
			}
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"id": "demo-model", "owned_by": "demo"}},
			})
		}))
		defer srv.Close()

		models, err := FetchListingModels(context.Background(), DefaultListing(), "demo", srv.URL, "demo-key")
		if err != nil {
			t.Fatal(err)
		}
		if len(models) != 1 || models[0].ID != "demo-model" {
			t.Fatalf("解析结果不对: %+v", models)
		}
	})
}
