package storage

import (
	"context"
	"mime/multipart"
	"testing"
)

type fakeDriver struct {
	name string
}

func (d fakeDriver) Name() string {
	return d.name
}

func (d fakeDriver) Put(_ context.Context, _ string, _ []byte) error {
	return nil
}

func (d fakeDriver) PutFile(_ context.Context, objectPath string, _ *multipart.FileHeader) (string, error) {
	return objectPath + "/file.txt", nil
}

func (d fakeDriver) Delete(_ context.Context, _ ...string) error {
	return nil
}

func (d fakeDriver) Exists(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (d fakeDriver) URL(objectPath string) string {
	return "/files/" + objectPath
}

func TestManagerNormalizesDiskNames(t *testing.T) {
	manager := NewManager(" LOCAL ")
	driver := fakeDriver{name: "local"}

	manager.AddDisk(" LOCAL ", driver, DiskConfig{}, nil)

	disk, err := manager.Get("")
	if err != nil {
		t.Fatalf("get default disk failed: %v", err)
	}
	if disk.Config.Name != "local" {
		t.Fatalf("expected normalized disk name, got %q", disk.Config.Name)
	}
}

func TestBuildObjectDirCleansTraversal(t *testing.T) {
	dir := buildObjectDir("console", "../../avatars")
	if dir != "console/avatars" {
		t.Fatalf("unexpected object dir: %s", dir)
	}
}

func TestSaveUploadedFileRequiresFile(t *testing.T) {
	manager := NewManager("local")
	manager.AddDisk("local", fakeDriver{name: "local"}, DiskConfig{}, nil)

	if _, err := manager.SaveUploadedFile(context.Background(), "local", "avatars", nil); err == nil {
		t.Fatal("expected nil upload file error")
	}
}
