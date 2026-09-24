package model

import (
	"regexp"
	"strings"
)

// tokenCountTagPattern 匹配 "262.1K"/"1.1M"/"200K"/"128K"/"1M" 这类由上游元数据按
// context 长度自动生成的数字标签。它们不属于模型能力标签，展示时应剔除。
var tokenCountTagPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?(?:[KkMm])?$`)

// FilterTokenCountTags 移除逗号分隔 tags 中的纯数字或带 K/M 单位的数字标签。
func FilterTokenCountTags(tags string) string {
	if strings.TrimSpace(tags) == "" {
		return ""
	}
	parts := strings.Split(tags, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" || tokenCountTagPattern.MatchString(trimmed) {
			continue
		}
		out = append(out, trimmed)
	}
	return strings.Join(out, ",")
}
