package internal

import "strings"

func MergeRedactKeys(groups [][]string) []string {
	seen := map[string]struct{}{}
	output := make([]string, 0)

	for _, group := range groups {
		if group == nil {
			continue
		}

		for _, key := range group {
			normalized := strings.ToLower(strings.TrimSpace(key))
			if normalized == "" {
				continue
			}

			if _, exists := seen[normalized]; exists {
				continue
			}

			if len(output) >= maxExtraRedactKeys {
				return output
			}

			seen[normalized] = struct{}{}
			output = append(output, normalized)
		}
	}

	return output
}

func keysWithExtras(extraKeys []string) map[string]struct{} {
	keys := make(map[string]struct{}, len(redactSensitiveKeys)+len(extraKeys))
	for key := range redactSensitiveKeys {
		keys[key] = struct{}{}
	}

	for _, key := range extraKeys {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if normalized == "" {
			continue
		}

		keys[normalized] = struct{}{}
	}

	return keys
}

func isSensitiveKey(key string, keys map[string]struct{}) bool {
	_, ok := keys[strings.ToLower(key)]
	return ok
}

func redactRecord(input map[string]any, keys map[string]struct{}) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		if isSensitiveKey(key, keys) {
			output[key] = Redacted
			continue
		}

		output[key] = redactValue(value, keys, 1)
	}

	return output
}

func redactValue(value any, keys map[string]struct{}, depth int) any {
	if depth > maxRedactDepth {
		return value
	}

	switch typed := value.(type) {
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = redactValue(item, keys, depth+1)
		}
		return out
	case map[string]any:
		return redactRecord(typed, keys)
	default:
		if typed == nil {
			return nil
		}

		return value
	}
}

func redactHeaders(headers map[string]any, extraKeys []string) map[string]any {
	return redactRecord(headers, keysWithExtras(extraKeys))
}

func redactBody(value any, extraKeys []string) any {
	return redactValue(value, keysWithExtras(extraKeys), 0)
}
