package transcriptions

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

const maxJSONBodyBytes = 32 << 10

func accessFromRequest(r *http.Request) (AccessContext, bool) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return AccessContext{}, false
	}
	return AccessContext{
		UserID:              principal.UserID,
		AccountID:           firstNonEmpty(principal.AccountID, principal.TenantID),
		Role:                string(principal.Role),
		StoreIDs:            append([]string{}, principal.StoreIDs...),
		Permissions:         append([]string{}, principal.Permissions...),
		PermissionsResolved: principal.PermissionsResolved,
	}, true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if normalized := strings.TrimSpace(value); normalized != "" {
			return normalized
		}
	}
	return ""
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	return json.NewDecoder(
		http.MaxBytesReader(w, r.Body, maxJSONBodyBytes),
	).Decode(destination)
}

func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	requireAccount := middleware.RequireAuthWithAccount

	mux.Handle("GET /v1/operations/transcriptions/feature", requireAccount(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			access, ok := accessFromRequest(r)
			if !ok {
				writeError(w, r, ErrForbidden)
				return
			}
			feature, err := service.GetRecordingFeature(r.Context(), access)
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpapi.WriteJSON(w, http.StatusOK, map[string]any{"feature": feature})
		},
	)))

	mux.Handle("PUT /v1/operations/transcriptions/feature", requireAccount(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			access, ok := accessFromRequest(r)
			if !ok {
				writeError(w, r, ErrForbidden)
				return
			}
			var input PutRecordingFeatureInput
			if err := decodeJSON(w, r, &input); err != nil {
				httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
				return
			}
			feature, err := service.PutRecordingFeature(r.Context(), access, input)
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpapi.WriteJSON(w, http.StatusOK, map[string]any{"feature": feature})
		},
	)))

	mux.Handle("GET /v1/operations/transcriptions/config", requireAccount(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			access, ok := accessFromRequest(r)
			if !ok {
				writeError(w, r, ErrForbidden)
				return
			}
			config, err := service.GetAnalysisConfig(r.Context(), access)
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpapi.WriteJSON(w, http.StatusOK, map[string]any{"config": config})
		},
	)))

	mux.Handle("PUT /v1/operations/transcriptions/config", requireAccount(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			access, ok := accessFromRequest(r)
			if !ok {
				writeError(w, r, ErrForbidden)
				return
			}
			var input PutAnalysisConfigInput
			if err := decodeJSON(w, r, &input); err != nil {
				httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
				return
			}
			config, err := service.PutAnalysisConfig(r.Context(), access, input)
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpapi.WriteJSON(w, http.StatusOK, map[string]any{"config": config})
		},
	)))

	mux.Handle("POST /v1/operations/transcriptions", requireAccount(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			access, ok := accessFromRequest(r)
			if !ok {
				writeError(w, r, ErrForbidden)
				return
			}
			var input CreateRecordingInput
			if err := decodeJSON(w, r, &input); err != nil {
				httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
				return
			}
			recording, err := service.Create(r.Context(), access, input)
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"recording": recording})
		},
	)))

	mux.Handle("PUT /v1/operations/transcriptions/{recordingId}/chunks/{sequence}", requireAccount(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			access, ok := accessFromRequest(r)
			if !ok {
				writeError(w, r, ErrForbidden)
				return
			}
			sequence, err := strconv.Atoi(strings.TrimSpace(r.PathValue("sequence")))
			if err != nil {
				writeError(w, r, ErrValidation)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxChunkBytes+1)
			recording, err := service.SaveChunk(
				r.Context(),
				access,
				r.PathValue("recordingId"),
				sequence,
				r.Header.Get("Content-Type"),
				r.Body,
			)
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpapi.WriteJSON(w, http.StatusOK, map[string]any{"recording": recording})
		},
	)))

	mux.Handle("POST /v1/operations/transcriptions/{recordingId}/complete", requireAccount(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			access, ok := accessFromRequest(r)
			if !ok {
				writeError(w, r, ErrForbidden)
				return
			}
			var input CompleteRecordingInput
			if err := decodeJSON(w, r, &input); err != nil {
				httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
				return
			}
			recording, err := service.Complete(
				r.Context(),
				access,
				r.PathValue("recordingId"),
				input,
			)
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpapi.WriteJSON(w, http.StatusOK, map[string]any{"recording": recording})
		},
	)))

	mux.Handle("GET /v1/operations/transcriptions", requireAccount(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			access, ok := accessFromRequest(r)
			if !ok {
				writeError(w, r, ErrForbidden)
				return
			}
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
			response, err := service.List(r.Context(), access, ListFilter{
				StoreID:      strings.TrimSpace(r.URL.Query().Get("storeId")),
				ConsultantID: strings.TrimSpace(r.URL.Query().Get("consultantId")),
				Limit:        limit,
				Offset:       offset,
			})
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpapi.WriteJSON(w, http.StatusOK, response)
		},
	)))

	mux.Handle("POST /v1/operations/transcriptions/{recordingId}/transcribe", requireAccount(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			access, ok := accessFromRequest(r)
			if !ok {
				writeError(w, r, ErrForbidden)
				return
			}
			recording, err := service.RequestTranscription(
				r.Context(),
				access,
				r.PathValue("recordingId"),
			)
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpapi.WriteJSON(w, http.StatusAccepted, map[string]any{"recording": recording})
		},
	)))

	mux.Handle("POST /v1/operations/transcriptions/{recordingId}/analyze", requireAccount(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			access, ok := accessFromRequest(r)
			if !ok {
				writeError(w, r, ErrForbidden)
				return
			}
			recording, err := service.RequestAnalysis(
				r.Context(),
				access,
				r.PathValue("recordingId"),
			)
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpapi.WriteJSON(w, http.StatusAccepted, map[string]any{"recording": recording})
		},
	)))

	mux.Handle("GET /v1/operations/transcriptions/{recordingId}/audio", requireAccount(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			access, ok := accessFromRequest(r)
			if !ok {
				writeError(w, r, ErrForbidden)
				return
			}
			audio, err := service.OpenAudio(
				r.Context(),
				access,
				r.PathValue("recordingId"),
			)
			if err != nil {
				writeError(w, r, err)
				return
			}
			defer func() { _ = audio.File.Close() }()

			w.Header().Set("Content-Type", audio.MimeType)
			w.Header().Set("Cache-Control", "private, max-age=60")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set(
				"Content-Disposition",
				fmt.Sprintf(`inline; filename="%s"`, audio.FileName),
			)
			http.ServeContent(w, r, audio.FileName, audio.ModTime, audio.File)
		},
	)))
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar as transcricoes.")
	case errors.Is(err, ErrFeatureDisabled):
		httpapi.WriteError(w, r, http.StatusConflict, "feature_disabled", "A gravacao experimental esta desativada.")
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "recording_not_found", "Gravacao ou atendimento nao encontrado.")
	case errors.Is(err, ErrValidation):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Verifique os dados da gravacao.")
	case errors.Is(err, ErrUnsupported):
		httpapi.WriteError(w, r, http.StatusUnsupportedMediaType, "unsupported_audio", "Formato de audio nao suportado.")
	case errors.Is(err, ErrTooLarge):
		httpapi.WriteError(w, r, http.StatusRequestEntityTooLarge, "audio_too_large", "O bloco de audio ultrapassou o limite.")
	case errors.Is(err, ErrChunkConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "chunk_conflict", "Esta parte do audio ja existe com outro conteudo.")
	case errors.Is(err, ErrNotReady):
		httpapi.WriteError(w, r, http.StatusConflict, "recording_not_ready", "A gravacao ainda nao possui todas as partes.")
	case errors.Is(err, ErrCredentialUnavailable):
		httpapi.WriteError(w, r, http.StatusConflict, "ai_credential_missing", "Selecione uma credencial global de IA valida.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Nao foi possivel processar a gravacao.")
	}
}
