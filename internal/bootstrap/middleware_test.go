package bootstrap

import "testing"

func TestGlobalAllowsNilConfig(t *testing.T) {
	loader := NewMiddlewareLoader(nil, "api")

	middlewares := loader.Global()
	if len(middlewares) != 4 {
		t.Fatalf("expected 4 default middlewares, got %d", len(middlewares))
	}
}
