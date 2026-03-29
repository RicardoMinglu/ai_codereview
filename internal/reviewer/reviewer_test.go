package reviewer

import (
	"context"
	"testing"

	"github.com/RicardoMinglu/ai_codereview/internal/ai"
	"github.com/RicardoMinglu/ai_codereview/internal/config"
	ghclient "github.com/RicardoMinglu/ai_codereview/internal/github"
)

type mockProvider struct {
	response string
	err      error
}

func (m *mockProvider) Review(ctx context.Context, prompt string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func (m *mockProvider) Name() string {
	return "mock/test"
}

func TestReview_EmptyDiff(t *testing.T) {
	cfg := &config.ReviewConfig{
		MaxDiffLines:   5000,
		Language:       "zh",
		IgnorePatterns: []string{"*.lock"},
	}
	provider := &mockProvider{response: `{"score":85,"summary":"ok","issues":[]}`}
	rev := New(provider, cfg)

	req := &ReviewRequest{
		RepoFullName: "owner/repo",
		EventType:    "push",
		Ref:          "main",
		CommitSHA:    "abc123",
		Author:       "test",
		CommitMsg:    "fix",
		HTMLURL:      "https://github.com/owner/repo/commit/abc123",
		Diff:         &ghclient.DiffResult{Files: []ghclient.FileDiff{}},
	}

	rpt, err := rev.Review(context.Background(), req)
	if err != nil {
		t.Fatalf("Review failed: %v", err)
	}
	if rpt.Score != 100 {
		t.Errorf("expected score 100 for empty diff, got %d", rpt.Score)
	}
	if rpt.Summary != "No reviewable changes found (all files matched ignore patterns or diff is empty)." {
		t.Errorf("unexpected summary: %s", rpt.Summary)
	}
}

func TestReview_AllFiltered(t *testing.T) {
	cfg := &config.ReviewConfig{
		MaxDiffLines:   5000,
		Language:       "zh",
		IgnorePatterns: []string{"*.lock", "*.sum", "vendor/*"},
	}
	provider := &mockProvider{response: `{"score":85,"summary":"ok","issues":[]}`}
	rev := New(provider, cfg)

	req := &ReviewRequest{
		RepoFullName: "owner/repo",
		EventType:    "push",
		Ref:          "main",
		CommitSHA:    "abc123",
		Author:       "test",
		CommitMsg:    "fix",
		HTMLURL:      "https://github.com/owner/repo/commit/abc123",
		Diff: &ghclient.DiffResult{
			Files: []ghclient.FileDiff{
				{Filename: "go.sum", Patch: "+xxx", AddLines: 1, DelLines: 0},
				// 单层 vendor/* 才符合 filepath.Match（* 不跨目录）
				{Filename: "vendor/module.go", Patch: "+yyy", AddLines: 1, DelLines: 0},
			},
			TotalAdd: 2,
			TotalDel: 0,
		},
	}

	rpt, err := rev.Review(context.Background(), req)
	if err != nil {
		t.Fatalf("Review failed: %v", err)
	}
	if rpt.Score != 100 {
		t.Errorf("expected score 100 when all filtered, got %d", rpt.Score)
	}
}

func TestReview_WithValidDiff(t *testing.T) {
	cfg := &config.ReviewConfig{
		MaxDiffLines:   5000,
		Language:       "zh",
		IgnorePatterns: []string{"*.lock"},
	}
	provider := &mockProvider{
		response: `{"score":88,"summary":"Good changes","issues":[{"file":"main.go","line":10,"severity":"warning","category":"style","message":"use tabs","suggest":"fix it"}]}`,
	}
	rev := New(provider, cfg)

	req := &ReviewRequest{
		RepoFullName: "owner/repo",
		EventType:    "push",
		Ref:          "main",
		CommitSHA:    "abc123",
		Author:       "test",
		CommitMsg:    "fix",
		HTMLURL:      "https://github.com/owner/repo/commit/abc123",
		Diff: &ghclient.DiffResult{
			Files: []ghclient.FileDiff{
				{Filename: "main.go", Patch: "+fmt.Println()", AddLines: 1, DelLines: 0},
			},
			TotalAdd: 1,
			TotalDel: 0,
		},
	}

	rpt, err := rev.Review(context.Background(), req)
	if err != nil {
		t.Fatalf("Review failed: %v", err)
	}
	if rpt.Score != 88 {
		t.Errorf("expected score 88, got %d", rpt.Score)
	}
	if rpt.Summary != "Good changes" {
		t.Errorf("unexpected summary: %s", rpt.Summary)
	}
	if len(rpt.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(rpt.Issues))
	}
	if rpt.Issues[0].File != "main.go" || rpt.Issues[0].Line != 10 || rpt.Issues[0].Severity != "warning" {
		t.Errorf("unexpected issue: %+v", rpt.Issues[0])
	}
}

func TestReview_AIError(t *testing.T) {
	cfg := &config.ReviewConfig{MaxDiffLines: 5000, IgnorePatterns: []string{}}
	provider := &mockProvider{err: context.DeadlineExceeded}
	rev := New(provider, cfg)

	req := &ReviewRequest{
		RepoFullName: "owner/repo",
		Diff: &ghclient.DiffResult{
			Files: []ghclient.FileDiff{
				{Filename: "a.go", Patch: "+x", AddLines: 1, DelLines: 0},
			},
			TotalAdd: 1,
			TotalDel: 0,
		},
	}

	_, err := rev.Review(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from AI provider")
	}
}

func TestReview_InvalidJSONFallback(t *testing.T) {
	cfg := &config.ReviewConfig{MaxDiffLines: 5000, IgnorePatterns: []string{}}
	provider := &mockProvider{response: "not valid json at all"}
	rev := New(provider, cfg)

	req := &ReviewRequest{
		RepoFullName: "owner/repo",
		Diff: &ghclient.DiffResult{
			Files: []ghclient.FileDiff{
				{Filename: "a.go", Patch: "+x", AddLines: 1, DelLines: 0},
			},
			TotalAdd: 1,
			TotalDel: 0,
		},
	}

	rpt, err := rev.Review(context.Background(), req)
	if err != nil {
		t.Fatalf("Review should not fail on parse error: %v", err)
	}
	if rpt.Score != 70 {
		t.Errorf("expected fallback score 70, got %d", rpt.Score)
	}
	if rpt.Summary != "not valid json at all" {
		t.Errorf("expected raw response as summary, got %s", rpt.Summary)
	}
}

func TestReview_JSONInMarkdownBlock(t *testing.T) {
	cfg := &config.ReviewConfig{MaxDiffLines: 5000, IgnorePatterns: []string{}}
	provider := &mockProvider{
		response: "```json\n{\"score\":75,\"summary\":\"wrapped\",\"issues\":[]}\n```",
	}
	rev := New(provider, cfg)

	req := &ReviewRequest{
		RepoFullName: "owner/repo",
		Diff: &ghclient.DiffResult{
			Files: []ghclient.FileDiff{
				{Filename: "a.go", Patch: "+x", AddLines: 1, DelLines: 0},
			},
			TotalAdd: 1,
			TotalDel: 0,
		},
	}

	rpt, err := rev.Review(context.Background(), req)
	if err != nil {
		t.Fatalf("Review failed: %v", err)
	}
	if rpt.Score != 75 {
		t.Errorf("expected score 75, got %d", rpt.Score)
	}
	if rpt.Summary != "wrapped" {
		t.Errorf("expected summary 'wrapped', got %s", rpt.Summary)
	}
}

// Ensure mockProvider implements ai.Provider
var _ ai.Provider = (*mockProvider)(nil)
