package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zhimma/grove/pkg/logger"
)

// ==================== 核心类型 ====================

// Client HTTP客户端
type Client struct {
	baseURL       string
	httpClient    *http.Client
	headers       map[string]string
	queryParams   map[string]string
	retryCount    int
	retryDelay    time.Duration
	beforeRequest []BeforeRequestFunc
	afterResponse []AfterResponseFunc
}

// BeforeRequestFunc 请求前钩子
type BeforeRequestFunc func(req *http.Request) error

// AfterResponseFunc 响应后钩子
type AfterResponseFunc func(resp *Response) error

// Response HTTP响应包装
type Response struct {
	StatusCode int
	Status     string
	Headers    http.Header
	Body       []byte
	Request    *http.Request
}

// Config HTTP客户端配置
type Config struct {
	BaseURL    string
	Timeout    time.Duration
	Headers    map[string]string
	RetryCount int
	RetryDelay time.Duration
}

// FileField 文件字段
type FileField struct {
	FieldName string
	FileName  string
	Content   []byte
	FilePath  string
	Header    textproto.MIMEHeader
}

// RequestBuilder 链式请求构建器
type RequestBuilder struct {
	client *Client
	method string
	path   string
	body   any
	files  map[string]FileField
}

// ==================== 构造函数 ====================

// New 创建新的HTTP客户端
func New() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		headers:     make(map[string]string),
		queryParams: make(map[string]string),
		retryCount:  0,
		retryDelay:  1 * time.Second,
	}
}

// NewWithConfig 使用配置创建客户端
func NewWithConfig(config Config) *Client {
	client := New()
	if config.BaseURL != "" {
		client.baseURL = config.BaseURL
	}
	if config.Timeout > 0 {
		client.httpClient.Timeout = config.Timeout
	}
	if config.RetryCount > 0 {
		client.retryCount = config.RetryCount
	}
	if config.RetryDelay > 0 {
		client.retryDelay = config.RetryDelay
	}
	for k, v := range config.Headers {
		client.headers[k] = v
	}
	return client
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		Timeout:    30 * time.Second,
		RetryCount: 0,
		RetryDelay: 1 * time.Second,
		Headers:    make(map[string]string),
	}
}

// ==================== 配置方法 ====================

// BaseURL 设置基础URL
func (c *Client) BaseURL(url string) *Client {
	c.baseURL = strings.TrimRight(url, "/")
	return c
}

// Timeout 设置超时时间
func (c *Client) Timeout(timeout time.Duration) *Client {
	c.httpClient.Timeout = timeout
	return c
}

// WithHeader 设置请求头
func (c *Client) WithHeader(key, value string) *Client {
	c.headers[key] = value
	return c
}

// WithHeaders 批量设置请求头
func (c *Client) WithHeaders(headers map[string]string) *Client {
	for k, v := range headers {
		c.headers[k] = v
	}
	return c
}

// WithQueryParam 设置查询参数
func (c *Client) WithQueryParam(key, value string) *Client {
	c.queryParams[key] = value
	return c
}

// WithQueryParams 批量设置查询参数
func (c *Client) WithQueryParams(params map[string]string) *Client {
	for k, v := range params {
		c.queryParams[k] = v
	}
	return c
}

// WithRetry 设置重试策略
func (c *Client) WithRetry(count int, delay time.Duration) *Client {
	c.retryCount = count
	c.retryDelay = delay
	return c
}

// BeforeRequest 添加请求前钩子
func (c *Client) BeforeRequest(fn BeforeRequestFunc) *Client {
	c.beforeRequest = append(c.beforeRequest, fn)
	return c
}

// AfterResponse 添加响应后钩子
func (c *Client) AfterResponse(fn AfterResponseFunc) *Client {
	c.afterResponse = append(c.afterResponse, fn)
	return c
}

// Clone 克隆客户端（保留配置）
func (c *Client) Clone() *Client {
	cloned := New()
	cloned.baseURL = c.baseURL
	cloned.httpClient.Timeout = c.httpClient.Timeout
	cloned.httpClient.Transport = c.httpClient.Transport
	cloned.retryCount = c.retryCount
	cloned.retryDelay = c.retryDelay

	for k, v := range c.headers {
		cloned.headers[k] = v
	}
	for k, v := range c.queryParams {
		cloned.queryParams[k] = v
	}
	cloned.beforeRequest = c.beforeRequest
	cloned.afterResponse = c.afterResponse

	return cloned
}

