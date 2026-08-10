package transcriptions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultAttendancePrompt = `Você analisa atendimentos presenciais de varejo em português do Brasil.
O texto vem de reconhecimento de voz e pode conter erros fonéticos. Use loja, consultor e desfecho como contexto, corrija nomes somente quando houver evidência e nunca invente fatos.
Produza um resumo executivo curto e um relatório estruturado com intenção do cliente, necessidades, produtos citados, objeções, compromissos, próximos passos, oportunidades, alertas, sentimento e confiança.
O prompt configurado é a política soberana da análise.`

func defaultAnalysisConfig() AnalysisConfig {
	return AnalysisConfig{
		Enabled:               true,
		TranscriptionProvider: "local",
		TranscriptionModel:    defaultWhisperModel,
		TranscriptionLanguage: defaultWhisperLanguage,
		CredentialID:          "",
		Provider:              "openai",
		Model:                 "gpt-4.1-mini",
		SystemPrompt:          defaultAttendancePrompt,
		Temperature:           0.2,
	}
}

func normalizeAnalysisConfig(config AnalysisConfig) AnalysisConfig {
	defaults := defaultAnalysisConfig()
	config.TranscriptionProvider = "local"
	config.TranscriptionModel = strings.TrimSpace(config.TranscriptionModel)
	if config.TranscriptionModel == "" {
		config.TranscriptionModel = defaults.TranscriptionModel
	}
	if len(config.TranscriptionModel) > 180 {
		config.TranscriptionModel = config.TranscriptionModel[:180]
	}
	config.TranscriptionLanguage = strings.ToLower(strings.TrimSpace(config.TranscriptionLanguage))
	if config.TranscriptionLanguage == "" {
		config.TranscriptionLanguage = defaults.TranscriptionLanguage
	}
	if len(config.TranscriptionLanguage) > 12 {
		config.TranscriptionLanguage = config.TranscriptionLanguage[:12]
	}
	config.CredentialID = strings.TrimSpace(config.CredentialID)
	config.Provider = strings.ToLower(strings.TrimSpace(config.Provider))
	if config.Provider != "openai" && config.Provider != "gemini" {
		config.Provider = defaults.Provider
	}
	config.Model = strings.TrimSpace(config.Model)
	if config.Model == "" {
		if config.Provider == "gemini" {
			config.Model = "gemini-2.5-flash"
		} else {
			config.Model = defaults.Model
		}
	}
	if len(config.Model) > 180 {
		config.Model = config.Model[:180]
	}
	config.SystemPrompt = strings.TrimSpace(config.SystemPrompt)
	if config.SystemPrompt == "" {
		config.SystemPrompt = defaults.SystemPrompt
	}
	if len(config.SystemPrompt) > 20_000 {
		config.SystemPrompt = config.SystemPrompt[:20_000]
	}
	if config.Temperature < 0 {
		config.Temperature = 0
	}
	if config.Temperature > 1 {
		config.Temperature = 1
	}
	return config
}

func (service *Service) GetAnalysisConfig(ctx context.Context, access AccessContext) (AnalysisConfigView, error) {
	if !canManageTranscriptionsRead(access) {
		return AnalysisConfigView{}, ErrForbidden
	}
	if access.AccountID == "" || service.analysis == nil {
		return AnalysisConfigView{}, ErrValidation
	}
	config, err := service.analysis.GetAnalysisConfig(ctx, access.AccountID)
	if err != nil {
		return AnalysisConfigView{}, err
	}
	return AnalysisConfigView{AnalysisConfig: normalizeAnalysisConfig(config)}, nil
}

