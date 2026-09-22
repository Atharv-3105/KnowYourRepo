package api

import "testing"

func TestExtractSnippet_NoRangeReturnsFullContent(t *testing.T) {
	content := "line1\nline2\nline3"
	got := extractSnippet(content, 0, 0)
	if got != content {
		t.Errorf("expected full content when no range given, got %q", got)
	}
}

func TestExtractSnippet_ReturnsRequestedRange(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5"
	got := extractSnippet(content, 2, 4)
	want := "line2\nline3\nline4"
	if got != want {
		t.Errorf("extractSnippet(2, 4) = %q, want %q", got, want)
	}
}

func TestExtractSnippet_ClampsOutOfBoundsRange(t *testing.T) {
	content := "line1\nline2\nline3"
	got := extractSnippet(content, 2, 100)
	want := "line2\nline3"
	if got != want {
		t.Errorf("expected end_line to clamp to the last line, got %q", got)
	}
}

func TestExtractSnippet_StartPastEndOfFileReturnsEmpty(t *testing.T) {
	content := "line1\nline2"
	got := extractSnippet(content, 50, 60)
	if got != "" {
		t.Errorf("expected empty string when start_line is past the end of the file, got %q", got)
	}
}

func TestExtractSnippet_EndBeforeStartFallsBackToSingleLine(t *testing.T) {
	content := "line1\nline2\nline3"
	got := extractSnippet(content, 2, 1)
	want := "line2"
	if got != want {
		t.Errorf("expected a malformed end_line < start_line to fall back to just start_line, got %q", got)
	}
}
