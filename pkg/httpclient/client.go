package httpclient

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const (
	DefaultTimeout              = 30 * time.Second
	DefaultMaxResponseBodyBytes = int64(10 << 20)
)

type Client struct {
	transport http.RoundTripper
	baseURL   string
	timeout   time.Duration
	tracing   bool
}

type Config struct {
	BaseURL   string
	Timeout   time.Duration
	Transport http.RoundTripper
}

type Response struct {
	StatusCode int
	Status     string
	Headers    http.Header
	Body       []byte
	Request    *http.Request
}

func New(config Config) *Client {
	if config.Timeout <= 0 {
		config.Timeout = DefaultTimeout
	}
	if config.Transport == nil {
		config.Transport = newDefaultTransport()
	}
	return &Client{
		transport: config.Transport,
		baseURL:   strings.TrimRight(strings.TrimSpace(config.BaseURL), "/"),
		timeout:   config.Timeout,
	}
}

func DefaultConfig() Config {
	return Config{
		Timeout:   DefaultTimeout,
		Transport: newDefaultTransport(),
	}
}

func newDefaultTransport() *http.Transport {
	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
}

func (c *Client) BaseURL(rawURL string) *Client {
	cloned := c.Clone()
	cloned.baseURL = strings.TrimRight(strings.TrimSpace(rawURL), "/")
	return cloned
}

func (c *Client) Timeout(timeout time.Duration) *Client {
	cloned := c.Clone()
	cloned.timeout = timeout
	return cloned
}

func (c *Client) WithTransport(transport http.RoundTripper) *Client {
	cloned := c.Clone()
	if transport == nil {
		transport = newDefaultTransport()
	}
	cloned.transport = transport
	cloned.tracing = false
	return cloned
}

func (c *Client) WithTracing() *Client {
	cloned := c.Clone()
	if cloned.tracing {
		return cloned
	}
	cloned.transport = otelhttp.NewTransport(cloned.transport)
	cloned.tracing = true
	return cloned
}

func (c *Client) Clone() *Client {
	if c == nil {
		return New(DefaultConfig())
	}
	cloned := *c
	return &cloned
}

func (c *Client) Get(path string) (*Response, error) {
	return c.GetWithContext(context.Background(), path)
}

func (c *Client) GetWithContext(ctx context.Context, path string) (*Response, error) {
	return c.NewRequest(http.MethodGet, path).DoWithContext(ctx)
}

func (c *Client) Post(path string, body any) (*Response, error) {
	return c.PostWithContext(context.Background(), path, body)
}

func (c *Client) PostWithContext(ctx context.Context, path string, body any) (*Response, error) {
	return c.NewRequest(http.MethodPost, path).Body(body).DoWithContext(ctx)
}

func (c *Client) Put(path string, body any) (*Response, error) {
	return c.PutWithContext(context.Background(), path, body)
}

func (c *Client) PutWithContext(ctx context.Context, path string, body any) (*Response, error) {
	return c.NewRequest(http.MethodPut, path).Body(body).DoWithContext(ctx)
}

func (c *Client) Patch(path string, body any) (*Response, error) {
	return c.PatchWithContext(context.Background(), path, body)
}

func (c *Client) PatchWithContext(ctx context.Context, path string, body any) (*Response, error) {
	return c.NewRequest(http.MethodPatch, path).Body(body).DoWithContext(ctx)
}

func (c *Client) Delete(path string) (*Response, error) {
	return c.DeleteWithContext(context.Background(), path)
}

func (c *Client) DeleteWithContext(ctx context.Context, path string) (*Response, error) {
	return c.NewRequest(http.MethodDelete, path).DoWithContext(ctx)
}

func (c *Client) Request(ctx context.Context, method, path string, body any) (*Response, error) {
	return c.NewRequest(method, path).Body(body).DoWithContext(ctx)
}

func (c *Client) PostForm(path string, data map[string]string) (*Response, error) {
	return c.PostFormWithContext(context.Background(), path, data)
}

func (c *Client) PostFormWithContext(ctx context.Context, path string, data map[string]string) (*Response, error) {
	return c.NewRequest(http.MethodPost, path).Form(data).DoWithContext(ctx)
}

func (c *Client) PostMultipart(path string, fields map[string]string, files map[string]FileField) (*Response, error) {
	builder := c.NewRequest(http.MethodPost, path).Form(fields)
	for fieldName, file := range files {
		if strings.TrimSpace(file.FieldName) == "" {
			file.FieldName = fieldName
		}
		builder.addFile(file)
	}
	return builder.Do()
}

func (c *Client) Download(path string) (*Response, error) {
	resp, err := c.Get(path)
	if err != nil {
		return resp, err
	}
	if resp.IsError() {
		return resp, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}
	return resp, nil
}

func (r *Response) IsSuccess() bool {
	return r != nil && r.StatusCode >= http.StatusOK && r.StatusCode < http.StatusMultipleChoices
}

func (r *Response) IsError() bool {
	return r != nil && r.StatusCode >= http.StatusBadRequest
}

func (r *Response) JSON(value any) error {
	if r == nil || len(r.Body) == 0 {
		return nil
	}
	return json.Unmarshal(r.Body, value)
}

func (r *Response) XML(value any) error {
	if r == nil || len(r.Body) == 0 {
		return nil
	}
	return xml.Unmarshal(r.Body, value)
}

func (r *Response) String() string {
	if r == nil {
		return ""
	}
	return string(r.Body)
}

func (r *Response) Bytes() []byte {
	if r == nil {
		return nil
	}
	return append([]byte(nil), r.Body...)
}

func (r *Response) Header(key string) string {
	if r == nil {
		return ""
	}
	return r.Headers.Get(key)
}
