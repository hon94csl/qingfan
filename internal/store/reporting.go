package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/hon94csl/qingfan/internal/reporting"
)

// BindPublication attaches one post to an existing output and leaves that file unchanged.
func (s *Store) BindPublication(outputID, url, videoID, shop, publishedAt string) (reporting.Publication, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM outputs WHERE id=?`, outputID).Scan(&n); err != nil {
		return reporting.Publication{}, err
	}
	pub, err := reporting.Bind(outputID, url, videoID, shop, publishedAt, n == 1)
	if err != nil {
		return reporting.Publication{}, err
	}
	id, err := newID()
	if err != nil {
		return reporting.Publication{}, err
	}
	created := nowText()
	_, err = s.db.Exec(`INSERT INTO publications(id, output_id, url, video_id, shop, published_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, pub.OutputID, pub.URL, pub.VideoID, pub.Shop, pub.PublishedAt, created)
	if err != nil {
		return reporting.Publication{}, err
	}
	pub.ID = id
	return pub, nil
}

// ListPublications returns every post bound to one output, oldest first.
func (s *Store) ListPublications(outputID string) ([]reporting.Publication, error) {
	rows, err := s.db.Query(`SELECT id, output_id, url, video_id, shop, published_at FROM publications WHERE output_id=? ORDER BY created_at, id`, outputID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pubs []reporting.Publication
	for rows.Next() {
		var pub reporting.Publication
		if err := rows.Scan(&pub.ID, &pub.OutputID, &pub.URL, &pub.VideoID, &pub.Shop, &pub.PublishedAt); err != nil {
			return nil, err
		}
		pubs = append(pubs, pub)
	}
	return pubs, rows.Err()
}

// PreviewMetrics checks a CSV without writing rows or a batch.
func (s *Store) PreviewMetrics(body []byte) (reporting.Preview, error) {
	return s.reviewMetrics(body, false)
}

// ImportMetrics writes new video-day rows. The same file hash is not applied twice.
func (s *Store) ImportMetrics(body []byte) (reporting.Preview, error) {
	return s.reviewMetrics(body, true)
}

func (s *Store) reviewMetrics(body []byte, write bool) (reporting.Preview, error) {
	preview, err := reporting.Inspect(body)
	if err != nil {
		return preview, err
	}
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	seen := false
	if write {
		seen, err = s.batchSeen(hash)
		if err != nil {
			return preview, err
		}
	}
	bound, err := s.boundVideos()
	if err != nil {
		return preview, err
	}
	existing, err := s.metricKeys()
	if err != nil {
		return preview, err
	}
	keep, extra, unbound := reporting.Classify(preview.Rows, bound, existing)
	preview.Errors = append(preview.Errors, extra...)
	preview.Unbound = unbound
	if seen {
		preview.Repeated = true
		preview.Accepted = 0
		preview.Rows = nil
		return preview, nil
	}
	preview.Accepted = len(keep)
	preview.Rows = keep
	if !write || len(keep) == 0 {
		return preview, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return preview, err
	}
	created := nowText()
	for _, row := range keep {
		id, err := newID()
		if err != nil {
			tx.Rollback()
			return preview, err
		}
		if _, err := tx.Exec(`INSERT INTO metrics_daily(id, source, shop, account, video_id, day, views, orders, gmv_minor, currency, timezone, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, row.Source, row.Shop, row.Account, row.VideoID, row.Date, metricValue(row.Views), metricValue(row.Orders), metricValue(row.GMVMinor), row.Currency, row.Timezone, created); err != nil {
			tx.Rollback()
			return preview, err
		}
	}
	for _, source := range sourcesOf(keep) {
		id, err := newID()
		if err != nil {
			tx.Rollback()
			return preview, err
		}
		if _, err := tx.Exec(`INSERT INTO import_batches(id, source, sha256, created_at) VALUES (?, ?, ?, ?)`, id, source, hash, created); err != nil {
			tx.Rollback()
			return preview, err
		}
	}
	if err := tx.Commit(); err != nil {
		return preview, err
	}
	return preview, nil
}

func (s *Store) batchSeen(hash string) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM import_batches WHERE sha256=?`, hash).Scan(&n)
	return n > 0, err
}

func (s *Store) boundVideos() (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT DISTINCT video_id FROM publications`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	bound := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		bound[id] = true
	}
	return bound, rows.Err()
}

func (s *Store) metricKeys() (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT source, shop, account, video_id, day FROM metrics_daily`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := map[string]bool{}
	for rows.Next() {
		var row reporting.Row
		if err := rows.Scan(&row.Source, &row.Shop, &row.Account, &row.VideoID, &row.Date); err != nil {
			return nil, err
		}
		keys[row.Key()] = true
	}
	return keys, rows.Err()
}

func sourcesOf(rows []reporting.Row) []string {
	var sources []string
	for _, row := range rows {
		found := false
		for _, source := range sources {
			if source == row.Source {
				found = true
				break
			}
		}
		if !found {
			sources = append(sources, row.Source)
		}
	}
	return sources
}

func metricValue(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

type metricState struct {
	Views  sql.NullInt64
	Orders sql.NullInt64
	GMV    sql.NullInt64
}

func (s *Store) metricByDay(source, videoID, day string) (metricState, error) {
	var state metricState
	err := s.db.QueryRow(`SELECT views, orders, gmv_minor FROM metrics_daily WHERE source=? AND video_id=? AND day=?`, source, videoID, day).Scan(&state.Views, &state.Orders, &state.GMV)
	if err != nil {
		return metricState{}, err
	}
	return state, nil
}

func (s *Store) countMetrics() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM metrics_daily`).Scan(&n)
	return n, err
}

func (s *Store) outputSnapshot(id string) (string, string, error) {
	var path, manifest string
	err := s.db.QueryRow(`SELECT path, manifest_json FROM outputs WHERE id=?`, id).Scan(&path, &manifest)
	if err != nil {
		return "", "", fmt.Errorf("%w", err)
	}
	return path, manifest, nil
}
