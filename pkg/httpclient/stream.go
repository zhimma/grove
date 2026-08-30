package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxStreamErrorBodyBytes = int64(64 << 10)

type multipartBody struct {
	fields url.Values
	files  []FileField
}

type multipartPart struct {
	field  FileField
	reader io.Reader
	closer io.Closer
}

func newMultipartBody(fields url.Values, files []FileField) requestBody {
	clonedFields := make(url.Values, len(fields))
	for key, values := range fields {
		clonedFields[key] = append([]string(nil), values...)
	}
	clonedFiles := make([]FileField, len(files))
	copy(clonedFiles, files)
	return &multipartBody{fields: clonedFields, files: clonedFiles}
}

func (b *multipartBody) Open() (io.ReadCloser, string, int64, error) {
	parts := make([]multipartPart, 0, len(b.files))
	for _, file := range b.files {
		part := multipartPart{field: file}
		if file.FilePath != "" {
			opened, err := os.Open(file.FilePath)
			if err != nil {
				closeMultipartParts(parts)
				return nil, "", 0, fmt.Errorf("open multipart file %s: %w", file.FilePath, err)
			}
			part.reader = opened
			part.closer = opened
		} else {
			part.reader = bytes.NewReader(file.Content)
		}
		parts = append(parts, part)
	}

	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	contentType := multipartWriter.FormDataContentType()
	go func() {
		defer closeMultipartParts(parts)
		if err := writeMultipart(multipartWriter, b.fields, parts); err != nil {
			_ = writer.CloseWithError(err)
			return
		}
		_ = writer.Close()
	}()
	return reader, contentType, -1, nil
}

func writeMultipart(writer *multipart.Writer, fields url.Values, parts []multipartPart) error {
	for key, values := range fields {
		for _, value := range values {
			if err := writer.WriteField(key, value); err != nil {
				return fmt.Errorf("write multipart field %s: %w", key, err)
			}
		}
	}
	for _, part := range parts {
		header := cloneMIMEHeader(part.field.Header)
		if header == nil {
			header = make(textproto.MIMEHeader)
		}
		if header.Get("Content-Disposition") == "" {
			header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, part.field.FieldName, part.field.FileName))
		}
		if header.Get("Content-Type") == "" {
			contentType := mime.TypeByExtension(filepath.Ext(part.field.FileName))
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			header.Set("Content-Type", contentType)
		}
		fileWriter, err := writer.CreatePart(header)
		if err != nil {
			return fmt.Errorf("create multipart file %s: %w", part.field.FileName, err)
		}
		if _, err := io.Copy(fileWriter, part.reader); err != nil {
			return fmt.Errorf("write multipart file %s: %w", part.field.FileName, err)
		}
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}
	return nil
}

func closeMultipartParts(parts []multipartPart) {
	for _, part := range parts {
		if part.closer != nil {
			_ = part.closer.Close()
		}
	}
}

func (c *Client) Stream(ctx context.Context, method, path string, body any, handler func(chunk []byte) error) error {
	return c.NewRequest(method, path).Body(body).Stream(ctx, handler)
}

func (rb *RequestBuilder) Stream(ctx context.Context, handler func(chunk []byte) error) (err error) {
	if rb == nil {
		return fmt.Errorf("http request is nil")
	}
	if rb.err != nil {
		return rb.err
	}
	if handler == nil {
		return fmt.Errorf("stream handler is required")
	}
	ctx, cancel := rb.withTimeout(ctx)
	defer cancel()
	resp, err := rb.openStreamResponse(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close stream response: %w", closeErr)
		}
	}()
	buffer := make([]byte, 32<<10)
	for {
		count, readErr := resp.Body.Read(buffer)
		if count > 0 {
			if err := handler(buffer[:count]); err != nil {
				return fmt.Errorf("handle stream chunk: %w", err)
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return fmt.Errorf("read stream response: %w", readErr)
		}
	}
}

func (c *Client) DownloadToFile(path, localPath string) error {
	return c.NewRequest(http.MethodGet, path).DownloadToFile(context.Background(), localPath)
}

func (rb *RequestBuilder) DownloadToFile(ctx context.Context, localPath string) (err error) {
	if rb == nil {
		return fmt.Errorf("http request is nil")
	}
	if rb.err != nil {
		return rb.err
	}
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create download directory: %w", err)
	}
	ctx, cancel := rb.withTimeout(ctx)
	defer cancel()
	resp, err := rb.openStreamResponse(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close download response: %w", closeErr)
		}
	}()

	tempFile, err := os.CreateTemp(dir, "."+filepath.Base(localPath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create download temp file: %w", err)
	}
	tempPath := tempFile.Name()
	keep := false
	defer func() {
		_ = tempFile.Close()
		if !keep {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		return fmt.Errorf("stream download: %w", err)
	}
	if err := tempFile.Chmod(0644); err != nil {
		return fmt.Errorf("chmod download: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close download: %w", err)
	}
	if err := os.Rename(tempPath, localPath); err != nil {
		return fmt.Errorf("replace download target: %w", err)
	}
	keep = true
	return nil
}

func (rb *RequestBuilder) openStreamResponse(ctx context.Context) (*http.Response, error) {
	maxRetries := rb.retry.maxRetries
	if !rb.retryAllowed() {
		maxRetries = 0
	}
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			if err := waitForRetry(ctx, rb.retry.delay*time.Duration(attempt)); err != nil {
				return nil, err
			}
		}
		req, err := rb.buildHTTPRequest(ctx)
		if err != nil {
			return nil, err
		}
		httpResp, err := (&http.Client{Transport: rb.client.transport}).Do(req)
		if err != nil {
			if attempt < maxRetries {
				continue
			}
			return nil, fmt.Errorf("stream request failed after %d retries: %w", attempt, err)
		}
		if httpResp.StatusCode < http.StatusBadRequest {
			return httpResp, nil
		}
		body, readErr := io.ReadAll(io.LimitReader(httpResp.Body, maxStreamErrorBodyBytes))
		_ = httpResp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read stream error response: %w", readErr)
		}
		statusErr := fmt.Errorf("stream request failed with status %d: %s", httpResp.StatusCode, strings.TrimSpace(string(body)))
		if httpResp.StatusCode >= http.StatusInternalServerError && attempt < maxRetries {
			continue
		}
		return nil, statusErr
	}
	return nil, fmt.Errorf("stream request failed")
}
