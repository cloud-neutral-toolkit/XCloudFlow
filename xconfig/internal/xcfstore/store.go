package xcfstore

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, dsn string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	ctxPing, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(ctxPing); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

// CreateRun inserts into xcf.runs and returns the generated run_id (UUID string).
// Requires the schema from XCloudFlow/sql/schema.sql to be applied.
func (s *Store) CreateRun(ctx context.Context, stack, env, phase, status, actor, configRef string, inputsJSON []byte) (string, error) {
	if env == "" {
		env = ""
	}
	if len(inputsJSON) == 0 {
		inputsJSON = []byte("{}")
	}
	var runID string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO xcf.runs (stack, env, phase, status, actor, config_ref, inputs)
		VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb)
		RETURNING run_id::text
	`, stack, env, phase, status, nullIfEmpty(actor), nullIfEmpty(configRef), string(inputsJSON)).Scan(&runID)
	return runID, err
}

func (s *Store) FinishRun(ctx context.Context, runID, status string, resultJSON []byte) error {
	if len(resultJSON) == 0 {
		resultJSON = []byte("{}")
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE xcf.runs
		SET status=$2, finished_at=now(), result=$3::jsonb
		WHERE run_id=$1::uuid
	`, runID, status, string(resultJSON))
	return err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

