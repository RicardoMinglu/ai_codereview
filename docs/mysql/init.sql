-- Optional: initialize MySQL schema (e.g. Docker docker-entrypoint-initdb.d).
-- The service also runs equivalent DDL on first connect (internal/report/store_mysql.go migrateMySQL).
-- Create database first: CREATE DATABASE ai_review CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

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
);

CREATE INDEX idx_reports_repo ON reports(repo_full_name);
CREATE INDEX idx_reports_created ON reports(created_at DESC);
