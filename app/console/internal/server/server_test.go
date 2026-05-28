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
					Driver:  "local",
					BaseURL: "/storage",
					Root:    root,
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
}