package report

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/RicardoMinglu/ai_codereview/internal/config"
)

// StoreCloser 组合 Store 与 Close 方法，便于 main 中 defer Close
type StoreCloser interface {
	Store
	Close() error
}

// NewStore 根据配置创建对应的 Store（storage.type 为空时默认 mysql，与 config.Load 默认一致）
func NewStore(cfg *config.StorageConfig) (StoreCloser, error) {
	storageType := cfg.Type
	if storageType == "" {
		storageType = "mysql"
	}
	switch storageType {
	case "sqlite":
		if cfg.Path == "" {
			cfg.Path = "./data/reviews.db"
		}
		if err := os.MkdirAll(filepath.Dir(cfg.Path), 0755); err != nil {
			return nil, fmt.Errorf("create storage dir: %w", err)
		}
		return NewSQLiteStore(cfg.Path)
	case "mysql":
		if cfg.DSN == "" {
			return nil, fmt.Errorf("storage.dsn is required when type is mysql")
		}
		return NewMySQLStore(cfg.DSN)
	case "pgsql", "postgres":
		if cfg.DSN == "" {
			return nil, fmt.Errorf("storage.dsn is required when type is pgsql")
		}
		return NewPgSQLStore(cfg.DSN)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.Type)
	}
}
