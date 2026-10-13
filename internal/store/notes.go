package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hon94csl/qingfan/internal/ai"
)

func recipeHook(value string) string {
	if value == "" {
		return "manual"
	}
	return value
}

func recipeVersion(value string) string {
	if value == "" {
		return "workbench-m2"
	}
	return value
}

// GetSKU reads one product.
func (s *Store) GetSKU(id string) (SKU, error) {
	var sku SKU
	err := s.db.QueryRow(`SELECT id, project_id, name, facts FROM skus WHERE id=?`, id).Scan(&sku.ID, &sku.ProjectID, &sku.Name, &sku.Facts)
	if errors.Is(err, sql.ErrNoRows) {
		return SKU{}, fmt.Errorf("商品不存在")
	}
	if err != nil {
		return SKU{}, err
	}
	return sku, nil
}

// UpdateFacts replaces the confirmed product facts.
func (s *Store) UpdateFacts(id, facts string) error {
	res, err := s.db.Exec(`UPDATE skus SET facts=? WHERE id=?`, facts, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("商品不存在")
	}
	return nil
}

// SaveNote stores an operator-corrected shot description.
func (s *Store) SaveNote(skuID string, note ai.ShotNote) error {
	points := note.SellingPoints
	if points == nil {
		points = []string{}
	}
	encoded, err := json.Marshal(points)
	if err != nil {
		return err
	}
	manual := 0
	if note.Manual {
		manual = 1
	}
	_, err = s.db.Exec(`INSERT INTO clip_notes(clip_id, sku_id, subject, action, purpose, selling_points, source_in_ms, source_out_ms, manual, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(clip_id) DO UPDATE SET subject=excluded.subject, action=excluded.action, purpose=excluded.purpose, selling_points=excluded.selling_points, source_in_ms=excluded.source_in_ms, source_out_ms=excluded.source_out_ms, manual=excluded.manual, updated_at=excluded.updated_at`,
		note.ClipID, skuID, note.Subject, note.Action, note.Purpose, string(encoded), note.SourceInMS, note.SourceOutMS, manual, nowText())
	return err
}

// ListNotes returns corrected descriptions for one product.
func (s *Store) ListNotes(skuID string) ([]ai.ShotNote, error) {
	rows, err := s.db.Query(`SELECT clip_id, subject, action, purpose, selling_points, source_in_ms, source_out_ms, manual FROM clip_notes WHERE sku_id=? ORDER BY clip_id`, skuID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var notes []ai.ShotNote
	for rows.Next() {
		var note ai.ShotNote
		var points string
		var manual int
		if err := rows.Scan(&note.ClipID, &note.Subject, &note.Action, &note.Purpose, &points, &note.SourceInMS, &note.SourceOutMS, &manual); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(points), &note.SellingPoints); err != nil {
			return nil, err
		}
		note.Manual = manual == 1
		notes = append(notes, note)
	}
	return notes, rows.Err()
}
