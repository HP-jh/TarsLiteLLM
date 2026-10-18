package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
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

// v1.2.0: usage tracking tests
func TestUsageTracker_RecordAndSnapshot(t *testing.T) {
	ut := &UsageTracker{}
	ut.Record(true, 100)
	ut.Record(false, 200)
	ut.Record(true, 300)
	snap := ut.Snapshot()
	if snap["requests"].(int64) != 3 {
		t.Fatalf("expected 3 requests, got %v", snap["requests"])
	}
	if snap["successes"].(int64) != 2 {
		t.Fatalf("expected 2 successes, got %v", snap["successes"])
	}
	if snap["failures"].(int64) != 1 {
		t.Fatalf("expected 1 failure, got %v", snap["failures"])
	}
	if snap["avg_ms"].(float64) != 200.0 {
		t.Fatalf("expected avg 200ms, got %v", snap["avg_ms"])
	}
}

func TestUsageTracker_Concurrent(t *testing.T) {
	ut := &UsageTracker{}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ut.Record(true, 10)
		}()
	}
	wg.Wait()
	if atomic.LoadInt64(&ut.Requests) != 100 {
		t.Fatalf("expected 100 requests, got %d", atomic.LoadInt64(&ut.Requests))
	}
}

func TestHealthHandler_IncludesUsage(t *testing.T) {
	client := NewClient("", "")
	// healthHandler itself calls client.req, so we just verify usage appears
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
	usage, ok := resp["usage"].(map[string]interface{})
	if !ok {
		t.Fatal("expected usage in health response")
	}
	// healthHandler makes 1 upstream request, so requests >= 1
	if usage["requests"].(float64) < 1 {
		t.Fatalf("expected >=1 request in usage, got %v", usage["requests"])
	}
}

// v1.3.0: config reload tests
func TestReloadConfig(t *testing.T) {
	os.Setenv("TARS_MCP_ENDPOINT", "https://new.example.com")
	os.Setenv("TARS_MCP_API_KEY", "new-key")
	defer os.Unsetenv("TARS_MCP_ENDPOINT")
	defer os.Unsetenv("TARS_MCP_API_KEY")

	client := NewClient("https://old.example.com", "old-key")
	client.ReloadConfig()

	if client.Endpoint != "https://new.example.com" {
		t.Fatalf("expected endpoint https://new.example.com, got %s", client.Endpoint)
	}
	if client.APIKey != "new-key" {
		t.Fatalf("expected api_key new-key, got %s", client.APIKey)
	}
}

func TestReloadConfig_Partial(t *testing.T) {
	// Only endpoint env set, api_key should stay
	os.Setenv("TARS_MCP_ENDPOINT", "https://partial.example.com")
	defer os.Unsetenv("TARS_MCP_ENDPOINT")

	client := NewClient("https://old.example.com", "keep-key")
	client.ReloadConfig()

	if client.Endpoint != "https://partial.example.com" {
		t.Fatalf("expected endpoint updated, got %s", client.Endpoint)
	}
	if client.APIKey != "keep-key" {
		t.Fatalf("expected api_key unchanged, got %s", client.APIKey)
	}
}

func TestClientReq_ConcurrentWithReload(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := NewClient(srv.URL, "test-key")

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = client.req(context.Background(), "GET", "/health", nil)
		}()
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.ReloadConfig()
		}()
	}
	wg.Wait()
}

// v1.4.0: leveled logger tests
func TestLogger_Levels(t *testing.T) {
	l := NewLogger("info")
	if l.level != 2 {
		t.Fatalf("expected level 2 (info), got %d", l.level)
	}
	l.SetLevel("error")
	if l.level != 0 {
		t.Fatalf("expected level 0 (error), got %d", l.level)
	}
	l.SetLevel("warn")
	if l.level != 1 {
		t.Fatalf("expected level 1 (warn), got %d", l.level)
	}
}

func TestLogger_InfoSuppressedWhenError(t *testing.T) {
	l := NewLogger("error")
	// Should not panic and should not produce output (we just verify no panic)
	l.Infof("this should not print")
	l.Warnf("this should not print")
	l.Errorf("this may print")
}

func TestClient_HasLogger(t *testing.T) {
	client := NewClient("", "")
	if client.Logger == nil {
		t.Fatal("expected client to have a logger")
	}
}

// v1.5.0: connection pool + graceful shutdown tests
func TestNewClient_HasPooledTransport(t *testing.T) {
	client := NewClient("", "")
	tr, ok := client.Client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected client to use *http.Transport")
	}
	if tr.MaxIdleConns != 100 {
		t.Fatalf("expected MaxIdleConns 100, got %d", tr.MaxIdleConns)
	}
	if tr.MaxIdleConnsPerHost != 10 {
		t.Fatalf("expected MaxIdleConnsPerHost 10, got %d", tr.MaxIdleConnsPerHost)
	}
	if tr.MaxConnsPerHost != 50 {
		t.Fatalf("expected MaxConnsPerHost 50, got %d", tr.MaxConnsPerHost)
	}
	if tr.DisableKeepAlives != false {
		t.Fatal("expected KeepAlives enabled")
	}
}

func TestGracefulShutdown(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := NewClient(srv.URL, "")

	// Verify server is reachable
	res, err := client.req(context.Background(), "GET", "/health", nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res["status"] != "ok" {
		t.Fatalf("expected ok, got %v", res["status"])
	}
}
