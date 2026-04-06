package report

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/RicardoMinglu/ai_codereview/internal/project"
	"github.com/google/uuid"
	_ "github.com/go-sql-driver/mysql"
)

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(dsn string) (*MySQLStore, error) {
	if !strings.Contains(dsn, "parseTime=") {
		if strings.Contains(dsn, "?") {
			dsn += "&parseTime=true"
		} else {
			dsn += "?parseTime=true"
		}
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	return &MySQLStore{db: db}, nil
}

// AnyProjectRow 实现 project.Reader：表中是否存在至少一行（用于首页/引导是否提示「尚未登记」）。
func (s *MySQLStore) AnyProjectRow(ctx context.Context) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM github_projects`).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func scanPushBranchesJSON(ns sql.NullString) []string {
	if !ns.Valid || strings.TrimSpace(ns.String) == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(ns.String), &out); err != nil || len(out) == 0 {
		return nil
	}
	for i := range out {
		out[i] = project.NormalizePushBranchName(out[i])
	}
	return out
}

func pushBranchesArg(branches []string) (interface{}, error) {
	if len(branches) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(branches)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// GetProjectRow 实现 project.Reader：按 owner/repo 取项目配置。
func (s *MySQLStore) GetProjectRow(ctx context.Context, repoFullName string) (*project.Record, error) {
	var rec project.Record
	var tok, wh sql.NullString
	var revJSON, notifyJSON sql.NullString

	var pushBr sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT repo_full_name, enabled, github_token, webhook_secret, review_json, notify_json, push_branches
		FROM github_projects WHERE repo_full_name = ?
	`, repoFullName).Scan(
		&rec.RepoFullName, &rec.Enabled, &tok, &wh, &revJSON, &notifyJSON, &pushBr,
	)
	if err != nil {
		return nil, err
	}
	if tok.Valid {
		rec.GitHubToken = tok.String
	}
	if wh.Valid {
		rec.WebhookSecret = wh.String
	}
	if revJSON.Valid && revJSON.String != "" {
		rec.ReviewJSON = []byte(revJSON.String)
	}
	if notifyJSON.Valid && notifyJSON.String != "" {
		rec.NotifyJSON = []byte(notifyJSON.String)
	}
	rec.PushBranches = scanPushBranchesJSON(pushBr)
	return &rec, nil
}

func (s *MySQLStore) Save(ctx context.Context, r *ReviewReport) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	if r.Status == "" {
		r.Status = "success"
	}
	if r.Issues == nil {
		r.Issues = []Issue{}
	}
	issuesJSON, err := json.Marshal(r.Issues)
	if err != nil {
		return fmt.Errorf("marshal issues: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO reports (id, created_at, repo_full_name, event_type, ref, commit_sha, author, commit_msg, html_url, status, error_msg, score, summary, issues, files_num, lines_num, ai_model, duration)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.CreatedAt, r.RepoFullName, r.EventType, r.Ref, r.CommitSHA,
		r.Author, r.CommitMsg, r.HTMLURL, r.Status, r.ErrorMsg, r.Score, r.Summary,
		string(issuesJSON), r.FilesNum, r.LinesNum, r.AIModel, r.Duration,
	)
	return err
}

