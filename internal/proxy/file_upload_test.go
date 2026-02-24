package proxy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"linkyun-edge-proxy/internal/config"
)

func TestUploadFile_Success(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method and path
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/edge/files/upload" {
			t.Errorf("expected /api/v1/edge/files/upload, got %s", r.URL.Path)
		}

		// Verify auth header
		if token := r.Header.Get("X-Edge-Token"); token != "test_token" {
			t.Errorf("expected X-Edge-Token=test_token, got %s", token)
		}

		// Verify multipart
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("failed to parse multipart: %v", err)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("failed to get form file: %v", err)
		}
		defer file.Close()

		if header.Filename != "test.mp3" {
			t.Errorf("expected filename test.mp3, got %s", header.Filename)
		}

		data, _ := io.ReadAll(file)
		if string(data) != "fake audio data" {
			t.Errorf("unexpected file content: %s", string(data))
		}

		// Respond
		resp := map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"token":        "ft_test-uuid",
				"download_url": "/api/v1/files/ft_test-uuid/download",
				"expires_in":   86400,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestProxy(server.URL, "test_token")

	result, err := p.UploadFile(context.Background(), []byte("fake audio data"), "test.mp3", "audio/mp3")
	if err != nil {
		t.Fatalf("UploadFile failed: %v", err)
	}

	if result.Token != "ft_test-uuid" {
		t.Errorf("expected token ft_test-uuid, got %s", result.Token)
	}
	expectedURL := server.URL + "/api/v1/files/ft_test-uuid/download"
	if result.DownloadURL != expectedURL {
		t.Errorf("expected download_url %s, got %s", expectedURL, result.DownloadURL)
	}
	if result.ExpiresIn != 86400 {
		t.Errorf("expected expires_in 86400, got %d", result.ExpiresIn)
	}
}

func TestUploadFile_AuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid edge token",
		})
	}))
	defer server.Close()

	p := newTestProxy(server.URL, "bad_token")

	_, err := p.UploadFile(context.Background(), []byte("data"), "test.mp3", "audio/mp3")
	if err == nil {
		t.Fatal("expected error for auth failure, got nil")
	}
	if !isNonRetryable(err) {
		t.Errorf("expected non-retryable error, got: %v", err)
	}
}

func TestUploadFile_FileTooLarge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
	}))
	defer server.Close()

	p := newTestProxy(server.URL, "test_token")

	_, err := p.UploadFile(context.Background(), []byte("data"), "big.bin", "application/octet-stream")
	if err == nil {
		t.Fatal("expected error for file too large, got nil")
	}
	if !isNonRetryable(err) {
		t.Errorf("expected non-retryable error, got: %v", err)
	}
}

func newTestProxy(serverURL, edgeToken string) *Proxy {
	cfg := &config.Config{
		ServerURL: serverURL,
		EdgeToken: edgeToken,
		AgentUUID: "test-agent",
	}
	return &Proxy{
		config:     cfg,
		httpClient: &http.Client{},
	}
}
