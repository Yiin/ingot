package paths

import (
	"path/filepath"
	"testing"
)

// clearEnv unsets every variable Resolve reads, then restores the
// original environment after the test. Tests that want a variable set
// call t.Setenv on top of this.
func clearEnv(t *testing.T) {
	t.Helper()
	// Setting each var to "" rather than unsetting it is deliberate: an
	// empty value is exactly the "counts as unset" case Resolve must
	// handle, and t.Setenv restores the original value afterward either
	// way.
	for _, name := range []string{
		"XDG_DATA_HOME", "XDG_CONFIG_HOME", "XDG_STATE_HOME",
		"INGOT_DATA_DIR", "INGOT_CONFIG_DIR", "INGOT_STATE_DIR",
	} {
		t.Setenv(name, "")
	}
}

func TestResolve_DefaultsUnderHome(t *testing.T) {
	clearEnv(t)
	t.Setenv("HOME", "/home/tester")

	l, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if want := "/home/tester/.local/share/ingot"; l.Data != want {
		t.Errorf("Data = %q, want %q", l.Data, want)
	}
	if want := "/home/tester/.config/ingot"; l.Config != want {
		t.Errorf("Config = %q, want %q", l.Config, want)
	}
	if want := "/home/tester/.local/state/ingot"; l.State != want {
		t.Errorf("State = %q, want %q", l.State, want)
	}
	if want := filepath.Join(l.Data, "projects"); l.Projects != want {
		t.Errorf("Projects = %q, want %q", l.Projects, want)
	}
	if want := filepath.Join(l.Data, "meta"); l.Meta != want {
		t.Errorf("Meta = %q, want %q", l.Meta, want)
	}
	if want := filepath.Join(l.Data, "backups"); l.Backups != want {
		t.Errorf("Backups = %q, want %q", l.Backups, want)
	}
	if want := filepath.Join(l.Data, "trash"); l.Trash != want {
		t.Errorf("Trash = %q, want %q", l.Trash, want)
	}
}

func TestResolve_HonorsXDGVars(t *testing.T) {
	clearEnv(t)
	t.Setenv("HOME", "/home/tester")
	t.Setenv("XDG_DATA_HOME", "/data")
	t.Setenv("XDG_CONFIG_HOME", "/cfg")
	t.Setenv("XDG_STATE_HOME", "/state")

	l, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if want := "/data/ingot"; l.Data != want {
		t.Errorf("Data = %q, want %q", l.Data, want)
	}
	if want := "/cfg/ingot"; l.Config != want {
		t.Errorf("Config = %q, want %q", l.Config, want)
	}
	if want := "/state/ingot"; l.State != want {
		t.Errorf("State = %q, want %q", l.State, want)
	}
}

func TestResolve_EmptyOrRelativeXDGCountsAsUnset(t *testing.T) {
	clearEnv(t)
	t.Setenv("HOME", "/home/tester")
	t.Setenv("XDG_DATA_HOME", "") // empty
	t.Setenv("XDG_CONFIG_HOME", "relative/path")

	l, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if want := "/home/tester/.local/share/ingot"; l.Data != want {
		t.Errorf("Data = %q, want default %q (empty XDG_DATA_HOME should count as unset)", l.Data, want)
	}
	if want := "/home/tester/.config/ingot"; l.Config != want {
		t.Errorf("Config = %q, want default %q (relative XDG_CONFIG_HOME should count as unset)", l.Config, want)
	}
}

func TestResolve_IngotOverrideWinsOverXDG(t *testing.T) {
	clearEnv(t)
	t.Setenv("HOME", "/home/tester")
	t.Setenv("XDG_DATA_HOME", "/data")
	t.Setenv("INGOT_DATA_DIR", "/custom/ingot-data")

	l, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if want := "/custom/ingot-data"; l.Data != want {
		t.Errorf("Data = %q, want %q", l.Data, want)
	}
	if want := filepath.Join(l.Data, "projects"); l.Projects != want {
		t.Errorf("Projects = %q, want %q", l.Projects, want)
	}
}

func TestResolve_RelativeIngotOverrideCountsAsUnset(t *testing.T) {
	clearEnv(t)
	t.Setenv("HOME", "/home/tester")
	t.Setenv("INGOT_CONFIG_DIR", "relative/override")

	l, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if want := "/home/tester/.config/ingot"; l.Config != want {
		t.Errorf("Config = %q, want default %q (relative INGOT_CONFIG_DIR should count as unset)", l.Config, want)
	}
}

func TestProjectFile(t *testing.T) {
	l := Layout{Projects: "/data/ingot/projects"}

	tests := []struct {
		name    string
		slug    string
		want    string
		wantErr bool
	}{
		{"simple slug", "home-garden", "/data/ingot/projects/home-garden.md", false},
		{"empty slug rejected", "", "", true},
		{"dot rejected", ".", "", true},
		{"dot-dot rejected", "..", "", true},
		{"embedded slash rejected", "a/b", "", true},
		{"embedded backslash rejected", "a\\b", "", true},
		{"leading slash traversal rejected", "/etc/passwd", "", true},
		{"traversal prefix rejected", "../../etc/passwd", "", true},
		// A slug like this from an untrusted meta sidecar would produce
		// ".foo.tmp-1.md" — a name isTempName (sweep.go) would otherwise
		// mistake for an AtomicWrite leftover and delete on sight.
		// Rejecting every dot closes that hole; see ProjectFile's doc.
		{"embedded dot rejected", ".foo.tmp-1", "", true},
		{"trailing dot rejected", "foo.", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ProjectFile(l, tt.slug)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ProjectFile(%q) error = nil, want error", tt.slug)
				}
				return
			}
			if err != nil {
				t.Fatalf("ProjectFile(%q) error = %v", tt.slug, err)
			}
			if got != tt.want {
				t.Errorf("ProjectFile(%q) = %q, want %q", tt.slug, got, tt.want)
			}
		})
	}
}
