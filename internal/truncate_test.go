package internal

import (
	"testing"
	"unicode/utf8"
)

func TestTruncateReturnsOriginalWhenUnderCap(t *testing.T) {
	t.Parallel()

	got := truncateField("hello", 100)
	if got != "hello" {
		t.Fatalf("got %q", got)
	}
}

func TestTruncateCutsUtf8AtByteCap(t *testing.T) {
	t.Parallel()

	value := ""
	for i := 0; i < 20; i++ {
		value += "é"
	}
	truncated := truncateField(value, 5)
	if len([]byte(truncated)) > 5 {
		t.Fatalf("got %d bytes", len([]byte(truncated)))
	}
	if !utf8.ValidString(truncated) {
		t.Fatal("truncated string is not valid utf8")
	}
}

func TestTruncateDoesNotSplitCodepoint(t *testing.T) {
	t.Parallel()

	truncated := truncateField("é", 1)
	if truncated != "" {
		t.Fatalf("got %q", truncated)
	}
}

func TestTruncateEmptyWhenMaxBytesZero(t *testing.T) {
	t.Parallel()

	got := truncateField("ab", 0)
	if got != "" {
		t.Fatalf("got %q", got)
	}
}
