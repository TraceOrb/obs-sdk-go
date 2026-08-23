package internal

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

func RecordUnhandledError(store *Store, err any) {
	if store == nil {
		return
	}

	fields := resolveErrorFields(err)
	if store.errorMessage == "" {
		store.errorMessage = truncateChars(fields.message, maxErrorMessageChars)
	}

	if len(store.events) >= maxEventsPerRequest {
		return
	}

	event := IngestEvent{
		Seq:       len(store.events),
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Name:      "unhandled.error",
		Level:     EventLevelError,
	}

	attrs := buildErrorAttrs(fields)
	if len(attrs) > 0 {
		event.Attrs = attrs
	}

	store.events = append(store.events, event)
}

type errorFields struct {
	message string
	name    string
	stack   string
}

func resolveErrorFields(err any) errorFields {
	if err == nil {
		return errorFields{message: "Error", name: "Error"}
	}

	asError, ok := err.(error)
	if !ok {
		return errorFields{
			message: fmt.Sprint(err),
			name:    "Error",
		}
	}

	return errorFields{
		message: asError.Error(),
		name:    fmt.Sprintf("%T", asError),
		stack:   stackFromRuntime(),
	}
}

func stackFromRuntime() string {
	buf := make([]byte, maxStackChars*2)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

func buildErrorAttrs(fields errorFields) map[string]string {
	attrs := map[string]string{}
	if fields.name != "" {
		attrs["type"] = fields.name
	}

	if fields.stack != "" {
		attrs["stack"] = truncateChars(fields.stack, maxStackChars)
	}

	file, line := resolveAppFrame(fields.stack)
	if file != "" {
		attrs["file"] = file
		attrs["line"] = line
	}

	if len(attrs) == 0 {
		return nil
	}

	return attrs
}

func resolveAppFrame(stack string) (string, string) {
	if stack == "" {
		return "", ""
	}

	lines := strings.Split(stack, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.Contains(trimmed, "runtime/") {
			continue
		}

		if strings.HasPrefix(trimmed, "runtime.") {
			continue
		}

		file, lineNo, ok := parseGoFrame(trimmed)
		if !ok {
			continue
		}

		if strings.Contains(file, "/runtime/") {
			continue
		}

		return file, lineNo
	}

	return "", ""
}

func parseGoFrame(line string) (string, string, bool) {
	if !strings.Contains(line, ".go:") {
		return "", "", false
	}

	idx := strings.LastIndex(line, ".go:")
	file := line[:idx+3]
	rest := line[idx+4:]
	lineNo := ""
	for _, r := range rest {
		if r < '0' || r > '9' {
			break
		}

		lineNo += string(r)
	}

	if file == "" || lineNo == "" {
		return "", "", false
	}

	return file, lineNo, true
}

func truncateChars(value string, maxChars int) string {
	if len(value) <= maxChars {
		return value
	}

	return value[:maxChars]
}
