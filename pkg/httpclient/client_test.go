package httpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func newTestClient(t *testing.T, handler func(*http.Request) (*http.Response, error)) *Client {
	t.Helper()

	client := New().BaseURL("https://example.test")
	client.httpClient.Transport = roundTripFunc(handler)
	return client
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestNew(t *testing.T) {
	client := New()
	if client == nil {
		t.Fatal("expected client to be non-nil")
	}
	if client.httpClient.Timeout != 30*time.Second {
		t.Fatalf("expected timeout 30s, got %v", client.httpClient.Timeout)
	}
}

func TestNewWithConfig(t *testing.T) {
	config := Config{
		BaseURL:    "https://api.example.com",
		Timeout:    60 * time.Second,
		RetryCount: 3,
		RetryDelay: 2 * time.Second,
		Headers: map[string]string{
			"Authorization": "Bearer token",
		},
	}

	client := NewWithConfig(config)
	if client.baseURL != "https://api.example.com" {
		t.Fatalf("expected baseURL, got %s", client.baseURL)
	}
	if client.httpClient.Timeout != 60*time.Second {
		t.Fatalf("expected timeout 60s, got %v", client.httpClient.Timeout)
	}
	if client.retryCount != 3 {
		t.Fatalf("expected retry count 3, got %d", client.retryCount)
	}
	if client.headers["Authorization"] != "Bearer token" {
		t.Fatalf("expected auth header")
	}
}

func TestClientGet(t *testing.T) {
	client := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/users" {
			t.Errorf("expected /users, got %s", r.URL.Path)
		}
		return jsonResponse(http.StatusOK, `{"message":"success"}`), nil
	})
	resp, err := client.Get("/users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.IsSuccess() {
		t.Fatalf("expected success, got %d", resp.StatusCode)
	}

	var result map[string]string
	if err := resp.JSON(&result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if result["message"] != "success" {
		t.Fatalf("unexpected response: %v", result)
	}
}

func TestClientPost(t *testing.T) {
	client := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// 读取请求体
		body, _ := io.ReadAll(r.Body)
		var data map[string]any
		json.Unmarshal(body, &data)

		if data["name"] != "John" {
			t.Errorf("expected name John, got %v", data["name"])
		}

		return jsonResponse(http.StatusCreated, `{"id":1,"name":"John"}`), nil
	})
	payload := map[string]string{"name": "John", "email": "john@example.com"}
	resp, err := client.Post("/users", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := resp.JSON(&result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if result["id"] != float64(1) {
		t.Fatalf("unexpected id: %v", result["id"])
	}
}

func TestClientWithQueryParams(t *testing.T) {
	client := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		query := r.URL.Query()
		if query.Get("page") != "1" {
			t.Errorf("expected page=1, got %s", query.Get("page"))
		}
		if query.Get("limit") != "10" {
			t.Errorf("expected limit=10, got %s", query.Get("limit"))
		}
		return jsonResponse(http.StatusOK, `{}`), nil
	}).WithQueryParam("page", "1").
		WithQueryParam("page", "1").
		WithQueryParam("limit", "10")

	_, err := client.Get("/users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientWithHeaders(t *testing.T) {
	client := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %s", auth)
		}
		return jsonResponse(http.StatusOK, `{}`), nil
	}).WithHeader("Authorization", "Bearer test-token")

	_, err := client.Get("/users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResponseIsSuccess(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{200, true},
		{201, true},
		{204, true},
		{301, false},
		{400, false},
		{500, false},
	}

	for _, test := range tests {
		resp := &Response{StatusCode: test.code}
		if resp.IsSuccess() != test.expected {
			t.Errorf("IsSuccess() for status %d: expected %v, got %v",
				test.code, test.expected, resp.IsSuccess())
		}
	}
}

func TestResponseIsError(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{200, false},
		{301, false},
		{400, true},
		{404, true},
		{500, true},
	}

	for _, test := range tests {
		resp := &Response{StatusCode: test.code}
		if resp.IsError() != test.expected {
			t.Errorf("IsError() for status %d: expected %v, got %v",
				test.code, test.expected, resp.IsError())
		}
	}
}

func TestClientPostForm(t *testing.T) {
	client := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("failed to parse form: %v", err)
		}
		if r.FormValue("username") != "john" {
			t.Errorf("expected username john, got %s", r.FormValue("username"))
		}
		return jsonResponse(http.StatusOK, `{}`), nil
	})
	data := map[string]string{
		"username": "john",
		"password": "secret",
	}
	resp, err := client.PostForm("/login", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.IsSuccess() {
		t.Fatalf("expected success, got %d", resp.StatusCode)
	}
}

func TestClientWithContext(t *testing.T) {
	client := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		select {
		case <-time.After(100 * time.Millisecond):
			return jsonResponse(http.StatusOK, `{}`), nil
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := client.GetWithContext(ctx, "/slow")
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestClientRetry(t *testing.T) {
	attemptCount := 0
	client := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		attemptCount++
		if attemptCount < 3 {
			return jsonResponse(http.StatusServiceUnavailable, `{}`), nil
		}
		return jsonResponse(http.StatusOK, `{}`), nil
	}).WithRetry(3, 10*time.Millisecond)

	resp, err := client.Get("/flaky")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.IsSuccess() {
		t.Fatalf("expected success after retry, got %d", resp.StatusCode)
	}
	if attemptCount != 3 {
		t.Fatalf("expected 3 attempts, got %d", attemptCount)
	}
}

func TestClientClone(t *testing.T) {
	client := New().
		BaseURL("https://api.example.com").
		WithHeader("X-API-Key", "secret").
		WithQueryParam("lang", "zh-CN").
		WithRetry(3, 1*time.Second)

	cloned := client.Clone()

	// 修改克隆后的客户端
	cloned.WithHeader("X-Custom", "value")

	// 原始客户端不应该受影响
	if _, ok := client.headers["X-Custom"]; ok {
		t.Fatal("original client should not have X-Custom header")
	}

	// 基础配置应该保留
	if cloned.baseURL != client.baseURL {
		t.Fatal("cloned client should have same baseURL")
	}
	if cloned.headers["X-API-Key"] != "secret" {
		t.Fatal("cloned client should have X-API-Key header")
	}
	if cloned.queryParams["lang"] != "zh-CN" {
		t.Fatal("cloned client should copy query params")
	}
}

func TestRequestBuilder(t *testing.T) {
	client := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer token" {
			t.Errorf("expected auth header, got %s", auth)
		}

		body, _ := io.ReadAll(r.Body)
		var data map[string]any
		json.Unmarshal(body, &data)

		if data["name"] != "test" {
			t.Errorf("expected name test, got %v", data["name"])
		}

		return jsonResponse(http.StatusOK, `{"name":"test"}`), nil
	})
	resp, err := client.NewRequest(http.MethodPost, "/users").
		WithHeader("Authorization", "Bearer token").
		JSON(map[string]string{"name": "test"}).
		Do()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.IsSuccess() {
		t.Fatalf("expected success, got %d", resp.StatusCode)
	}
}

func ExampleClient_Get() {
	// 使用客户端
	client := New().BaseURL("https://example.test")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"message": "Hello, World!"}`), nil
	})
	resp, err := client.Get("/hello")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Body: %s", resp.String())
	// Output:
	// Status: 200
	// Body: {"message": "Hello, World!"}
	//
}

func TestClientRequestReturnsTransportError(t *testing.T) {
	client := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("boom")
	})

	_, err := client.Get("/users")
	if err == nil {
		t.Fatal("expected transport error")
	}
}
