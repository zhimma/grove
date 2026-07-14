package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalDriverPutStreamWritesFile(t *testing.T) {
	root := t.TempDir()
	driver, err := NewLocalDriver(LocalConfig{Root: root, BaseURL: "/storage"})
	if err != nil {
		t.Fatalf("new local driver: %v", err)
	}
	content := bytes.Repeat([]byte("streamed-content"), 4096)
	if err := driver.PutStream(context.Background(), "documents/file.txt", bytes.NewReader(content), int64(len(content)), "text/plain"); err != nil {
		t.Fatalf("put stream: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "documents", "file.txt"))
	if err != nil {
		t.Fatalf("read stored file: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatal("stored content does not match stream")
	}
}

func TestLocalDriverPutStreamRemovesPartialFileOnReadError(t *testing.T) {
	root := t.TempDir()
	driver, err := NewLocalDriver(LocalConfig{Root: root})
	if err != nil {
		t.Fatalf("new local driver: %v", err)
	}
	readErr := errors.New("source failed")
	reader := io.MultiReader(bytes.NewBufferString("partial"), failingReader{err: readErr})
	err = driver.PutStream(context.Background(), "documents/file.txt", reader, 128, "text/plain")
	if !errors.Is(err, readErr) {
		t.Fatalf("expected source error, got %v", err)
	}
	if exists, statErr := driver.Exists(context.Background(), "documents/file.txt"); statErr != nil || exists {
		t.Fatalf("partial target must not remain: exists=%v err=%v", exists, statErr)
	}
	entries, err := os.ReadDir(filepath.Join(root, "documents"))
	if err != nil {
		t.Fatalf("read target directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary files were not cleaned up: %#v", entries)
	}
}

type failingReader struct {
	err error
}

func (r failingReader) Read(_ []byte) (int, error) {
	return 0, r.err
}
