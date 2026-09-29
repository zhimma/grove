package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/internal/provider"
	"github.com/zhimma/grove/internal/testkit"
	"github.com/zhimma/grove/pkg/database"
)

func TestNewCoreServerRequiresConfig(t *testing.T) {
	core, cleanup, err := NewCoreServer(nil, "api", "8080")
	if err == nil {
		t.Fatal("expected error when config is nil")
	}
	if core != nil {
		t.Fatal("expected nil core server when config is nil")
	}
	if cleanup != nil {
		t.Fatal("expected nil cleanup when config is nil")
	}
}

func TestNewCoreServerRegistersHealthChecks(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testServerConfig(t)

	core, cleanup, err := NewCoreServer(cfg, "api", "8080")
	if err != nil {
		t.Fatalf("new core server: %v", err)
	}
	t.Cleanup(cleanup)

	for _, path := range []string{"/health", "/health/live", "/health/ready"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()
		core.Router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("%s expected 200, got %d", path, resp.Code)
		}

		var payload map[string]any
		if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode health response: %v", err)
		}
		if payload["status"] != "ok" {
			t.Fatalf("unexpected status: %#v", payload["status"])
		}
		if payload["service"] != "api" {
			t.Fatalf("unexpected service: %#v", payload["service"])
		}
	}
}

func TestReadinessFailureDoesNotExposeDependencyError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testServerConfig(t)
	cfg.Databases.Default.Enabled = true
	db := testkit.OpenDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	registerHealthChecks(engine, "api", &provider.Provider{
		Config: cfg,
		DB:     database.NewConnectionsFromDBs(db, nil),
	}, cfg)

	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d body=%s", resp.Code, resp.Body.String())
	}
	if strings.Contains(resp.Body.String(), "database is closed") {
		t.Fatalf("readiness response leaked internal error: %s", resp.Body.String())
	}
}

func TestCoreServerExportsPrometheusMetricsWhenEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testServerConfig(t)
	cfg.Observability = config.ObservabilityConfig{
		Enabled:          true,
		MetricsEnabled:   true,
		MetricsPath:      "/metrics",
		ReadinessTimeout: 1,
		TraceSampleRatio: 1,
	}
	core, cleanup, err := NewCoreServer(cfg, "api", "8080", provider.WithObservability())
	if err != nil {
		t.Fatalf("new core server: %v", err)
	}
	t.Cleanup(cleanup)

	core.Router.GET("/test", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	core.Router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/test", nil))
	resp := httptest.NewRecorder()
	core.Router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "grove_http_server_requests") {
		t.Fatalf("unexpected metrics response: %d %s", resp.Code, resp.Body.String())
	}
}

func TestNewCoreServerDoesNotTrustForwardedIPByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)

	core, cleanup, err := NewCoreServer(testServerConfig(t), "api", "8080")
	if err != nil {
		t.Fatalf("new core server: %v", err)
	}
	t.Cleanup(cleanup)
	core.Router.GET("/client-ip", func(c *gin.Context) {
		c.String(http.StatusOK, c.ClientIP())
	})

	req := httptest.NewRequest(http.MethodGet, "/client-ip", nil)
	req.RemoteAddr = "192.0.2.10:4321"
	req.Header.Set("X-Forwarded-For", "203.0.113.20")
	resp := httptest.NewRecorder()
	core.Router.ServeHTTP(resp, req)

	if got := resp.Body.String(); got != "192.0.2.10" {
		t.Fatalf("expected direct peer IP, got %q", got)
	}
}

func TestNewCoreServerTrustsOnlyConfiguredProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testServerConfig(t)
	cfg.Security.TrustedProxies = []string{"192.0.2.10"}
	core, cleanup, err := NewCoreServer(cfg, "api", "8080")
	if err != nil {
		t.Fatalf("new core server: %v", err)
	}
	t.Cleanup(cleanup)
	core.Router.GET("/client-ip", func(c *gin.Context) {
		c.String(http.StatusOK, c.ClientIP())
	})

	req := httptest.NewRequest(http.MethodGet, "/client-ip", nil)
	req.RemoteAddr = "192.0.2.10:4321"
	req.Header.Set("X-Forwarded-For", "203.0.113.20")
	resp := httptest.NewRecorder()
	core.Router.ServeHTTP(resp, req)

	if got := resp.Body.String(); got != "203.0.113.20" {
		t.Fatalf("expected forwarded client IP, got %q", got)
	}
}

