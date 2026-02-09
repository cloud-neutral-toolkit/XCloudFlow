-- XCloudFlow: state/memory schema for Agent + StackFlow control plane
--
-- Target DB: postgresql.svc.plus (PostgreSQL 14+ recommended)
-- Notes:
--   - XCloudFlow is stateless; persist all run state/memory/cache in Postgres.
--   - This schema is intentionally minimal and safe to evolve via migrations.

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE SCHEMA IF NOT EXISTS xcf;

-- ------------------------------------------------------------
-- Agents / Sessions / Events (audit + memory timeline)
-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS xcf.agents (
  agent_id   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name       TEXT NOT NULL UNIQUE,
  mode       TEXT NOT NULL DEFAULT 'worker',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS xcf.agent_sessions (
  session_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  agent_id   UUID NOT NULL REFERENCES xcf.agents(agent_id) ON DELETE CASCADE,
  kind       TEXT NOT NULL DEFAULT 'run',
  status     TEXT NOT NULL DEFAULT 'active',
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ended_at   TIMESTAMPTZ,
  metadata   JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS xcf.agent_events (
  event_id   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id UUID NOT NULL REFERENCES xcf.agent_sessions(session_id) ON DELETE CASCADE,
  ts         TIMESTAMPTZ NOT NULL DEFAULT now(),
  level      TEXT NOT NULL DEFAULT 'info',
  event_type TEXT NOT NULL,
  message    TEXT NOT NULL,
  data       JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS agent_events_session_ts
  ON xcf.agent_events(session_id, ts DESC);

-- ------------------------------------------------------------
-- StackFlow runs (plan/apply) + artifacts
-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS xcf.runs (
  run_id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  stack       TEXT NOT NULL,
  env         TEXT NOT NULL DEFAULT '',
  phase       TEXT NOT NULL,
  status      TEXT NOT NULL,
  actor       TEXT,
  config_ref  TEXT,
  started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  finished_at TIMESTAMPTZ,
  inputs      JSONB NOT NULL DEFAULT '{}'::jsonb,
  plan        JSONB NOT NULL DEFAULT '{}'::jsonb,
  result      JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS runs_stack_env_phase_started
  ON xcf.runs(stack, env, phase, started_at DESC);

CREATE TABLE IF NOT EXISTS xcf.run_artifacts (
  artifact_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id      UUID NOT NULL REFERENCES xcf.runs(run_id) ON DELETE CASCADE,
  kind        TEXT NOT NULL,
  uri         TEXT NOT NULL,
  checksum    TEXT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  metadata    JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS run_artifacts_run_id
  ON xcf.run_artifacts(run_id);

-- ------------------------------------------------------------
-- Leases (distributed lock) for multi-instance Agent runs
-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS xcf.leases (
  lease_key   TEXT PRIMARY KEY,
  owner       TEXT NOT NULL,
  acquired_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at  TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS leases_expires_at
  ON xcf.leases(expires_at);

-- ------------------------------------------------------------
-- MCP registry + tools cache
-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS xcf.mcp_servers (
  server_id    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name         TEXT NOT NULL UNIQUE,
  base_url     TEXT NOT NULL,
  kind         TEXT NOT NULL DEFAULT 'generic',
  auth_type    TEXT NOT NULL DEFAULT 'none',
  audience     TEXT,
  enabled      BOOLEAN NOT NULL DEFAULT TRUE,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_seen_at TIMESTAMPTZ,
  metadata     JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS xcf.mcp_tools_cache (
  server_id  UUID PRIMARY KEY REFERENCES xcf.mcp_servers(server_id) ON DELETE CASCADE,
  fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  tools      JSONB NOT NULL DEFAULT '[]'::jsonb,
  etag       TEXT
);

-- ------------------------------------------------------------
-- Skills: external sources + cached docs (Cloud Run friendly)
-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS xcf.skill_sources (
  source_id  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name       TEXT NOT NULL UNIQUE,
  type       TEXT NOT NULL,
  uri        TEXT NOT NULL,
  ref        TEXT,
  base_path  TEXT,
  enabled    BOOLEAN NOT NULL DEFAULT TRUE,
  auth       JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS xcf.skill_docs (
  source_id  UUID NOT NULL REFERENCES xcf.skill_sources(source_id) ON DELETE CASCADE,
  path       TEXT NOT NULL,
  sha256     TEXT NOT NULL,
  content    TEXT NOT NULL,
  fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (source_id, path)
);

-- ------------------------------------------------------------
-- Generic KV for small state (feature flags, cursors, etc.)
-- ------------------------------------------------------------

CREATE TABLE IF NOT EXISTS xcf.kv (
  namespace  TEXT NOT NULL,
  key        TEXT NOT NULL,
  value      JSONB NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (namespace, key)
);

COMMIT;