// ==================== HTTP 方法 ====================

// Get 发送GET请求
func (c *Client) Get(path string) (*Response, error) {
	return c.Request(context.Background(), http.MethodGet, path, nil)
}

// GetWithContext 发送带上下文的GET请求
func (c *Client) GetWithContext(ctx context.Context, path string) (*Response, error) {
	return c.Request(ctx, http.MethodGet, path, nil)
}

// Post 发送POST请求
func (c *Client) Post(path string, body any) (*Response, error) {
	return c.Request(context.Background(), http.MethodPost, path, body)
}

// PostWithContext 发送带上下文的POST请求
func (c *Client) PostWithContext(ctx context.Context, path string, body any) (*Response, error) {
	return c.Request(ctx, http.MethodPost, path, body)
}

// Put 发送PUT请求
func (c *Client) Put(path string, body any) (*Response, error) {
	return c.Request(context.Background(), http.MethodPut, path, body)
}

// PutWithContext 发送带上下文的PUT请求
func (c *Client) PutWithContext(ctx context.Context, path string, body any) (*Response, error) {
	return c.Request(ctx, http.MethodPut, path, body)
}

// Patch 发送PATCH请求
func (c *Client) Patch(path string, body any) (*Response, error) {
	return c.Request(context.Background(), http.MethodPatch, path, body)
}

// PatchWithContext 发送带上下文的PATCH请求
func (c *Client) PatchWithContext(ctx context.Context, path string, body any) (*Response, error) {
	return c.Request(ctx, http.MethodPatch, path, body)
}

// Delete 发送DELETE请求
func (c *Client) Delete(path string) (*Response, error) {
	return c.Request(context.Background(), http.MethodDelete, path, nil)
}

// DeleteWithContext 发送带上下文的DELETE请求
func (c *Client) DeleteWithContext(ctx context.Context, path string) (*Response, error) {
	return c.Request(ctx, http.MethodDelete, path, nil)
}

// Request 发送HTTP请求
func (c *Client) Request(ctx context.Context, method, path string, body any) (*Response, error) {
	fullURL := c.buildURL(path)

	// 构建请求体
	var bodyBytes []byte
	var bodyReader io.Reader
	var contentType string

	if body != nil {
		switch v := body.(type) {
		case string:
			bodyBytes = []byte(v)
		case []byte:
			bodyBytes = v
		case io.Reader:
			var err error
			bodyBytes, err = io.ReadAll(v)
			if err != nil {
				return nil, fmt.Errorf("read request body: %w", err)
			}
		default:
			// 默认JSON编码
			jsonBody, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("marshal request body: %w", err)
			}
			bodyBytes = jsonBody
			contentType = "application/json"
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if bodyBytes != nil {
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(bodyBytes)), nil
		}
	}

	// 设置请求头
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	if contentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", contentType)
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}

	// 执行请求前钩子
	for _, fn := range c.beforeRequest {
		if err := fn(req); err != nil {
			return nil, fmt.Errorf("before request hook: %w", err)
		}
	}

	// 发送请求（带重试）
	resp, err := c.doWithRetry(req)
	if err != nil {
		return nil, err
	}

	// 执行响应后钩子
	for _, fn := range c.afterResponse {
		if err := fn(resp); err != nil {
			return nil, fmt.Errorf("after response hook: %w", err)
		}
	}

	return resp, nil
}

// doWithRetry 带重试的请求执行
func (c *Client) doWithRetry(req *http.Request) (*Response, error) {
	var lastErr error

	for i := 0; i <= c.retryCount; i++ {
		attemptReq := req
		if i > 0 && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("reset request body: %w", err)
			}
			attemptReq = req.Clone(req.Context())
			attemptReq.Body = body
			attemptReq.GetBody = req.GetBody
		}
		if i > 0 {
			logger.Debug().
				Str("url", req.URL.String()).
				Int("attempt", i).
				Msg("请求重试中")
			time.Sleep(c.retryDelay * time.Duration(i))
		}

		resp, err := c.do(attemptReq)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// 如果是客户端错误（4xx），不重试
		if resp != nil && resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return resp, err
		}
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", c.retryCount, lastErr)
}

