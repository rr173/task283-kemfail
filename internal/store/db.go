// Package store 提供 SQLite 持久化：建表迁移、CRUD 与统计查询。
// 使用纯 Go 驱动 modernc.org/sqlite，CGO 无关，离线可构建。
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB 封装 SQLite 连接与迁移状态。
type DB struct {
	sql *sql.DB
}

// Open 打开（必要时创建）SQLite 数据库文件并执行迁移。
func Open(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	sqlDB.SetMaxOpenConns(1) // SQLite 单写者
	db := &DB{sql: sqlDB}
	if err := db.migrate(); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return db, nil
}

// SQL 暴露底层连接（供事务与查询使用）。
func (d *DB) SQL() *sql.DB { return d.sql }

// Close 关闭数据库连接。
func (d *DB) Close() error { return d.sql.Close() }

// migrate 创建全部表结构。建表语句幂等（IF NOT EXISTS）。
func (d *DB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS run_batches (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'receiving',
			owner TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			sealed_at DATETIME
		);`,

		`CREATE TABLE IF NOT EXISTS param_sets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id INTEGER NOT NULL REFERENCES run_batches(id),
			name TEXT NOT NULL,
			kem_algorithm TEXT NOT NULL,
			version TEXT NOT NULL,
			parent_id INTEGER REFERENCES param_sets(id),
			params_digest TEXT NOT NULL,
			is_baseline INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL,
			UNIQUE(run_id, name)
		);`,

		`CREATE TABLE IF NOT EXISTS devices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id INTEGER NOT NULL REFERENCES run_batches(id),
			name TEXT NOT NULL,
			model TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			noise_level REAL NOT NULL DEFAULT 0,
			noise_events INTEGER NOT NULL DEFAULT 0,
			firmware TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			UNIQUE(run_id, name)
		);`,

		`CREATE TABLE IF NOT EXISTS failure_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id INTEGER NOT NULL REFERENCES run_batches(id),
			param_id INTEGER NOT NULL REFERENCES param_sets(id),
			device_id INTEGER NOT NULL REFERENCES devices(id),
			event_key TEXT NOT NULL UNIQUE,
			ciphertext_digest TEXT NOT NULL,
			failure_code TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'raw',
			received_at DATETIME NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_events_run ON failure_events(run_id);`,

		`CREATE TABLE IF NOT EXISTS source_candidates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id INTEGER NOT NULL REFERENCES run_batches(id),
			event_id INTEGER NOT NULL REFERENCES failure_events(id),
			kind TEXT NOT NULL,
			score REAL NOT NULL,
			evidence TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'candidate',
			created_at DATETIME NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_candidates_run ON source_candidates(run_id);`,
		`CREATE INDEX IF NOT EXISTS idx_candidates_event ON source_candidates(event_id);`,

		`CREATE TABLE IF NOT EXISTS diagnostic_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id INTEGER NOT NULL REFERENCES run_batches(id),
			title TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'draft',
			param_baseline INTEGER NOT NULL,
			candidate_ids TEXT NOT NULL DEFAULT '[]',
			created_at DATETIME NOT NULL,
			published_at DATETIME
		);`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_run ON diagnostic_snapshots(run_id);`,
	}

	for _, s := range stmts {
		if _, err := d.sql.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}
