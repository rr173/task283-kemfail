package event

import (
	"strings"
	"testing"

	"task283-kemfail/internal/model"
)

func TestBuildEventKey(t *testing.T) {
	k1 := BuildEventKey(1, 2, 3, strings.Repeat("ab", 32))
	k2 := BuildEventKey(1, 2, 3, strings.Repeat("ab", 32))
	if k1 != k2 {
		t.Error("event key should be deterministic")
	}
	k3 := BuildEventKey(1, 2, 3, strings.Repeat("ac", 32))
	if k1 == k3 {
		t.Error("different digest should yield different key")
	}
	k4 := BuildEventKey(1, 2, 4, strings.Repeat("ab", 32))
	if k1 == k4 {
		t.Error("different device should yield different key")
	}
}

func TestValidateDigest(t *testing.T) {
	if err := ValidateDigest(strings.Repeat("ab", 32)); err != nil {
		t.Errorf("valid digest rejected: %v", err)
	}
	if err := ValidateDigest(strings.Repeat("ab", 31)); err == nil {
		t.Error("short digest accepted")
	}
	if err := ValidateDigest(strings.Repeat("gz", 32)); err == nil {
		t.Error("non-hex digest accepted")
	}
}

func TestValidateFailureCode(t *testing.T) {
	if err := ValidateFailureCode("decapsulation_error"); err != nil {
		t.Errorf("valid code rejected: %v", err)
	}
	if err := ValidateFailureCode(""); err == nil {
		t.Error("empty code accepted")
	}
	long := strings.Repeat("x", 64)
	if err := ValidateFailureCode(long); err == nil {
		t.Error("overlong code accepted")
	}
}

func TestPreclassify(t *testing.T) {
	if s, _ := Preclassify(0.9, 5, true); s != model.EventNoiseRelated {
		t.Errorf("high noise should be noise_related, got %s", s)
	}
	if s, _ := Preclassify(0.2, 0, true); s != model.EventRaw {
		t.Errorf("normal case should be raw, got %s", s)
	}
	if s, _ := Preclassify(0.1, 0, false); s != model.EventParamConflict {
		t.Errorf("param inconsistency should be param_conflict, got %s", s)
	}
}
