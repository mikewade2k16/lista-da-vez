package calendar

import (
	"context"
	"errors"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFinanceWhisperLocalIndependentOfChatModel(t *testing.T) {
	calls, checks := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if err := r.ParseMultipartForm(1024); err != nil {
			t.Error(err)
			return
		}
		defer r.MultipartForm.RemoveAll()
		if r.FormValue("provider") != "local" || r.FormValue("apiKey") != "" {
			t.Error("local audio used paid provider/key")
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Error(err)
			return
		}
		defer file.Close()
		body, err := io.ReadAll(file)
		if err != nil || string(body) != "synthetic-audio" {
			t.Error("audio not forwarded")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"consulta de teste"}`))
	}))
	defer server.Close()
	service := (&Service{}).WithChat("", server.URL).WithFinanceTranscriptionAuthorization(func(context.Context, string, auth.Principal) error { checks++; return nil })
	service.financeAssistantRuntime = func(context.Context, string) (AssistantRuntime, error) {
		t.Fatal("transcription depends on chat model")
		return AssistantRuntime{}, ErrAssistantDisabled
	}
	p := auth.Principal{AccountID: "account", PermissionsResolved: true, Permissions: []string{"finance.sheets.view"}}
	text, err := service.AssistantChatTranscribe(context.Background(), "account", p, AssistantSurfaceFinance, "test.webm", "audio/webm", []byte("synthetic-audio"))
	if err != nil || text != "consulta de teste" || calls != 1 || checks != 2 {
		t.Fatal(text, err, calls, checks)
	}
	service.financeTranscriptionAuthorize = func(context.Context, string, auth.Principal) error { return ErrAssistantNoCapability }
	if _, err = service.AssistantChatTranscribe(context.Background(), "account", p, AssistantSurfaceFinance, "test.webm", "audio/webm", []byte("synthetic-audio")); !errors.Is(err, ErrAssistantNoCapability) || calls != 1 {
		t.Fatal("revoked access sent audio", err)
	}
	checks = 0
	service.financeTranscriptionAuthorize = func(context.Context, string, auth.Principal) error {
		checks++
		if checks > 1 {
			return ErrAssistantNoCapability
		}
		return nil
	}
	if text, err = service.AssistantChatTranscribe(context.Background(), "account", p, AssistantSurfaceFinance, "test.webm", "audio/webm", []byte("synthetic-audio")); text != "" || !errors.Is(err, ErrAssistantNoCapability) {
		t.Fatal("revoked response exposed", text, err)
	}
}
