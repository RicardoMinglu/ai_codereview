package reviewer

import "testing"

func TestBuildReviewSummary_Success(t *testing.T) {
	input := ReviewSummaryInput{
		RepoFullName: "RicardoMinglu/ai_codereview",
		Branch:       "main",
		Author:       "dev",
		Score:        92,
	}

	out, err := BuildReviewSummary(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty summary")
	}
}

func TestBuildReviewSummary_InvalidInput(t *testing.T) {
	input := ReviewSummaryInput{
		RepoFullName: "invalid repo",
		Branch:       "main",
		Author:       "dev",
		Score:        80,
	}

	_, err := BuildReviewSummary(input)
	if err == nil {
		t.Fatal("expected validation error")
	}
}
