package app

import (
	"sync"
	"testing"

	"github.com/Yiin/ingot/internal/store"
)

func TestSafe_RecoversPanic(t *testing.T) {
	ran := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic escaped safe(): %v", r)
			}
		}()
		safe("test", func() {
			ran = true
			panic("boom")
		})()
	}()
	if !ran {
		t.Errorf("wrapped fn never ran")
	}
}

func TestSafeText_RecoversPanic(t *testing.T) {
	var got string
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic escaped safeText(): %v", r)
			}
		}()
		safeText("test", func(text string) {
			got = text
			panic("boom")
		})("hello")
	}()
	if got != "hello" {
		t.Errorf("got = %q, want %q", got, "hello")
	}
}

func TestSafeEvent_RecoversPanic(t *testing.T) {
	var got store.Event
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic escaped safeEvent(): %v", r)
			}
		}()
		safeEvent("test", func(ev store.Event) {
			got = ev
			panic("boom")
		})(store.SaveFailed{})
	}()
	if _, ok := got.(store.SaveFailed); !ok {
		t.Errorf("got = %#v, want store.SaveFailed{}", got)
	}
}

func TestGoSafe_RecoversPanicOnGoroutine(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	ran := false
	goSafe("test", func() {
		defer wg.Done()
		ran = true
		panic("boom")
	})
	wg.Wait()
	if !ran {
		t.Errorf("goroutine never ran")
	}
}
