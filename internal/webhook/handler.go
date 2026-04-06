package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/RicardoMinglu/ai_codereview/internal/config"
	ghclient "github.com/RicardoMinglu/ai_codereview/internal/github"
	"github.com/RicardoMinglu/ai_codereview/internal/notify"
	"github.com/RicardoMinglu/ai_codereview/internal/project"
	"github.com/RicardoMinglu/ai_codereview/internal/report"
	"github.com/RicardoMinglu/ai_codereview/internal/reviewer"
)

type Handler struct {
	cfg      *config.Config
	proj     project.Reader
	reviewer *reviewer.Reviewer
	store    report.Store
	notifier notify.Notifier
}

func NewHandler(cfg *config.Config, proj project.Reader, rev *reviewer.Reviewer, store report.Store, notifier notify.Notifier) *Handler {
	if proj == nil {
		proj = project.NoopReader{}
	}
	return &Handler{
		cfg:      cfg,
		proj:     proj,
		reviewer: rev,
		store:    store,
		notifier: notifier,
	}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body error", http.StatusBadRequest)
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	repoPeek := peekRepoFullName(eventType, body)
	verifyCtx := r.Context()
	projRow, err := h.proj.GetProjectRow(verifyCtx, repoPeek)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Printf("get project row for verify: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		projRow = nil
	}

	webhookSecret := ""
	if projRow != nil && projRow.WebhookSecret != "" {
		webhookSecret = projRow.WebhookSecret
	}

	if webhookSecret != "" {
		sig := r.Header.Get("X-Hub-Signature-256")
		if !verifySignature(body, sig, webhookSecret) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Return 200 immediately, process async
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"status":"accepted"}`)

	go h.processEvent(eventType, body)
}

func (h *Handler) processEvent(eventType string, body []byte) {
	switch eventType {
	case "push":
		h.handlePush(body)
	case "pull_request":
		h.handlePullRequest(body)
	default:
		log.Printf("ignoring event type: %s", eventType)
	}
}

func (h *Handler) handlePush(body []byte) {
	var event PushEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("parse push event error: %v", err)
		return
	}

	// Skip branch delete events
	if event.Deleted {
		return
	}

	// Skip if no commits
	if len(event.Commits) == 0 {
		return
	}

	// 仅评审分支 push，忽略 tag push
	if strings.HasPrefix(event.Ref, "refs/tags/") {
		return
	}

	repoFullName := event.Repository.FullName
	ctx := context.Background()
	rec, ghClient, rev, notifier, ok := h.resolveProjectStack(ctx, repoFullName)
	if !ok {
		return
	}
	if !project.PushAllowed(rec, event.Ref) {
		log.Printf("skip push: ref %q not in push_branches allowlist %q for %s", event.Ref, rec.PushBranches, repoFullName)
		return
	}

	owner, repo, err := ghclient.ParseRepoFullName(repoFullName)
	if err != nil {
		log.Printf("parse repo name error: %v", err)
		return
	}

	headCommit := event.HeadCommit
	log.Printf("processing push to %s: %s by %s", repoFullName, headCommit.ID[:8], headCommit.Author.Name)

	ref := event.Ref
	if strings.HasPrefix(ref, "refs/heads/") {
		ref = strings.TrimPrefix(ref, "refs/heads/")
	}

	rpt := &report.ReviewReport{
		RepoFullName: repoFullName,
		EventType:    "push",
		Ref:          ref,
		CommitSHA:    headCommit.ID,
		Author:       headCommit.Author.Name,
		CommitMsg:    headCommit.Message,
		HTMLURL:      headCommit.URL,
		Status:       "pending",
		Summary:      "",
		Issues:       []report.Issue{},
	}
	if err := h.store.Save(ctx, rpt); err != nil {
		log.Printf("save pending report error: %v", err)
		return
	}

	diff, err := ghClient.GetCommitDiff(ctx, owner, repo, headCommit.ID)
	if err != nil {
		log.Printf("get commit diff error: %v", err)
		rpt.Status, rpt.ErrorMsg = "failed", err.Error()
		_ = h.store.Update(ctx, rpt)
		return
	}

	req := &reviewer.ReviewRequest{
		RepoFullName: repoFullName,
		EventType:    "push",
		Ref:          ref,
		CommitSHA:    headCommit.ID,
		Author:       headCommit.Author.Name,
		CommitMsg:    headCommit.Message,
		HTMLURL:      headCommit.URL,
		Diff:         diff,
	}

	h.runReview(ctx, req, rpt, rev, notifier)
}

func (h *Handler) handlePullRequest(body []byte) {
	var event PullRequestEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("parse PR event error: %v", err)
		return
	}

	// Only process opened and synchronize actions
	if event.Action != "opened" && event.Action != "synchronize" {
		return
	}

	repoFullName := event.Repository.FullName
	ctx := context.Background()
	_, ghClient, rev, notifier, ok := h.resolveProjectStack(ctx, repoFullName)
	if !ok {
		return
	}

	owner, repo, err := ghclient.ParseRepoFullName(repoFullName)
	if err != nil {
		log.Printf("parse repo name error: %v", err)
		return
	}

	pr := event.PullRequest
	log.Printf("processing PR #%d on %s: %s by %s", pr.Number, repoFullName, pr.Title, pr.User.Login)

	rpt := &report.ReviewReport{
		RepoFullName: repoFullName,
		EventType:    "pull_request",
		Ref:          fmt.Sprintf("#%d", pr.Number),
		CommitSHA:    pr.Head.SHA,
		Author:       pr.User.Login,
		CommitMsg:    pr.Title,
		HTMLURL:      pr.HTMLURL,
		Status:       "pending",
		Summary:      "",
		Issues:       []report.Issue{},
	}
	if err := h.store.Save(ctx, rpt); err != nil {
		log.Printf("save pending report error: %v", err)
		return
	}

	diff, err := ghClient.GetPRDiff(ctx, owner, repo, pr.Number)
	if err != nil {
		log.Printf("get PR diff error: %v", err)
		rpt.Status, rpt.ErrorMsg = "failed", err.Error()
		_ = h.store.Update(ctx, rpt)
		return
	}

	req := &reviewer.ReviewRequest{
		RepoFullName: repoFullName,
		EventType:    "pull_request",
		Ref:          fmt.Sprintf("#%d", pr.Number),
		CommitSHA:    pr.Head.SHA,
		Author:       pr.User.Login,
		CommitMsg:    pr.Title,
		HTMLURL:      pr.HTMLURL,
		Diff:         diff,
	}

	h.runReview(ctx, req, rpt, rev, notifier)
}

// RetryReview 对指定报告再次执行评审
func (h *Handler) RetryReview(ctx context.Context, reportID string) error {
	rpt, err := h.store.Get(ctx, reportID)
	if err != nil {
		return err
	}
	_, ghClient, rev, notifier, ok := h.resolveProjectStack(ctx, rpt.RepoFullName)
	if !ok {
		return fmt.Errorf("repo %s is not enabled for review", rpt.RepoFullName)
	}
	owner, repo, err := ghclient.ParseRepoFullName(rpt.RepoFullName)
	if err != nil {
		return fmt.Errorf("parse repo: %w", err)
	}

	var diff *ghclient.DiffResult
	if rpt.EventType == "push" {
		diff, err = ghClient.GetCommitDiff(ctx, owner, repo, rpt.CommitSHA)
	} else {
		var prNum int
		if _, err := fmt.Sscanf(rpt.Ref, "#%d", &prNum); err != nil {
			return fmt.Errorf("parse PR number from ref %q: %w", rpt.Ref, err)
		}
		diff, err = ghClient.GetPRDiff(ctx, owner, repo, prNum)
	}
	if err != nil {
		return fmt.Errorf("get diff: %w", err)
	}

	req := &reviewer.ReviewRequest{
		RepoFullName: rpt.RepoFullName,
		EventType:    rpt.EventType,
		Ref:          rpt.Ref,
		CommitSHA:    rpt.CommitSHA,
		Author:       rpt.Author,
		CommitMsg:    rpt.CommitMsg,
		HTMLURL:      rpt.HTMLURL,
		Diff:         diff,
	}

	rpt.Status = "pending"
	rpt.ErrorMsg = ""
	_ = h.store.Update(ctx, rpt)

	h.runReview(ctx, req, rpt, rev, notifier)
	return nil
}

func (h *Handler) runReview(ctx context.Context, req *reviewer.ReviewRequest, rpt *report.ReviewReport, rev *reviewer.Reviewer, notifier notify.Notifier) {
	if rev == nil {
		rev = h.reviewer
	}
	if notifier == nil {
		notifier = h.notifier
	}
	result, err := rev.Review(ctx, req)
	if err != nil {
		log.Printf("review error: %v", err)
		rpt.Status, rpt.ErrorMsg = "failed", err.Error()
		_ = h.store.Update(ctx, rpt)
		return
	}

	rpt.Score = result.Score
	rpt.Summary = result.Summary
	rpt.Issues = result.Issues
	rpt.FilesNum = result.FilesNum
	rpt.LinesNum = result.LinesNum
	rpt.AIModel = result.AIModel
	rpt.Duration = result.Duration
	rpt.Status = "success"
	rpt.ErrorMsg = ""

	if err := h.store.Update(ctx, rpt); err != nil {
		log.Printf("update report error: %v", err)
		return
	}

	reportURL := fmt.Sprintf("%s/report/%s", h.cfg.Server.BaseURL, rpt.ID)
	log.Printf("review complete for %s %s: score=%d, url=%s", rpt.RepoFullName, rpt.CommitSHA[:8], rpt.Score, reportURL)

	if err := notifier.Send(ctx, rpt, reportURL); err != nil {
		log.Printf("notify error: %v", err)
	}
}

// resolveProjectStack 仅从 github_projects 读取 GitHub/评审/通知等逻辑配置。
func (h *Handler) resolveProjectStack(ctx context.Context, repoFullName string) (*project.Record, *ghclient.Client, *reviewer.Reviewer, notify.Notifier, bool) {
	row, err := h.proj.GetProjectRow(ctx, repoFullName)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Printf("get project: %v", err)
		return nil, nil, nil, nil, false
	}
	var rec *project.Record
	if err == nil {
		rec = row
	}
	if rec == nil || !rec.Enabled {
		log.Printf("skip: repo %q not in github_projects or disabled", repoFullName)
		return nil, nil, nil, nil, false
	}
	if rec.GitHubToken == "" {
		log.Printf("skip: github_projects.github_token empty for %q", repoFullName)
		return nil, nil, nil, nil, false
	}
	ghClient := ghclient.NewClient(&config.GitHubConfig{Token: rec.GitHubToken})

	mergedReview, merr := config.MergeReview(config.DefaultReviewConfig(), rec.ReviewJSON)
	if merr != nil {
		log.Printf("merge review_json: %v", merr)
		mergedReview = config.DefaultReviewConfig()
	}
	rev := h.reviewer.WithConfig(&mergedReview)
	notifier := notify.NotifierFromProjectJSON(rec.NotifyJSON)

	return rec, ghClient, rev, notifier, true
}

func peekRepoFullName(_ string, body []byte) string {
	type repoWrap struct {
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	var ev repoWrap
	if err := json.Unmarshal(body, &ev); err != nil {
		return ""
	}
	return ev.Repository.FullName
}

func verifySignature(payload []byte, signature, secret string) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	sig := strings.TrimPrefix(signature, "sha256=")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(sig), []byte(expected))
}

// GitHub webhook event types

type PushEvent struct {
	Ref        string     `json:"ref"`
	Before     string     `json:"before"`
	After      string     `json:"after"`
	Deleted    bool       `json:"deleted"`
	HeadCommit CommitInfo `json:"head_commit"`
	Commits    []CommitInfo `json:"commits"`
	Repository RepoInfo   `json:"repository"`
}

type CommitInfo struct {
	ID      string     `json:"id"`
	Message string     `json:"message"`
	URL     string     `json:"url"`
	Author  AuthorInfo `json:"author"`
}

type AuthorInfo struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type PullRequestEvent struct {
	Action      string   `json:"action"`
	PullRequest PRInfo   `json:"pull_request"`
	Repository  RepoInfo `json:"repository"`
}

type PRInfo struct {
	Number  int      `json:"number"`
	Title   string   `json:"title"`
	HTMLURL string   `json:"html_url"`
	User    UserInfo `json:"user"`
	Head    HeadInfo `json:"head"`
}

type UserInfo struct {
	Login string `json:"login"`
}

type HeadInfo struct {
	SHA string `json:"sha"`
}

type RepoInfo struct {
	FullName string `json:"full_name"`
}
