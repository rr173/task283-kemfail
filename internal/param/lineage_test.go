package param

import (
	"testing"

	"task283-kemfail/internal/model"
)

func TestCompareVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"1.10.0", "1.9.0", 1},
		{"1.9a", "1.10", -1},
		{"2.0", "1.99.99", 1},
		{"1.1", "1.1.0", -1}, // 更短版本视为更低
	}
	for _, c := range cases {
		if got := compareVersion(c.a, c.b); got != c.want {
			t.Errorf("compareVersion(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestVersionLess(t *testing.T) {
	if !versionLess("1.0.1", "1.0.2") {
		t.Error("1.0.1 should be less than 1.0.2")
	}
	if versionLess("1.0.2", "1.0.1") {
		t.Error("1.0.2 should not be less than 1.0.1")
	}
}

func TestComputeDigestDeterministic(t *testing.T) {
	in := DigestInput{
		KEMAlgorithm: "ML-KEM",
		Version:      "1.0.0",
		Params:       map[string]string{"K": "1024", "HASH": "SHAKE256"},
	}
	d1 := ComputeDigest(in)
	d2 := ComputeDigest(in)
	if d1 != d2 {
		t.Error("digest should be deterministic")
	}
	if len(d1) != 64 {
		t.Errorf("digest length = %d, want 64", len(d1))
	}
	// 键顺序不影响摘要
	in2 := DigestInput{
		KEMAlgorithm: "ML-KEM",
		Version:      "1.0.0",
		Params:       map[string]string{"HASH": "SHAKE256", "K": "1024"},
	}
	if d1 != ComputeDigest(in2) {
		t.Error("digest should be key-order independent")
	}
}

func TestVerifyConsistency(t *testing.T) {
	p := &model.ParamSet{Name: "ml-kem-768", Version: "1.0.0", ParamsDigest: "aabbccdd"}
	ok, ev := VerifyConsistency("aabbccdd", p)
	if !ok || ev != "" {
		t.Errorf("consistent case: ok=%v ev=%q", ok, ev)
	}
	ok, ev = VerifyConsistency("eeff0000", p)
	if ok || ev == "" {
		t.Errorf("inconsistent case: ok=%v ev=%q", ok, ev)
	}
}

func TestMatchingBaseline(t *testing.T) {
	baselines := []model.ParamSet{
		{ID: 1, ParamsDigest: "aaaa"},
		{ID: 2, ParamsDigest: "bbbb"},
	}
	if id, ok := MatchingBaseline("bbbb", baselines); !ok || id != 2 {
		t.Errorf("match: id=%d ok=%v", id, ok)
	}
	if _, ok := MatchingBaseline("cccc", baselines); ok {
		t.Error("should not match unknown digest")
	}
}
