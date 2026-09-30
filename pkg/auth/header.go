package auth

import "strings"

// ExtractBearer extracts a token from a Bearer Authorization header.
// Empty headers, missing tokens and other authorization schemes are rejected.
func ExtractBearer(header string) (string, bool) {
	header = strings.TrimSpace(header)
	const scheme = "bearer "
	if len(header) < len(scheme) || !strings.EqualFold(header[:len(scheme)], scheme) {
		return "", false
	}
	token := strings.TrimSpace(header[len(scheme):])
	if token == "" {
		return "", false
	}
	return token, true
}
