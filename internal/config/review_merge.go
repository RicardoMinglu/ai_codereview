package config

import (
	"encoding/json"
	"fmt"
)

type reviewOverrides struct {
	MaxDiffLines   *int     `json:"max_diff_lines"`
	Language       *string  `json:"language"`
	IgnorePatterns []string `json:"ignore_patterns"`
}

// MergeReview 将 JSON 覆盖层与基础 review 配置合并；jsonRaw 为空则返回 base 的副本。
func MergeReview(base ReviewConfig, jsonRaw []byte) (ReviewConfig, error) {
	out := base
	if len(jsonRaw) == 0 {
		return out, nil
	}
	var o reviewOverrides
	if err := json.Unmarshal(jsonRaw, &o); err != nil {
		return out, fmt.Errorf("parse review_json: %w", err)
	}
	if o.MaxDiffLines != nil {
		out.MaxDiffLines = *o.MaxDiffLines
	}
	if o.Language != nil {
		out.Language = *o.Language
	}
	if o.IgnorePatterns != nil {
		out.IgnorePatterns = o.IgnorePatterns
	}
	return out, nil
}
