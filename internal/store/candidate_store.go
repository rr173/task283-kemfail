package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"task283-kemfail/internal/model"
)

// CreateCandidate 创建来源候选。
func (d *DB) CreateCandidate(c *model.SourceCandidate) (*model.SourceCandidate, error) {
	now := time.Now().UTC()
	res, err := d.sql.Exec(
		`INSERT INTO source_candidates (run_id, event_id, kind, score, evidence, status, created_at)
		 VALUES (?, ?, ?, ?, ?, 'candidate', ?)`,
		c.RunID, c.EventID, c.Kind, c.Score, c.Evidence, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create candidate: %w", err)
	}
	c.ID, _ = res.LastInsertId()
	c.Status = model.CandOpen
	c.CreatedAt = now
	return c, nil
}

// GetCandidate 按 ID 读取来源候选。
func (d *DB) GetCandidate(id int64) (*model.SourceCandidate, error) {
	row := d.sql.QueryRow(
		`SELECT id, run_id, event_id, kind, score, evidence, status, created_at
		 FROM source_candidates WHERE id = ?`, id)
	var c model.SourceCandidate
	if err := row.Scan(&c.ID, &c.RunID, &c.EventID, &c.Kind, &c.Score, &c.Evidence, &c.Status, &c.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrCandidateNotFound
		}
		return nil, err
	}
	return &c, nil
}

