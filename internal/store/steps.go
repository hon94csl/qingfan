package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/hon94csl/qingfan/internal/jobs"
)

// PutStep records one step. Succeeded requires a non-empty artifact on disk.
func (s *Store) PutStep(jobID, key, status, artifact, message string) error {
	if key == "" || status == "" {
		return fmt.Errorf("步骤缺少标识或状态")
	}
	if status == jobs.Succeeded && !jobs.FileReady(artifact) {
		return fmt.Errorf("步骤还没有可用产物，不能标成成功")
	}
	now := nowText()
	var id string
	err := s.db.QueryRow(`SELECT id FROM job_steps WHERE job_id=? AND step_key=?`, jobID, key).Scan(&id)
	if err == nil {
		_, err = s.db.Exec(`UPDATE job_steps SET status=?, artifact=?, error=?, updated_at=? WHERE id=?`, status, artifact, message, now, id)
		return err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	id, err = newID()
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO job_steps(id, job_id, step_key, status, artifact, error, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, jobID, key, status, artifact, message, now, now)
	return err
}

// Steps lists the recorded steps for one job, oldest first.
func (s *Store) Steps(jobID string) ([]jobs.Step, error) {
	rows, err := s.db.Query(`SELECT id, step_key, status, artifact, error FROM job_steps WHERE job_id=? ORDER BY created_at, id`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var steps []jobs.Step
	for rows.Next() {
		var step jobs.Step
		var message string
		if err := rows.Scan(&step.ID, &step.Key, &step.Status, &step.Artifact, &message); err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

// RecoverJob rewrites interrupted steps after a restart.
// A stored progress value is not kept, and it cannot make a step successful.
func (s *Store) RecoverJob(jobID string) ([]jobs.Step, error) {
	steps, err := s.Steps(jobID)
	if err != nil {
		return nil, err
	}
	next := jobs.Recover(steps, jobs.FileReady)
	for i, step := range next {
		if step.Status == steps[i].Status && step.Artifact == steps[i].Artifact {
			continue
		}
		if _, err := s.db.Exec(`UPDATE job_steps SET status=?, artifact=?, updated_at=? WHERE id=?`, step.Status, step.Artifact, nowText(), step.ID); err != nil {
			return nil, err
		}
	}
	return next, nil
}
