package traceorb

import (
	"testing"
)

func TestParseClientPolicyDefaults(t *testing.T) {
	t.Parallel()

	policy, err := ParseClientPolicy(Options{})
	if err != nil {
		t.Fatal(err)
	}
	if policy.SampleRate != 1 {
		t.Fatalf("got sampleRate %v", policy.SampleRate)
	}
	if policy.Capture.ResponseBody != CaptureAlways {
		t.Fatalf("got responseBody %q", policy.Capture.ResponseBody)
	}
}

func TestParseClientPolicyRejectsSampleRateOutsideRange(t *testing.T) {
	t.Parallel()

	high := 1.5
	_, err := ParseClientPolicy(Options{SampleRate: &high})
	if err == nil {
		t.Fatal("expected error for 1.5")
	}

	low := -0.1
	_, err = ParseClientPolicy(Options{SampleRate: &low})
	if err == nil {
		t.Fatal("expected error for -0.1")
	}
}

func TestParseCaptureModeRejectsUnknown(t *testing.T) {
	t.Parallel()

	_, err := ParseCaptureMode("only500")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPolicyForRouteMergesFieldByField(t *testing.T) {
	t.Parallel()

	rate := 0.1
	one := 1.0
	always := CaptureAlways
	policy, err := ParseClientPolicy(Options{
		SampleRate: &rate,
		Capture: Capture{
			ResponseBody: CaptureErrors,
			RequestBody:  CaptureErrors,
		},
		Routes: map[string]RoutePolicy{
			"/webhooks": {
				SampleRate: &one,
				Capture: &CaptureOverride{
					RequestBody: &always,
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	resolved := PolicyForRoute(policy, "/webhooks")
	if resolved.SampleRate != 1 {
		t.Fatalf("got sampleRate %v", resolved.SampleRate)
	}
	if resolved.Capture.RequestBody != CaptureAlways {
		t.Fatalf("got requestBody %q", resolved.Capture.RequestBody)
	}
	if resolved.Capture.ResponseBody != CaptureErrors {
		t.Fatalf("got responseBody %q", resolved.Capture.ResponseBody)
	}

	other := PolicyForRoute(policy, "/v1/orders")
	if other.SampleRate != 0.1 {
		t.Fatalf("got sampleRate %v", other.SampleRate)
	}
	if other.Capture.RequestBody != CaptureErrors {
		t.Fatalf("got requestBody %q", other.Capture.RequestBody)
	}
}

func TestShouldSampleErrorsAlwaysEvenAtRateZero(t *testing.T) {
	t.Parallel()

	if !ShouldSample(0, 500, 1) {
		t.Fatal("expected 500 at rate 0")
	}
	if ShouldSample(0, 200, 0) {
		t.Fatal("expected drop 200 at rate 0")
	}
	if !ShouldSample(1, 200, 1) {
		t.Fatal("expected 200 at rate 1")
	}
	if !ShouldSample(0.1, 200, 0.05) {
		t.Fatal("expected sample when random < rate")
	}
	if ShouldSample(0.1, 200, 0.2) {
		t.Fatal("expected drop when random >= rate")
	}
	if !ShouldSample(0.1, 404, 1) {
		t.Fatal("expected 404 always")
	}
}

func TestShouldIncludeFieldRespectsModes(t *testing.T) {
	t.Parallel()

	if ShouldIncludeField(CaptureNever, 500) {
		t.Fatal("never should omit")
	}
	if !ShouldIncludeField(CaptureAlways, 200) {
		t.Fatal("always should include")
	}
	if ShouldIncludeField(CaptureErrors, 200) {
		t.Fatal("errors should omit 200")
	}
	if !ShouldIncludeField(CaptureErrors, 400) {
		t.Fatal("errors should include 400")
	}
}