// ListCandidates 列出某运行的来源候选（按分数倒序）。
func (d *DB) ListCandidates(runID int64) ([]model.SourceCandidate, error) {
	rows, err := d.sql.Query(
		`SELECT id, run_id, event_id, kind, score, evidence, status, created_at
		 FROM source_candidates WHERE run_id = ? ORDER BY score DESC`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.SourceCandidate
	for rows.Next() {
		var c model.SourceCandidate
		if err := rows.Scan(&c.ID, &c.RunID, &c.EventID, &c.Kind, &c.Score, &c.Evidence, &c.Status, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListCandidatesByEvent 列出某事件的全部候选。
func (d *DB) ListCandidatesByEvent(eventID int64) ([]model.SourceCandidate, error) {
	rows, err := d.sql.Query(
		`SELECT id, run_id, event_id, kind, score, evidence, status, created_at
		 FROM source_candidates WHERE event_id = ? ORDER BY score DESC`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.SourceCandidate
	for rows.Next() {
		var c model.SourceCandidate
		if err := rows.Scan(&c.ID, &c.RunID, &c.EventID, &c.Kind, &c.Score, &c.Evidence, &c.Status, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetCandidateStatus 确认/否决候选。
func (d *DB) SetCandidateStatus(id int64, status model.CandidateStatus) error {
	if _, err := d.GetCandidate(id); err != nil {
		return err
	}
	_, err := d.sql.Exec(`UPDATE source_candidates SET status=? WHERE id=?`, status, id)
	return err
}

// CountCandidates 统计某运行的候选（按状态）。
func (d *DB) CountCandidates(runID int64, status model.CandidateStatus) (int, error) {
	var n int
	err := d.sql.QueryRow(
		`SELECT COUNT(*) FROM source_candidates WHERE run_id=? AND status=?`, runID, status).Scan(&n)
	return n, err
}

// ClearCandidates 清除某运行的全部来源候选（全量归因重跑前调用）。
func (d *DB) ClearCandidates(runID int64) error {
	_, err := d.sql.Exec(`DELETE FROM source_candidates WHERE run_id=?`, runID)
	return err
}

// CreateSnapshot 创建诊断快照（草稿），candidate_ids 序列化为 JSON。
func (d *DB) CreateSnapshot(s *model.DiagnosticSnapshot) (*model.DiagnosticSnapshot, error) {
	now := time.Now().UTC()
	ids, err := json.Marshal(s.CandidateIDs)
	if err != nil {
		return nil, err
	}
	res, err := d.sql.Exec(
		`INSERT INTO diagnostic_snapshots (run_id, title, status, param_baseline, candidate_ids, created_at)
		 VALUES (?, ?, 'draft', ?, ?, ?)`,
		s.RunID, s.Title, s.ParamBaseline, string(ids), now,
	)
	if err != nil {
		return nil, fmt.Errorf("create snapshot: %w", err)
	}
	s.ID, _ = res.LastInsertId()
	s.Status = model.SnapDraft
	s.CreatedAt = now
	return s, nil
}

// GetSnapshot 按 ID 读取诊断快照。
func (d *DB) GetSnapshot(id int64) (*model.DiagnosticSnapshot, error) {
	row := d.sql.QueryRow(
		`SELECT id, run_id, title, status, param_baseline, candidate_ids, created_at, published_at
		 FROM diagnostic_snapshots WHERE id = ?`, id)
	var s model.DiagnosticSnapshot
	var ids string
	var pub sql.NullTime
	if err := row.Scan(&s.ID, &s.RunID, &s.Title, &s.Status, &s.ParamBaseline, &ids, &s.CreatedAt, &pub); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrSnapshotNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal([]byte(ids), &s.CandidateIDs); err != nil {
		return nil, err
	}
	if pub.Valid {
		t := pub.Time
		s.PublishedAt = &t
	}
	return &s, nil
}

// ListSnapshots 列出某运行的诊断快照。
func (d *DB) ListSnapshots(runID int64) ([]model.DiagnosticSnapshot, error) {
	rows, err := d.sql.Query(
		`SELECT id, run_id, title, status, param_baseline, candidate_ids, created_at, published_at
		 FROM diagnostic_snapshots WHERE run_id = ? ORDER BY id DESC`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.DiagnosticSnapshot
	for rows.Next() {
		var s model.DiagnosticSnapshot
		var ids string
		var pub sql.NullTime
		if err := rows.Scan(&s.ID, &s.RunID, &s.Title, &s.Status, &s.ParamBaseline, &ids, &s.CreatedAt, &pub); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(ids), &s.CandidateIDs); err != nil {
			return nil, err
		}
		if pub.Valid {
			t := pub.Time
			s.PublishedAt = &t
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// PublishSnapshot 发布快照：置 published + 发布时间，并同步运行状态为已发布。
func (d *DB) PublishSnapshot(id int64) (*model.DiagnosticSnapshot, error) {
	s, err := d.GetSnapshot(id)
	if err != nil {
		return nil, err
	}
	if s.Status == model.SnapPublished || s.Status == model.SnapSuperseded {
		return nil, model.ErrSnapshotFrozen
	}
	now := time.Now().UTC()
	tx, err := d.sql.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`UPDATE diagnostic_snapshots SET status='published', published_at=? WHERE id=?`, now, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(
		`UPDATE run_batches SET status='published', updated_at=? WHERE id=?`, now, s.RunID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	s.Status = model.SnapPublished
	s.PublishedAt = &now
	return s, nil
}

// SupersedeSnapshot 替代快照：旧快照置 superseded，新快照置 published。
func (d *DB) SupersedeSnapshot(oldID, newID int64) (*model.DiagnosticSnapshot, error) {
	oldS, err := d.GetSnapshot(oldID)
	if err != nil {
		return nil, err
	}
	newS, err := d.GetSnapshot(newID)
	if err != nil {
		return nil, err
	}
	if oldS.RunID != newS.RunID {
		return nil, model.ErrInvalidState
	}
	if oldS.Status != model.SnapPublished {
		return nil, model.ErrSnapshotFrozen
	}
	if newS.Status != model.SnapDraft {
		return nil, model.ErrInvalidState
	}
	now := time.Now().UTC()
	tx, err := d.sql.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`UPDATE diagnostic_snapshots SET status='superseded' WHERE id=?`, oldID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(
		`UPDATE diagnostic_snapshots SET status='published', published_at=? WHERE id=?`, now, newID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	oldS.Status = model.SnapSuperseded
	newS.Status = model.SnapPublished
	newS.PublishedAt = &now
	return newS, nil
}
