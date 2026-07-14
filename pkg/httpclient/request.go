package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrResponseTooLarge  = errors.New("http response body exceeds limit")
	ErrBodyNotReplayable = errors.New("http request body is not replayable")
)

type BeforeRequestFunc func(req *http.Request) error

type AfterResponseFunc func(resp *Response) error

type FileField struct {
	FieldName string
	FileName  string
	Content   []byte
	FilePath  string
	Header    textproto.MIMEHeader
}

type RequestBuilder struct {
	client           *Client
	method           string
	path             string
	headers          http.Header
	query            url.Values
	body             requestBody
	form             url.Values
	files            []FileField
	retry            retryPolicy
	idempotencyKey   string
	maxResponseBytes int64
	beforeRequest    []BeforeRequestFunc
	afterResponse    []AfterResponseFunc
	err              error
}

type requestBody interface {
	Open() (body io.ReadCloser, contentType string, contentLength int64, err error)
}

type bytesBody struct {
	data        []byte
	contentType string
}

func (b bytesBody) Open() (io.ReadCloser, string, int64, error) {
	return io.NopCloser(bytes.NewReader(b.data)), b.contentType, int64(len(b.data)), nil
}

type readerBody struct {
	mu     sync.Mutex
	reader io.Reader
	used   bool
}

func (b *readerBody) Open() (io.ReadCloser, string, int64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.used {
		return nil, "", 0, ErrBodyNotReplayable
	}
	b.used = true
	if closer, ok := b.reader.(io.ReadCloser); ok {
		return closer, "", -1, nil
	}
	return io.NopCloser(b.reader), "", -1, nil
}

func (c *Client) NewRequest(method, path string) *RequestBuilder {
	if c == nil {
		c = New()
	}
	return &RequestBuilder{
		client:  c,
		method:  strings.ToUpper(strings.TrimSpace(method)),
		path:    strings.TrimSpace(path),
		headers: make(http.Header),
		query:   make(url.Values),
		retry: retryPolicy{
			maxRetries: defaultRetryCount,
			delay:      defaultRetryDelay,
		},
		maxResponseBytes: DefaultMaxResponseBodyBytes,
	}
}

func (rb *RequestBuilder) Body(value any) *RequestBuilder {
	if rb.err != nil {
		return rb
	}
	if len(rb.files) > 0 && value != nil {
		rb.err = fmt.Errorf("multipart files cannot be combined with a regular request body")
		return rb
	}
	rb.form = nil
	switch value := value.(type) {
	case nil:
		rb.body = nil
	case string:
		rb.body = bytesBody{data: []byte(value)}
	case []byte:
		rb.body = bytesBody{data: append([]byte(nil), value...)}
	case io.Reader:
		rb.body = &readerBody{reader: value}
	default:
		return rb.JSON(value)
	}
	return rb
}

func (rb *RequestBuilder) JSON(value any) *RequestBuilder {
	if rb.err != nil {
		return rb
	}
	if len(rb.files) > 0 {
		rb.err = fmt.Errorf("multipart files cannot be combined with a JSON request body")
		return rb
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		rb.err = fmt.Errorf("marshal request body: %w", err)
		return rb
	}
	rb.form = nil
	rb.body = bytesBody{data: encoded, contentType: "application/json"}
	return rb
}

func (rb *RequestBuilder) Form(data map[string]string) *RequestBuilder {
	if rb.err != nil {
		return rb
	}
	values := make(url.Values, len(data))
	for key, value := range data {
		values.Set(key, value)
	}
	rb.form = values
	rb.body = bytesBody{data: []byte(values.Encode()), contentType: "application/x-www-form-urlencoded"}
	return rb
}

func (rb *RequestBuilder) AddFile(fieldName, fileName string, content []byte) *RequestBuilder {
	return rb.addFile(FileField{
		FieldName: fieldName,
		FileName:  fileName,
		Content:   append([]byte(nil), content...),
	})
}

