package storage

import (
	"strings"
)

// sanitizeFilename 移除可能导致 Content-Disposition 头注入的字符
func sanitizeFilename(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		// 移除控制字符、换行符、空字节、引号、分号、反斜杠
		if r < 0x20 || r == 0x7f || r == '"' || r == ';' || r == '\\' || r == '\x00' {
			b.WriteRune('_')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// isInlineContentType 判断浏览器是否应该内联显示的媒体类型
// 支持内联显示：图片、视频、音频、PDF。其他类型触发下载（Content-Disposition: attachment）
func isInlineContentType(ct string) bool {
	return strings.HasPrefix(ct, "image/") ||
		strings.HasPrefix(ct, "video/") ||
		strings.HasPrefix(ct, "audio/") ||
		ct == "application/pdf"
}
