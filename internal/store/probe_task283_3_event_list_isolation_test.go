package store

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"task283-kemfail/internal/model"
)

func newProbeDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "probe.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestEventListsDoNotShareBackingArray(t *testing.T) {
	db := newProbeDB(t)
	runA, _ := db.CreateRun("run-a", "probe")
	runB, _ := db.CreateRun("run-b", "probe")
	pA, _ := db.CreateParamSet(&model.ParamSet{
		RunID: runA.ID, Name: "p-a", KEMAlgorithm: "ML-KEM", Version: "1.0.0", ParamsDigest: "da",
	})
	pB, _ := db.CreateParamSet(&model.ParamSet{
		RunID: runB.ID, Name: "p-b", KEMAlgorithm: "ML-KEM", Version: "1.0.0", ParamsDigest: "db",
	})
	dA, _ := db.CreateDevice(&model.Device{RunID: runA.ID, Name: "d-a", Model: "m", NoiseLevel: 0.1, Firmware: "f"})
	dB, _ := db.CreateDevice(&model.Device{RunID: runB.ID, Name: "d-b", Model: "m", NoiseLevel: 0.1, Firmware: "f"})
	digest := strings.Repeat("ab", 32)
	for i := 0; i < 2; i++ {
		key := fmt.Sprintf("key-a-%d", i)
		if _, err := db.CreateEvent(&model.FailureEvent{
			RunID: runA.ID, ParamID: pA.ID, DeviceID: dA.ID, EventKey: key,
			CiphertextDigest: digest, FailureCode: "decapsulation_error",
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.CreateEvent(&model.FailureEvent{
		RunID: runB.ID, ParamID: pB.ID, DeviceID: dB.ID, EventKey: "key-b-1",
		CiphertextDigest: strings.Repeat("cd", 32), FailureCode: "decapsulation_error",
	}); err != nil {
		t.Fatal(err)
	}
	first, err := db.ListEvents(runA.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) < 2 {
		t.Fatalf("want 2 events in run A, got %d", len(first))
	}
	runAID := first[0].RunID
	second, err := db.ListEvents(runB.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) == 0 {
		t.Fatal("run B should have events")
	}
	if first[0].RunID != runAID || first[0].RunID != runA.ID {
		t.Fatalf("list A overwritten: first run_id=%d want=%d", first[0].RunID, runA.ID)
	}
}
