package httpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func newTestClient(t *testing.T, handler func(*http.Request) (*http.Response, error)) *Client {
	t.Helper()
	return New(Config{
		BaseURL:   "https://example.test",
		Timeout:   time.Second,
		Transport: roundTripFunc(handler),
	})
}

func textResponse(status int, body io.ReadCloser) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       body,
	}
}

func jsonResponse(status int, body string) *http.Response {
	return textResponse(status, io.NopCloser(strings.NewReader(body)))
}

func TestNewUsesSafeDefaults(t *testing.T) {
	client := New(DefaultConfig())
	if client.timeout != 30*time.Second {
		t.Fatalf("timeout = %v", client.timeout)
	}
	transport, ok := client.transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T", client.transport)
	}
	if transport.TLSHandshakeTimeout <= 0 || transport.ResponseHeaderTimeout <= 0 {
		t.Fatalf("transport timeouts are not configured: %#v", transport)
	}
	if transport.MaxIdleConns <= 0 || transport.MaxIdleConnsPerHost <= 0 {
		t.Fatalf("transport pool is not configured: %#v", transport)
	}
}

func TestClientConfigurationReturnsCopies(t *testing.T) {
	original := New(DefaultConfig())
	transport := &http.Transport{}
	configured := original.
		BaseURL("https://api.example.com/").
		Timeout(time.Minute).
		WithTransport(transport)

	if original.baseURL != "" || original.timeout == time.Minute || original.transport == transport {
		t.Fatal("configuration mutated original client")
	}
	if configured.baseURL != "https://api.example.com" || configured.timeout != time.Minute || configured.transport != transport {
		t.Fatalf("unexpected configured client: %#v", configured)
	}
}

func TestWithTracingInjectsTraceContext(t *testing.T) {
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	t.Cleanup(func() { _ = tracerProvider.Shutdown(context.Background()) })
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	client := newTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("traceparent") == "" {
			t.Fatal("missing traceparent header")
		}
		return jsonResponse(http.StatusOK, `{}`), nil
	}).WithTracing()
	ctx, span := tracerProvider.Tracer("test").Start(context.Background(), "parent")
	defer span.End()
	if _, err := client.GetWithContext(ctx, "/trace"); err != nil {
		t.Fatal(err)
	}
}

func TestRequestBuildersIsolateConcurrentState(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) (*http.Response, error) {
		want := req.URL.Query().Get("request")
		if got := req.Header.Get("X-Request"); got != want {
			return nil, fmt.Errorf("header %q does not match query %q", got, want)
		}
		return jsonResponse(http.StatusOK, `{}`), nil
	})

	const workers = 32
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value := fmt.Sprintf("request-%d", i)
			_, err := client.NewRequest(http.MethodGet, "/users").
				WithHeader("X-Request", value).
				WithQueryParam("request", value).
				Do()
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestRequestJSONAndHooks(t *testing.T) {
	var beforeCalls atomic.Int64
	var afterCalls atomic.Int64
	client := newTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Authorization") != "Bearer token" {
			t.Fatal("missing request header")
		}
		var payload map[string]string
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["name"] != "grove" {
			t.Fatalf("payload = %#v", payload)
		}
		return jsonResponse(http.StatusCreated, `{"ok":true}`), nil
	})

	resp, err := client.NewRequest(http.MethodPost, "/users").
		WithHeader("Authorization", "Bearer token").
		BeforeRequest(func(*http.Request) error {
			beforeCalls.Add(1)
			return nil
		}).
		AfterResponse(func(*Response) error {
			afterCalls.Add(1)
			return nil
		}).
		JSON(map[string]string{"name": "grove"}).
		Do()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated || beforeCalls.Load() != 1 || afterCalls.Load() != 1 {
		t.Fatalf("response=%#v before=%d after=%d", resp, beforeCalls.Load(), afterCalls.Load())
	}
}

