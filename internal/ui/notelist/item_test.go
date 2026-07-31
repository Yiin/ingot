package notelist

import (
	"testing"
	"time"
)

func TestNewItemIsARealNote(t *testing.T) {
	it := NewItem("n1", "s1", "hello", false)
	if it.IsPlaceholder() {
		t.Errorf("NewItem returned a placeholder")
	}
	if it.ID != "n1" || it.SectionID != "s1" || it.Body != "hello" || it.Done {
		t.Errorf("NewItem = %+v, fields not set as given", it)
	}
	if it.Born.IsZero() {
		t.Errorf("NewItem left Born zero")
	}
}

func TestJustInsertedGating(t *testing.T) {
	born := time.Now()

	if playing, left := justInserted(born, born); !playing || left <= 0 {
		t.Errorf("justInserted at t=born = (%v, %v), want (true, >0)", playing, left)
	}

	mid := born.Add(InsertAnimDuration / 2)
	if playing, left := justInserted(born, mid); !playing || left != InsertAnimDuration/2 {
		t.Errorf("justInserted at t=born+half = (%v, %v), want (true, %v)", playing, left, InsertAnimDuration/2)
	}

	after := born.Add(InsertAnimDuration + time.Millisecond)
	if playing, left := justInserted(born, after); playing || left != 0 {
		t.Errorf("justInserted after duration = (%v, %v), want (false, 0)", playing, left)
	}

	exactly := born.Add(InsertAnimDuration)
	if playing, _ := justInserted(born, exactly); playing {
		t.Errorf("justInserted exactly at the duration boundary = true, want false (a re-bound old note must never carry just-inserted)")
	}
}
