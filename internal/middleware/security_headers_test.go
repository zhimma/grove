package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSecurityHeadersAndExplicitHSTS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name string
		hsts bool
	}{
		{name: "without hsts", hsts: false},
		{name: "with hsts", hsts: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine := gin.New()
			engine.Use(SecurityHeaders(test.hsts))
			engine.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
			resp := httptest.NewRecorder()
			engine.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/", nil))

			for name, expected := range map[string]string{
				"X-Content-Type-Options": "nosniff",
				"X-Frame-Options":        "DENY",
				"Referrer-Policy":        "strict-origin-when-cross-origin",
				"Permissions-Policy":     "camera=(), microphone=(), geolocation=()",
			} {
				if got := resp.Header().Get(name); got != expected {
					t.Fatalf("header %s = %q, expected %q", name, got, expected)
				}
			}
			hsts := resp.Header().Get("Strict-Transport-Security")
			if test.hsts && hsts == "" {
				t.Fatal("HSTS header missing")
			}
			if !test.hsts && hsts != "" {
				t.Fatalf("HSTS must be opt-in, got %q", hsts)
			}
		})
	}
}
