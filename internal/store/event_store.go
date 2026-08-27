package store

import (
	"database/sql"
	"fmt"
	"time"

	"task283-kemfail/internal/model"
)

// CreateEvent 写入失败事件；event_key 唯一（幂等），重复返回 ErrDuplicateEvent。
func (d *DB) CreateEvent(e *model.FailureEvent) (*model.FailureEvent, error) {
	now := time.Now().UTC()
	res, err := d.sql.Exec(
		`INSERT INTO failure_events (run_id, param_id, device_id, event_key, ciphertext_digest, failure_code, status, received_at)
		 VALUES (?, ?, ?, ?, ?, ?, 'raw', ?)`,
		e.RunID, e.ParamID, e.DeviceID, e.EventKey, e.CiphertextDigest, e.FailureCode, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create event: %w", err)
	}
	e.ID, _ = res.LastInsertId()
	e.Status = model.EventRaw
	e.ReceivedAt = now
	return e, nil
}

// GetEvent 按 ID 读取失败事件。
func (d *DB) GetEvent(id int64) (*model.FailureEvent, error) {
	row := d.sql.QueryRow(
		`SELECT id, run_id, param_id, device_id, event_key, ciphertext_digest, failure_code, status, received_at
		 FROM failure_events WHERE id = ?`, id)
	var e model.FailureEvent
	if err := row.Scan(&e.ID, &e.RunID, &e.ParamID, &e.DeviceID, &e.EventKey,
		&e.CiphertextDigest, &e.FailureCode, &e.Status, &e.ReceivedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrEventNotFound
		}
		return nil, err
	}
	return &e, nil
}

// GetEventByKey 按幂等键读取失败事件。
func (d *DB) GetEventByKey(key string) (*model.FailureEvent, error) {
	row := d.sql.QueryRow(
		`SELECT id, run_id, param_id, device_id, event_key, ciphertext_digest, failure_code, status, received_at
		 FROM failure_events WHERE event_key = ?`, key)
	var e model.FailureEvent
	if err := row.Scan(&e.ID, &e.RunID, &e.ParamID, &e.DeviceID, &e.EventKey,
		&e.CiphertextDigest, &e.FailureCode, &e.Status, &e.ReceivedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrEventNotFound
		}
		return nil, err
	}
	return &e, nil
}

// ListEvents 列出某运行的失败事件（按接收时间倒序）。
func (d *DB) ListEvents(runID int64) ([]model.FailureEvent, error) {
	rows, err := d.sql.Query(
		`SELECT id, run_id, param_id, device_id, event_key, ciphertext_digest, failure_code, status, received_at
		 FROM failure_events WHERE run_id = ? ORDER BY id DESC`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.FailureEvent
	for rows.Next() {
		var e model.FailureEvent
		if err := rows.Scan(&e.ID, &e.RunID, &e.ParamID, &e.DeviceID, &e.EventKey,
			&e.CiphertextDigest, &e.FailureCode, &e.Status, &e.ReceivedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// SetEventStatus 更新事件状态。排除后禁止再参与归因（上层约束）。
func (d *DB) SetEventStatus(id int64, status model.EventStatus) error {
	e, err := d.GetEvent(id)
	if err != nil {
		return err
	}
	if e.Status == model.EventExcluded {
		return model.ErrEventExcluded
	}
	_, err = d.sql.Exec(`UPDATE failure_events SET status=? WHERE id=?`, status, id)
	return err
}