func (service *Service) PutAnalysisConfig(ctx context.Context, access AccessContext, input PutAnalysisConfigInput) (AnalysisConfigView, error) {
	if !canManageTranscriptionsWrite(access) {
		return AnalysisConfigView{}, ErrForbidden
	}
	if access.AccountID == "" || service.analysis == nil || service.credentials == nil {
		return AnalysisConfigView{}, ErrValidation
	}
	config := normalizeAnalysisConfig(input.AnalysisConfig)
	if config.Enabled {
		if config.CredentialID == "" {
			return AnalysisConfigView{}, ErrCredentialUnavailable
		}
		credential, err := service.credentials.ResolveCredential(ctx, access.AccountID, config.CredentialID)
		if err != nil {
			return AnalysisConfigView{}, ErrCredentialUnavailable
		}
		provider := strings.ToLower(strings.TrimSpace(credential.Provider))
		if provider != "openai" && provider != "gemini" {
			return AnalysisConfigView{}, ErrUnsupported
		}
		config.Provider = provider
	}
	if err := service.analysis.PutAnalysisConfig(ctx, access.AccountID, config, access.UserID); err != nil {
		return AnalysisConfigView{}, err
	}
	return service.GetAnalysisConfig(ctx, access)
}

func (service *Service) RequestAnalysis(ctx context.Context, access AccessContext, recordingID string) (RecordingView, error) {
	recordingID = strings.TrimSpace(recordingID)
	if !canManageTranscriptionsWrite(access) {
		return RecordingView{}, ErrForbidden
	}
	if access.AccountID == "" || recordingID == "" || service.analysis == nil {
		return RecordingView{}, ErrValidation
	}
	recording, err := service.repository.GetRecording(ctx, access.AccountID, recordingID)
	if err != nil {
		return RecordingView{}, err
	}
	if !storeAllowed(access, recording.StoreID) {
		return RecordingView{}, ErrNotFound
	}
	recording, err = service.analysis.RequestAnalysis(ctx, access.AccountID, recordingID)
	if err != nil {
		return RecordingView{}, err
	}
	return recordingView(recording), nil
}

type N8nAnalyzerConfig struct {
	WebhookURL string
	Timeout    time.Duration
}

type N8nAnalyzer struct {
	webhookURL string
	client     *http.Client
}

func N8nAnalyzerConfigFromEnv() N8nAnalyzerConfig {
	config := N8nAnalyzerConfig{
		WebhookURL: strings.TrimSpace(os.Getenv("ATTENDANCE_ANALYSIS_WEBHOOK_URL")),
		Timeout:    20 * time.Minute,
	}
	if timeout, err := time.ParseDuration(strings.TrimSpace(os.Getenv("ATTENDANCE_ANALYSIS_TIMEOUT"))); err == nil && timeout > 0 {
		config.Timeout = timeout
	}
	return config
}

func NewN8nAnalyzer(config N8nAnalyzerConfig) *N8nAnalyzer {
	return &N8nAnalyzer{
		webhookURL: strings.TrimSpace(config.WebhookURL),
		client:     &http.Client{Timeout: config.Timeout},
	}
}

