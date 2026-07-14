package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

const (
	defaultRetryCount = 2
	defaultRetryDelay = 100 * time.Millisecond
)

type retryPolicy struct {
	maxRetries int
	delay      time.Duration
	explicit   bool
}

func (rb *RequestBuilder) doWithRetry(ctx context.Context) (*Response, error) {
	maxRetries := rb.retry.maxRetries
	if !rb.retryAllowed() {
		maxRetries = 0
	}
	var lastResp *Response
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			if err := waitForRetry(ctx, rb.retry.delay*time.Duration(attempt)); err != nil {
				return lastResp, err
			}
		}
		req, err := rb.buildHTTPRequest(ctx)
		if err != nil {
			return lastResp, err
		}
		httpClient := &http.Client{Transport: rb.client.transport}
		httpResp, err := httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("do request: %w", err)
			if attempt < maxRetries {
				continue
			}
			return lastResp, fmt.Errorf("request failed after %d retries: %w", attempt, lastErr)
		}
		resp, err := rb.readResponse(req, httpResp)
		lastResp = resp
		if err != nil {
			return resp, err
		}
		if resp.StatusCode >= http.StatusInternalServerError {
			lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
			if attempt < maxRetries {
				continue
			}
			if hookErr := rb.runAfterResponse(resp); hookErr != nil {
				return resp, hookErr
			}
			return resp, fmt.Errorf("request failed after %d retries: %w", attempt, lastErr)
		}
		if err := rb.runAfterResponse(resp); err != nil {
			return resp, err
		}
		return resp, nil
	}
	return lastResp, lastErr
}

func (rb *RequestBuilder) retryAllowed() bool {
	if rb.retry.maxRetries <= 0 {
		return false
	}
	if rb.retry.explicit || rb.idempotencyKey != "" {
		return true
	}
	switch rb.method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
