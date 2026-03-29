package report

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
	if err := migrateMySQL(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate mysql: %w", err)
	}
	return &MySQLStore{db: db}, nil
}

func migrateMySQL(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS reports (
			id VARCHAR(36) PRIMARY KEY,
			created_at DATETIME(6) NOT NULL,
			repo_full_name VARCHAR(255) NOT NULL,
			event_type VARCHAR(32) NOT NULL,
			ref VARCHAR(255) NOT NULL,
			commit_sha VARCHAR(64) NOT NULL,
			author VARCHAR(255) NOT NULL,
			commit_msg TEXT NOT NULL,
			html_url TEXT NOT NULL,
			status VARCHAR(32) NOT NULL DEFAULT 'success',
			error_msg TEXT,
			score INT NOT NULL,
			summary TEXT NOT NULL,
			issues JSON NOT NULL,
			files_num INT NOT NULL,
			lines_num INT NOT NULL,
			ai_model VARCHAR(128) NOT NULL,
			duration DOUBLE NOT NULL
		)
	`)
	if err != nil {
		return err
	}
	// MySQL 无 CREATE INDEX IF NOT EXISTS，忽略已存在错误
	_, _ = db.Exec("CREATE INDEX idx_reports_repo ON reports(repo_full_name)")
	_, _ = db.Exec("CREATE INDEX idx_reports_created ON reports(created_at DESC)")
	// 兼容旧表
	_, _ = db.Exec("ALTER TABLE reports ADD COLUMN status VARCHAR(32) NOT NULL DEFAULT 'success'")
	_, _ = db.Exec("ALTER TABLE reports ADD COLUMN error_msg TEXT")
	return nil
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
