package skills

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultHTTPTimeout  = 10 * time.Second
	defaultMaxRetries   = 3
	defaultRetryDelay   = 1 * time.Second
)

// HTTPClient 带超时和重试的 HTTP 客户端
type HTTPClient struct {
	client     *http.Client
	maxRetries int
	retryDelay time.Duration
}

// NewHTTPClient 创建 HTTP 客户端
func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
		maxRetries: defaultMaxRetries,
		retryDelay: defaultRetryDelay,
	}
}

// Do 执行 HTTP 请求，带重试
func (c *HTTPClient) Do(method, url string, body string, headers map[string]string) (string, int, error) {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(c.retryDelay * time.Duration(attempt))
		}

		var bodyReader io.Reader
		if body != "" {
			bodyReader = strings.NewReader(body)
		}

		req, err := http.NewRequest(method, url, bodyReader)
		if err != nil {
			return "", 0, fmt.Errorf("failed to create request: %w", err)
		}

		for k, v := range headers {
			req.Header.Set(k, v)
		}
		if body != "" && req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		// 5xx 错误重试，其他直接返回
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("server error: status %d", resp.StatusCode)
			continue
		}

		return string(respBody), resp.StatusCode, nil
	}

	return "", 0, fmt.Errorf("request failed after %d attempts: %w", c.maxRetries+1, lastErr)
}
