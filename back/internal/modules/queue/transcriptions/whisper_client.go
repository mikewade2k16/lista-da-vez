package transcriptions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultWhisperBaseURL  = "http://localhost:8010"
	defaultWhisperModel    = "Systran/faster-whisper-base"
	defaultWhisperLanguage = "pt"
	defaultWhisperTimeout  = 10 * time.Minute
)

type WhisperConfig struct {
	BaseURL  string
	Model    string
	Language string
	Timeout  time.Duration
}

type WhisperClient struct {
	endpoint string
	model    string
	language string
	client   *http.Client
}

func WhisperConfigFromEnv() WhisperConfig {
	config := WhisperConfig{
		BaseURL:  strings.TrimSpace(os.Getenv("ATTENDANCE_TRANSCRIPTION_BASE_URL")),
		Model:    strings.TrimSpace(os.Getenv("ATTENDANCE_TRANSCRIPTION_MODEL")),
		Language: strings.TrimSpace(os.Getenv("ATTENDANCE_TRANSCRIPTION_LANGUAGE")),
		Timeout:  defaultWhisperTimeout,
	}
	if config.BaseURL == "" {
		config.BaseURL = defaultWhisperBaseURL
	}
	if config.Model == "" {
		config.Model = defaultWhisperModel
	}
	if config.Language == "" {
		config.Language = defaultWhisperLanguage
	}
	if timeout, err := time.ParseDuration(strings.TrimSpace(os.Getenv("ATTENDANCE_TRANSCRIPTION_TIMEOUT"))); err == nil && timeout > 0 {
		config.Timeout = timeout
	}
	return config
}

func NewWhisperClient(config WhisperConfig) *WhisperClient {
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = defaultWhisperTimeout
	}
	return &WhisperClient{
		endpoint: strings.TrimRight(config.BaseURL, "/") + "/v1/audio/transcriptions",
		model:    config.Model,
		language: config.Language,
		client:   &http.Client{Timeout: timeout},
	}
}

func (client *WhisperClient) Transcribe(ctx context.Context, audio OpenedAudio) (string, error) {
	return client.TranscribeWithOptions(ctx, audio, TranscriptionOptions{})
}

func (client *WhisperClient) TranscribeWithOptions(ctx context.Context, audio OpenedAudio, options TranscriptionOptions) (string, error) {
	model := strings.TrimSpace(options.Model)
	if model == "" {
		model = client.model
	}
	language := strings.TrimSpace(options.Language)
	if language == "" {
		language = client.language
	}
	bodyReader, bodyWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(bodyWriter)
	contentType := multipartWriter.FormDataContentType()

	go func() {
		writeErr := multipartWriter.WriteField("model", model)
		if writeErr == nil {
			writeErr = multipartWriter.WriteField("language", language)
		}
		if writeErr == nil {
			writeErr = multipartWriter.WriteField("response_format", "json")
		}
		var part io.Writer
		if writeErr == nil {
			part, writeErr = multipartWriter.CreateFormFile("file", audio.FileName)
		}
		if writeErr == nil {
			_, writeErr = io.Copy(part, audio.File)
		}
		if closeErr := multipartWriter.Close(); writeErr == nil {
			writeErr = closeErr
		}
		_ = bodyWriter.CloseWithError(writeErr)
	}()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, bodyReader)
	if err != nil {
		_ = bodyReader.CloseWithError(err)
		return "", fmt.Errorf("create whisper request: %w", err)
	}
	request.Header.Set("Content-Type", contentType)

	response, err := client.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("call whisper: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		return "", fmt.Errorf("whisper status: %d", response.StatusCode)
	}

	var payload struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode whisper response: %w", err)
	}
	return strings.TrimSpace(payload.Text), nil
}
