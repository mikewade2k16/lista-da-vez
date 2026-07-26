package transcriptions

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestWhisperClientStreamsConfiguredMultipart(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/transcriptions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			http.Error(w, "invalid multipart", http.StatusBadRequest)
			return
		}
		if r.FormValue("model") != "local-model" || r.FormValue("language") != "pt" {
			t.Errorf("form model=%q language=%q", r.FormValue("model"), r.FormValue("language"))
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Errorf("FormFile: %v", err)
			http.Error(w, "missing file", http.StatusBadRequest)
			return
		}
		defer func() { _ = file.Close() }()
		content, err := io.ReadAll(file)
		if err != nil || string(content) != "audio-bytes" {
			t.Errorf("audio = %q, err=%v", content, err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":" atendimento transcrito "}`))
	}))
	defer server.Close()

	audioFile, err := os.CreateTemp(t.TempDir(), "attendance-*.webm")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer func() { _ = audioFile.Close() }()
	if _, err := audioFile.WriteString("audio-bytes"); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if _, err := audioFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Seek: %v", err)
	}

	client := NewWhisperClient(WhisperConfig{
		BaseURL:  server.URL,
		Model:    "local-model",
		Language: "pt",
		Timeout:  time.Second,
	})
	text, err := client.Transcribe(context.Background(), OpenedAudio{
		File:     audioFile,
		FileName: "atendimento.webm",
		MimeType: "audio/webm",
	})
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if text != "atendimento transcrito" {
		t.Fatalf("text = %q", text)
	}
}
