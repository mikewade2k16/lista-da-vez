package feedback

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

type listResponse struct {
	Feedbacks []FeedbackView `json:"feedbacks"`
}

type feedbackResponse struct {
	Feedback FeedbackView `json:"feedback"`
}

type messagesResponse struct {
	Messages []FeedbackMessageView `json:"messages"`
}

type messageResponse struct {
	Message FeedbackMessageView `json:"message"`
}

type createRequest struct {
	Kind    string `json:"kind"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type updateRequest struct {
	Status    *string `json:"status"`
	AdminNote *string `json:"admin_note"`
}

type createMessageRequest struct {
	Body string `json:"body"`
}

func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	mux.Handle("POST /v1/feedback", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		input, err := readCreateInput(r)
		if err != nil {
			if errors.Is(err, ErrInvalidImage) {
				writeServiceError(w, r, err)
				return
			}
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		result, err := service.Create(r.Context(), principal, input)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusCreated, feedbackResponse{
			Feedback: *result,
		})
	})))

	mux.Handle("GET /v1/feedback", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		since, err := parseOptionalTime(r.URL.Query().Get("since"))
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_since", "Data de atualizacao invalida.")
			return
		}

		feedbacks, err := service.List(r.Context(), principal, ListInput{
			Kind:   strings.TrimSpace(r.URL.Query().Get("kind")),
			Status: strings.TrimSpace(r.URL.Query().Get("status")),
			Since:  since,
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, listResponse{
			Feedbacks: feedbacks,
		})
	})))

	mux.Handle("GET /v1/feedback/me", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		since, err := parseOptionalTime(r.URL.Query().Get("since"))
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_since", "Data de atualizacao invalida.")
			return
		}

		feedbacks, err := service.ListMine(r.Context(), principal, ListInput{
			Kind:   strings.TrimSpace(r.URL.Query().Get("kind")),
			Status: strings.TrimSpace(r.URL.Query().Get("status")),
			Since:  since,
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, listResponse{
			Feedbacks: feedbacks,
		})
	})))

	mux.Handle("PATCH /v1/feedback/{id}", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		id := strings.TrimSpace(r.PathValue("id"))
		if id == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_id", "ID invalido.")
			return
		}

		var request updateRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		result, err := service.Update(r.Context(), principal, id, UpdateInput{
			Status:    request.Status,
			AdminNote: request.AdminNote,
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, feedbackResponse{
			Feedback: *result,
		})
	})))

	mux.Handle("GET /v1/feedback/{id}/messages", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		id := strings.TrimSpace(r.PathValue("id"))
		if id == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_id", "ID invalido.")
			return
		}

		after, err := parseOptionalTime(r.URL.Query().Get("after"))
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_after", "Data de mensagem invalida.")
			return
		}

		messages, err := service.ListMessages(r.Context(), principal, id, ListMessagesInput{
			After: after,
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, messagesResponse{
			Messages: messages,
		})
	})))

	mux.Handle("POST /v1/feedback/{id}/read", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		id := strings.TrimSpace(r.PathValue("id"))
		if id == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_id", "ID invalido.")
			return
		}

		result, err := service.MarkRead(r.Context(), principal, id)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, feedbackResponse{
			Feedback: *result,
		})
	})))

	mux.Handle("POST /v1/feedback/{id}/messages", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		id := strings.TrimSpace(r.PathValue("id"))
		if id == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_id", "ID invalido.")
			return
		}

		input, err := readCreateMessageInput(r)
		if err != nil {
			if errors.Is(err, ErrInvalidImage) {
				writeServiceError(w, r, err)
				return
			}
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		message, err := service.CreateMessage(r.Context(), principal, id, input)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusCreated, messageResponse{
			Message: *message,
		})
	})))
}

func readCreateInput(r *http.Request) (CreateInput, error) {
	if isMultipartRequest(r) {
		if err := r.ParseMultipartForm(maxFeedbackMultipartMemory); err != nil {
			return CreateInput{}, ErrInvalidImage
		}

		image, err := readOptionalImageUpload(r, "image")
		if err != nil {
			return CreateInput{}, err
		}

		return CreateInput{
			Kind:    strings.TrimSpace(r.FormValue("kind")),
			Subject: strings.TrimSpace(r.FormValue("subject")),
			Body:    strings.TrimSpace(r.FormValue("body")),
			Image:   image,
		}, nil
	}

	var request createRequest
	if err := httpapi.ReadJSON(r, &request); err != nil {
		return CreateInput{}, err
	}

	return CreateInput{
		Kind:    request.Kind,
		Subject: request.Subject,
		Body:    request.Body,
	}, nil
}

func readCreateMessageInput(r *http.Request) (CreateMessageInput, error) {
	if isMultipartRequest(r) {
		if err := r.ParseMultipartForm(maxFeedbackMultipartMemory); err != nil {
			return CreateMessageInput{}, ErrInvalidImage
		}

		image, err := readOptionalImageUpload(r, "image")
		if err != nil {
			return CreateMessageInput{}, err
		}

		return CreateMessageInput{
			Body:  strings.TrimSpace(r.FormValue("body")),
			Image: image,
		}, nil
	}

	var request createMessageRequest
	if err := httpapi.ReadJSON(r, &request); err != nil {
		return CreateMessageInput{}, err
	}

	return CreateMessageInput{Body: request.Body}, nil
}

func isMultipartRequest(r *http.Request) bool {
	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	return strings.HasPrefix(contentType, "multipart/form-data")
}

func readOptionalImageUpload(r *http.Request, fieldName string) (*ImageUpload, error) {
	file, header, err := r.FormFile(fieldName)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return nil, nil
		}
		return nil, ErrInvalidImage
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, maxFeedbackImageBytes+1))
	if err != nil {
		return nil, ErrInvalidImage
	}
	if len(content) == 0 || len(content) > maxFeedbackImageBytes {
		return nil, ErrInvalidImage
	}

	return &ImageUpload{
		FileName:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		Content:     content,
	}, nil
}

func parseOptionalTime(rawValue string) (*time.Time, error) {
	value := strings.TrimSpace(rawValue)
	if value == "" {
		return nil, nil
	}

	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, err
	}

	return &parsed, nil
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "feedback_not_found", "Feedback nao encontrado.")
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar este recurso.")
	case errors.Is(err, ErrClosed):
		httpapi.WriteError(w, r, http.StatusConflict, "feedback_closed", "Este chamado esta encerrado e nao aceita novas mensagens.")
	case errors.Is(err, ErrInvalidImage):
		httpapi.WriteError(w, r, http.StatusBadRequest, "feedback_invalid_image", "Envie uma imagem JPG, PNG ou WebP com ate 1 MB.")
	case errors.Is(err, ErrInvalid):
		httpapi.WriteError(w, r, http.StatusBadRequest, "feedback_invalid", "Dados do feedback invalidos.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao processar feedback.")
	}
}
