package handler

import "testing"

// A name that is already taken gets the lowest free numeric suffix, so an
// unattended enrollment never fails on a duplicate hostname and the result
// stays predictable (web01, web01-2, web01-3).
func TestUniqueFriendlyName(t *testing.T) {
	cases := []struct {
		name     string
		existing []string
		want     string
	}{
		{"web01", nil, "web01"},
		{"web01", []string{"web01"}, "web01-2"},
		{"web01", []string{"web01", "web01-2"}, "web01-3"},
		{"web01", []string{"web01", "web01-3"}, "web01-2"},
		{"web01", []string{"web01-2"}, "web01"},
		{"web01", []string{"WEB01"}, "web01-2"},
		{"web01", []string{"web01", "web01-x", "web01-2"}, "web01-3"},
	}
	for _, c := range cases {
		if got := uniqueFriendlyName(c.name, c.existing); got != c.want {
			t.Errorf("uniqueFriendlyName(%q, %v) = %q, want %q", c.name, c.existing, got, c.want)
		}
	}
}
