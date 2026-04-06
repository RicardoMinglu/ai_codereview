package reviewer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/RicardoMinglu/ai_codereview/internal/ai"
	"github.com/RicardoMinglu/ai_codereview/internal/config"
	ghclient "github.com/RicardoMinglu/ai_codereview/internal/github"
	"github.com/RicardoMinglu/ai_codereview/internal/report"
)

type Reviewer struct {
	provider ai.Provider
	cfg      *config.ReviewConfig
}

type ReviewRequest struct {
	RepoFullName string
	EventType    string
	Ref          string
	CommitSHA    string
	Author       string
	CommitMsg    string
	HTMLURL      string
	Diff         *ghclient.DiffResult
}

func New(provider ai.Provider, cfg *config.ReviewConfig) *Reviewer {
	return &Reviewer{provider: provider, cfg: cfg}
}

// WithConfig 返回共享同一 AI Provider、但使用另一套评审配置的 Reviewer（用于按仓库覆盖 review 配置）。
func (r *Reviewer) WithConfig(cfg *config.ReviewConfig) *Reviewer {
	if cfg == nil {
		return r
	}
	return &Reviewer{provider: r.provider, cfg: cfg}
}

func (r *Reviewer) Review(ctx context.Context, req *ReviewRequest) (*report.ReviewReport, error) {
	// Filter ignored files
	filteredDiff := r.filterFiles(req.Diff)

	// Check diff size
	totalLines := 0
	for _, f := range filteredDiff.Files {
		totalLines += f.AddLines + f.DelLines
	}
	if totalLines == 0 {
		return r.emptyReport(req), nil
	}
	if totalLines > r.cfg.MaxDiffLines {
		log.Printf("diff too large (%d lines), truncating to %d", totalLines, r.cfg.MaxDiffLines)
		filteredDiff = r.truncateDiff(filteredDiff)
	}

	// Build prompt
	prompt := r.buildPrompt(req, filteredDiff)

	// Call AI
	start := time.Now()
	response, err := r.provider.Review(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI review: %w", err)
	}
	duration := time.Since(start).Seconds()

	// Parse result
	result, err := r.parseResult(response)
	if err != nil {
		log.Printf("parse AI result warning: %v, using raw response", err)
		result = &AIReviewResult{
			Score:   70,
			Summary: response,
		}
	}

	rpt := &report.ReviewReport{
		RepoFullName: req.RepoFullName,
		EventType:    req.EventType,
		Ref:          req.Ref,
		CommitSHA:    req.CommitSHA,
		Author:       req.Author,
		CommitMsg:    req.CommitMsg,
		HTMLURL:      req.HTMLURL,
		Score:        result.Score,
		Summary:      result.Summary,
		Issues:       result.Issues,
		FilesNum:     len(filteredDiff.Files),
		LinesNum:     totalLines,
		AIModel:      r.provider.Name(),
		Duration:     duration,
	}
	return rpt, nil
}

func (r *Reviewer) filterFiles(diff *ghclient.DiffResult) *ghclient.DiffResult {
	filtered := &ghclient.DiffResult{}
	for _, f := range diff.Files {
		if r.shouldIgnore(f.Filename) {
			continue
		}
		filtered.Files = append(filtered.Files, f)
		filtered.TotalAdd += f.AddLines
		filtered.TotalDel += f.DelLines
	}
	return filtered
}

func (r *Reviewer) shouldIgnore(filename string) bool {
	for _, pattern := range r.cfg.IgnorePatterns {
		matched, err := filepath.Match(pattern, filename)
		if err == nil && matched {
			return true
		}
		// Also try matching just the base name
		matched, err = filepath.Match(pattern, filepath.Base(filename))
		if err == nil && matched {
			return true
		}
	}
	return false
}

func (r *Reviewer) truncateDiff(diff *ghclient.DiffResult) *ghclient.DiffResult {
	truncated := &ghclient.DiffResult{}
	totalLines := 0
	for _, f := range diff.Files {
		lines := f.AddLines + f.DelLines
		if totalLines+lines > r.cfg.MaxDiffLines {
			break
		}
		truncated.Files = append(truncated.Files, f)
		truncated.TotalAdd += f.AddLines
		truncated.TotalDel += f.DelLines
		totalLines += lines
	}
	return truncated
}

func (r *Reviewer) buildPrompt(req *ReviewRequest, diff *ghclient.DiffResult) string {
	lang := "中文"
	if r.cfg.Language == "en" {
		lang = "English"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`You are a senior code reviewer. Please review the following code changes and respond in %s.

Repository: %s
Branch/PR: %s
Author: %s
Commit message: %s

Please analyze the code changes and provide your review in the following JSON format (respond with ONLY the JSON, no other text):

{
  "score": <0-100 integer, overall code quality score>,
  "summary": "<overall review summary>",
  "issues": [
    {
      "file": "<filename>",
      "line": <line number or 0 if not applicable>,
      "severity": "<critical|warning|info|suggestion>",
      "category": "<bug|security|performance|style|best_practice>",
      "message": "<description of the issue>",
      "suggest": "<suggested fix or improvement>"
    }
  ]
}

Review dimensions:
1. **Bugs**: Logic errors, edge cases, null/nil handling, race conditions
2. **Security**: SQL injection, XSS, hardcoded secrets, insecure operations
3. **Performance**: N+1 queries, unnecessary allocations, blocking operations
4. **Code quality**: Readability, naming, duplication, complexity
5. **Best practices**: Error handling, logging, testing, documentation

Code changes:
`, lang, req.RepoFullName, req.Ref, req.Author, req.CommitMsg))

	for _, f := range diff.Files {
		sb.WriteString(fmt.Sprintf("\n--- File: %s (%s, +%d -%d) ---\n", f.Filename, f.Status, f.AddLines, f.DelLines))
		if f.Patch != "" {
			sb.WriteString(f.Patch)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

type AIReviewResult struct {
	Score   int            `json:"score"`
	Summary string         `json:"summary"`
	Issues  []report.Issue `json:"issues"`
}

func (r *Reviewer) parseResult(response string) (*AIReviewResult, error) {
	// Try to extract JSON from the response
	response = strings.TrimSpace(response)

	// Handle markdown code blocks
	if strings.HasPrefix(response, "```") {
		lines := strings.Split(response, "\n")
		var jsonLines []string
		inBlock := false
		for _, line := range lines {
			if strings.HasPrefix(line, "```") {
				inBlock = !inBlock
				continue
			}
			if inBlock {
				jsonLines = append(jsonLines, line)
			}
		}
		response = strings.Join(jsonLines, "\n")
	}

	var result AIReviewResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	// Clamp score
	if result.Score < 0 {
		result.Score = 0
	}
	if result.Score > 100 {
		result.Score = 100
	}

	return &result, nil
}

func (r *Reviewer) emptyReport(req *ReviewRequest) *report.ReviewReport {
	return &report.ReviewReport{
		RepoFullName: req.RepoFullName,
		EventType:    req.EventType,
		Ref:          req.Ref,
		CommitSHA:    req.CommitSHA,
		Author:       req.Author,
		CommitMsg:    req.CommitMsg,
		HTMLURL:      req.HTMLURL,
		Score:        100,
		Summary:      "No reviewable changes found (all files matched ignore patterns or diff is empty).",
		AIModel:      r.provider.Name(),
	}
}
