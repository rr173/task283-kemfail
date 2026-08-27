package attribution

import (
	"testing"

	"task283-kemfail/internal/model"
)

func TestEvaluateParamMismatch(t *testing.T) {
	scores := Evaluate(Input{
		RunID: 1, EventID: 1,
		ParamConsistent: false,
		HasBaseline:     true,
		DeviceNoise:     0.2,
	})
	best := Best(scores)
	if best == nil || best.Kind != model.CandParam {
		t.Fatalf("param mismatch should rank param first, got %+v", best)
	}
	if best.Score < 0.8 {
		t.Errorf("param score = %f, want >= 0.8", best.Score)
	}
}

func TestEvaluateDeviceNoise(t *testing.T) {
	scores := Evaluate(Input{
		RunID: 1, EventID: 1,
		ParamConsistent:   true,
		HasBaseline:       true,
		DeviceNoise:       0.9,
		DeviceNoiseEvents: 5,
	})
	best := Best(scores)
	if best == nil || best.Kind != model.CandDevice {
		t.Fatalf("high noise should rank device first, got %+v", best)
	}
	if best.Score < 0.75 {
		t.Errorf("device score = %f, want >= 0.75", best.Score)
	}
}

func TestEvaluateCiphertextBurst(t *testing.T) {
	scores := Evaluate(Input{
		RunID: 1, EventID: 1,
		ParamConsistent: true,
		HasBaseline:     true,
		DeviceNoise:     0.3,
		BurstEvents:     4,
		BurstDevices:    3,
	})
	best := Best(scores)
	if best == nil || best.Kind != model.CandCiphertext {
		t.Fatalf("multi-device burst should rank ciphertext first, got %+v", best)
	}
}

func TestEvaluateDigestUnknown(t *testing.T) {
	scores := Evaluate(Input{
		RunID: 1, EventID: 1,
		DigestUnknown: true,
	})
	best := Best(scores)
	if best == nil || best.Kind != model.CandCiphertext {
		t.Fatalf("unparseable digest should rank ciphertext first, got %+v", best)
	}
}

func TestApplyToEvent(t *testing.T) {
	scores := []Score{{Kind: model.CandParam, Score: 0.9}, {Kind: model.CandDevice, Score: 0.4}}
	st, err := ApplyToEvent(model.EventRaw, scores)
	if err != nil {
		t.Fatal(err)
	}
	if st != model.EventParamConflict {
		t.Errorf("want param_conflict, got %s", st)
	}
	// 排除后不改
	st, _ = ApplyToEvent(model.EventExcluded, scores)
	if st != model.EventExcluded {
		t.Errorf("excluded event should stay excluded, got %s", st)
	}
}

func TestSummarize(t *testing.T) {
	s := Summarize([]Score{{Kind: model.CandParam, Score: 0.9}})
	if s == "" {
		t.Error("summarize should produce non-empty output")
	}
	if Summarize(nil) == "" {
		t.Error("empty summarize should say no candidates")
	}
}
