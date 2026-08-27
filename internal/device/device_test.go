package device

import (
	"math"
	"testing"

	"task283-kemfail/internal/model"
)

func TestIsolateRecover(t *testing.T) {
	dev := &model.Device{Status: model.DevActive}
	next, err := Isolate(dev)
	if err != nil || next != model.DevIsolated {
		t.Fatalf("isolate: %v %s", err, next)
	}
	// 幂等
	next, _ = Isolate(&model.Device{Status: model.DevIsolated})
	if next != model.DevIsolated {
		t.Error("re-isolate should be idempotent")
	}
	next, err = Recover(&model.Device{Status: model.DevIsolated})
	if err != nil || next != model.DevActive {
		t.Fatalf("recover: %v %s", err, next)
	}
}

func TestGuardTransition(t *testing.T) {
	if err := GuardTransition(model.DevActive, model.DevIsolated); err != nil {
		t.Errorf("active->isolated should be allowed: %v", err)
	}
	if err := GuardTransition(model.DevIsolated, model.DevActive); err != nil {
		t.Errorf("isolated->active should be allowed: %v", err)
	}
	if err := GuardTransition(model.DevIsolated, model.DevIsolated); err != nil {
		t.Errorf("same state should be allowed: %v", err)
	}
}

func TestClassifyNoise(t *testing.T) {
	if ClassifyNoise(0.95) != "critical" {
		t.Error("0.95 should be critical")
	}
	if ClassifyNoise(0.8) != "high" {
		t.Error("0.8 should be high")
	}
	if ClassifyNoise(0.6) != "medium" {
		t.Error("0.6 should be medium")
	}
	if ClassifyNoise(0.1) != "low" {
		t.Error("0.1 should be low")
	}
}

func TestUpdateExponential(t *testing.T) {
	p := NoiseProfile{}
	p = UpdateExponential(p, 0.5, 0.3)
	if p.Count != 1 || p.Mean != 0.5 {
		t.Fatalf("first update: %+v", p)
	}
	p = UpdateExponential(p, 0.7, 0.3)
	if math.Abs(p.Mean-0.56) > 1e-9 {
		t.Errorf("mean = %f, want 0.56", p.Mean)
	}
	if p.Count != 2 {
		t.Errorf("count = %d, want 2", p.Count)
	}
}

func TestAnomalyScore(t *testing.T) {
	p := NoiseProfile{Count: 1, Mean: 0.5, Variance: 0.01}
	s := AnomalyScore(p, 0.9)
	if s <= 0.5 {
		t.Errorf("far-from-mean value should score high, got %f", s)
	}
	if s > 1.0 {
		t.Errorf("score should be in [0,1], got %f", s)
	}
}

func TestNeedsIsolation(t *testing.T) {
	if !NeedsIsolation(&model.Device{NoiseLevel: 0.9, NoiseEvents: 4}) {
		t.Error("high noise device should need isolation")
	}
	if NeedsIsolation(&model.Device{NoiseLevel: 0.5, NoiseEvents: 4}) {
		t.Error("medium noise device should not need isolation")
	}
}
