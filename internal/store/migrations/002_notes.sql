CREATE TABLE clip_notes (
  clip_id TEXT PRIMARY KEY REFERENCES clips(id),
  sku_id TEXT NOT NULL,
  subject TEXT NOT NULL,
  action TEXT NOT NULL,
  purpose TEXT NOT NULL,
  selling_points TEXT NOT NULL,
  source_in_ms INTEGER NOT NULL,
  source_out_ms INTEGER NOT NULL,
  manual INTEGER NOT NULL,
  updated_at TEXT NOT NULL
);
