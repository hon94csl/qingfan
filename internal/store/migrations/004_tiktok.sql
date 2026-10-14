CREATE TABLE tiktok_oauth_states (
  state TEXT PRIMARY KEY,
  expires_at INTEGER NOT NULL
);

CREATE TABLE tiktok_shops (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  region TEXT NOT NULL,
  seller_type TEXT NOT NULL,
  cipher TEXT NOT NULL,
  code TEXT NOT NULL,
  open_id TEXT NOT NULL,
  seller_name TEXT NOT NULL,
  access_token TEXT NOT NULL,
  refresh_token TEXT NOT NULL,
  access_expire INTEGER NOT NULL,
  refresh_expire INTEGER NOT NULL,
  scopes TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE tiktok_video_facts (
  shop_id TEXT NOT NULL REFERENCES tiktok_shops(id),
  video_id TEXT NOT NULL,
  title TEXT NOT NULL,
  account TEXT NOT NULL,
  start_date TEXT NOT NULL,
  end_date TEXT NOT NULL,
  views INTEGER,
  orders INTEGER,
  gmv_minor INTEGER,
  currency TEXT NOT NULL,
  latest_available TEXT NOT NULL,
  fetched_at TEXT NOT NULL,
  PRIMARY KEY (shop_id, video_id, start_date, end_date)
);
