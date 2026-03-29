package reviewer

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	maxRepoNameLength = 100
	maxBranchLength   = 100
)

var (
	repoPattern   = regexp.MustCompile(`^[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+$`)
	branchPattern = regexp.MustCompile(`^[a-zA-Z0-9_./-]+$`)
)

type ReviewSummaryInput struct {
	RepoFullName string
	Branch       string
	Author       string
	Score        int
}

func BuildReviewSummary(input ReviewSummaryInput) (string, error) {
	if err := validateSummaryInput(input); err != nil {
		return "", err
	}

	var b strings.Builder
	b.Grow(256)

	fmt.Fprintf(&b, "代码评审结果\n")
	fmt.Fprintf(&b, "仓库: %s\n", input.RepoFullName)
	fmt.Fprintf(&b, "分支: %s\n", input.Branch)
	fmt.Fprintf(&b, "作者: %s\n", input.Author)
	fmt.Fprintf(&b, "评分: %d/100\n", input.Score)
	fmt.Fprintf(&b, "结论: %s", scoreConclusion(input.Score))

	return b.String(), nil
}

func validateSummaryInput(input ReviewSummaryInput) error {
	if input.RepoFullName == "" || len(input.RepoFullName) > maxRepoNameLength || !repoPattern.MatchString(input.RepoFullName) {
		return errors.New("invalid repo full name")
	}
	if input.Branch == "" || len(input.Branch) > maxBranchLength || !branchPattern.MatchString(input.Branch) {
		return errors.New("invalid branch")
	}
	if strings.TrimSpace(input.Author) == "" {
		return errors.New("author is required")
	}
	if input.Score < 0 || input.Score > 100 {
		return errors.New("score must be in range [0,100]")
	}
	return nil
}

func scoreConclusion(score int) string {
	switch {
	case score >= 85:
		return "可直接合并"
	case score >= 70:
		return "建议修复后合并"
	default:
		return "建议先整改再提交"
	}
}
