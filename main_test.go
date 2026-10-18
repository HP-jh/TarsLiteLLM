package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestHealthHandler(t *testing.T) {
	client := NewClient("", "")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	withClient(client, healthHandler)(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("expected ok, got %v", resp["status"])
	}
}

func TestToolsHandler(t *testing.T) {
	client := NewClient("", "")
	req := httptest.NewRequest(http.MethodGet, "/tools", nil)
	w := httptest.NewRecorder()
	withClient(client, toolsHandler)(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	tools, ok := resp["tools"].([]interface{})
	if !ok || len(tools) == 0 {
		t.Fatal("expected tools")
	}
}

func TestInvokeHandler_MethodNotAllowed(t *testing.T) {
	client := NewClient("", "")
	req := httptest.NewRequest(http.MethodGet, "/invoke", nil)
	w := httptest.NewRecorder()
	withClient(client, invokeHandler)(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestInvokeHandler_UnknownTool(t *testing.T) {
	client := NewClient("", "")
	body, _ := json.Marshal(map[string]interface{}{"tool": "xxx_unknown", "params": map[string]interface{}{}})
	req := httptest.NewRequest(http.MethodPost, "/invoke", bytes.NewReader(body))
	w := httptest.NewRecorder()
	withClient(client, invokeHandler)(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestClientReq_MockServer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := NewClient(srv.URL, "test-key")
	res, err := client.req(context.Background(), "GET", "/health", nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res["status"] != "ok" {
		t.Fatalf("expected ok, got %v", res["status"])
	}
}

// v1.1.0: retry with exponential backoff tests
func TestRetry_SuccessAfterTransientFailure(t *testing.T) {
	var attempts int64
	mux := http.NewServeMux()
	mux.HandleFunc("/flaky", func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt64(&attempts, 1)
		if count < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := NewClient(srv.URL, "")
	// Reduce delays for test speed
	origBase := baseDelay
	origMax := maxDelay
	baseDelay = 10 * time.Millisecond
	maxDelay = 50 * time.Millisecond
	defer func() { baseDelay = origBase; maxDelay = origMax }()

	res, err := client.req(context.Background(), "GET", "/flaky", nil)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if res["ok"] != true {
		t.Fatalf("expected ok true, got %v", res["ok"])
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetry_NoRetryOn4xx(t *testing.T) {
	var attempts int64
	mux := http.NewServeMux()
	mux.HandleFunc("/bad", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&attempts, 1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "bad request"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := NewClient(srv.URL, "")

	_, err := client.req(context.Background(), "GET", "/bad", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt (no retry on 4xx), got %d", attempts)
	}
}

func TestCalculateBackoff(t *testing.T) {
	// Ensure backoff increases with attempt number
	b0 := calculateBackoff(0)
	b1 := calculateBackoff(1)
	b2 := calculateBackoff(2)
	if b0 > b1 || b1 > b2 {
		t.Fatalf("backoff should increase: %v %v %v", b0, b1, b2)
	}
	// Ensure max delay is respected
	b10 := calculateBackoff(10)
	if b10 > maxDelay {
		t.Fatalf("backoff should not exceed maxDelay: %v > %v", b10, maxDelay)
	}
}