func (rb *RequestBuilder) AddFileFromPath(fieldName, filePath string) *RequestBuilder {
	if strings.TrimSpace(filePath) == "" {
		rb.err = fmt.Errorf("multipart file path is required")
		return rb
	}
	return rb.addFile(FileField{
		FieldName: fieldName,
		FileName:  filepath.Base(filePath),
		FilePath:  filePath,
	})
}

func (rb *RequestBuilder) addFile(file FileField) *RequestBuilder {
	if rb.err != nil {
		return rb
	}
	if rb.body != nil && rb.form == nil {
		rb.err = fmt.Errorf("multipart files cannot be combined with a regular request body")
		return rb
	}
	file.FieldName = strings.TrimSpace(file.FieldName)
	file.FileName = strings.TrimSpace(file.FileName)
	file.FilePath = strings.TrimSpace(file.FilePath)
	if file.FieldName == "" || file.FileName == "" {
		rb.err = fmt.Errorf("multipart field and file name are required")
		return rb
	}
	if strings.ContainsAny(file.FieldName, "\r\n") || strings.ContainsAny(file.FileName, "\r\n") {
		rb.err = fmt.Errorf("multipart field and file name must not contain line breaks")
		return rb
	}
	if file.FilePath == "" {
		file.Content = append([]byte(nil), file.Content...)
	}
	file.Header = cloneMIMEHeader(file.Header)
	rb.files = append(rb.files, file)
	return rb
}

func (rb *RequestBuilder) WithHeader(key, value string) *RequestBuilder {
	if rb.err == nil {
		rb.headers.Set(key, value)
	}
	return rb
}

func (rb *RequestBuilder) WithHeaders(headers map[string]string) *RequestBuilder {
	for key, value := range headers {
		rb.WithHeader(key, value)
	}
	return rb
}

func (rb *RequestBuilder) WithQueryParam(key, value string) *RequestBuilder {
	if rb.err == nil {
		rb.query.Set(key, value)
	}
	return rb
}

func (rb *RequestBuilder) WithQueryParams(params map[string]string) *RequestBuilder {
	for key, value := range params {
		rb.WithQueryParam(key, value)
	}
	return rb
}

func (rb *RequestBuilder) WithRetry(count int, delay time.Duration) *RequestBuilder {
	if count < 0 || delay < 0 {
		rb.err = fmt.Errorf("retry count and delay must not be negative")
		return rb
	}
	rb.retry = retryPolicy{maxRetries: count, delay: delay, explicit: true}
	return rb
}

func (rb *RequestBuilder) WithIdempotencyKey(key string) *RequestBuilder {
	key = strings.TrimSpace(key)
	if key == "" {
		rb.err = fmt.Errorf("idempotency key is required")
		return rb
	}
	rb.idempotencyKey = key
	return rb
}

func (rb *RequestBuilder) MaxResponseBytes(limit int64) *RequestBuilder {
	if limit <= 0 {
		rb.err = fmt.Errorf("response body limit must be positive")
		return rb
	}
	rb.maxResponseBytes = limit
	return rb
}

func (rb *RequestBuilder) BeforeRequest(fn BeforeRequestFunc) *RequestBuilder {
	if fn != nil {
		rb.beforeRequest = append(rb.beforeRequest, fn)
	}
	return rb
}

func (rb *RequestBuilder) AfterResponse(fn AfterResponseFunc) *RequestBuilder {
	if fn != nil {
		rb.afterResponse = append(rb.afterResponse, fn)
	}
	return rb
}

func (rb *RequestBuilder) Do() (*Response, error) {
	return rb.DoWithContext(context.Background())
}

func (rb *RequestBuilder) DoWithContext(ctx context.Context) (*Response, error) {
	if rb == nil {
		return nil, fmt.Errorf("http request is nil")
	}
	if rb.err != nil {
		return nil, rb.err
	}
	ctx, cancel := rb.withTimeout(ctx)
	defer cancel()
	return rb.doWithRetry(ctx)
}

