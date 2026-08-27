package fingerprint

import "testing"

func TestGroup(t *testing.T) {
	events := []KeyedEvent{
		{ParamID: 1, ParamName: "p1", FailureCode: "err_a", Digest: "aaaa...", DeviceID: 10},
		{ParamID: 1, ParamName: "p1", FailureCode: "err_a", Digest: "aaaa...", DeviceID: 11},
		{ParamID: 1, ParamName: "p1", FailureCode: "err_a", Digest: "aaaa...", DeviceID: 10},
		{ParamID: 2, ParamName: "p2", FailureCode: "err_b", Digest: "bbbb...", DeviceID: 12},
	}
	groups := Group(events)
	if len(groups) != 2 {
		t.Fatalf("want 2 groups, got %d", len(groups))
	}
	// 找 p1 组
	var g Fingerprint
	for _, f := range groups {
		if f.ParamName == "p1" {
			g = f
		}
	}
	if g.EventCount != 3 {
		t.Errorf("p1 group event count = %d, want 3", g.EventCount)
	}
	if len(g.DeviceIDs) != 2 {
		t.Errorf("p1 group devices = %v, want 2 distinct", g.DeviceIDs)
	}
	if !IsBurst(g) {
		t.Error("p1 group should be burst (3 events, 2 devices)")
	}
}
