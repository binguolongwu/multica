package util

import "regexp"

// Mention 表示从 Markdown 内容中解析的 @提及
type Mention struct {
	Type string // 类型："member"（成员）、"agent"（代理）、"issue"（问题）或 "all"（所有人）
	ID   string // 用户 ID、代理 ID、问题 ID 或 "all"
}

// MentionRe 匹配 Markdown 中的 [@Label](mention://type/id) 或 [Label](mention://issue/id)
// @ 前缀是可选的，以支持使用 [MUL-123](mention://issue/...) 的问题提及
var MentionRe = regexp.MustCompile(`\[@?[^\]]*\]\(mention://(member|agent|issue|all)/([0-9a-fA-F-]+|all)\)`)

// IsMentionAll 如果提及是 @all 则返回 true
func (m Mention) IsMentionAll() bool {
	return m.Type == "all"
}

// ParseMentions 从 Markdown 内容中提取去重的提及
func ParseMentions(content string) []Mention {
	matches := MentionRe.FindAllStringSubmatch(content, -1)
	seen := make(map[string]bool)
	var result []Mention
	for _, m := range matches {
		key := m[1] + ":" + m[2]
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, Mention{Type: m[1], ID: m[2]})
	}
	return result
}

// HasMentionAll 如果切片中的任何提及是 @all 则返回 true
func HasMentionAll(mentions []Mention) bool {
	for _, m := range mentions {
		if m.IsMentionAll() {
			return true
		}
	}
	return false
}
