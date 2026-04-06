package report

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// testMySQLDSN 设置后可跑与 Store 相关的集成测试（须已建库，且能执行 docs/mysql/init.sql）。
// CI 中由 workflow 注入；本地不配则测试跳过。
func testMySQLDSN(t *testing.T) string {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run MySQL store tests, e.g. user:pass@tcp(127.0.0.1:3306)/ai_review_test?parseTime=true&multiStatements=true")
	}
	if !strings.Contains(dsn, "parseTime=") {
		if strings.Contains(dsn, "?") {
			dsn += "&parseTime=true"
		} else {
			dsn += "?parseTime=true"
		}
	}
	if !strings.Contains(dsn, "multiStatements=") {
		if strings.Contains(dsn, "?") {
			dsn += "&multiStatements=true"
		} else {
			dsn += "?multiStatements=true"
		}
	}
	return dsn
}

func initSQLPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	dir := filepath.Dir(file)
	p := filepath.Join(dir, "..", "..", "docs", "mysql", "init.sql")
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatalf("abs init sql: %v", err)
	}
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("stat init.sql: %v", err)
	}
	return abs
}

func setupTestMySQLStore(t *testing.T) *MySQLStore {
	t.Helper()
	dsn := testMySQLDSN(t)
	b, err := os.ReadFile(initSQLPath(t))
	if err != nil {
		t.Fatalf("read init.sql: %v", err)
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open mysql: %v", err)
	}
	if _, err := db.Exec(string(b)); err != nil {
		db.Close()
		t.Fatalf("apply init.sql: %v", err)
	}
	db.Close()

	store, err := NewMySQLStore(dsn)
	if err != nil {
		t.Fatalf("NewMySQLStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	if _, err := store.db.ExecContext(context.Background(), "TRUNCATE TABLE reports"); err != nil {
		store.Close()
		t.Fatalf("truncate reports: %v", err)
	}
	return store
}

func TestStore_SaveAndGet(t *testing.T) {
	store := setupTestMySQLStore(t)
	ctx := context.Background()

	rpt := &ReviewReport{
		RepoFullName: "owner/repo",
		EventType:    "push",
		Ref:          "main",
		CommitSHA:    "abc123",
		Author:       "alice",
		CommitMsg:    "fix bug",
		HTMLURL:      "https://github.com/owner/repo/commit/abc123",
		Score:        85,
		Summary:      "Good",
		Issues:       []Issue{{File: "a.go", Line: 1, Severity: "warning", Category: "style", Message: "msg"}},
		FilesNum:     1,
		LinesNum:     10,
		AIModel:      "claude/test",
		Duration:     1.5,
	}

	if err := store.Save(ctx, rpt); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if rpt.ID == "" {
		t.Error("expected ID to be set after Save")
	}
	if rpt.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set after Save")
	}

	got, err := store.Get(ctx, rpt.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.RepoFullName != rpt.RepoFullName || got.Score != rpt.Score || got.Summary != rpt.Summary {
		t.Errorf("Get: got %+v", got)
	}
	if len(got.Issues) != 1 || got.Issues[0].File != "a.go" {
		t.Errorf("Get: issues = %+v", got.Issues)
	}
}

func TestStore_GetNotFound(t *testing.T) {
	store := setupTestMySQLStore(t)
	ctx := context.Background()

	_, err := store.Get(ctx, "non-existent-id")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestStore_List(t *testing.T) {
	store := setupTestMySQLStore(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		rpt := &ReviewReport{
			RepoFullName: "owner/repo",
			EventType:    "push",
			Ref:          "main",
			CommitSHA:    "abc123",
			Author:       "alice",
			CommitMsg:    "fix",
			HTMLURL:      "https://x",
			Score:        80,
			Summary:      "ok",
			Issues:       nil,
			FilesNum:     1,
			LinesNum:     5,
			AIModel:      "test",
			Duration:     1,
			CreatedAt:    time.Now().Add(-time.Duration(i) * time.Hour),
		}
		if err := store.Save(ctx, rpt); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	reports, total, err := store.List(ctx, "", 1, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	if len(reports) != 3 {
		t.Errorf("expected 3 reports, got %d", len(reports))
	}
}

func TestStore_ListWithRepoFilter(t *testing.T) {
	store := setupTestMySQLStore(t)
	ctx := context.Background()

	rpt1 := &ReviewReport{
		RepoFullName: "owner/repo1",
		EventType:    "push",
		Ref:          "main",
		CommitSHA:    "a",
		Author:       "a",
		CommitMsg:    "a",
		HTMLURL:      "https://x",
		Score:        80,
		Summary:      "a",
		Issues:       nil,
		FilesNum:     1,
		LinesNum:     1,
		AIModel:      "test",
		Duration:     1,
	}
	rpt2 := &ReviewReport{
		RepoFullName: "owner/repo2",
		EventType:    "push",
		Ref:          "main",
		CommitSHA:    "b",
		Author:       "b",
		CommitMsg:    "b",
		HTMLURL:      "https://x",
		Score:        80,
		Summary:      "b",
		Issues:       nil,
		FilesNum:     1,
		LinesNum:     1,
		AIModel:      "test",
		Duration:     1,
	}
	if err := store.Save(ctx, rpt1); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Save(ctx, rpt2); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reports, total, err := store.List(ctx, "owner/repo1", 1, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
	if len(reports) != 1 || reports[0].RepoFullName != "owner/repo1" {
		t.Errorf("expected repo1, got %+v", reports)
	}
}

func TestStore_ListPagination(t *testing.T) {
	store := setupTestMySQLStore(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		rpt := &ReviewReport{
			RepoFullName: "owner/repo",
			EventType:    "push",
			Ref:          "main",
			CommitSHA:    "abc",
			Author:       "a",
			CommitMsg:    "a",
			HTMLURL:      "https://x",
			Score:        80,
			Summary:      "a",
			Issues:       nil,
			FilesNum:     1,
			LinesNum:     1,
			AIModel:      "test",
			Duration:     1,
			CreatedAt:    time.Now().Add(-time.Duration(i) * time.Minute),
		}
		if err := store.Save(ctx, rpt); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	page1, total, _ := store.List(ctx, "owner/repo", 1, 2)
	if total != 5 || len(page1) != 2 {
		t.Errorf("page 1: total=%d len=%d", total, len(page1))
	}
	page2, _, _ := store.List(ctx, "owner/repo", 2, 2)
	if len(page2) != 2 {
		t.Errorf("page 2: len=%d", len(page2))
	}
	page3, _, _ := store.List(ctx, "owner/repo", 3, 2)
	if len(page3) != 1 {
		t.Errorf("page 3: len=%d", len(page3))
	}
}
