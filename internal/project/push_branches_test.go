package project

import "testing"

func TestPushAllowed(t *testing.T) {
	rec := &Record{PushBranches: []string{"main", "release/1"}}
	if !PushAllowed(rec, "refs/heads/main") {
		t.Fatal("main should allow")
	}
	recFold := &Record{PushBranches: []string{"Main"}}
	if !PushAllowed(recFold, "refs/heads/main") {
		t.Fatal("Main in allowlist should match main ref (case-insensitive)")
	}
	// 库中曾误存为 JSON ["\"main\""] 时，元素为带引号的字符串
	recQuoted := &Record{PushBranches: []string{`"main"`}}
	if !PushAllowed(recQuoted, "refs/heads/main") {
		t.Fatal("quoted branch token in allowlist should match")
	}
	if !PushAllowed(rec, "refs/heads/release/1") {
		t.Fatal("release/1 should allow")
	}
	if PushAllowed(rec, "refs/heads/dev") {
		t.Fatal("dev should reject")
	}
	if PushAllowed(rec, "refs/tags/v1.0.0") {
		t.Fatal("tag should reject when allowlist set")
	}
	if !PushAllowed(&Record{}, "refs/heads/anything") {
		t.Fatal("empty allowlist should allow")
	}
	if !PushAllowed(nil, "refs/heads/x") {
		t.Fatal("nil rec should allow")
	}
}

func TestParsePushBranchesInput(t *testing.T) {
	if ParsePushBranchesInput("") != nil {
		t.Fatal("empty -> nil")
	}
	got := ParsePushBranchesInput(" main , develop ")
	if len(got) != 2 || got[0] != "main" || got[1] != "develop" {
		t.Fatalf("got %v", got)
	}
	got2 := ParsePushBranchesInput("main，develop")
	if len(got2) != 2 || got2[0] != "main" || got2[1] != "develop" {
		t.Fatalf("fullwidth comma: got %v", got2)
	}
}
