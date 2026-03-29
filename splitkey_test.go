package dynago

import (
	"testing"
)

func TestSplitKey_HashDelimiter(t *testing.T) {
	parts := SplitKey("ORDER#2024-01-15#abc", "#")
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}
	if parts[0] != "ORDER" {
		t.Errorf("expected ORDER, got %s", parts[0])
	}
	if parts[1] != "2024-01-15" {
		t.Errorf("expected 2024-01-15, got %s", parts[1])
	}
	if parts[2] != "abc" {
		t.Errorf("expected abc, got %s", parts[2])
	}
}

func TestSplitKey_PipeDelimiter(t *testing.T) {
	parts := SplitKey("USER|123|active", "|")
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}
	if parts[0] != "USER" {
		t.Errorf("expected USER, got %s", parts[0])
	}
	if parts[1] != "123" {
		t.Errorf("expected 123, got %s", parts[1])
	}
	if parts[2] != "active" {
		t.Errorf("expected active, got %s", parts[2])
	}
}

func TestSplitKey_NoDelimiterMatch(t *testing.T) {
	parts := SplitKey("NODELMITER", "#")
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if parts[0] != "NODELMITER" {
		t.Errorf("expected NODELMITER, got %s", parts[0])
	}
}

func TestSplitKey_EmptyString(t *testing.T) {
	parts := SplitKey("", "#")
	if len(parts) != 1 {
		t.Fatalf("expected 1 part (empty string), got %d", len(parts))
	}
	if parts[0] != "" {
		t.Errorf("expected empty string, got %q", parts[0])
	}
}

func TestSplitKey_MultiCharDelimiter(t *testing.T) {
	parts := SplitKey("USER::123::active", "::")
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}
	if parts[0] != "USER" {
		t.Errorf("expected USER, got %s", parts[0])
	}
	if parts[1] != "123" {
		t.Errorf("expected 123, got %s", parts[1])
	}
	if parts[2] != "active" {
		t.Errorf("expected active, got %s", parts[2])
	}
}
