// Package db opens and owns the PostgreSQL connection pool.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nguyentanvinh/my-claude/backend/internal/platform/config"
)

func Connect(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.ConnectDatabaseString())
	if err != nil {
		return nil, fmt.Errorf("db: parse connection string: %w", err)
	}

	poolConfig.MaxConns = cfg.Database.MaxConns
	poolConfig.MinConns = cfg.Database.MinConns
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime()
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime()

	poolConfig.ConnConfig.ConnectTimeout = cfg.ConnectTimeout()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout())
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping %s: %w", cfg.RedactedDatabaseTarget(), err)
	}

	return pool, nil
}
