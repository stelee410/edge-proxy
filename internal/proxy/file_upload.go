package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"linkyun-edge-proxy/internal/logger"
)

// FileUploadResult 文件上传结果
type FileUploadResult struct {
	Token       string `json:"token"`
	DownloadURL string `json:"download_url"` // 完整绝对 URL
	ExpiresIn   int    `json:"expires_in"`
}

// UploadFile 将文件数据上传到服务器，返回令牌和完整下载 URL。
// data: 文件内容二进制
// filename: 原始文件名
// contentType: MIME 类型（如 "audio/mp3"）
func (p *Proxy) UploadFile(ctx context.Context, data []byte, filename string, contentType string) (*FileUploadResult, error) {
	maxAttempts := 3
	delay := 1 * time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, err := p.doUpload(ctx, data, filename, contentType)
		if err == nil {
			return result, nil
		}

		// Don't retry on client errors (auth, too large)
		if isNonRetryable(err) {
			return nil, err
		}

		if attempt < maxAttempts {
			logger.Warn("File upload attempt %d/%d failed: %v (retry in %s)", attempt, maxAttempts, err, delay)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
			delay *= 2
		}
	}
	return nil, fmt.Errorf("file upload failed after %d attempts", maxAttempts)
}

func (p *Proxy) doUpload(ctx context.Context, data []byte, filename string, contentType string) (*FileUploadResult, error) {
	// Build multipart body
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write file data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	url := p.config.ServerURL + "/api/v1/edge/files/upload"
	req, err := http.NewRequestWithContext(ctx, "POST", url, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create upload request: %w", err)
	}
	req.Header.Set("X-Edge-Token", p.config.EdgeToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read upload response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// Parse response
		var envelope struct {
			Success bool `json:"success"`
			Data    struct {
				Token       string `json:"token"`
				DownloadURL string `json:"download_url"`
				ExpiresIn   int    `json:"expires_in"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &envelope); err != nil {
			return nil, fmt.Errorf("failed to parse upload response: %w", err)
		}
		if !envelope.Success {
			return nil, fmt.Errorf("server returned success=false")
		}

		// Build full absolute download URL
		fullURL := p.config.ServerURL + envelope.Data.DownloadURL

		return &FileUploadResult{
			Token:       envelope.Data.Token,
			DownloadURL: fullURL,
			ExpiresIn:   envelope.Data.ExpiresIn,
		}, nil

	case http.StatusUnauthorized:
		return nil, &nonRetryableError{msg: "authentication failed: invalid edge token"}
	case http.StatusRequestEntityTooLarge:
		return nil, &nonRetryableError{msg: "file size exceeds server limit"}
	default:
		return nil, fmt.Errorf("upload failed (status %d): %s", resp.StatusCode, string(respBody))
	}
}

// DownloadFileByToken 根据 token 从服务器下载文件（Creator 上传的图片等）
// 用于 Edge 模式下 image_upload 附件的多模态 LLM 输入
func (p *Proxy) DownloadFileByToken(ctx context.Context, token string) ([]byte, string, error) {
	base := strings.TrimSuffix(p.config.ServerURL, "/")
	url := base + "/api/v1/files/" + token + "/download"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create download request: %w", err)
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("download failed (status %d): %s", resp.StatusCode, string(body))
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read download body: %w", err)
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	return data, contentType, nil
}

// nonRetryableError marks errors that should not be retried.
type nonRetryableError struct {
	msg string
}

func (e *nonRetryableError) Error() string { return e.msg }

func isNonRetryable(err error) bool {
	_, ok := err.(*nonRetryableError)
	return ok
}
