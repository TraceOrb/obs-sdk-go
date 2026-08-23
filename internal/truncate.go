package internal

import "unicode/utf8"

func truncateField(value string, maxBytes int) string {
	encoded := []byte(value)
	if len(encoded) <= maxBytes {
		return value
	}

	end := maxBytes
	for end > 0 {
		if utf8.RuneStart(encoded[end]) {
			break
		}

		end -= 1
	}

	return string(encoded[:end])
}
