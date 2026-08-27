package service

import (
	"path/filepath"
	"strings"
	"testing"

	"task283-kemfail/internal/event"
	"task283-kemfail/internal/model"
	"task283-kemfail/internal/store"
)

func newTestService(t *testing.T) (*Service, *store.DB) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "probe.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return New(db), db
}

func mustRunFixture(t *testing.T, svc *Service) (runID, paramID, devID int64) {
	t.Helper()
	run, err := svc.CreateRun("probe-run", "probe")
	if err != nil {
		t.Fatal(err)
	}
	params := map[string]string{"K": "1024", "HASH": "SHAKE256"}
	p, err := svc.RegisterParamSet(&model.ParamSet{
		RunID: run.ID, Name: "ml-kem-768", KEMAlgorithm: "ML-KEM", Version: "1.0.0",
	}, params)
	if err != nil {
		t.Fatal(err)
	}
	dev, err := svc.RegisterDevice(&model.Device{
		RunID: run.ID, Name: "hsm-a", Model: "HSM-3000", NoiseLevel: 0.2, Firmware: "1.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	return run.ID, p.ID, dev.ID
}

func TestReIngestMarksDuplicateStatus(t *testing.T) {
	svc, _ := newTestService(t)
	runID, paramID, devID := mustRunFixture(t, svc)
	digest := strings.Repeat("ab", 32)
	in := event.IngestInput{
		RunID: runID, ParamID: paramID, DeviceID: devID,
		CiphertextDigest: digest, FailureCode: "decapsulation_error", NoiseLevel: 0.2,
	}
	ev, isNew, err := svc.IngestEvent(in)
	if err != nil || !isNew {
		t.Fatalf("first ingest: isNew=%v err=%v", isNew, err)
	}
	if _, isNew2, err := svc.IngestEvent(in); err != nil || isNew2 {
		t.Fatalf("duplicate ingest: isNew=%v err=%v", isNew2, err)
	}
	got, err := svc.GetEvent(ev.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.EventDuplicate {
		t.Fatalf("duplicate status=%s want=%s", got.Status, model.EventDuplicate)
	}
}
