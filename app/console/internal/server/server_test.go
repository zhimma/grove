package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/internal/config"
)

func TestRegisterLocalStorageRoutesIgnoresInvalidDisks(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	registerLocalStorageRoutes(engine, &config.Config{
		Storage: config.StorageConfig{
			Disks: map[string]config.StorageDiskConfig{
				"s3": {
					Driver:  "s3",
					BaseURL: "/storage",
					Root:    t.TempDir(),
				},
				"missing-root": {
					Driver:  "local",
					BaseURL: "/storage2",
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/storage/file.txt", nil)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for ignored disks, got %d", resp.Code)
	}
}

func TestRegisterLocalStorageRoutesServesLocalDisk(t *testing.T) {
	gin.SetMode(gin.TestMode)

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	engine := gin.New()
	registerLocalStorageRoutes(engine, &config.Config{
		Storage: config.StorageConfig{
			Disks: map[string]config.StorageDiskConfig{
				"local": {
					Driver:      "local",
					BaseURL:     "/storage",
					Root:        root,
					Public:      true,
					ServeStatic: true,
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/storage/hello.txt", nil)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if resp.Body.String() != "hello" {
		t.Fatalf("unexpected body: %q", resp.Body.String())
	}
	if got := resp.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected nosniff header, got %q", got)
	}
}

func TestRegisterLocalStorageRoutesDefaultsToPrivate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	engine := gin.New()
	registerLocalStorageRoutes(engine, &config.Config{
		Storage: config.StorageConfig{
			Disks: map[string]config.StorageDiskConfig{
				"local": {
					Driver:  "local",
					BaseURL: "/storage",
					Root:    root,
				},
			},
		},
	})

	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/storage/secret.txt", nil))
	if resp.Code != http.StatusNotFound {
		t.Fatalf("private disk must not be served statically, got %d", resp.Code)
	}
}

func TestRegisterLocalStorageRoutesRequiresPublicAndServeStatic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	for name, disk := range map[string]config.StorageDiskConfig{
		"public-without-static": {
			Driver:  "local",
			BaseURL: "/storage-public",
			Root:    root,
			Public:  true,
		},
		"static-without-public": {
			Driver:      "local",
			BaseURL:     "/storage-private",
			Root:        root,
			ServeStatic: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			engine := gin.New()
			registerLocalStorageRoutes(engine, &config.Config{Storage: config.StorageConfig{Disks: map[string]config.StorageDiskConfig{"local": disk}}})
			resp := httptest.NewRecorder()
			engine.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, disk.BaseURL+"/hello.txt", nil))
			if resp.Code != http.StatusNotFound {
				t.Fatalf("disk must require both public and serve_static, got %d", resp.Code)
			}
		})
	}
}
