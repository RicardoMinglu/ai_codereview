package report

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type PgSQLStore struct {
	db *sql.DB
}

func NewPgSQLStore(dsn string) (*PgSQLStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	if err := migratePgSQL(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate postgres: %w", err)
	}
	return &PgSQLStore{db: db}, nil
}

func migratePgSQL(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS reports (
			id TEXT PRIMARY KEY,
			created_at TIMESTAMP NOT NULL,
			repo_full_name TEXT NOT NULL,
			event_type TEXT NOT NULL,
			ref TEXT NOT NULL,
			commit_sha TEXT NOT NULL,
			author TEXT NOT NULL,
			commit_msg TEXT NOT NULL,
			html_url TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'success',
			error_msg TEXT,
			score INTEGER NOT NULL,
			summary TEXT NOT NULL,
			issues JSONB NOT NULL,
			files_num INTEGER NOT NULL,
			lines_num INTEGER NOT NULL,
			ai_model TEXT NOT NULL,
			duration DOUBLE PRECISION NOT NULL
		)
	`)
	if err != nil {
		return err
	}
	_, _ = db.Exec("CREATE INDEX IF NOT EXISTS idx_reports_repo ON reports(repo_full_name)")
	_, _ = db.Exec("CREATE INDEX IF NOT EXISTS idx_reports_created ON reports(created_at DESC)")
	_, _ = db.Exec("ALTER TABLE reports ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'success'")
	_, _ = db.Exec("ALTER TABLE reports ADD COLUMN IF NOT EXISTS error_msg TEXT")
	return nil
}

func (s *PgSQLStore) Save(ctx context.Context, r *ReviewReport) error {
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
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)`,
		r.ID, r.CreatedAt, r.RepoFullName, r.EventType, r.Ref, r.CommitSHA,
		r.Author, r.CommitMsg, r.HTMLURL, r.Status, r.ErrorMsg, r.Score, r.Summary,
		string(issuesJSON), r.FilesNum, r.LinesNum, r.AIModel, r.Duration,
	)
	return err
}

func (s *PgSQLStore) Update(ctx context.Context, r *ReviewReport) error {
	if r.Issues == nil {
		r.Issues = []Issue{}
	}
	issuesJSON, err := json.Marshal(r.Issues)
	if err != nil {
		return fmt.Errorf("marshal issues: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE reports SET status=$1, error_msg=$2, score=$3, summary=$4, issues=$5, files_num=$6, lines_num=$7, ai_model=$8, duration=$9 WHERE id=$10`,
		r.Status, r.ErrorMsg, r.Score, r.Summary, string(issuesJSON),
		r.FilesNum, r.LinesNum, r.AIModel, r.Duration, r.ID,
	)
	return err
}

func (s *PgSQLStore) Get(ctx context.Context, id string) (*ReviewReport, error) {
	r := &ReviewReport{}
	var issuesJSON string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, created_at, repo_full_name, event_type, ref, commit_sha, author, commit_msg, html_url, COALESCE(status,'success'), error_msg, score, summary, issues::text, files_num, lines_num, ai_model, duration
		FROM reports WHERE id = $1`, id,
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

func (s *PgSQLStore) List(ctx context.Context, repo string, page, pageSize int) ([]*ReviewReport, int, error) {
	var total int
	if repo != "" {
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM reports WHERE repo_full_name = $1", repo).Scan(&total); err != nil {
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
			SELECT id, created_at, repo_full_name, event_type, ref, commit_sha, author, commit_msg, html_url, COALESCE(status,'success'), error_msg, score, summary, issues::text, files_num, lines_num, ai_model, duration
			FROM reports WHERE repo_full_name = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, repo, pageSize, offset)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, created_at, repo_full_name, event_type, ref, commit_sha, author, commit_msg, html_url, COALESCE(status,'success'), error_msg, score, summary, issues::text, files_num, lines_num, ai_model, duration
			FROM reports ORDER BY created_at DESC LIMIT $1 OFFSET $2`, pageSize, offset)
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	reports, err := scanRows(rows)
	return reports, total, err
}

func (s *PgSQLStore) Close() error {
	return s.db.Close()
}
