package store

import (
	"database/sql"
	"fmt"
	"time"

	"task283-kemfail/internal/model"
)

// CreateDevice 创建设备（活跃）。
func (d *DB) CreateDevice(dev *model.Device) (*model.Device, error) {
	now := time.Now().UTC()
	res, err := d.sql.Exec(
		`INSERT INTO devices (run_id, name, model, status, noise_level, noise_events, firmware, created_at)
		 VALUES (?, ?, ?, 'active', ?, ?, ?, ?)`,
		dev.RunID, dev.Name, dev.Model, dev.NoiseLevel, dev.NoiseEvents, dev.Firmware, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create device: %w", err)
	}
	dev.ID, _ = res.LastInsertId()
	dev.Status = model.DevActive
	dev.CreatedAt = now
	return dev, nil
}

// GetDevice 按 ID 读取设备。
func (d *DB) GetDevice(id int64) (*model.Device, error) {
	row := d.sql.QueryRow(
		`SELECT id, run_id, name, model, status, noise_level, noise_events, firmware, created_at
		 FROM devices WHERE id = ?`, id)
	var dev model.Device
	if err := row.Scan(&dev.ID, &dev.RunID, &dev.Name, &dev.Model, &dev.Status,
		&dev.NoiseLevel, &dev.NoiseEvents, &dev.Firmware, &dev.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrDeviceNotFound
		}
		return nil, err
	}
	return &dev, nil
}

// ListDevices 列出某运行的全部设备。
func (d *DB) ListDevices(runID int64) ([]model.Device, error) {
	rows, err := d.sql.Query(
		`SELECT id, run_id, name, model, status, noise_level, noise_events, firmware, created_at
		 FROM devices WHERE run_id = ? ORDER BY id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Device
	for rows.Next() {
		var dev model.Device
		if err := rows.Scan(&dev.ID, &dev.RunID, &dev.Name, &dev.Model, &dev.Status,
			&dev.NoiseLevel, &dev.NoiseEvents, &dev.Firmware, &dev.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, dev)
	}
	return out, rows.Err()
}

// SetDeviceStatus 更新设备状态。活跃→隔离任意时刻允许；隔离→活跃视为恢复。
// 通过状态参数显式传递，禁止直接倒退（由上层 device 包约束）。
func (d *DB) SetDeviceStatus(id int64, status model.DeviceStatus) error {
	dev, err := d.GetDevice(id)
	if err != nil {
		return err
	}
	if dev.Status == status {
		return nil
	}
	if dev.Status == model.DevIsolated && status == model.DevActive {
		// 恢复时清零噪声计数（新的观测基线）。
		_, err = d.sql.Exec(`UPDATE devices SET status=?, noise_events=0 WHERE id=?`, status, id)
		return err
	}
	_, err = d.sql.Exec(`UPDATE devices SET status=? WHERE id=?`, status, id)
	return err
}

// RecordDeviceNoise 追加噪声观测：更新噪声强度（指数滑动平均）并递增计数。
func (d *DB) RecordDeviceNoise(id int64, level float64) (*model.Device, error) {
	dev, err := d.GetDevice(id)
	if err != nil {
		return nil, err
	}
	// 指数滑动平均：new = 0.7*old + 0.3*level
	merged := 0.7*dev.NoiseLevel + 0.3*level
	if _, err := d.sql.Exec(
		`UPDATE devices SET noise_level=?, noise_events=noise_events+1 WHERE id=?`,
		merged, id); err != nil {
		return nil, err
	}
	dev.NoiseLevel = merged
	dev.NoiseEvents++
	return dev, nil
}
