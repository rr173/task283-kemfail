package store

import (
	"database/sql"

	"task283-kemfail/internal/model"
)

// GetStats 汇总某运行的统计摘要。
func (d *DB) GetStats(runID int64) (*model.Stats, error) {
	s := &model.Stats{RunID: runID}

	var total, pc, nr, dup, ex sql.NullInt64
	row := d.sql.QueryRow(
		`SELECT
			COUNT(*),
			SUM(CASE WHEN status='param_conflict' THEN 1 ELSE 0 END),
			SUM(CASE WHEN status='noise_related' THEN 1 ELSE 0 END),
			SUM(CASE WHEN status='duplicate' THEN 1 ELSE 0 END),
			SUM(CASE WHEN status='excluded' THEN 1 ELSE 0 END)
		 FROM failure_events WHERE run_id=?`, runID)
	if err := row.Scan(&total, &pc, &nr, &dup, &ex); err != nil {
		return nil, err
	}
	s.TotalEvents = int(total.Int64)
	s.ParamConflict = int(pc.Int64)
	s.NoiseRelated = int(nr.Int64)
	s.Duplicates = int(dup.Int64)
	s.Excluded = int(ex.Int64)

	var tc, cf, rj sql.NullInt64
	row = d.sql.QueryRow(
		`SELECT
			COUNT(*),
			SUM(CASE WHEN status='confirmed' THEN 1 ELSE 0 END),
			SUM(CASE WHEN status='rejected' THEN 1 ELSE 0 END)
		 FROM source_candidates WHERE run_id=?`, runID)
	if err := row.Scan(&tc, &cf, &rj); err != nil {
		return nil, err
	}
	s.TotalCandidates = int(tc.Int64)
	s.Confirmed = int(cf.Int64)
	s.Rejected = int(rj.Int64)

	var iso sql.NullInt64
	row = d.sql.QueryRow(
		`SELECT COUNT(*) FROM devices WHERE run_id=? AND status='isolated'`, runID)
	if err := row.Scan(&iso); err != nil {
		return nil, err
	}
	s.IsolatedDevices = int(iso.Int64)

	var pub sql.NullInt64
	row = d.sql.QueryRow(
		`SELECT COUNT(*) FROM diagnostic_snapshots WHERE run_id=? AND status='published'`, runID)
	if err := row.Scan(&pub); err != nil {
		return nil, err
	}
	s.PublishedSnapshot = int(pub.Int64)
	return s, nil
}
