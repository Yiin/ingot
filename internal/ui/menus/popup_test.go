package menus

import (
	"reflect"
	"testing"
)

func TestResolveClickSelectionInsideSelectionKeepsIt(t *testing.T) {
	got := ResolveClickSelection(2, []int{1, 2})
	if !reflect.DeepEqual(got, []int{1, 2}) {
		t.Errorf("ResolveClickSelection(2, [1,2]) = %v, want [1 2]", got)
	}
}

func TestResolveClickSelectionOutsideSelectionReplacesIt(t *testing.T) {
	got := ResolveClickSelection(5, []int{1, 2})
	if !reflect.DeepEqual(got, []int{5}) {
		t.Errorf("ResolveClickSelection(5, [1,2]) = %v, want [5]", got)
	}
}

func TestResolveClickSelectionEmptySelection(t *testing.T) {
	got := ResolveClickSelection(3, nil)
	if !reflect.DeepEqual(got, []int{3}) {
		t.Errorf("ResolveClickSelection(3, nil) = %v, want [3]", got)
	}
}

func TestOtherProjectsExcludesCurrent(t *testing.T) {
	projects := []Project{{ID: "p1"}, {ID: "p2"}, {ID: "p3"}}
	got := otherProjects(projects, "p2")
	want := []Project{{ID: "p1"}, {ID: "p3"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("otherProjects = %v, want %v", got, want)
	}
}
