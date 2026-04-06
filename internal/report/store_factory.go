package report

import (
	"fmt"
	"strings"

	"github.com/RicardoMinglu/ai_codereview/internal/config"
)

// StoreCloser 组合 Store 与 Close 方法，便于 main 中 defer Close
type StoreCloser interface {
	Store
	Close() error
}

// NewStore 仅支持 MySQL；storage.type 留空时视为 mysql。
func NewStore(cfg *config.StorageConfig) (StoreCloser, error) {
	storageType := strings.TrimSpace(cfg.Type)
	if storageType == "" {
		storageType = "mysql"
	}
	if storageType != "mysql" {
		return nil, fmt.Errorf("unsupported storage type %q: only \"mysql\" is supported", storageType)
	}
	if cfg.DSN == "" {
		return nil, fmt.Errorf("storage.dsn is required")
	}
	return NewMySQLStore(cfg.DSN)
}
