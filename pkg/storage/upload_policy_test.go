package storage

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestUploadPolicyAcceptsMatchingImageAndPreservesStream(t *testing.T) {
	policy := mustUploadPolicy(t, UploadPolicyConfig{
		Name:       "avatar",
		Directory:  "avatars",
		MaxBytes:   1024,
		Extensions: []string{".png"},
		MIMETypes:  []string{"image/png"},
	})
	content := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 32)...)

	upload, err := policy.Inspect("avatar.PNG", int64(len(content)), bytes.NewReader(content))
	if err != nil {
		t.Fatalf("inspect valid PNG: %v", err)
	}
	if upload.ContentType != "image/png" || upload.Extension != ".png" || upload.Directory != "avatars" {
		t.Fatalf("unexpected validated upload: %#v", upload)
	}
	got, err := io.ReadAll(upload.Reader)
	if err != nil {
		t.Fatalf("read validated stream: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatal("validation consumed bytes from upload stream")
	}
}

func TestUploadPolicyRejectsOversizedAndEmptyFiles(t *testing.T) {
	policy := mustUploadPolicy(t, UploadPolicyConfig{
		Name:       "document",
		Directory:  "documents",
		MaxBytes:   4,
		Extensions: []string{".txt"},
		MIMETypes:  []string{"text/plain"},
	})

	if _, err := policy.Inspect("notes.txt", 5, bytes.NewBufferString("hello")); !errors.Is(err, ErrUploadTooLarge) {
		t.Fatalf("expected ErrUploadTooLarge, got %v", err)
	}
	if _, err := policy.Inspect("notes.txt", 0, bytes.NewReader(nil)); !errors.Is(err, ErrUploadEmpty) {
		t.Fatalf("expected ErrUploadEmpty, got %v", err)
	}
}

func TestUploadPolicyRejectsExtensionMIMEAndMagicMismatch(t *testing.T) {
	policy := mustUploadPolicy(t, UploadPolicyConfig{
		Name:       "avatar",
		Directory:  "avatars",
		MaxBytes:   1024,
		Extensions: []string{".jpg", ".png"},
		MIMETypes:  []string{"image/jpeg", "image/png"},
	})

	if _, err := policy.Inspect("avatar.exe", 4, bytes.NewBufferString("MZ00")); !errors.Is(err, ErrUploadExtension) {
		t.Fatalf("expected extension rejection, got %v", err)
	}
	pdf := []byte("%PDF-1.7\n")
	if _, err := policy.Inspect("avatar.png", int64(len(pdf)), bytes.NewReader(pdf)); !errors.Is(err, ErrUploadMIME) {
		t.Fatalf("expected MIME rejection, got %v", err)
	}
	jpegNameWithPNG := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 16)...)
	if _, err := policy.Inspect("avatar.jpg", int64(len(jpegNameWithPNG)), bytes.NewReader(jpegNameWithPNG)); !errors.Is(err, ErrUploadMagic) {
		t.Fatalf("expected magic mismatch, got %v", err)
	}
}

func TestUploadPolicyRejectsActiveContent(t *testing.T) {
	policy := mustUploadPolicy(t, UploadPolicyConfig{
		Name:       "document",
		Directory:  "documents",
		MaxBytes:   1024,
		Extensions: []string{".html", ".svg", ".txt"},
		MIMETypes:  []string{"image/svg+xml", "text/html", "text/plain"},
	})

	for _, test := range []struct {
		name    string
		content string
	}{
		{name: "page.html", content: "<!doctype html><script>alert(1)</script>"},
		{name: "image.svg", content: `<svg xmlns="http://www.w3.org/2000/svg"><script/></svg>`},
		{name: "notes.txt", content: "<!doctype html><script>alert(1)</script>"},
	} {
		if _, err := policy.Inspect(test.name, int64(len(test.content)), bytes.NewBufferString(test.content)); !errors.Is(err, ErrUploadActiveContent) {
			t.Fatalf("expected active content rejection for %s, got %v", test.name, err)
		}
	}
}

func mustUploadPolicy(t *testing.T, cfg UploadPolicyConfig) UploadPolicy {
	t.Helper()
	policy, err := NewUploadPolicy(cfg)
	if err != nil {
		t.Fatalf("new upload policy: %v", err)
	}
	return policy
}
