package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/internal/service"
	pkgstorage "github.com/zhimma/grove/pkg/storage"
)

func TestStorageDownloadStreamsObjectAndSetsSafeHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "documents"), 0o700); err != nil {
		t.Fatalf("create object directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "documents", "report.txt"), []byte("report"), 0o600); err != nil {
		t.Fatalf("write object: %v", err)
	}
	driver, err := pkgstorage.NewLocalDriver(pkgstorage.LocalConfig{Root: root})
	if err != nil {
		t.Fatalf("new local driver: %v", err)
	}
	manager := pkgstorage.NewManager("local")
	manager.AddDisk("local", driver, pkgstorage.DiskConfig{}, nil)
	h := &StorageHandler{service: consoleservice.NewStorageService(manager)}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/console/v1/storage/download?disk=local&path=documents/report.txt", nil)
	h.Download(c)

	if recorder.Code != http.StatusOK || recorder.Body.String() != "report" {
		t.Fatalf("unexpected download response: status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("download must set nosniff header: %q", recorder.Header().Get("X-Content-Type-Options"))
	}
	if recorder.Header().Get("Content-Disposition") == "" {
		t.Fatal("download must set Content-Disposition")
	}
}
