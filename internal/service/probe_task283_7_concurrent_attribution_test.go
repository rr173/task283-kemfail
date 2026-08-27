package service

import (
	"path/filepath"
	"strings"
	"sync"
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

func TestConcurrentAttributionStableCandidateCount(t *testing.T) {
	svc, db := newTestService(t)
	runID, paramID, devID := mustRunFixture(t, svc)
	digest := strings.Repeat("ab", 32)
	ev, _, err := svc.IngestEvent(event.IngestInput{
		RunID: runID, ParamID: paramID, DeviceID: devID,
		CiphertextDigest: digest, FailureCode: "decapsulation_error", NoiseLevel: 0.2,
	})
	if err != nil {
		t.Fatal(err)
	}
	const workers = 20
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.RunAttribution(runID); err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
	cands, err := db.ListCandidatesByEvent(ev.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 3 {
		t.Fatalf("event %d candidates=%d want=3", ev.ID, len(cands))
	}
}
