package project

import (
	"strconv"
	"strings"
)

// NormalizePushBranchName 去掉误输入的外层引号（例如输入框里写了 "main" 会存成 ["\"main\""]）。
func NormalizePushBranchName(s string) string {
	s = strings.TrimSpace(s)
	for s != "" {
		if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
			if u, err := strconv.Unquote(s); err == nil {
				s = strings.TrimSpace(u)
				continue
			}
			break
		}
		if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
			s = strings.TrimSpace(s[1 : len(s)-1])
			continue
		}
		break
	}
	return strings.TrimSpace(s)
}

// ParsePushBranchesInput 将管理页「逗号分隔」输入解析为分支列表；空输入表示不限分支。
func ParsePushBranchesInput(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// 兼容中文逗号、顿号（避免看起来像写了 main 实际整段当成一个分支名）
	s = strings.ReplaceAll(s, "，", ",")
	s = strings.ReplaceAll(s, "、", ",")
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = NormalizePushBranchName(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// PushAllowed 判断 Git push 的 ref 是否应触发评审。
// PushBranches 为空或 nil 时表示不限制（所有分支的 push，以及非分支 ref 均允许）。
// PushBranches 非空时：仅允许 refs/heads/<name> 且 name 精确匹配列表中任一分支名。
func PushAllowed(rec *Record, gitRef string) bool {
	if rec == nil || len(rec.PushBranches) == 0 {
		return true
	}
	const pfx = "refs/heads/"
	if !strings.HasPrefix(gitRef, pfx) {
		return false
	}
	branch := strings.TrimSpace(strings.TrimPrefix(gitRef, pfx))
	for _, allow := range rec.PushBranches {
		allow = NormalizePushBranchName(allow)
		if allow == "" {
			continue
		}
		// GitHub ref 多为 lowercase；本地配置常写成 Main，避免误判
		if strings.EqualFold(allow, branch) {
			return true
		}
	}
	return false
}
