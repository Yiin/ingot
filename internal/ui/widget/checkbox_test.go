package widget

import "testing"

func TestCheckboxProgressNotAnimating(t *testing.T) {
	c := &Checkbox{}
	if fill, tick := c.progress(); fill != 0 || tick != 0 {
		t.Errorf("progress() unchecked, idle = (%v, %v), want (0, 0)", fill, tick)
	}

	c.checked = true
	if fill, tick := c.progress(); fill != 1 || tick != 1 {
		t.Errorf("progress() checked, idle = (%v, %v), want (1, 1)", fill, tick)
	}
}

func TestCheckboxProgressAnimatingReversesWhenUnchecking(t *testing.T) {
	c := &Checkbox{checked: false, animating: true, elapsed: checkDuration}
	fill, tick := c.progress()
	if fill != 0 || tick != 0 {
		t.Errorf("progress() unchecking at full elapsed = (%v, %v), want (0, 0)", fill, tick)
	}

	c.checked = true
	fill, tick = c.progress()
	if fill != 1 || tick != 1 {
		t.Errorf("progress() checking at full elapsed = (%v, %v), want (1, 1)", fill, tick)
	}
}
