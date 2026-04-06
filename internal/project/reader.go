package project

import (
	"context"
	"database/sql"
)

// Reader 从存储读取多仓库配置；非 MySQL 实现可用 NoopReader。
type Reader interface {
	// AnyProjectRow 为 true 表示表中至少有一行：进入「仅处理已登记仓库」模式。
	AnyProjectRow(ctx context.Context) (bool, error)
	// GetProjectRow 按仓库全名取一行；不存在时返回 sql.ErrNoRows。
	GetProjectRow(ctx context.Context, repoFullName string) (*Record, error)
}

// NoopReader 占位实现（生产路径应使用 MySQL 并可读 github_projects）。
type NoopReader struct{}

func (NoopReader) AnyProjectRow(ctx context.Context) (bool, error) {
	return false, nil
}

func (NoopReader) GetProjectRow(ctx context.Context, repoFullName string) (*Record, error) {
	return nil, sql.ErrNoRows
}
