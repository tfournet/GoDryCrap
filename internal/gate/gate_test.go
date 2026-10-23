package gate

import "testing"

func TestLevel(t *testing.T) {
	cases := map[float64]Status{8.99: OK, 9: Warn, 9.5: Warn, 10: Fail, 30: Fail}
	for v, want := range cases {
		if got := Level(v, 9, 10); got != want {
			t.Errorf("Level(%v) = %v, want %v", v, got, want)
		}
	}
}

func TestWorstAndStrings(t *testing.T) {
	if Worst(nil) != OK || Worst([]Status{Warn, OK, Fail, Warn}) != Fail {
		t.Error("Worst is wrong")
	}
	for s, want := range map[Status]string{OK: "ok", Warn: "warn", Fail: "fail", Error: "error"} {
		if s.String() != want || int(s) != map[string]int{"ok": 0, "warn": 1, "fail": 2, "error": 3}[want] {
			t.Errorf("%d = %s, want %s", s, s, want)
		}
	}
}
