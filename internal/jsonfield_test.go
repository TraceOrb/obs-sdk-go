package internal

import (
	"encoding/json"
	"testing"
)

func TestToHeadersJsonRedactsSensitiveHeaders(t *testing.T) {
	t.Parallel()

	raw := toHeadersJSON(map[string]any{
		"Authorization": "Bearer secret-token",
		"Accept":        "application/json",
	}, 32*1024, nil)
	if raw == "" {
		t.Fatal("expected json")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["Authorization"] != Redacted {
		t.Fatalf("got Authorization %v", parsed["Authorization"])
	}
	if parsed["Accept"] != "application/json" {
		t.Fatalf("got Accept %v", parsed["Accept"])
	}
}

func TestToHeadersJsonTruncatesOverByteCap(t *testing.T) {
	t.Parallel()

	raw := toHeadersJSON(map[string]any{
		"Accept": repeatX(200),
	}, 20, nil)
	if raw == "" {
		t.Fatal("expected json")
	}
	if len([]byte(raw)) > 20 {
		t.Fatalf("got %d bytes", len([]byte(raw)))
	}
}

func TestToBodyJsonUndefinedIsEmpty(t *testing.T) {
	t.Parallel()

	if toBodyJSON(nil, 100, nil) != "" {
		t.Fatal("nil body should omit")
	}
}

func TestToBodyJsonRedactsSecrets(t *testing.T) {
	t.Parallel()

	raw := toBodyJSON(map[string]any{
		"sku":      "abc",
		"password": "hunter2",
	}, 32*1024, nil)
	if raw == "" {
		t.Fatal("expected json")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["sku"] != "abc" {
		t.Fatalf("got sku %v", parsed["sku"])
	}
	if parsed["password"] != Redacted {
		t.Fatalf("got password %v", parsed["password"])
	}
}

func TestToBodyJsonReturnsEmptyWhenSerializationFails(t *testing.T) {
	t.Parallel()

	ch := make(chan int)
	if toBodyJSON(ch, 1000, nil) != "" {
		t.Fatal("channel should not serialize")
	}
}

func repeatX(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}