// do 执行单次请求
func (c *Client) do(req *http.Request) (*Response, error) {
	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	resp := &Response{
		StatusCode: httpResp.StatusCode,
		Status:     httpResp.Status,
		Headers:    httpResp.Header,
		Body:       body,
		Request:    req,
	}

	// 5xx 错误视为可重试错误
	if httpResp.StatusCode >= 500 {
		return resp, fmt.Errorf("server error: %d", httpResp.StatusCode)
	}

	return resp, nil
}

// buildURL 构建完整URL
func (c *Client) buildURL(path string) string {
	// 如果已经是完整URL
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return c.appendQueryParams(path)
	}

	// 拼接基础URL
	fullURL := c.baseURL
	if fullURL == "" {
		fullURL = path
	} else {
		path = strings.TrimLeft(path, "/")
		fullURL = fmt.Sprintf("%s/%s", fullURL, path)
	}

	return c.appendQueryParams(fullURL)
}

// appendQueryParams 添加查询参数
func (c *Client) appendQueryParams(urlStr string) string {
	if len(c.queryParams) == 0 {
		return urlStr
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return urlStr
	}

	q := u.Query()
	for k, v := range c.queryParams {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	return u.String()
}

// ==================== RequestBuilder 方法 ====================

// NewRequest 创建新的请求构建器
func (c *Client) NewRequest(method, path string) *RequestBuilder {
	return &RequestBuilder{
		client: c.Clone(),
		method: method,
		path:   path,
		files:  make(map[string]FileField),
	}
}

// Body 设置请求体
func (rb *RequestBuilder) Body(body any) *RequestBuilder {
	rb.body = body
	return rb
}

// JSON 设置JSON请求体
func (rb *RequestBuilder) JSON(v any) *RequestBuilder {
	rb.client.WithHeader("Content-Type", "application/json")
	rb.body = v
	return rb
}

// Form 设置表单请求体（application/x-www-form-urlencoded）
func (rb *RequestBuilder) Form(data map[string]string) *RequestBuilder {
	rb.client.WithHeader("Content-Type", "application/x-www-form-urlencoded")

	// 构建表单数据
	formValues := url.Values{}
	for k, v := range data {
		formValues.Set(k, v)
	}
	rb.body = formValues.Encode()
	return rb
}

// AddFile 添加文件（从字节）
func (rb *RequestBuilder) AddFile(fieldName, fileName string, content []byte) *RequestBuilder {
	rb.files[fieldName] = FileField{
		FieldName: fieldName,
		FileName:  fileName,
		Content:   content,
	}
	return rb
}

// AddFileFromPath 添加文件（从路径）
func (rb *RequestBuilder) AddFileFromPath(fieldName, filePath string) *RequestBuilder {
	content, err := os.ReadFile(filePath)
	if err != nil {
		// 保存错误，在 Do 时返回
		rb.body = fmt.Errorf("read file %s: %w", filePath, err)
		return rb
	}

	fileName := filepath.Base(filePath)
	rb.files[fieldName] = FileField{
		FieldName: fieldName,
		FileName:  fileName,
		Content:   content,
		FilePath:  filePath,
	}
	return rb
}

// WithHeader 设置请求头
func (rb *RequestBuilder) WithHeader(key, value string) *RequestBuilder {
	rb.client.WithHeader(key, value)
	return rb
}

// WithQueryParam 设置查询参数
func (rb *RequestBuilder) WithQueryParam(key, value string) *RequestBuilder {
	rb.client.WithQueryParam(key, value)
	return rb
}

// Do 执行请求
func (rb *RequestBuilder) Do() (*Response, error) {
	// 检查之前的错误
	if err, ok := rb.body.(error); ok {
		return nil, err
	}

	// 如果有文件，构建 multipart 请求
	if len(rb.files) > 0 {
		return rb.doMultipart(context.Background())
	}

	return rb.client.Request(context.Background(), rb.method, rb.path, rb.body)
}

// DoWithContext 带上下文执行请求
func (rb *RequestBuilder) DoWithContext(ctx context.Context) (*Response, error) {
	// 检查之前的错误
	if err, ok := rb.body.(error); ok {
		return nil, err
	}

	// 如果有文件，构建 multipart 请求
	if len(rb.files) > 0 {
		return rb.doMultipart(ctx)
	}

	return rb.client.Request(ctx, rb.method, rb.path, rb.body)
}

// doMultipart 执行 multipart 请求
func (rb *RequestBuilder) doMultipart(ctx context.Context) (*Response, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// 添加文件
	for _, file := range rb.files {
		part, err := writer.CreateFormFile(file.FieldName, file.FileName)
		if err != nil {
			return nil, fmt.Errorf("create form file: %w", err)
		}
		if _, err := part.Write(file.Content); err != nil {
			return nil, fmt.Errorf("write file content: %w", err)
		}
	}

	// 添加其他表单字段（如果 body 是 map）
	if formData, ok := rb.body.(map[string]string); ok {
		for key, value := range formData {
			if err := writer.WriteField(key, value); err != nil {
				return nil, fmt.Errorf("write form field: %w", err)
			}
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	// 设置 content-type
	rb.client.WithHeader("Content-Type", writer.FormDataContentType())

	return rb.client.Request(ctx, rb.method, rb.path, &body)
}

// ==================== 便捷方法 ====================

// PostForm 发送表单请求（application/x-www-form-urlencoded）
func (c *Client) PostForm(path string, data map[string]string) (*Response, error) {
	return c.NewRequest(http.MethodPost, path).Form(data).Do()
}

// PostFormWithContext 发送带上下文的表单请求
func (c *Client) PostFormWithContext(ctx context.Context, path string, data map[string]string) (*Response, error) {
	return c.NewRequest(http.MethodPost, path).Form(data).DoWithContext(ctx)
}

// PostMultipart 发送 multipart 请求（文件上传）
func (c *Client) PostMultipart(path string, fields map[string]string, files map[string]FileField) (*Response, error) {
	builder := c.NewRequest(http.MethodPost, path)

	builder.Body(fields)

	// 添加文件
	for _, file := range files {
		builder.files[file.FieldName] = file
	}

	return builder.Do()
}

// Download 下载文件
func (c *Client) Download(path string) (*Response, error) {
	resp, err := c.Get(path)
	if err != nil {
		return nil, err
	}

	if resp.IsError() {
		return resp, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	return resp, nil
}

// DownloadToFile 下载文件到本地
func (c *Client) DownloadToFile(path, localPath string) error {
	resp, err := c.Download(path)
	if err != nil {
		return err
	}

	// 创建目录
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(localPath, resp.Body, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// Stream 流式请求（大文件下载）
func (c *Client) Stream(ctx context.Context, method, path string, body any, handler func(chunk []byte) error) error {
	fullURL := c.buildURL(path)

	var bodyReader io.Reader
	if body != nil {
		switch v := body.(type) {
		case string:
			bodyReader = strings.NewReader(v)
		case []byte:
			bodyReader = bytes.NewReader(v)
		case io.Reader:
			bodyReader = v
		default:
			jsonBody, err := json.Marshal(body)
			if err != nil {
				return fmt.Errorf("marshal request body: %w", err)
			}
			bodyReader = bytes.NewReader(jsonBody)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// 设置请求头
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// 流式读取
	buffer := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if err := handler(buffer[:n]); err != nil {
				return fmt.Errorf("handle chunk: %w", err)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read response body: %w", err)
		}
	}

	return nil
}

// ==================== Response 方法 ====================

// IsSuccess 是否成功响应（2xx）
func (r *Response) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// IsError 是否错误响应（4xx/5xx）
func (r *Response) IsError() bool {
	return r.StatusCode >= 400
}

// JSON 解析JSON响应
func (r *Response) JSON(v any) error {
	if len(r.Body) == 0 {
		return nil
	}
	return json.Unmarshal(r.Body, v)
}

// XML 解析XML响应
func (r *Response) XML(v any) error {
	if len(r.Body) == 0 {
		return nil
	}
	return xml.Unmarshal(r.Body, v)
}

// String 获取响应文本
func (r *Response) String() string {
	return string(r.Body)
}

// Bytes 获取响应字节
func (r *Response) Bytes() []byte {
	return r.Body
}

// Header 获取响应头
func (r *Response) Header(key string) string {
	return r.Headers.Get(key)
}
