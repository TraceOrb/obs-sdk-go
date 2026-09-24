package traceorb

import (
	"errors"

	"github.com/TraceOrb/obs-sdk-go/internal"
)

type CaptureMode string

const (
	CaptureAlways CaptureMode = "always"
	CaptureErrors CaptureMode = "errors"
	CaptureNever  CaptureMode = "never"
)

type Capture struct {
	Headers      CaptureMode
	Query        CaptureMode
	RequestBody  CaptureMode
	ResponseBody CaptureMode
}

type CaptureOverride struct {
	Headers      *CaptureMode
	Query        *CaptureMode
	RequestBody  *CaptureMode
	ResponseBody *CaptureMode
}

type RoutePolicy struct {
	SampleRate *float64
	Capture    *CaptureOverride
}

type ClientPolicy struct {
	SampleRate float64
	Capture    Capture
	Routes     map[string]RoutePolicy
}

type ResolvedRoutePolicy struct {
	SampleRate float64
	Capture    Capture
}

const defaultSampleRate = 1.0

func ParseCaptureMode(value string) (CaptureMode, error) {
	switch CaptureMode(value) {
	case CaptureAlways, CaptureErrors, CaptureNever:
		return CaptureMode(value), nil
	default:
		return "", errors.New("invalid capture mode")
	}
}

func ParseClientPolicy(opts Options) (ClientPolicy, error) {
	sampleRate, err := parseSampleRate(opts.SampleRate)
	if err != nil {
		return ClientPolicy{}, err
	}

	capture, err := parseCapture(opts.Capture)
	if err != nil {
		return ClientPolicy{}, err
	}

	routes := map[string]RoutePolicy{}
	for key, route := range opts.Routes {
		parsed, err := parseRoutePolicy(route)
		if err != nil {
			return ClientPolicy{}, err
		}
		routes[key] = parsed
	}

	return ClientPolicy{
		SampleRate: sampleRate,
		Capture:    capture,
		Routes:     routes,
	}, nil
}

func PolicyForRoute(policy ClientPolicy, routePattern string) ResolvedRoutePolicy {
	route, ok := policy.Routes[routePattern]
	if !ok {
		return ResolvedRoutePolicy{
			SampleRate: policy.SampleRate,
			Capture:    policy.Capture,
		}
	}

	sampleRate := policy.SampleRate
	if route.SampleRate != nil {
		sampleRate = *route.SampleRate
	}

	capture := policy.Capture
	if route.Capture != nil {
		capture = Capture{
			Headers:      mergeCaptureField(policy.Capture.Headers, route.Capture.Headers),
			Query:        mergeCaptureField(policy.Capture.Query, route.Capture.Query),
			RequestBody:  mergeCaptureField(policy.Capture.RequestBody, route.Capture.RequestBody),
			ResponseBody: mergeCaptureField(policy.Capture.ResponseBody, route.Capture.ResponseBody),
		}
	}

	return ResolvedRoutePolicy{
		SampleRate: sampleRate,
		Capture:    capture,
	}
}

func ShouldSample(sampleRate float64, statusCode int, random float64) bool {
	if statusCode >= 400 {
		return true
	}
	if sampleRate == 1 {
		return true
	}
	if sampleRate == 0 {
		return false
	}
	return random < sampleRate
}

func ShouldIncludeField(mode CaptureMode, statusCode int) bool {
	return internal.ShouldIncludeField(string(mode), statusCode)
}

func parseSampleRate(value *float64) (float64, error) {
	if value == nil {
		return defaultSampleRate, nil
	}
	if *value < 0 || *value > 1 {
		return 0, errors.New("sampleRate must be between 0 and 1")
	}
	return *value, nil
}

func resolveCaptureMode(value CaptureMode) (CaptureMode, error) {
	if value == "" {
		return CaptureAlways, nil
	}
	return ParseCaptureMode(string(value))
}

func parseCapture(partial Capture) (Capture, error) {
	headers, err := resolveCaptureMode(partial.Headers)
	if err != nil {
		return Capture{}, err
	}
	query, err := resolveCaptureMode(partial.Query)
	if err != nil {
		return Capture{}, err
	}
	requestBody, err := resolveCaptureMode(partial.RequestBody)
	if err != nil {
		return Capture{}, err
	}
	responseBody, err := resolveCaptureMode(partial.ResponseBody)
	if err != nil {
		return Capture{}, err
	}
	return Capture{
		Headers:      headers,
		Query:        query,
		RequestBody:  requestBody,
		ResponseBody: responseBody,
	}, nil
}

func optionalParseCaptureMode(value *CaptureMode) (*CaptureMode, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := ParseCaptureMode(string(*value))
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseRouteCapture(partial *CaptureOverride) (*CaptureOverride, error) {
	if partial == nil {
		return nil, nil
	}
	headers, err := optionalParseCaptureMode(partial.Headers)
	if err != nil {
		return nil, err
	}
	query, err := optionalParseCaptureMode(partial.Query)
	if err != nil {
		return nil, err
	}
	requestBody, err := optionalParseCaptureMode(partial.RequestBody)
	if err != nil {
		return nil, err
	}
	responseBody, err := optionalParseCaptureMode(partial.ResponseBody)
	if err != nil {
		return nil, err
	}
	return &CaptureOverride{
		Headers:      headers,
		Query:        query,
		RequestBody:  requestBody,
		ResponseBody: responseBody,
	}, nil
}

func parseRoutePolicy(input RoutePolicy) (RoutePolicy, error) {
	if input.SampleRate != nil {
		if *input.SampleRate < 0 || *input.SampleRate > 1 {
			return RoutePolicy{}, errors.New("sampleRate must be between 0 and 1")
		}
	}
	capture, err := parseRouteCapture(input.Capture)
	if err != nil {
		return RoutePolicy{}, err
	}
	return RoutePolicy{
		SampleRate: input.SampleRate,
		Capture:    capture,
	}, nil
}

func mergeCaptureField(global CaptureMode, override *CaptureMode) CaptureMode {
	if override != nil {
		return *override
	}
	return global
}
