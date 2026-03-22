package persistence

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/config"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func Open(ctx context.Context, cfg config.DBConfig) (*sql.DB, error) {
	db, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
}
