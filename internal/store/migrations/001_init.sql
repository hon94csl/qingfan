CREATE TABLE projects (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL DEFAULT 'local',
  name TEXT NOT NULL,
  market TEXT NOT NULL,
  locale TEXT NOT NULL
);

CREATE TABLE skus (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES projects(id),
  name TEXT NOT NULL,
  facts TEXT NOT NULL DEFAULT ''
);

CREATE TABLE content_probes (
  sha256 TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  duration_ms INTEGER NOT NULL,
  width INTEGER NOT NULL DEFAULT 0,
  height INTEGER NOT NULL DEFAULT 0,
  has_video INTEGER NOT NULL,
  has_audio INTEGER NOT NULL,
  rotation INTEGER NOT NULL DEFAULT 0,
  variable_frame_rate INTEGER NOT NULL DEFAULT 0,
  thumbnail_path TEXT NOT NULL DEFAULT '',
  cuts_json TEXT NOT NULL
);

CREATE TABLE assets (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL DEFAULT 'local',
  sku_id TEXT NOT NULL REFERENCES skus(id),
  kind TEXT NOT NULL,
  source TEXT NOT NULL,
  rights TEXT NOT NULL,
  sha256 TEXT NOT NULL,
  analysis_valid INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE asset_locations (
  id TEXT PRIMARY KEY,
  asset_id TEXT NOT NULL REFERENCES assets(id),
  path TEXT NOT NULL UNIQUE,
  sha256 TEXT NOT NULL
);

CREATE TABLE clips (
  id TEXT PRIMARY KEY,
  asset_id TEXT NOT NULL REFERENCES assets(id),
  sku_id TEXT NOT NULL,
  source_in_ms INTEGER NOT NULL,
  source_out_ms INTEGER NOT NULL,
  origin TEXT NOT NULL,
  valid INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE drafts (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES projects(id),
  sku_id TEXT NOT NULL REFERENCES skus(id),
  revision INTEGER NOT NULL,
  market TEXT NOT NULL,
  locale TEXT NOT NULL,
  font_asset_id TEXT NOT NULL DEFAULT '',
  bgm_asset_id TEXT NOT NULL DEFAULT '',
  bgm_gain_db REAL NOT NULL DEFAULT -18,
  body_json TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE outputs (
  id TEXT PRIMARY KEY,
  draft_id TEXT NOT NULL REFERENCES drafts(id),
  draft_revision INTEGER NOT NULL,
  path TEXT NOT NULL UNIQUE,
  manifest_json TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE jobs (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  status TEXT NOT NULL,
  error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
