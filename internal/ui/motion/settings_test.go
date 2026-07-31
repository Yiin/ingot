package motion

import "testing"

// TestOverrideEnableAnimations exercises the override seam only — never
// the live gtk.SettingsGetDefault() path, which (like every other
// display-dependent call in internal/ui) needs a real GTK display and is
// left to a future //go:build integration test, matching every sibling
// package's own precedent.
func TestOverrideEnableAnimations(t *testing.T) {
	restore := OverrideEnableAnimations(false)
	if EnableAnimations() {
		t.Fatal("EnableAnimations() = true after OverrideEnableAnimations(false)")
	}
	restore()

	restore = OverrideEnableAnimations(true)
	if !EnableAnimations() {
		t.Fatal("EnableAnimations() = false after OverrideEnableAnimations(true)")
	}
	restore()
}

// TestOverrideEnableAnimationsNests confirms restore puts back whatever
// override (if any) was active before, not unconditionally nil — so a
// helper that itself calls OverrideEnableAnimations can be nested inside
// a test that already forced one.
func TestOverrideEnableAnimationsNests(t *testing.T) {
	outer := OverrideEnableAnimations(false)
	defer outer()

	inner := OverrideEnableAnimations(true)
	if !EnableAnimations() {
		t.Fatal("EnableAnimations() = false inside inner override(true)")
	}
	inner()

	if EnableAnimations() {
		t.Fatal("EnableAnimations() = true after inner restore, want outer override(false) back")
	}
}
