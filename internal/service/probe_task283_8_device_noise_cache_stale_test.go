package service

import (
	"path/filepath"
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

func TestAttributionUsesUpdatedDeviceNoise(t *testing.T) {
	svc, _ := newTestService(t)
	runID, paramID, devID := mustRunFixture(t, svc)
	p, err := svc.GetParamSet(paramID)
	if err != nil {
		t.Fatal(err)
	}
	ev, _, err := svc.IngestEvent(event.IngestInput{
		RunID: runID, ParamID: paramID, DeviceID: devID,
		CiphertextDigest: p.ParamsDigest, FailureCode: "decapsulation_error", NoiseLevel: 0.2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RunAttribution(runID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if _, err := svc.RecordNoise(devID, 0.95); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.RunAttribution(runID); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetEvent(ev.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.EventNoiseRelated {
		t.Fatalf("after noise update status=%s want=%s", got.Status, model.EventNoiseRelated)
	}
}
