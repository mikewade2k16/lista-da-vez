package tasks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteServiceErrorDistinguishesVideoStorageFailures(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		status     int
		code       string
		retryAfter string
	}{
		{name: "metrics", err: ErrVideoMetricsUnavailable, status: http.StatusServiceUnavailable, code: "video_metrics_unavailable", retryAfter: "2"},
		{name: "quota", err: ErrVideoQuotaExceeded, status: http.StatusTooManyRequests, code: "video_storage_quota_exceeded"},
		{name: "provider", err: ErrVideoUnavailable, status: http.StatusServiceUnavailable, code: "video_unavailable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/tasks/task-1/videos", nil)
			recorder := httptest.NewRecorder()

			writeServiceError(recorder, request, test.err)

			if recorder.Code != test.status {
				t.Fatalf("expected status %d, got %d", test.status, recorder.Code)
			}
			if !strings.Contains(recorder.Body.String(), `"code":"`+test.code+`"`) {
				t.Fatalf("expected code %q, got %s", test.code, recorder.Body.String())
			}
			if got := recorder.Header().Get("Retry-After"); got != test.retryAfter {
				t.Fatalf("expected Retry-After %q, got %q", test.retryAfter, got)
			}
		})
	}
}
