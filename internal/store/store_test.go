package store

import (
	"path/filepath"
	"strings"
	"testing"

	"task283-kemfail/internal/model"
)

// newTestDB 创建临时数据库。
func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRunLifecycle(t *testing.T) {
	db := newTestDB(t)
	run, err := db.CreateRun("r1", "owner")
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != model.RunReceiving {
		t.Errorf("initial status = %s", run.Status)
	}
	got, err := db.GetRun(run.ID)
	if err != nil || got.Name != "r1" {
		t.Fatalf("get run: %v %+v", err, got)
	}
	if err := db.SetRunStatus(run.ID, model.RunSealed); err != nil {
		t.Fatal(err)
	}
	if err := db.SetRunStatus(run.ID, model.RunPending); err == nil {
		t.Error("sealed run should reject writes")
	}
	if _, err := db.GetRun(9999); err != model.ErrRunNotFound {
		t.Errorf("missing run should return ErrRunNotFound, got %v", err)
	}
}

func TestEventIdempotency(t *testing.T) {
	db := newTestDB(t)
	run, _ := db.CreateRun("r", "o")
	p, _ := db.CreateParamSet(&model.ParamSet{RunID: run.ID, Name: "p", KEMAlgorithm: "ML-KEM", Version: "1.0.0", ParamsDigest: "dd"})
	dev, _ := db.CreateDevice(&model.Device{RunID: run.ID, Name: "d", Model: "m", NoiseLevel: 0.1, Firmware: "f"})

	e := &model.FailureEvent{RunID: run.ID, ParamID: p.ID, DeviceID: dev.ID, EventKey: "key-1", CiphertextDigest: strings.Repeat("ab", 32), FailureCode: "err"}
	created, err := db.CreateEvent(e)
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != model.EventRaw {
		t.Errorf("initial event status = %s", created.Status)
	}
	// 幂等：同 key 拒绝
	dup := &model.FailureEvent{RunID: run.ID, ParamID: p.ID, DeviceID: dev.ID, EventKey: "key-1", CiphertextDigest: strings.Repeat("ab", 32), FailureCode: "err"}
	if _, err := db.CreateEvent(dup); err == nil {
		t.Error("duplicate key should be rejected")
	}
	got, err := db.GetEventByKey("key-1")
	if err != nil || got.ID != created.ID {
		t.Errorf("get by key: %v %+v", err, got)
	}
}

func TestParamBaselineExclusive(t *testing.T) {
	db := newTestDB(t)
	run, _ := db.CreateRun("r", "o")
	p1, _ := db.CreateParamSet(&model.ParamSet{RunID: run.ID, Name: "p1", KEMAlgorithm: "ML-KEM", Version: "1.0.0", ParamsDigest: "d1", IsBaseline: true})
	p2, _ := db.CreateParamSet(&model.ParamSet{RunID: run.ID, Name: "p2", KEMAlgorithm: "ML-KEM", Version: "1.1.0", ParamsDigest: "d2"})
	if err := db.MarkParamBaseline(p2.ID); err != nil {
		t.Fatal(err)
	}
	got1, _ := db.GetParamSet(p1.ID)
	got2, _ := db.GetParamSet(p2.ID)
	if got1.IsBaseline {
		t.Error("p1 should no longer be baseline")
	}
	if !got2.IsBaseline {
		t.Error("p2 should be baseline")
	}
}

func TestSnapshotLifecycle(t *testing.T) {
	db := newTestDB(t)
	run, _ := db.CreateRun("r", "o")
	ss, err := db.CreateSnapshot(&model.DiagnosticSnapshot{
		RunID: run.ID, Title: "s", ParamBaseline: 1, CandidateIDs: []int64{1, 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ss.Status != model.SnapDraft {
		t.Errorf("initial snapshot status = %s", ss.Status)
	}
	pub, err := db.PublishSnapshot(ss.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pub.Status != model.SnapPublished {
		t.Errorf("published status = %s", pub.Status)
	}
	// 已发布再发布拒绝
	if _, err := db.PublishSnapshot(ss.ID); err == nil {
		t.Error("published snapshot should reject re-publish")
	}
	// 运行状态同步
	run2, _ := db.GetRun(run.ID)
	if run2.Status != model.RunPublished {
		t.Errorf("run status should sync to published, got %s", run2.Status)
	}
	// 替代
	ss2, _ := db.CreateSnapshot(&model.DiagnosticSnapshot{
		RunID: run.ID, Title: "s2", ParamBaseline: 1, CandidateIDs: []int64{3},
	})
	if _, err := db.SupersedeSnapshot(ss.ID, ss2.ID); err != nil {
		t.Fatal(err)
	}
	oldS, _ := db.GetSnapshot(ss.ID)
	if oldS.Status != model.SnapSuperseded {
		t.Errorf("old snapshot should be superseded, got %s", oldS.Status)
	}
}

func TestDeviceNoiseMerge(t *testing.T) {
	db := newTestDB(t)
	run, _ := db.CreateRun("r", "o")
	dev, _ := db.CreateDevice(&model.Device{RunID: run.ID, Name: "d", Model: "m", NoiseLevel: 0.2, Firmware: "f"})
	got, err := db.RecordDeviceNoise(dev.ID, 0.8)
	if err != nil {
		t.Fatal(err)
	}
	// 0.7*0.2 + 0.3*0.8 = 0.38
	if got.NoiseLevel < 0.37 || got.NoiseLevel > 0.39 {
		t.Errorf("merged noise = %f, want ~0.38", got.NoiseLevel)
	}
	if got.NoiseEvents != 1 {
		t.Errorf("noise events = %d, want 1", got.NoiseEvents)
	}
}
