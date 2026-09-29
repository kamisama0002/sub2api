package service

import (
	"sort"
	"strings"

	kiropkg "github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
)

// resolveKiroClaudeAlias 在精确键和通配符都未命中时，用上游 ID 的对外名对齐客户请求。
// 先比归一化后的 Claude 键；再比映射值里 Claude 上游 ID 去掉点号后的名字。
// 精确自定义行已经在调用前命中，这里不会盖掉它。
func resolveKiroClaudeAlias(mapping map[string]string, requested string) (string, bool) {
	alias, ok := kiropkg.ClaudeAliasKey(requested)
	if !ok || len(mapping) == 0 {
		return "", false
	}

	keyMatches := make([]string, 0, 1)
	for key := range mapping {
		keyAlias, keyOK := kiropkg.ClaudeAliasKey(key)
		if keyOK && keyAlias == alias {
			keyMatches = append(keyMatches, key)
		}
	}
	if len(keyMatches) > 0 {
		sort.Strings(keyMatches)
		chosen := keyMatches[0]
		for _, key := range keyMatches {
			if strings.EqualFold(strings.TrimSpace(key), alias) {
				chosen = key
				break
			}
		}
		return mapping[chosen], true
	}

	valueMatches := make([]string, 0, 1)
	seen := make(map[string]struct{})
	for _, value := range mapping {
		valueAlias, valueOK := kiropkg.ClaudeAliasKey(value)
		if !valueOK || valueAlias != alias {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		valueMatches = append(valueMatches, value)
	}
	if len(valueMatches) == 0 {
		return "", false
	}
	sort.Strings(valueMatches)
	for _, value := range valueMatches {
		if strings.Contains(value, ".") {
			return value, true
		}
	}
	return valueMatches[0], true
}
