-- MySQL 库表初始化（部署前执行一次，或与 Docker docker-entrypoint-initdb.d 一并挂载）。
-- 服务启动时不再自动建表，请确保已执行本脚本。
-- 先建库: CREATE DATABASE ai_review CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

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

CREATE TABLE IF NOT EXISTS github_projects (
	id INT AUTO_INCREMENT PRIMARY KEY,
	repo_full_name VARCHAR(255) NOT NULL UNIQUE,
	enabled TINYINT(1) NOT NULL DEFAULT 1,
	github_token TEXT NULL,
	webhook_secret VARCHAR(512) NULL,
	review_json JSON NULL,
	notify_json JSON NULL,
	push_branches JSON NULL COMMENT '["main","develop"]；NULL 或空数组表示所有分支',
	created_at DATETIME(6) DEFAULT CURRENT_TIMESTAMP(6),
	updated_at DATETIME(6) DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6)
);

CREATE INDEX idx_github_projects_enabled ON github_projects(enabled);
