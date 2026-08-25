package internal

import (
	"reflect"
	"strconv"
	"testing"
)

func TestRedactReplacesAuthorizationInHeaders(t *testing.T) {
	t.Parallel()

	output := redactHeaders(map[string]any{
		"Authorization": "Bearer secret-token",
		"Accept":        "application/json",
	}, nil)

	if output["Authorization"] != Redacted {
		t.Fatalf("got Authorization %v", output["Authorization"])
	}
	if output["Accept"] != "application/json" {
		t.Fatalf("got Accept %v", output["Accept"])
	}
}

func TestRedactAccessAndRefreshToken(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"accessToken":  "live-jwt",
		"refreshToken": "live-refresh",
		"nested":       map[string]any{"access_token": "x", "ok": true},
	}
	out, _ := redactBody(input, nil).(map[string]any)
	if out["accessToken"] != Redacted {
		t.Fatalf("accessToken: %v", out["accessToken"])
	}
	if out["refreshToken"] != Redacted {
		t.Fatalf("refreshToken: %v", out["refreshToken"])
	}
	nested, _ := out["nested"].(map[string]any)
	if nested["access_token"] != Redacted {
		t.Fatalf("nested access_token: %v", nested["access_token"])
	}
}

func TestRedactReplacesNestedBodySecrets(t *testing.T) {
	t.Parallel()

	output := redactBody(map[string]any{
		"user":     "ada",
		"password": "hunter2",
		"nested": map[string]any{
			"token": "abc",
			"ok":    true,
		},
	}, nil)

	want := map[string]any{
		"user":     "ada",
		"password": Redacted,
		"nested": map[string]any{
			"token": Redacted,
			"ok":    true,
		},
	}
	if !reflect.DeepEqual(output, want) {
		t.Fatalf("got %#v want %#v", output, want)
	}
}

func TestRedactReplacesExtraKeysInHeadersAndBodies(t *testing.T) {
	t.Parallel()

	headers := redactHeaders(map[string]any{
		"Accept":  "application/json",
		"X-Email": "ada@example.com",
	}, []string{"x-email"})
	if headers["X-Email"] != Redacted {
		t.Fatalf("got X-Email %v", headers["X-Email"])
	}
	if headers["Accept"] != "application/json" {
		t.Fatalf("got Accept %v", headers["Accept"])
	}

	body := redactBody(map[string]any{
		"email": "ada@example.com",
		"sku":   "abc",
	}, []string{"Email"})
	want := map[string]any{
		"email": Redacted,
		"sku":   "abc",
	}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("got %#v want %#v", body, want)
	}
}

func TestRedactSecretsInsideArrays(t *testing.T) {
	t.Parallel()

	output := redactBody([]any{
		map[string]any{"password": "hunter2"},
		map[string]any{"sku": "abc"},
	}, nil)

	want := []any{
		map[string]any{"password": Redacted},
		map[string]any{"sku": "abc"},
	}
	if !reflect.DeepEqual(output, want) {
		t.Fatalf("got %#v want %#v", output, want)
	}
}

func TestRedactSkipsBlankExtraKeys(t *testing.T) {
	t.Parallel()

	output := redactHeaders(map[string]any{
		"Accept": "application/json",
	}, []string{"", "  "})
	if output["Accept"] != "application/json" {
		t.Fatalf("got Accept %v", output["Accept"])
	}
}

func TestRedactLeavesPrimitivesUnchanged(t *testing.T) {
	t.Parallel()

	if redactBody("plain", nil) != "plain" {
		t.Fatal("string should pass through")
	}
	if redactBody(nil, nil) != nil {
		t.Fatal("nil should pass through")
	}
}

func TestMergeRedactKeysNormalizes(t *testing.T) {
	t.Parallel()

	got := MergeRedactKeys([][]string{
		{" Email ", "cpf"},
		{"email", "ssn"},
		nil,
	})
	want := []string{"email", "cpf", "ssn"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestMergeRedactKeysSkipsBlankNames(t *testing.T) {
	t.Parallel()

	got := MergeRedactKeys([][]string{{"", "  ", "email"}})
	want := []string{"email"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestMergeRedactKeysStopsAtCap(t *testing.T) {
	t.Parallel()

	keys := make([]string, maxExtraRedactKeys+8)
	for i := range keys {
		keys[i] = "k" + strconv.Itoa(i)
	}
	got := MergeRedactKeys([][]string{keys})
	if len(got) != maxExtraRedactKeys {
		t.Fatalf("got %d keys", len(got))
	}
}