func TestBeforeRequestErrorOnBodylessRequest(t *testing.T) {
	hookErr := errors.New("reject request")
	client := newTestClient(t, func(*http.Request) (*http.Response, error) {
		t.Fatal("transport must not run")
		return nil, nil
	})

	_, err := client.NewRequest(http.MethodGet, "/users").
		BeforeRequest(func(*http.Request) error { return hookErr }).
		Do()
	if !errors.Is(err, hookErr) {
		t.Fatalf("error = %v", err)
	}
}

func TestSafeMethodRetriesByDefault(t *testing.T) {
	var attempts atomic.Int64
	client := newTestClient(t, func(*http.Request) (*http.Response, error) {
		if attempts.Add(1) < 3 {
			return jsonResponse(http.StatusServiceUnavailable, `{}`), nil
		}
		return jsonResponse(http.StatusOK, `{}`), nil
	})

	resp, err := client.Get("/flaky")
	if err != nil {
		t.Fatal(err)
	}
	if !resp.IsSuccess() || attempts.Load() != 3 {
		t.Fatalf("status=%d attempts=%d", resp.StatusCode, attempts.Load())
	}
}

func TestSafeMethodRetriesTransportErrors(t *testing.T) {
	var attempts atomic.Int64
	client := newTestClient(t, func(*http.Request) (*http.Response, error) {
		if attempts.Add(1) == 1 {
			return nil, errors.New("connection reset")
		}
		return jsonResponse(http.StatusOK, `{}`), nil
	})

	resp, err := client.Get("/flaky-transport")
	if err != nil || !resp.IsSuccess() || attempts.Load() != 2 {
		t.Fatalf("response=%#v attempts=%d err=%v", resp, attempts.Load(), err)
	}
}

func TestUnsafeMethodDoesNotRetryWithoutOptIn(t *testing.T) {
	var attempts atomic.Int64
	client := newTestClient(t, func(*http.Request) (*http.Response, error) {
		attempts.Add(1)
		return jsonResponse(http.StatusServiceUnavailable, `{}`), nil
	})

	resp, err := client.Post("/orders", map[string]string{"item": "book"})
	if err == nil || resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("response=%#v err=%v", resp, err)
	}
	if attempts.Load() != 1 {
		t.Fatalf("attempts = %d", attempts.Load())
	}
}

func TestIdempotencyKeyEnablesDefaultRetry(t *testing.T) {
	var attempts atomic.Int64
	client := newTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Idempotency-Key") != "order-123" {
			t.Fatal("missing idempotency key")
		}
		if attempts.Add(1) == 1 {
			return jsonResponse(http.StatusBadGateway, `{}`), nil
		}
		return jsonResponse(http.StatusOK, `{}`), nil
	})

	resp, err := client.NewRequest(http.MethodPost, "/orders").
		WithIdempotencyKey("order-123").
		JSON(map[string]string{"item": "book"}).
		Do()
	if err != nil || !resp.IsSuccess() || attempts.Load() != 2 {
		t.Fatalf("response=%#v attempts=%d err=%v", resp, attempts.Load(), err)
	}
}

func TestExplicitRetryReplaysRequestBody(t *testing.T) {
	var attempts atomic.Int64
	client := newTestClient(t, func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != `{"name":"grove"}` {
			t.Fatalf("body = %q", body)
		}
		if attempts.Add(1) == 1 {
			return jsonResponse(http.StatusServiceUnavailable, `{}`), nil
		}
		return jsonResponse(http.StatusOK, `{}`), nil
	})

	resp, err := client.NewRequest(http.MethodPost, "/users").
		WithRetry(1, time.Millisecond).
		JSON(map[string]string{"name": "grove"}).
		Do()
	if err != nil || !resp.IsSuccess() || attempts.Load() != 2 {
		t.Fatalf("response=%#v attempts=%d err=%v", resp, attempts.Load(), err)
	}
}