func (s *MySQLStore) Update(ctx context.Context, r *ReviewReport) error {
	if r.Issues == nil {
		r.Issues = []Issue{}
	}
	issuesJSON, err := json.Marshal(r.Issues)
	if err != nil {
		return fmt.Errorf("marshal issues: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE reports SET status=?, error_msg=?, score=?, summary=?, issues=?, files_num=?, lines_num=?, ai_model=?, duration=? WHERE id=?`,
		r.Status, r.ErrorMsg, r.Score, r.Summary, string(issuesJSON),
		r.FilesNum, r.LinesNum, r.AIModel, r.Duration, r.ID,
	)
	return err
}

func (s *MySQLStore) Get(ctx context.Context, id string) (*ReviewReport, error) {
	r := &ReviewReport{}
	var issuesJSON string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, created_at, repo_full_name, event_type, ref, commit_sha, author, commit_msg, html_url, COALESCE(status,'success'), error_msg, score, summary, issues, files_num, lines_num, ai_model, duration
		FROM reports WHERE id = ?`, id,
	).Scan(&r.ID, &r.CreatedAt, &r.RepoFullName, &r.EventType, &r.Ref, &r.CommitSHA,
		&r.Author, &r.CommitMsg, &r.HTMLURL, &r.Status, &r.ErrorMsg, &r.Score, &r.Summary,
		&issuesJSON, &r.FilesNum, &r.LinesNum, &r.AIModel, &r.Duration,
	)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(issuesJSON), &r.Issues); err != nil {
		return nil, fmt.Errorf("unmarshal issues: %w", err)
	}
	return r, nil
}

func (s *MySQLStore) List(ctx context.Context, repo string, page, pageSize int) ([]*ReviewReport, int, error) {
	var total int
	if repo != "" {
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM reports WHERE repo_full_name = ?", repo).Scan(&total); err != nil {
			return nil, 0, err
		}
	} else {
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM reports").Scan(&total); err != nil {
			return nil, 0, err
		}
	}
	offset := (page - 1) * pageSize
	var rows *sql.Rows
	var err error
	if repo != "" {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, created_at, repo_full_name, event_type, ref, commit_sha, author, commit_msg, html_url, COALESCE(status,'success'), error_msg, score, summary, issues, files_num, lines_num, ai_model, duration
			FROM reports WHERE repo_full_name = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`, repo, pageSize, offset)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, created_at, repo_full_name, event_type, ref, commit_sha, author, commit_msg, html_url, COALESCE(status,'success'), error_msg, score, summary, issues, files_num, lines_num, ai_model, duration
			FROM reports ORDER BY created_at DESC LIMIT ? OFFSET ?`, pageSize, offset)
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	reports, err := scanRows(rows)
	return reports, total, err
}

func (s *MySQLStore) Close() error {
	return s.db.Close()
}

// ListProjects 获取所有项目配置（包含 ID）
func (s *MySQLStore) ListProjects(ctx context.Context) ([]project.Record, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, repo_full_name, enabled, github_token, webhook_secret, review_json, notify_json, push_branches
		FROM github_projects ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []project.Record
	for rows.Next() {
		var rec project.Record
		var tok, wh sql.NullString
		var revJSON, notifyJSON, pushBr sql.NullString

		if err := rows.Scan(&rec.ID, &rec.RepoFullName, &rec.Enabled, &tok, &wh, &revJSON, &notifyJSON, &pushBr); err != nil {
			return nil, err
		}

		if tok.Valid {
			rec.GitHubToken = tok.String
		}
		if wh.Valid {
			rec.WebhookSecret = wh.String
		}
		if revJSON.Valid && revJSON.String != "" {
			rec.ReviewJSON = []byte(revJSON.String)
		}
		if notifyJSON.Valid && notifyJSON.String != "" {
			rec.NotifyJSON = []byte(notifyJSON.String)
		}
		rec.PushBranches = scanPushBranchesJSON(pushBr)

		projects = append(projects, rec)
	}

	return projects, rows.Err()
}

// GetProject 根据 ID 获取项目配置
func (s *MySQLStore) GetProject(ctx context.Context, id int) (*project.Record, error) {
	var rec project.Record
	var tok, wh sql.NullString
	var revJSON, notifyJSON sql.NullString

	var pushBr sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT repo_full_name, enabled, github_token, webhook_secret, review_json, notify_json, push_branches
		FROM github_projects WHERE id = ?
	`, id).Scan(&rec.RepoFullName, &rec.Enabled, &tok, &wh, &revJSON, &notifyJSON, &pushBr)

	if err != nil {
		return nil, err
	}

	if tok.Valid {
		rec.GitHubToken = tok.String
	}
	if wh.Valid {
		rec.WebhookSecret = wh.String
	}
	if revJSON.Valid && revJSON.String != "" {
		rec.ReviewJSON = []byte(revJSON.String)
	}
	if notifyJSON.Valid && notifyJSON.String != "" {
		rec.NotifyJSON = []byte(notifyJSON.String)
	}
	rec.PushBranches = scanPushBranchesJSON(pushBr)

	return &rec, nil
}

// AddProject 添加项目配置
func (s *MySQLStore) AddProject(ctx context.Context, rec *project.Record) error {
	var githubToken, webhookSecret, reviewJSON, notifyJSON interface{}

	if rec.GitHubToken != "" {
		githubToken = rec.GitHubToken
	}
	if rec.WebhookSecret != "" {
		webhookSecret = rec.WebhookSecret
	}
	if len(rec.ReviewJSON) > 0 {
		reviewJSON = string(rec.ReviewJSON)
	}
	if len(rec.NotifyJSON) > 0 {
		notifyJSON = string(rec.NotifyJSON)
	}
	pushBr, err := pushBranchesArg(rec.PushBranches)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO github_projects (repo_full_name, enabled, github_token, webhook_secret, review_json, notify_json, push_branches)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, rec.RepoFullName, rec.Enabled, githubToken, webhookSecret, reviewJSON, notifyJSON, pushBr)

	return err
}

// UpdateProject 更新项目配置
func (s *MySQLStore) UpdateProject(ctx context.Context, rec *project.Record) error {
	// 从 context 获取 ID
	id, ok := ctx.Value("project_id").(int)
	if !ok {
		return fmt.Errorf("project_id not found in context")
	}

	var githubToken, webhookSecret, reviewJSON, notifyJSON interface{}

	if rec.GitHubToken != "" {
		githubToken = rec.GitHubToken
	}
	if rec.WebhookSecret != "" {
		webhookSecret = rec.WebhookSecret
	}
	if len(rec.ReviewJSON) > 0 {
		reviewJSON = string(rec.ReviewJSON)
	}
	if len(rec.NotifyJSON) > 0 {
		notifyJSON = string(rec.NotifyJSON)
	}
	pushBr, err := pushBranchesArg(rec.PushBranches)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE github_projects
		SET repo_full_name=?, enabled=?, github_token=?, webhook_secret=?, review_json=?, notify_json=?, push_branches=?
		WHERE id=?
	`, rec.RepoFullName, rec.Enabled, githubToken, webhookSecret, reviewJSON, notifyJSON, pushBr, id)

	return err
}

// DeleteProject 删除项目配置
func (s *MySQLStore) DeleteProject(ctx context.Context, id int) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM github_projects WHERE id=?`, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

