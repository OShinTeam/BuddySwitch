package plugin

import (
	"os"
	"strings"
)

// expandWindowsVars 把 %NAME% 形式的变量替换为环境变量的值，
// 以便同一份插件定义在 Windows 上也能用 APPDATA / USERPROFILE 之类的变量。
func expandWindowsVars(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] != '%' {
			b.WriteByte(s[i])
			i++
			continue
		}
		end := strings.IndexByte(s[i+1:], '%')
		if end < 0 {
			b.WriteString(s[i:])
			break
		}
		name := s[i+1 : i+1+end]
		if val, ok := os.LookupEnv(name); ok && name != "" {
			b.WriteString(val)
		} else {
			// 未定义的变量原样保留，避免把路径破坏得面目全非。
			b.WriteString(s[i : i+1+end+1])
		}
		i += end + 2
	}
	return b.String()
}

// stripJSONC 去掉 JSONC 中的 // 与 /* */ 注释，字符串字面量内的内容保持原样。
func stripJSONC(data []byte) []byte {
	out := make([]byte, 0, len(data))
	inString := false
	inLine := false
	inBlock := false

	for i := 0; i < len(data); i++ {
		c := data[i]

		switch {
		case inLine:
			if c == '\n' {
				inLine = false
				out = append(out, c)
			}
			continue
		case inBlock:
			if c == '*' && i+1 < len(data) && data[i+1] == '/' {
				inBlock = false
				i++
			}
			continue
		}

		if inString {
			out = append(out, c)
			if c == '\\' && i+1 < len(data) {
				out = append(out, data[i+1])
				i++
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}

		switch {
		case c == '"':
			inString = true
			out = append(out, c)
		case c == '/' && i+1 < len(data) && data[i+1] == '/':
			inLine = true
		case c == '/' && i+1 < len(data) && data[i+1] == '*':
			inBlock = true
			i++
		default:
			out = append(out, c)
		}
	}
	return out
}
