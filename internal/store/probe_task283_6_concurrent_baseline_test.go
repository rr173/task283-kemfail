package store

import (
	"path/filepath"
	"sync"
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

func TestConcurrentBaselineMarkingExclusive(t *testing.T) {
	db := newProbeDB(t)
	run, _ := db.CreateRun("baseline-race", "probe")
	p1, _ := db.CreateParamSet(&model.ParamSet{
		RunID: run.ID, Name: "p1", KEMAlgorithm: "ML-KEM", Version: "1.0.0", ParamsDigest: "d1", IsBaseline: true,
	})
	p2, _ := db.CreateParamSet(&model.ParamSet{
		RunID: run.ID, Name: "p2", KEMAlgorithm: "ML-KEM", Version: "1.1.0", ParamsDigest: "d2",
	})
	ids := []int64{p1.ID, p2.ID}
	const workers = 20
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := db.MarkParamBaseline(ids[i%2]); err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
	params, err := db.ListParamSets(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	baseline := 0
	for _, p := range params {
		if p.IsBaseline {
			baseline++
		}
	}
	if baseline != 1 {
		t.Fatalf("want exactly 1 baseline, got %d", baseline)
	}
}
