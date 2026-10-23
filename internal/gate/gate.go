// Package gate turns metric values into ok, warn and fail, and those into
// exit codes.
package gate

// Status is a row's or a run's verdict. Its value is the exit code.
type Status int

// Exit codes. Error covers usage errors, git and parse failures, and a
// missing, empty or mismatched coverage profile.
const (
	OK    Status = 0
	Warn  Status = 1
	Fail  Status = 2
	Error Status = 3
)

func (s Status) String() string {
	switch s {
	case OK:
		return "ok"
	case Warn:
		return "warn"
	case Fail:
		return "fail"
	}
	return "error"
}

// Level is Fail at or above fail, Warn at or above warn, and OK below.
func Level(value, warn, fail float64) Status {
	switch {
	case value >= fail:
		return Fail
	case value >= warn:
		return Warn
	}
	return OK
}

// Worst is the highest status in statuses, or OK.
func Worst(statuses []Status) Status {
	worst := OK
	for _, s := range statuses {
		worst = max(worst, s)
	}
	return worst
}
