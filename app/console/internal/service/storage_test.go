package service

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	pkgstorage "github.com/zhimma/grove/pkg/storage"
)

func TestStorageServiceOpenFileReadsValidatedObject(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "documents"), 0o750); err != nil {
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
	service := NewStorageService(manager)

	result, err := service.OpenFile(context.Background(), DownloadStorageFileInput{Disk: "local", Path: "documents/report.txt"})
	if err != nil {
		t.Fatalf("open object: %v", err)
	}
	t.Cleanup(func() {
		if err := result.Reader.Close(); err != nil {
			t.Errorf("close object reader: %v", err)
		}
	})
	content, err := io.ReadAll(result.Reader)
	if err != nil {
		t.Fatalf("read object: %v", err)
	}
	if string(content) != "report" || result.Filename != "report.txt" || result.ContentType != "text/plain" {
		t.Fatalf("unexpected object response: filename=%q content_type=%q body=%q", result.Filename, result.ContentType, content)
	}
}

func TestStorageServiceRejectsInvalidDownloadPath(t *testing.T) {
	manager := pkgstorage.NewManager("local")
	driver, err := pkgstorage.NewLocalDriver(pkgstorage.LocalConfig{Root: t.TempDir()})
	if err != nil {
		t.Fatalf("new local driver: %v", err)
	}
	manager.AddDisk("local", driver, pkgstorage.DiskConfig{}, nil)
	service := NewStorageService(manager)

	if _, err := service.OpenFile(context.Background(), DownloadStorageFileInput{Disk: "local", Path: "../secret.txt"}); err == nil {
		t.Fatal("expected invalid download path error")
	}
}
