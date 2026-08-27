package store

import (
	"database/sql"
	"fmt"
	"time"

	"task283-kemfail/internal/model"
)

// CreateRun 创建运行批次（接收中）。
func (d *DB) CreateRun(name, owner string) (*model.RunBatch, error) {
	now := time.Now().UTC()
	res, err := d.sql.Exec(
		`INSERT INTO run_batches (name, status, owner, created_at, updated_at)
		 VALUES (?, 'receiving', ?, ?, ?)`,
		name, owner, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create run: %w", err)
	}
	id, _ := res.LastInsertId()
	return &model.RunBatch{
		ID: id, Name: name, Status: model.RunReceiving,
		Owner: owner, CreatedAt: now, UpdatedAt: now,
	}, nil
}

// GetRun 按 ID 读取运行批次；不存在返回 ErrRunNotFound。
func (d *DB) GetRun(id int64) (*model.RunBatch, error) {
	row := d.sql.QueryRow(
		`SELECT id, name, status, owner, created_at, updated_at, sealed_at
		 FROM run_batches WHERE id = ?`, id)
	var r model.RunBatch
	var sealed sql.NullTime
	if err := row.Scan(&r.ID, &r.Name, &r.Status, &r.Owner, &r.CreatedAt, &r.UpdatedAt, &sealed); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrRunNotFound
		}
		return nil, err
	}
	if sealed.Valid {
		t := sealed.Time
		r.SealedAt = &t
	}
	return &r, nil
}

// ListRuns 列出运行批次（按创建时间倒序）。
func (d *DB) ListRuns() ([]model.RunBatch, error) {
	rows, err := d.sql.Query(
		`SELECT id, name, status, owner, created_at, updated_at, sealed_at
		 FROM run_batches ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.RunBatch
	for rows.Next() {
		var r model.RunBatch
		var sealed sql.NullTime
		if err := rows.Scan(&r.ID, &r.Name, &r.Status, &r.Owner, &r.CreatedAt, &r.UpdatedAt, &sealed); err != nil {
			return nil, err
		}
		if sealed.Valid {
			t := sealed.Time
			r.SealedAt = &t
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SetRunStatus 更新运行状态（校验封存不可改）。
func (d *DB) SetRunStatus(id int64, status model.RunStatus) error {
	run, err := d.GetRun(id)
	if err != nil {
		return err
	}
	if run.Status == model.RunSealed {
		return model.ErrRunSealed
	}
	now := time.Now().UTC()
	if status == model.RunSealed {
		_, err = d.sql.Exec(
			`UPDATE run_batches SET status=?, updated_at=?, sealed_at=? WHERE id=?`,
			status, now, now, id)
	} else {
		_, err = d.sql.Exec(
			`UPDATE run_batches SET status=?, updated_at=? WHERE id=?`, status, now, id)
	}
	return err
}

// CreateParamSet 创建参数集；谱系不变量（无环、版本递增、基线唯一）由上层校验。
func (d *DB) CreateParamSet(p *model.ParamSet) (*model.ParamSet, error) {
	now := time.Now().UTC()
	res, err := d.sql.Exec(
		`INSERT INTO param_sets (run_id, name, kem_algorithm, version, parent_id, params_digest, is_baseline, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.RunID, p.Name, p.KEMAlgorithm, p.Version, p.ParentID, p.ParamsDigest, boolInt(p.IsBaseline), now,
	)
	if err != nil {
		return nil, fmt.Errorf("create param set: %w", err)
	}
	p.ID, _ = res.LastInsertId()
	p.CreatedAt = now
	return p, nil
}

// GetParamSet 按 ID 读取参数集。
func (d *DB) GetParamSet(id int64) (*model.ParamSet, error) {
	row := d.sql.QueryRow(
		`SELECT id, run_id, name, kem_algorithm, version, parent_id, params_digest, is_baseline, created_at
		 FROM param_sets WHERE id = ?`, id)
	var p model.ParamSet
	var parent sql.NullInt64
	if err := row.Scan(&p.ID, &p.RunID, &p.Name, &p.KEMAlgorithm, &p.Version, &parent, &p.ParamsDigest, &p.IsBaseline, &p.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrParamNotFound
		}
		return nil, err
	}
	if parent.Valid {
		v := parent.Int64
		p.ParentID = &v
	}
	return &p, nil
}

// ListParamSets 列出某运行的参数集。
func (d *DB) ListParamSets(runID int64) ([]model.ParamSet, error) {
	rows, err := d.sql.Query(
		`SELECT id, run_id, name, kem_algorithm, version, parent_id, params_digest, is_baseline, created_at
		 FROM param_sets WHERE run_id = ? ORDER BY id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ParamSet
	for rows.Next() {
		var p model.ParamSet
		var parent sql.NullInt64
		if err := rows.Scan(&p.ID, &p.RunID, &p.Name, &p.KEMAlgorithm, &p.Version, &parent, &p.ParamsDigest, &p.IsBaseline, &p.CreatedAt); err != nil {
			return nil, err
		}
		if parent.Valid {
			v := parent.Int64
			p.ParentID = &v
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// MarkParamBaseline 将指定参数集标记为基线。
func (d *DB) MarkParamBaseline(id int64) error {
	p, err := d.GetParamSet(id)
	if err != nil {
		return err
	}
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE param_sets SET is_baseline=0 WHERE run_id=?`, p.RunID); err != nil {
		return err
	}
	time.Sleep(3 * time.Millisecond)
	if _, err := tx.Exec(`UPDATE param_sets SET is_baseline=1 WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
