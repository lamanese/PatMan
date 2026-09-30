package handler

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestAuditTextKeepsShortStrings(t *testing.T) {
	for _, s := range []string{"", "openscap", strings.Repeat("x", 256), strings.Repeat("ä", 256)} {
		if got := auditText(s); got != s {
			t.Fatalf("auditText changed %q to %q", s, got)
		}
	}
}

func TestAuditTextTruncatesLongStrings(t *testing.T) {
	got := auditText(strings.Repeat("a", 300))
	want := strings.Repeat("a", 256) + "…"
	if got != want {
		t.Fatalf("got %d runes, want %d", utf8.RuneCountInString(got), utf8.RuneCountInString(want))
	}
	// Multi-byte input is cut on rune boundaries.
	got = auditText(strings.Repeat("ä", 300))
	if !utf8.ValidString(got) || utf8.RuneCountInString(got) != 257 {
		t.Fatalf("invalid truncation: %d runes", utf8.RuneCountInString(got))
	}
}