func TestNewCoreServerRejectsInvalidTrustedProxy(t *testing.T) {
	cfg := testServerConfig(t)
	cfg.Security.TrustedProxies = []string{"not-a-proxy"}

	core, cleanup, err := NewCoreServer(cfg, "api", "8080")
	if err == nil {
		t.Fatal("expected invalid trusted proxy error")
	}
	if core != nil || cleanup != nil {
		t.Fatal("expected no server or cleanup function after startup failure")
	}
}

func TestCoreServerStartReturnsBindError(t *testing.T) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Errorf("close listener: %v", err)
		}
	})
	port := listener.Addr().(*net.TCPAddr).Port

	core, cleanup, err := NewCoreServer(testServerConfig(t), "api", fmt.Sprint(port))
	if err != nil {
		t.Fatalf("new core server: %v", err)
	}
	t.Cleanup(cleanup)
	if err := core.Start("api"); err == nil || !strings.Contains(err.Error(), "listen") {
		t.Fatalf("expected bind error, got %v", err)
	}
}

func TestCoreServerStartBindsBeforeReturning(t *testing.T) {
	core, cleanup, err := NewCoreServer(testServerConfig(t), "api", "0")
	if err != nil {
		t.Fatalf("new core server: %v", err)
	}
	t.Cleanup(cleanup)
	if err := core.Start("api"); err != nil {
		t.Fatalf("start core server: %v", err)
	}
	if core.listener == nil {
		t.Fatal("expected listener to be available after Start")
	}
	conn, err := net.DialTimeout("tcp", core.listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("dial started server: %v", err)
	}
	_ = conn.Close()
	if err := core.Stop(context.Background()); err != nil {
		t.Fatalf("stop core server: %v", err)
	}
}

func TestCoreServerReportsServeErrors(t *testing.T) {
	core, cleanup, err := NewCoreServer(testServerConfig(t), "api", "0")
	if err != nil {
		t.Fatalf("new core server: %v", err)
	}
	t.Cleanup(cleanup)
	if err := core.Start("api"); err != nil {
		t.Fatalf("start core server: %v", err)
	}
	if err := core.listener.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	select {
	case err := <-core.Errors():
		if err == nil {
			t.Fatal("expected serve error")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for serve error")
	}
}

func TestNewCoreServerUsesCaseInsensitiveProductionMode(t *testing.T) {
	previousMode := gin.Mode()
	t.Cleanup(func() { gin.SetMode(previousMode) })
	cfg := testServerConfig(t)
	cfg.App.Env = "ProDucTion"
	_, cleanup, err := NewCoreServer(cfg, "api", "0")
	if err != nil {
		t.Fatalf("new core server: %v", err)
	}
	t.Cleanup(cleanup)
	if gin.Mode() != gin.ReleaseMode {
		t.Fatalf("gin mode = %q, want %q", gin.Mode(), gin.ReleaseMode)
	}
}

func TestNewCoreServerAppliesIdleTimeoutAndSafeDefaults(t *testing.T) {
	cfg := testServerConfig(t)
	cfg.Server.IdleTimeout = 7
	core, cleanup, err := NewCoreServer(cfg, "api", "0")
	if err != nil {
		t.Fatalf("new core server: %v", err)
	}
	t.Cleanup(cleanup)
	if core.Server.IdleTimeout != 7*time.Second {
		t.Fatalf("idle timeout = %s, want 7s", core.Server.IdleTimeout)
	}

	cfg.Server.IdleTimeout = 0
	core, cleanup, err = NewCoreServer(cfg, "api", "0")
	if err != nil {
		t.Fatalf("new core server with zero idle timeout: %v", err)
	}
	t.Cleanup(cleanup)
	if core.Server.IdleTimeout <= 0 {
		t.Fatal("zero idle timeout must fall back to a positive default")
	}
}

func testServerConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		App:  config.AppConfig{Name: "grove", Env: "test"},
		Port: "8080",
		Log: config.LogConfig{
			Level:   "error",
			Path:    t.TempDir(),
			Console: false,
			Service: "api-test",
		},
		Server: config.ServerConfig{
			ReadTimeout:     5,
			WriteTimeout:    5,
			ShutdownTimeout: 5,
			MaxHeaderBytes:  1 << 20,
		},
	}
}
