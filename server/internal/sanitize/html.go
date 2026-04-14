package sanitize

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

// httpURL 仅匹配 http:// 和 https:// URL——阻止 javascript:、data: 等
var httpURL = regexp.MustCompile(`^https?://`)

// policy 是共享的 bluemonday 策略，允许安全的 Markdown HTML
// 同时剥离危险元素（script、iframe、object、embed、style、on* 事件）
var policy *bluemonday.Policy

func init() {
	policy = bluemonday.UGCPolicy()
	policy.AllowElements("div", "span")
	// 允许文件卡片数据属性，但将 data-href 限制为仅 http(s)
	// 以防止 javascript: 和其他危险的 URL 方案
	policy.AllowAttrs("data-type", "data-filename").OnElements("div")
	policy.AllowAttrs("data-href").Matching(httpURL).OnElements("div")
	policy.AllowAttrs("class").OnElements("code", "div", "span", "pre")
}

// fencedCodeBlock 匹配 ``` 或 ~~~ 围栏代码块（带可选语言标签）
var fencedCodeBlock = regexp.MustCompile("(?m)^(```|~~~)[^\n]*\n[\\s\\S]*?\n(```|~~~)[ \t]*$")

// inlineCode 匹配反引号分隔的行内代码片段
// 按最长分隔符优先排序，使三重反引号先于双/单重匹配
var inlineCode = regexp.MustCompile("```[^`]+```|``[^`]+``|`[^`]+`")

// HTML 清理用户提供的 HTML/Markdown 内容，剥离危险标签
//（script、iframe、object、embed 等）和事件处理程序属性
//
// 代码块和行内代码片段被逐字保留，以便 bluemonday
// 不会对其内容进行 HTML 转义（例如 && → &amp;&amp;）
func HTML(input string) string {
	// 1. 提取围栏代码块，替换为唯一占位符
	var blocks []string
	placeholder := func(i int) string { return fmt.Sprintf("\x00CODEBLOCK_%d\x00", i) }
	result := fencedCodeBlock.ReplaceAllStringFunc(input, func(m string) string {
		idx := len(blocks)
		blocks = append(blocks, m)
		return placeholder(idx)
	})

	// 2. 提取行内代码片段
	var inlines []string
	inlinePH := func(i int) string { return fmt.Sprintf("\x00INLINE_%d\x00", i) }
	result = inlineCode.ReplaceAllStringFunc(result, func(m string) string {
		idx := len(inlines)
		inlines = append(inlines, m)
		return inlinePH(idx)
	})

	// 3. 清理非代码部分
	result = policy.Sanitize(result)

	// 4. 恢复行内代码片段，然后恢复围栏代码块
	for i, code := range inlines {
		result = strings.Replace(result, inlinePH(i), code, 1)
	}
	for i, block := range blocks {
		result = strings.Replace(result, placeholder(i), block, 1)
	}

	return result
}
