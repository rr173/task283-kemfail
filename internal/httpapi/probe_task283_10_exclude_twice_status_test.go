package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"task283-kemfail/internal/service"
	"task283-kemfail/internal/store"
)

func newProbeServer(t *testing.T) http.Handler {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "probe-http.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return New(service.New(db)).Handler()
}

func mustProbeRun(t *testing.T, h http.Handler) (runID, paramID, devID int64) {
	t.Helper()
	body := strings.NewReader(`{"name":"probe-run","owner":"probe"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/runs", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create run=%d body=%s", rec.Code, rec.Body.String())
	}
	body = strings.NewReader(`{"run_id":1,"name":"ml-kem-768","kem_algorithm":"ML-KEM","version":"1.0.0","params":{"K":"1024","HASH":"SHAKE256"}}`)
	req = httptest.NewRequest(http.MethodPost, "/api/params", body)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create param=%d body=%s", rec.Code, rec.Body.String())
	}
	body = strings.NewReader(`{"run_id":1,"name":"hsm-a","model":"HSM-3000","noise_level":0.2,"firmware":"1.0"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/devices", body)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create device=%d body=%s", rec.Code, rec.Body.String())
	}
	return 1, 1, 1
}

func TestExcludeTwiceMapsConflict(t *testing.T) {
	h := newProbeServer(t)
	runID, paramID, devID := mustProbeRun(t, h)
	body := strings.NewReader(`{"run_id":` + fmt.Sprintf("%d", runID) +
		`,"param_id":` + fmt.Sprintf("%d", paramID) +
		`,"device_id":` + fmt.Sprintf("%d", devID) +
		`,"ciphertext_digest":"` + strings.Repeat("ab", 32) +
		`","failure_code":"decapsulation_error","noise_level":0.2}`)
	req := httptest.NewRequest(http.MethodPost, "/api/events", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("ingest=%d body=%s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/api/events/1/exclude", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first exclude=%d body=%s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/api/events/1/exclude", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("second exclude status=%d body=%s", rec.Code, rec.Body.String())
	}
}