func (analyzer *N8nAnalyzer) Analyze(ctx context.Context, recording Recording, config AnalysisConfig, apiKey string) (AnalysisResult, error) {
	if analyzer.webhookURL == "" {
		return AnalysisResult{}, errors.New("attendance analysis webhook not configured")
	}
	if strings.TrimSpace(apiKey) == "" {
		return AnalysisResult{}, errors.New("attendance analysis api key missing")
	}
	payload := map[string]any{
		"transcript": recording.TranscriptText,
		"context": map[string]any{
			"recordingId":    recording.ID,
			"serviceId":      recording.ServiceID,
			"storeName":      recording.StoreName,
			"consultantName": recording.ConsultantName,
			"startedAt":      recording.StartedAt,
			"endedAt":        recording.EndedAt,
			"finishOutcome":  recording.FinishOutcome,
		},
		"ai": map[string]any{
			"provider":     config.Provider,
			"model":        config.Model,
			"systemPrompt": config.SystemPrompt,
			"temperature":  config.Temperature,
			"apiKey":       apiKey,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return AnalysisResult{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, analyzer.webhookURL, bytes.NewReader(body))
	if err != nil {
		return AnalysisResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := analyzer.client.Do(request)
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("call attendance analysis workflow: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		return AnalysisResult{}, fmt.Errorf("attendance analysis workflow status: %d", response.StatusCode)
	}
	var result AnalysisResult
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
		return AnalysisResult{}, fmt.Errorf("decode attendance analysis: %w", err)
	}
	result.Summary = strings.TrimSpace(result.Summary)
	if result.Summary == "" || len(result.Summary) > 20_000 || !json.Valid(result.Report) {
		return AnalysisResult{}, errors.New("invalid attendance analysis response")
	}
	var reportObject map[string]any
	if err := json.Unmarshal(result.Report, &reportObject); err != nil || reportObject == nil {
		return AnalysisResult{}, errors.New("invalid attendance analysis report")
	}
	return result, nil
}

const (
	analysisLease        = 22 * time.Minute
	analysisPollInterval = time.Second
	maxAnalysisAttempts  = 3
)

type AnalysisWorker struct {
	repository  AnalysisRepository
	analyzer    AttendanceAnalyzer
	credentials CredentialResolver
	logger      *slog.Logger
}

func NewAnalysisWorker(repository AnalysisRepository, analyzer AttendanceAnalyzer, credentials CredentialResolver, logger *slog.Logger) *AnalysisWorker {
	return &AnalysisWorker{repository: repository, analyzer: analyzer, credentials: credentials, logger: logger}
}

func (worker *AnalysisWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(analysisPollInterval)
	defer ticker.Stop()
	for {
		for {
			processed, err := worker.processNext(ctx)
			if err != nil && !errors.Is(err, context.Canceled) {
				worker.logger.Warn("attendance_analysis_worker_failed", "error", err)
			}
			if !processed || err != nil {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (worker *AnalysisWorker) processNext(ctx context.Context) (bool, error) {
	recording, err := worker.repository.ClaimAnalysis(ctx, "queue-attendance-analysis", analysisLease)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	config, err := worker.repository.GetAnalysisConfig(ctx, recording.AccountID)
	if err != nil {
		return true, worker.persistFailure(ctx, recording, err)
	}
	config = normalizeAnalysisConfig(config)
	if worker.credentials == nil || config.CredentialID == "" {
		return true, worker.persistFailure(ctx, recording, ErrCredentialUnavailable)
	}
	credential, err := worker.credentials.ResolveCredential(ctx, recording.AccountID, config.CredentialID)
	if err != nil {
		return true, worker.persistFailure(ctx, recording, ErrCredentialUnavailable)
	}
	if strings.ToLower(strings.TrimSpace(credential.Provider)) != config.Provider {
		return true, worker.persistFailure(ctx, recording, ErrCredentialUnavailable)
	}
	result, err := worker.analyzer.Analyze(ctx, recording, config, credential.APIKey)
	if err != nil {
		return true, worker.persistFailure(ctx, recording, err)
	}
	snapshot, _ := json.Marshal(config)
	if err := worker.repository.CompleteAnalysis(ctx, recording.AccountID, recording.ID, result, snapshot); err != nil {
		return true, err
	}
	worker.logger.Info("attendance_analysis_completed", "recording_id", recording.ID)
	return true, nil
}

func (worker *AnalysisWorker) persistFailure(ctx context.Context, recording Recording, cause error) error {
	var retryAt *time.Time
	if recording.AnalysisAttemptCount < maxAnalysisAttempts {
		next := time.Now().UTC().Add(time.Duration(recording.AnalysisAttemptCount*15) * time.Second)
		retryAt = &next
	}
	message := "A IA de resumo nao respondeu. Confira a configuracao e tente novamente."
	if errors.Is(cause, ErrCredentialUnavailable) || strings.Contains(strings.ToLower(cause.Error()), "api key") {
		message = "Selecione uma chave global de IA valida para gerar o resumo."
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		message = "A IA excedeu o tempo limite ao analisar este atendimento."
	}
	if err := worker.repository.FailAnalysis(ctx, recording.AccountID, recording.ID, message, retryAt); err != nil {
		return err
	}
	worker.logger.Warn("attendance_analysis_attempt_failed",
		"recording_id", recording.ID,
		"attempt", recording.AnalysisAttemptCount,
		"will_retry", retryAt != nil,
	)
	return nil
}
