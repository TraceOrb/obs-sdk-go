package internal

import "encoding/json"

func serializeJSON(value any) (string, bool) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", false
	}

	return string(encoded), true
}

func toHeadersJSON(headers map[string]any, maxBytes int, extraKeys []string) string {
	raw, ok := serializeJSON(redactHeaders(headers, extraKeys))
	if !ok {
		return ""
	}

	return truncateField(raw, maxBytes)
}

func toBodyJSON(value any, maxBytes int, extraKeys []string) string {
	if value == nil {
		return ""
	}

	raw, ok := serializeJSON(redactBody(value, extraKeys))
	if !ok {
		return ""
	}

	return truncateField(raw, maxBytes)
}
