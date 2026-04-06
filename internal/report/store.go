package report

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// Store 持久化评审报告（当前实现仅为 MySQL）。
type Store interface {
	Save(ctx context.Context, r *ReviewReport) error
	Update(ctx context.Context, r *ReviewReport) error
	Get(ctx context.Context, id string) (*ReviewReport, error)
	List(ctx context.Context, repo string, page, pageSize int) ([]*ReviewReport, int, error)
}

// scanRows 从 sql.Rows 解析为 ReviewReport 列表。
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
