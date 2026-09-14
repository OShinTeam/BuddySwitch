package service

import "testing"

// safePluginID / safeName 是快照目录越权访问的唯一一道闸门，
// 它们直接决定 backup.Dir 拼出来的路径能不能跑出备份根目录。
func TestSafePluginID(t *testing.T) {
	cases := map[string]bool{
		"demo-agent": true,
		"agent_1":    true,
		"a.b":        true,
		"":           false,
		".":          false,
		"..":         false,
		"../etc":     false,
		"a/../../b":  false,
		"a/b":        false,
		`a\b`:        false,
		`..\windows`: false,
		"..hidden":   false, // 含 ".." 一并拒绝，宁可严格
		"agent id":   true,
		"中文插件":       true,
	}
	for in, want := range cases {
		if got := safePluginID(in); got != want {
			t.Errorf("safePluginID(%q) = %v, 期望 %v", in, got, want)
		}
	}
}

func TestSafeName(t *testing.T) {
	cases := map[string]bool{
		"models.json.2026-09-14_22-05-31.417.bak": true,
		"a.bak":       true,
		"":            false,
		"models.json": false, // 必须以 .bak 结尾
		"../a.bak":    false,
		"a/../b.bak":  false,
		"a/b.bak":     false,
		`a\b.bak`:     false,
		"..bak":       false,
	}
	for in, want := range cases {
		if got := safeName(in); got != want {
			t.Errorf("safeName(%q) = %v, 期望 %v", in, got, want)
		}
	}
}
