CREATE TABLE job_steps (
  id TEXT PRIMARY KEY,
  job_id TEXT NOT NULL DEFAULT '',
  step_key TEXT NOT NULL,
  status TEXT NOT NULL,
  artifact TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(job_id, step_key)
);

CREATE TABLE publications (
  id TEXT PRIMARY KEY,
  output_id TEXT NOT NULL REFERENCES outputs(id),
  url TEXT NOT NULL DEFAULT '',
  video_id TEXT NOT NULL,
  shop TEXT NOT NULL,
  published_at TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE import_batches (
  id TEXT PRIMARY KEY,
  source TEXT NOT NULL,
  sha256 TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(source, sha256)
);

CREATE TABLE metrics_daily (
  id TEXT PRIMARY KEY,
  source TEXT NOT NULL,
  shop TEXT NOT NULL,
  account TEXT NOT NULL DEFAULT '',
  video_id TEXT NOT NULL,
  day TEXT NOT NULL,
  views INTEGER,
  orders INTEGER,
  gmv_minor INTEGER,
  currency TEXT NOT NULL,
  timezone TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(source, shop, account, video_id, day)
);
