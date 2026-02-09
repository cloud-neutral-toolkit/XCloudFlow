package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
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

// ExecSQL executes raw SQL (used for schema bootstrap).
// Caller is responsible for idempotency (schema.sql should be).
func (s *Store) ExecSQL(ctx context.Context, sql string) error {
	_, err := s.pool.Exec(ctx, sql)
	return err
}

func (s *Store) CreateRun(ctx context.Context, r Run) (string, error) {
	if r.RunID == "" {
		r.RunID = uuid.NewString()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO xcf.runs (run_id, stack, env, phase, status, actor, config_ref, inputs, plan, result)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9::jsonb,$10::jsonb)
	`, r.RunID, r.Stack, r.Env, r.Phase, r.Status, nullIfEmpty(r.Actor), nullIfEmpty(r.ConfigRef),
		jsonOrEmpty(r.InputsJSON), jsonOrEmpty(r.PlanJSON), jsonOrEmpty(r.ResultJSON),
	)
	if err != nil {
		return "", err
	}
	return r.RunID, nil
}

func (s *Store) FinishRun(ctx context.Context, runID string, status string, resultJSON []byte) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE xcf.runs
		SET status=$2, finished_at=now(), result=$3::jsonb
		WHERE run_id=$1
	`, runID, status, jsonOrEmpty(resultJSON))
	return err
}

func (s *Store) UpsertMCPServer(ctx context.Context, srv MCPServer) (string, error) {
	if srv.ServerID == "" {
		srv.ServerID = uuid.NewString()
	}
	if srv.Kind == "" {
		srv.Kind = "generic"
	}
	if srv.AuthType == "" {
		srv.AuthType = "none"
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO xcf.mcp_servers (server_id, name, base_url, kind, auth_type, audience, enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (name) DO UPDATE SET
		  base_url=EXCLUDED.base_url,
		  kind=EXCLUDED.kind,
		  auth_type=EXCLUDED.auth_type,
		  audience=EXCLUDED.audience,
		  enabled=EXCLUDED.enabled,
		  updated_at=now()
	`, srv.ServerID, srv.Name, srv.BaseURL, srv.Kind, srv.AuthType, nullIfEmpty(srv.Audience), srv.Enabled)
	if err != nil {
		return "", err
	}
	return srv.ServerID, nil
}

func (s *Store) ListMCPServers(ctx context.Context) ([]MCPServer, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT server_id, name, base_url, kind, auth_type, COALESCE(audience,''), enabled, created_at, updated_at
		FROM xcf.mcp_servers
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MCPServer
	for rows.Next() {
		var srv MCPServer
		if err := rows.Scan(&srv.ServerID, &srv.Name, &srv.BaseURL, &srv.Kind, &srv.AuthType, &srv.Audience, &srv.Enabled, &srv.CreatedAt, &srv.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, srv)
	}
	return out, rows.Err()
}

func (s *Store) UpdateMCPToolsCache(ctx context.Context, serverID string, toolsJSON []byte, etag string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO xcf.mcp_tools_cache (server_id, tools, etag)
		VALUES ($1,$2::jsonb,$3)
		ON CONFLICT (server_id) DO UPDATE SET
		  tools=EXCLUDED.tools,
		  etag=EXCLUDED.etag,
		  fetched_at=now()
	`, serverID, jsonOrArray(toolsJSON), nullIfEmpty(etag))
	return err
}

func (s *Store) AddSkillSource(ctx context.Context, src SkillSource) (string, error) {
	if src.SourceID == "" {
		src.SourceID = uuid.NewString()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO xcf.skill_sources (source_id, name, type, uri, ref, base_path, enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (name) DO UPDATE SET
		  type=EXCLUDED.type,
		  uri=EXCLUDED.uri,
		  ref=EXCLUDED.ref,
		  base_path=EXCLUDED.base_path,
		  enabled=EXCLUDED.enabled,
		  updated_at=now()
	`, src.SourceID, src.Name, src.Type, src.URI, nullIfEmpty(src.Ref), nullIfEmpty(src.BasePath), src.Enabled)
	if err != nil {
		return "", err
	}
	return src.SourceID, nil
}

func (s *Store) ListSkillSources(ctx context.Context) ([]SkillSource, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT source_id, name, type, uri, COALESCE(ref,''), COALESCE(base_path,''), enabled
		FROM xcf.skill_sources
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SkillSource
	for rows.Next() {
		var src SkillSource
		if err := rows.Scan(&src.SourceID, &src.Name, &src.Type, &src.URI, &src.Ref, &src.BasePath, &src.Enabled); err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, rows.Err()
}

func (s *Store) UpsertSkillDoc(ctx context.Context, sourceID string, path string, sha256 string, content string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO xcf.skill_docs (source_id, path, sha256, content)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (source_id, path) DO UPDATE SET
		  sha256=EXCLUDED.sha256,
		  content=EXCLUDED.content,
		  fetched_at=now()
	`, sourceID, path, sha256, content)
	return err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func jsonOrEmpty(b []byte) string {
	if len(b) == 0 {
		return "{}"
	}
	return string(b)
}

func jsonOrArray(b []byte) string {
	if len(b) == 0 {
		return "[]"
	}
	return string(b)
}

