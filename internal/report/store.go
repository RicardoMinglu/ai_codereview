package report

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

type Store interface {
	Save(ctx context.Context, r *ReviewReport) error
	Update(ctx context.Context, r *ReviewReport) error
	Get(ctx context.Context, id string) (*ReviewReport, error)
	List(ctx context.Context, repo string, page, pageSize int) ([]*ReviewReport, int, error)
}

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS reports (
			id TEXT PRIMARY KEY,
			created_at DATETIME NOT NULL,
			repo_full_name TEXT NOT NULL,
			event_type TEXT NOT NULL,
			ref TEXT NOT NULL,
			commit_sha TEXT NOT NULL,
			author TEXT NOT NULL,
			commit_msg TEXT NOT NULL,
			html_url TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'success',
			error_msg TEXT DEFAULT '',
			score INTEGER NOT NULL,
			summary TEXT NOT NULL,
			issues TEXT NOT NULL,
			files_num INTEGER NOT NULL,
			lines_num INTEGER NOT NULL,
			ai_model TEXT NOT NULL,
			duration REAL NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_reports_repo ON reports(repo_full_name);
		CREATE INDEX IF NOT EXISTS idx_reports_created ON reports(created_at DESC);
	`)
	if err != nil {
		return err
	}
	// 兼容旧表：添加 status、error_msg 列（若已存在则忽略）
	_, _ = db.Exec("ALTER TABLE reports ADD COLUMN status TEXT DEFAULT 'success'")
	_, _ = db.Exec("ALTER TABLE reports ADD COLUMN error_msg TEXT DEFAULT ''")
	return nil
}

func (s *SQLiteStore) Save(ctx context.Context, r *ReviewReport) error {
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

func (s *SQLiteStore) Update(ctx context.Context, r *ReviewReport) error {
	if r.Issues == nil {
		r.Issues = []Issue{}
	}
	issuesJSON, err := json.Marshal(r.Issues)
	if err != nil {
		return fmt.Errorf("marshal issues: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE reports SET status=?, error_msg=?, score=?, summary=?, issues=?, files_num=?, lines_num=?, ai_model=?, duration=?
		WHERE id=?`,
		r.Status, r.ErrorMsg, r.Score, r.Summary, string(issuesJSON),
		r.FilesNum, r.LinesNum, r.AIModel, r.Duration, r.ID,
	)
	return err
}

func (s *SQLiteStore) Get(ctx context.Context, id string) (*ReviewReport, error) {
	r := &ReviewReport{}
	var issuesJSON string

	err := s.db.QueryRowContext(ctx, `
		SELECT id, created_at, repo_full_name, event_type, ref, commit_sha, author, commit_msg, html_url, status, error_msg, score, summary, issues, files_num, lines_num, ai_model, duration
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

func (s *SQLiteStore) List(ctx context.Context, repo string, page, pageSize int) ([]*ReviewReport, int, error) {
	var total int
	var countErr error
	if repo != "" {
		countErr = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM reports WHERE repo_full_name = ?", repo).Scan(&total)
	} else {
		countErr = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM reports").Scan(&total)
	}
	if countErr != nil {
		return nil, 0, countErr
	}

	offset := (page - 1) * pageSize
	var rows *sql.Rows
	var err error
	if repo != "" {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, created_at, repo_full_name, event_type, ref, commit_sha, author, commit_msg, html_url, status, error_msg, score, summary, issues, files_num, lines_num, ai_model, duration
			FROM reports WHERE repo_full_name = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`, repo, pageSize, offset)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, created_at, repo_full_name, event_type, ref, commit_sha, author, commit_msg, html_url, status, error_msg, score, summary, issues, files_num, lines_num, ai_model, duration
			FROM reports ORDER BY created_at DESC LIMIT ? OFFSET ?`, pageSize, offset)
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var reports []*ReviewReport
	for rows.Next() {
		r := &ReviewReport{}
		var issuesJSON string
		if err := rows.Scan(&r.ID, &r.CreatedAt, &r.RepoFullName, &r.EventType, &r.Ref, &r.CommitSHA,
			&r.Author, &r.CommitMsg, &r.HTMLURL, &r.Status, &r.ErrorMsg, &r.Score, &r.Summary,
			&issuesJSON, &r.FilesNum, &r.LinesNum, &r.AIModel, &r.Duration,
		); err != nil {
			return nil, 0, err
		}
		if err := json.Unmarshal([]byte(issuesJSON), &r.Issues); err != nil {
			return nil, 0, fmt.Errorf("unmarshal issues: %w", err)
		}
		if r.Status == "" {
			r.Status = "success"
		}
		reports = append(reports, r)
	}
	return reports, total, rows.Err()
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// scanRows 从 sql.Rows 解析为 ReviewReport 列表，供 MySQL/PostgreSQL 使用
func scanRows(rows *sql.Rows) ([]*ReviewReport, error) {
	var reports []*ReviewReport
	for rows.Next() {
		r := &ReviewReport{}
		var issuesJSON string
		if err := rows.Scan(&r.ID, &r.CreatedAt, &r.RepoFullName, &r.EventType, &r.Ref, &r.CommitSHA,
			&r.Author, &r.CommitMsg, &r.HTMLURL, &r.Status, &r.ErrorMsg, &r.Score, &r.Summary,
			&issuesJSON, &r.FilesNum, &r.LinesNum, &r.AIModel, &r.Duration,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(issuesJSON), &r.Issues); err != nil {
			return nil, fmt.Errorf("unmarshal issues: %w", err)
		}
		if r.Status == "" {
			r.Status = "success"
		}
		reports = append(reports, r)
	}
	return reports, rows.Err()
}