func TestRetryBackoffHonorsContextCancellation(t *testing.T) {
	var attempts atomic.Int64
	client := newTestClient(t, func(*http.Request) (*http.Response, error) {
		attempts.Add(1)
		return jsonResponse(http.StatusServiceUnavailable, `{}`), nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := client.NewRequest(http.MethodGet, "/slow").
		WithRetry(3, time.Second).
		DoWithContext(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v", err)
	}
	if attempts.Load() != 1 {
		t.Fatalf("attempts = %d", attempts.Load())
	}
}

func TestReaderBodyReportsNotReplayable(t *testing.T) {
	var attempts atomic.Int64
	client := newTestClient(t, func(req *http.Request) (*http.Response, error) {
		attempts.Add(1)
		_, _ = io.Copy(io.Discard, req.Body)
		return jsonResponse(http.StatusServiceUnavailable, `{}`), nil
	})

	resp, err := client.NewRequest(http.MethodGet, "/reader").
		Body(io.LimitReader(strings.NewReader("payload"), 7)).
		Do()
	if !errors.Is(err, ErrBodyNotReplayable) || resp == nil {
		t.Fatalf("response=%#v err=%v", resp, err)
	}
	if attempts.Load() != 1 {
		t.Fatalf("attempts = %d", attempts.Load())
	}
}

func TestResponseBodyLimit(t *testing.T) {
	client := newTestClient(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, "12345"), nil
	})

	resp, err := client.NewRequest(http.MethodGet, "/large").
		MaxResponseBytes(4).
		Do()
	if !errors.Is(err, ErrResponseTooLarge) || resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("response=%#v err=%v", resp, err)
	}
	if got := string(resp.Body); got != "1234" {
		t.Fatalf("limited body = %q", got)
	}
}

func TestFinalServerErrorRetainsResponse(t *testing.T) {
	client := newTestClient(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusServiceUnavailable, `{"error":"busy"}`), nil
	})

	resp, err := client.NewRequest(http.MethodGet, "/busy").WithRetry(1, time.Millisecond).Do()
	if err == nil || resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("response=%#v err=%v", resp, err)
	}
	if !strings.Contains(resp.String(), "busy") {
		t.Fatalf("body = %q", resp.String())
	}
}

func TestClientErrorDoesNotRetry(t *testing.T) {
	var attempts atomic.Int64
	client := newTestClient(t, func(*http.Request) (*http.Response, error) {
		attempts.Add(1)
		return jsonResponse(http.StatusBadRequest, `{"error":"bad"}`), nil
	})

	resp, err := client.Get("/bad")
	if err != nil || resp.StatusCode != http.StatusBadRequest || attempts.Load() != 1 {
		t.Fatalf("response=%#v attempts=%d err=%v", resp, attempts.Load(), err)
	}
}

func TestPostFormAndQuery(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Query().Get("source") != "console" {
			t.Fatal("missing query")
		}
		if err := req.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if req.FormValue("username") != "grove" {
			t.Fatalf("form = %#v", req.Form)
		}
		return jsonResponse(http.StatusOK, `{}`), nil
	})

	resp, err := client.NewRequest(http.MethodPost, "/login").
		WithQueryParam("source", "console").
		Form(map[string]string{"username": "grove"}).
		Do()
	if err != nil || !resp.IsSuccess() {
		t.Fatalf("response=%#v err=%v", resp, err)
	}
}

func TestMultipartFilePathOpensAtSendTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "avatar.txt")
	if err := os.WriteFile(path, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	client := newTestClient(t, func(req *http.Request) (*http.Response, error) {
		if _, ok := req.Body.(*io.PipeReader); !ok {
			t.Fatalf("multipart body type = %T, expected streaming pipe", req.Body)
		}
		mediaType, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
		if err != nil || mediaType != "multipart/form-data" {
			t.Fatalf("content type = %q err=%v", mediaType, err)
		}
		form, err := multipart.NewReader(req.Body, params["boundary"]).ReadForm(1024)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := form.RemoveAll(); err != nil {
				t.Errorf("remove multipart form: %v", err)
			}
		}()
		if got := form.Value["name"]; len(got) != 1 || got[0] != "grove" {
			t.Fatalf("fields = %#v", form.Value)
		}
		file, err := form.File["avatar"][0].Open()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				t.Errorf("close multipart file: %v", err)
			}
		}()
		content, _ := io.ReadAll(file)
		if string(content) != "hello" {
			t.Fatalf("content = %q", content)
		}
		return jsonResponse(http.StatusOK, `{}`), nil
	})

	builder := client.NewRequest(http.MethodPost, "/upload").
		Form(map[string]string{"name": "grove"}).
		AddFileFromPath("avatar", path)
	resp, err := builder.Do()
	if err != nil || !resp.IsSuccess() {
		t.Fatalf("response=%#v err=%v", resp, err)
	}

	missing := client.NewRequest(http.MethodPost, "/upload").AddFileFromPath("avatar", path)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := missing.Do(); err == nil {
		t.Fatal("expected send-time file open error")
	}
}

func TestMultipartRejectsConflictingBody(t *testing.T) {
	client := newTestClient(t, func(*http.Request) (*http.Response, error) {
		t.Fatal("transport must not run")
		return nil, nil
	})

	_, err := client.NewRequest(http.MethodPost, "/upload").
		JSON(map[string]string{"name": "grove"}).
		AddFile("avatar", "avatar.txt", []byte("hello")).
		Do()
	if err == nil {
		t.Fatal("expected conflicting multipart body error")
	}
}

func TestDownloadToFileCleansPartialFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "download.bin")
	if err := os.WriteFile(target, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	client := newTestClient(t, func(*http.Request) (*http.Response, error) {
		return textResponse(http.StatusOK, &failingReadCloser{reader: strings.NewReader("partial")}), nil
	})

	err := client.DownloadToFile("/download", target)
	if err == nil {
		t.Fatal("expected download error")
	}
	content, readErr := os.ReadFile(target)
	if readErr != nil || string(content) != "original" {
		t.Fatalf("target content=%q err=%v", content, readErr)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, ".download.bin.tmp-*"))
	if len(matches) != 0 {
		t.Fatalf("temporary files remain: %#v", matches)
	}
}

func TestStreamPropagatesHandlerError(t *testing.T) {
	handlerErr := errors.New("stop")
	client := newTestClient(t, func(*http.Request) (*http.Response, error) {
		return textResponse(http.StatusOK, io.NopCloser(strings.NewReader("payload"))), nil
	})

	err := client.NewRequest(http.MethodGet, "/stream").Stream(context.Background(), func([]byte) error {
		return handlerErr
	})
	if !errors.Is(err, handlerErr) {
		t.Fatalf("error = %v", err)
	}
}

func TestResponseHelpers(t *testing.T) {
	resp := &Response{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"X-Test": []string{"value"}},
		Body:       []byte(`{"ok":true}`),
	}
	var payload map[string]bool
	if !resp.IsSuccess() || resp.IsError() || resp.Header("X-Test") != "value" {
		t.Fatalf("unexpected response helpers: %#v", resp)
	}
	if err := resp.JSON(&payload); err != nil || !payload["ok"] {
		t.Fatalf("payload=%#v err=%v", payload, err)
	}
	if string(resp.Bytes()) != resp.String() {
		t.Fatal("byte and string response differ")
	}
}

type failingReadCloser struct {
	reader *strings.Reader
}

func (r *failingReadCloser) Read(p []byte) (int, error) {
	if r.reader.Len() == 0 {
		return 0, errors.New("read failed")
	}
	return r.reader.Read(p)
}

func (*failingReadCloser) Close() error { return nil }
