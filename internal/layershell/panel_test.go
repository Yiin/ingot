package layershell

import "testing"

func TestPanelHeight_CapsAtMaxHeightWithNoMonitor(t *testing.T) {
	cfg := Config{MaxHeight: 640, HeightFraction: 0.72}

	if got := panelHeight(0, cfg); got != 640 {
		t.Errorf("panelHeight(0, ...) = %d, want %d (no monitor reference, skip the fraction cap)", got, 640)
	}
	if got := panelHeight(-1, cfg); got != 640 {
		t.Errorf("panelHeight(-1, ...) = %d, want %d", got, 640)
	}
}

func TestPanelHeight_CapsAtFractionOnAShortMonitor(t *testing.T) {
	cfg := Config{MaxHeight: 640, HeightFraction: 0.72}

	// 800dp tall monitor: 72% is 576, under the 640 cap.
	if got := panelHeight(800, cfg); got != 576 {
		t.Errorf("panelHeight(800, ...) = %d, want %d", got, 576)
	}
}

func TestPanelHeight_TallMonitorHitsTheMaxHeightCapInstead(t *testing.T) {
	cfg := Config{MaxHeight: 640, HeightFraction: 0.72}

	// 2000dp tall monitor: 72% is 1440, well over the 640 cap.
	if got := panelHeight(2000, cfg); got != 640 {
		t.Errorf("panelHeight(2000, ...) = %d, want %d", got, 640)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Namespace != "ingot-panel" {
		t.Errorf("Namespace = %q, want %q", cfg.Namespace, "ingot-panel")
	}
	if cfg.Width != 360 {
		t.Errorf("Width = %d, want %d", cfg.Width, 360)
	}
	if cfg.MaxHeight != 640 {
		t.Errorf("MaxHeight = %d, want %d", cfg.MaxHeight, 640)
	}
	if cfg.MarginEdge != 12 {
		t.Errorf("MarginEdge = %d, want %d", cfg.MarginEdge, 12)
	}
	if cfg.HeightFraction != 0.72 {
		t.Errorf("HeightFraction = %v, want %v", cfg.HeightFraction, 0.72)
	}
}
