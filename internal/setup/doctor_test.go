package setup

import (
	"context"
	"testing"
)

type fakeCheck struct {
	name   string
	result Result
}

func (c fakeCheck) Name() string               { return c.name }
func (c fakeCheck) Run(context.Context) Result { return c.result }

func TestRunChecks(t *testing.T) {
	checks := []Check{
		fakeCheck{name: "a", result: ok()},
		fakeCheck{name: "b", result: Result{Severity: Warn, Reason: "degraded"}},
	}
	reports := RunChecks(context.Background(), checks)
	if len(reports) != 2 {
		t.Fatalf("len(reports) = %d, want 2", len(reports))
	}
	if reports[0].Name != "a" || reports[0].Result.Severity != OK {
		t.Errorf("reports[0] = %+v, want name a, OK", reports[0])
	}
	if reports[1].Name != "b" || reports[1].Result.Severity != Warn {
		t.Errorf("reports[1] = %+v, want name b, Warn", reports[1])
	}
}

func TestHealthy(t *testing.T) {
	allOK := []Report{{Name: "a", Result: ok()}, {Name: "b", Result: ok()}}
	if !Healthy(allOK) {
		t.Error("Healthy() with every check OK = false, want true")
	}

	oneWarn := []Report{{Name: "a", Result: ok()}, {Name: "b", Result: Result{Severity: Warn}}}
	if Healthy(oneWarn) {
		t.Error("Healthy() with one Warn = true, want false")
	}

	oneFatal := []Report{{Name: "a", Result: Result{Severity: Fatal}}}
	if Healthy(oneFatal) {
		t.Error("Healthy() with one Fatal = true, want false")
	}

	if Healthy(nil) != true {
		t.Error("Healthy(nil) = false, want true (vacuously healthy)")
	}
}

// TestDefaultChecks_Names guards against a check silently dropping out of
// the doctor table or two checks colliding on the same printed name.
func TestDefaultChecks_Names(t *testing.T) {
	checks := DefaultChecks(NewFakeInstaller(NotInstalled))
	wantCount := 10
	if len(checks) != wantCount {
		t.Fatalf("len(DefaultChecks(...)) = %d, want %d", len(checks), wantCount)
	}

	seen := make(map[string]bool, len(checks))
	for _, c := range checks {
		name := c.Name()
		if name == "" {
			t.Error("a check has an empty Name()")
		}
		if seen[name] {
			t.Errorf("duplicate check name %q", name)
		}
		seen[name] = true
	}
}

// TestDefaultChecks_Run is a smoke test: every real check must run to
// completion against this machine without panicking, regardless of
// whether it reports OK, Warn, or Fatal here.
func TestDefaultChecks_Run(t *testing.T) {
	checks := DefaultChecks(NewFakeInstaller(NotInstalled))
	reports := RunChecks(context.Background(), checks)
	if len(reports) != len(checks) {
		t.Fatalf("len(reports) = %d, want %d", len(reports), len(checks))
	}
	for _, r := range reports {
		if r.Result.Severity != OK && r.Result.Reason == "" {
			t.Errorf("check %q reported %v with no Reason", r.Name, r.Result.Severity)
		}
	}
}
