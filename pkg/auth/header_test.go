package auth

import "testing"

func TestExtractBearer(t *testing.T) {
	for _, tc := range []struct{ header, token string }{
		{"Bearer token", "token"},
		{"  bEaReR   token  ", "token"},
		{"", ""},
		{"Bearer", ""},
		{"Bearer ", ""},
		{"token", ""},
		{"Basic token", ""},
		{"Bearertoken", ""},
	} {
		t.Run(tc.header, func(t *testing.T) {
			token, ok := ExtractBearer(tc.header)
			if token != tc.token || ok != (tc.token != "") {
				t.Fatalf("got (%q, %t), want token %q", token, ok, tc.token)
			}
		})
	}
}
