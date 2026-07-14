package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"testing"
)

type fakeDriver struct {
	name        string
	contentType string
	content     []byte
	err         error
}

func (d fakeDriver) Name() string {
	return d.name
}

func (d fakeDriver) Put(_ context.Context, _ string, _ []byte) error {
	return nil
}

func (d *fakeDriver) PutStream(_ context.Context, _ string, reader io.Reader, _ int64, contentType string) error {
	if d.err != nil {
		return d.err
	}
	d.contentType = contentType
	d.content, _ = io.ReadAll(reader)
	return nil
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
	driver := &fakeDriver{name: "local"}

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
	manager.AddDisk("local", &fakeDriver{name: "local"}, DiskConfig{}, nil)

	if _, err := manager.SaveUploadedFile(context.Background(), "local", "avatar", nil); err == nil {
		t.Fatal("expected nil upload file error")
	}
}

func TestSaveUploadedFileUsesNamedPolicy(t *testing.T) {
	manager := NewManager("local")
	driver := &fakeDriver{name: "local"}
	manager.AddDisk("local", driver, DiskConfig{Prefix: "console"}, nil)
	policy := mustUploadPolicy(t, UploadPolicyConfig{
		Name:       "avatar",
		Directory:  "avatars",
		MaxBytes:   1024,
		Extensions: []string{".png"},
		MIMETypes:  []string{"image/png"},
	})
	manager.AddUploadPolicy(policy)
	manager.SetDefaultUploadPolicy("avatar")
	content := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 24)...)
	file := newMultipartFileHeader(t, "avatar.png", content)

	stored, err := manager.SaveUploadedFile(context.Background(), "local", "", file)
	if err != nil {
		t.Fatalf("save uploaded file: %v", err)
	}
	if stored.Purpose != "avatar" || stored.ContentType != "image/png" {
		t.Fatalf("unexpected stored file metadata: %#v", stored)
	}
	if driver.contentType != "image/png" || !bytes.Equal(driver.content, content) {
		t.Fatalf("driver did not receive validated stream: type=%q size=%d", driver.contentType, len(driver.content))
	}
	if wantPrefix := "console/avatars/"; len(stored.Path) <= len(wantPrefix) || stored.Path[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("policy directory was not applied: %q", stored.Path)
	}
}

func TestDescribeKeepsS3OnServerUploadUntilCredentialsAreExplicitlyIssued(t *testing.T) {
	manager := NewManager("s3")
	manager.AddDisk("s3", &fakeDriver{name: "s3"}, DiskConfig{Driver: "s3"}, fakeSTSProvider{})

	described, err := manager.Describe("s3")
	if err != nil {
		t.Fatalf("describe disk: %v", err)
	}
	if described.UploadMode != "server" || described.STS != nil {
		t.Fatalf("ordinary client config must use server upload: %#v", described)
	}
	issued, err := manager.IssueClientConfig(context.Background(), "s3", "admin")
	if err != nil {
		t.Fatalf("issue explicit STS config: %v", err)
	}
	if issued.UploadMode != "sts" || issued.STS == nil {
		t.Fatalf("explicit STS issuance should include credentials: %#v", issued)
	}
}

func TestSaveUploadedFileWrapsDriverFailure(t *testing.T) {
	manager := NewManager("local")
	writeErr := errors.New("disk full")
	manager.AddDisk("local", &fakeDriver{name: "local", err: writeErr}, DiskConfig{}, nil)
	content := []byte("plain text")
	file := newMultipartFileHeader(t, "notes.txt", content)

	_, err := manager.SaveUploadedFile(context.Background(), "local", "document", file)
	if !errors.Is(err, ErrUploadStore) || !errors.Is(err, writeErr) {
		t.Fatalf("driver failure must retain stable and original errors: %v", err)
	}
}

type fakeSTSProvider struct{}

func (fakeSTSProvider) IssueToken(context.Context, string) (*STSToken, error) {
	return &STSToken{AccessKeyID: "temporary", Expiration: 123}, nil
}

func newMultipartFileHeader(t *testing.T, filename string, content []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	req := httptest.NewRequest("POST", "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(int64(len(content)) + 1024); err != nil {
		t.Fatalf("parse multipart form: %v", err)
	}
	return req.MultipartForm.File["file"][0]
}