func (rb *RequestBuilder) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if rb.client.timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, rb.client.timeout)
}

func (rb *RequestBuilder) buildHTTPRequest(ctx context.Context) (*http.Request, error) {
	if rb.method == "" {
		return nil, fmt.Errorf("http method is required")
	}
	fullURL, err := rb.buildURL()
	if err != nil {
		return nil, err
	}
	bodySource := rb.body
	if len(rb.files) > 0 {
		bodySource = newMultipartBody(rb.form, rb.files)
	}
	var body io.ReadCloser
	var contentType string
	var contentLength int64
	if bodySource != nil {
		body, contentType, contentLength, err = bodySource.Open()
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, rb.method, fullURL, body)
	if err != nil {
		if body != nil {
			_ = body.Close()
		}
		return nil, fmt.Errorf("create request: %w", err)
	}
	if contentLength >= 0 {
		req.ContentLength = contentLength
	}
	req.Header = rb.headers.Clone()
	if contentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", contentType)
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}
	if rb.idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", rb.idempotencyKey)
	}
	for _, hook := range rb.beforeRequest {
		if err := hook(req); err != nil {
			if req.Body != nil {
				_ = req.Body.Close()
			}
			return nil, fmt.Errorf("before request hook: %w", err)
		}
	}
	return req, nil
}

func (rb *RequestBuilder) buildURL() (string, error) {
	requestURL, err := url.Parse(rb.path)
	if err != nil {
		return "", fmt.Errorf("parse request URL: %w", err)
	}
	if !requestURL.IsAbs() {
		if rb.client.baseURL == "" {
			return "", fmt.Errorf("relative request URL requires a base URL")
		}
		baseURL, err := url.Parse(rb.client.baseURL)
		if err != nil {
			return "", fmt.Errorf("parse base URL: %w", err)
		}
		baseURL.Path = strings.TrimRight(baseURL.Path, "/") + "/" + strings.TrimLeft(requestURL.Path, "/")
		baseQuery := baseURL.Query()
		for key, values := range requestURL.Query() {
			baseQuery.Del(key)
			for _, value := range values {
				baseQuery.Add(key, value)
			}
		}
		baseURL.RawQuery = baseQuery.Encode()
		baseURL.Fragment = requestURL.Fragment
		requestURL = baseURL
	}
	query := requestURL.Query()
	for key, values := range rb.query {
		query.Del(key)
		for _, value := range values {
			query.Add(key, value)
		}
	}
	requestURL.RawQuery = query.Encode()
	return requestURL.String(), nil
}

func (rb *RequestBuilder) readResponse(req *http.Request, httpResp *http.Response) (*Response, error) {
	if httpResp.Body == nil {
		httpResp.Body = http.NoBody
	}
	defer httpResp.Body.Close()
	limit := rb.maxResponseBytes
	body, err := io.ReadAll(io.LimitReader(httpResp.Body, limit+1))
	resp := &Response{
		StatusCode: httpResp.StatusCode,
		Status:     httpResp.Status,
		Headers:    httpResp.Header.Clone(),
		Request:    req,
	}
	if int64(len(body)) > limit {
		resp.Body = append([]byte(nil), body[:limit]...)
		return resp, fmt.Errorf("%w: limit %d bytes", ErrResponseTooLarge, limit)
	}
	resp.Body = append([]byte(nil), body...)
	if err != nil {
		return resp, fmt.Errorf("read response body: %w", err)
	}
	return resp, nil
}

func (rb *RequestBuilder) runAfterResponse(resp *Response) error {
	for _, hook := range rb.afterResponse {
		if err := hook(resp); err != nil {
			return fmt.Errorf("after response hook: %w", err)
		}
	}
	return nil
}

func cloneMIMEHeader(header textproto.MIMEHeader) textproto.MIMEHeader {
	if header == nil {
		return nil
	}
	cloned := make(textproto.MIMEHeader, len(header))
	for key, values := range header {
		cloned[key] = append([]string(nil), values...)
	}
	return cloned
}
