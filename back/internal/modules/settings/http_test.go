package settings

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleDebugSettingsFailureDisabledInProduction(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/v1/settings?__debugSettingsFailure=500", nil)
	recorder := httptest.NewRecorder()

	handled := handleDebugSettingsFailure(recorder, request, "production")
	if handled {
		t.Fatalf("expected debug failure hook to be disabled in production")
	}
}

func TestHandleDebugSettingsFailureReturnsInternalErrorInDevelopment(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/v1/settings?__debugSettingsFailure=500", nil)
	recorder := httptest.NewRecorder()

	handled := handleDebugSettingsFailure(recorder, request, "development")
	if !handled {
		t.Fatalf("expected debug failure hook to handle the request")
	}

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
	}
}

func TestHandleDebugSettingsFailureReadsCookieInDevelopment(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/v1/settings", nil)
	request.AddCookie(&http.Cookie{
		Name:  "ldv_debug_settings_failure",
		Value: "500",
	})
	recorder := httptest.NewRecorder()

	handled := handleDebugSettingsFailure(recorder, request, "development")
	if !handled {
		t.Fatalf("expected debug failure hook to read cookie mode")
	}

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
	}
}
