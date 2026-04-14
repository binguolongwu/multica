// Package mention 提供将问题标识符引用（例如 MUL-117）扩展为 Markdown 中可点击提及链接的工具
package mention

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// IssueResolver 按工作空间和编号查找问题
// 由 db.Queries 实现
type IssueResolver interface {
	GetIssueByNumber(ctx context.Context, arg db.GetIssueByNumberParams) (db.Issue, error)
}

// PrefixResolver 查找工作空间以获取其问题前缀
type PrefixResolver interface {
	GetWorkspace(ctx context.Context, id pgtype.UUID) (db.Workspace, error)
}

// Resolver 组合提及扩展所需的两个接口
type Resolver interface {
	IssueResolver
	PrefixResolver
}

// ExpandIssueIdentifiers 扫描 Markdown 内容中的裸问题标识符模式（例如 MUL-117）
// 并将其替换为提及链接：[MUL-117](mention://issue/<uuid>)
//
// 跳过以下情况中的标识符：
//   - 已在 Markdown 链接中：[MUL-117](...)
//   - 在行内代码中：`MUL-117`
//   - 在围栏代码块中：```...```
func ExpandIssueIdentifiers(ctx context.Context, resolver Resolver, workspaceID pgtype.UUID, content string) string {
	// 获取工作空间前缀
	ws, err := resolver.GetWorkspace(ctx, workspaceID)
	if err != nil || ws.IssuePrefix == "" {
		return content
	}
	prefix := ws.IssuePrefix

	// 构建匹配工作空间前缀后跟连字符和数字的正则表达式
	// 使用单词边界避免在更长字符串中匹配
	// 转义前缀以防包含正则特殊字符
	pattern := regexp.MustCompile(`(?:^|(?:\W))` + `(` + regexp.QuoteMeta(prefix) + `-(\d+))` + `(?:\W|$)`)

	// 首先，识别要跳过的区域：围栏代码块和行内代码
	skipRegions := findSkipRegions(content)

	// 查找所有匹配项并从右到左处理（以保持偏移量）
	allMatches := pattern.FindAllStringSubmatchIndex(content, -1)
	if len(allMatches) == 0 {
		return content
	}

	// 构建替换集合（偏移量 → 替换字符串）
	type replacement struct {
		start, end int
		text       string
	}
	var replacements []replacement

	for _, match := range allMatches {
		// match[2:4] is the full identifier (e.g. "MUL-117")
		// match[4:6] is the number part (e.g. "117")
		identStart, identEnd := match[2], match[3]
		numStr := content[match[4]:match[5]]

		// 如果在代码区域内则跳过
		if inSkipRegion(identStart, skipRegions) {
			continue
		}

		// 如果已在 Markdown 链接内则跳过：检查前面是否有 [ 或后面是否有 ](...)
		if isInsideMarkdownLink(content, identStart, identEnd) {
			continue
		}

		num, err := strconv.Atoi(numStr)
		if err != nil || num <= 0 {
			continue
		}

		// 查找问题
		issue, err := resolver.GetIssueByNumber(ctx, db.GetIssueByNumberParams{
			WorkspaceID: workspaceID,
			Number:      int32(num),
		})
		if err != nil {
			continue // 问题不存在——保持原样
		}

		identifier := content[identStart:identEnd]
		issueID := uuidToString(issue.ID)
		mentionLink := fmt.Sprintf("[%s](mention://issue/%s)", identifier, issueID)

		replacements = append(replacements, replacement{
			start: identStart,
			end:   identEnd,
			text:  mentionLink,
		})
	}

	if len(replacements) == 0 {
		return content
	}

	// 从右到左应用替换以保持偏移量
	result := content
	for i := len(replacements) - 1; i >= 0; i-- {
		r := replacements[i]
		result = result[:r.start] + r.text + result[r.end:]
	}

	return result
}

// skipRegion 表示不应修改的文本区域
type skipRegion struct {
	start, end int
}

// findSkipRegions 识别内容中的围栏代码块（```）和行内代码（`）区域
func findSkipRegions(content string) []skipRegion {
	var regions []skipRegion

	// 围栏代码块：```...```
	fenceRe := regexp.MustCompile("(?m)^```[^`]*\n[\\s\\S]*?\n```")
	for _, loc := range fenceRe.FindAllStringIndex(content, -1) {
		regions = append(regions, skipRegion{loc[0], loc[1]})
	}

	// 行内代码：`...`（但不在围栏块内——已处理）
	inlineRe := regexp.MustCompile("`[^`\n]+`")
	for _, loc := range inlineRe.FindAllStringIndex(content, -1) {
		regions = append(regions, skipRegion{loc[0], loc[1]})
	}

	return regions
}

// inSkipRegion 检查位置是否落在任何跳过区域内
func inSkipRegion(pos int, regions []skipRegion) bool {
	for _, r := range regions {
		if pos >= r.start && pos < r.end {
			return true
		}
	}
	return false
}

// isInsideMarkdownLink 检查 [start:end] 处的文本是否已是 Markdown 链接的一部分
// 例如 [MUL-117](mention://...) 或 [text](url)
func isInsideMarkdownLink(content string, start, end int) bool {
	// 检查前面是否有 '['（链接文本的一部分）
	if start > 0 {
		before := strings.TrimRight(content[:start], " ")
		if len(before) > 0 && before[len(before)-1] == '[' {
			return true
		}
	}
	// 检查后面是否有 '](，表明它是 Markdown 链接的链接文本
	after := content[end:]
	if strings.HasPrefix(after, "](") {
		return true
	}
	// 检查是否在链接的 URL 部分：...](mention://issue/...)
	// 向后查找 ]( 模式
	idx := strings.LastIndex(content[:start], "](")
	if idx >= 0 {
		// Check that we haven't passed a closing ) yet.
		between := content[idx:start]
		if !strings.Contains(between, ")") {
			return true
		}
	}
	return false
}

func uuidToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
