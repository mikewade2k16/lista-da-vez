package automation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestPrepareConnectionRestoresPairedSessionWithoutRequestingQR(t *testing.T) {
	t.Parallel()

	var statusCalls atomic.Int32
	var qrCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/sessions/default/start":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/api/sessions/default":
			if statusCalls.Add(1) == 1 {
				_, _ = w.Write([]byte(`{"name":"default","status":"STARTING"}`))
				return
			}
			_, _ = w.Write([]byte(`{"name":"default","status":"WORKING","me":{"id":"554299999999@c.us"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/default/auth/qr":
			qrCalls.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewWAHAClient(server.URL)
	status, phone, qr, err := client.prepareConnection(context.Background(), "default", "STOPPED", time.Second, time.Millisecond)
	if err != nil {
		t.Fatalf("prepareConnection() error = %v", err)
	}
	if status != statusWorking || phone != "554299999999" || qr != "" {
		t.Fatalf("prepareConnection() = status %q, phone %q, qr %q", status, phone, qr)
	}
	if got := qrCalls.Load(); got != 0 {
		t.Fatalf("QR endpoint called %d times; want 0", got)
	}
}

func TestPrepareConnectionReturnsQRWhenPairingIsRequired(t *testing.T) {
	t.Parallel()

	var qrCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/sessions/default/start":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/api/sessions/default":
			_, _ = w.Write([]byte(`{"name":"default","status":"SCAN_QR_CODE"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/default/auth/qr":
			qrCalls.Add(1)
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("png"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewWAHAClient(server.URL)
	status, phone, qr, err := client.prepareConnection(context.Background(), "default", "STOPPED", time.Second, time.Millisecond)
	if err != nil {
		t.Fatalf("prepareConnection() error = %v", err)
	}
	if status != statusScanQRCode || phone != "" || qr != "data:image/png;base64,cG5n" {
		t.Fatalf("prepareConnection() = status %q, phone %q, qr %q", status, phone, qr)
	}
	if got := qrCalls.Load(); got != 1 {
		t.Fatalf("QR endpoint called %d times; want 1", got)
	}
}
