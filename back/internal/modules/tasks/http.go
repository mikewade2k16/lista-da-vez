package tasks

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

type HTTPHandler struct {
	service *Service
}

func NewHTTPHandler(service *Service) *HTTPHandler {
	return &HTTPHandler{service: service}
}

func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	NewHTTPHandler(service).RegisterRoutes(mux, middleware)
}

func (handler *HTTPHandler) RegisterRoutes(mux *http.ServeMux, middleware *auth.Middleware) {
	mux.Handle("GET /v1/tasks/boards", middleware.RequireAuth(handler.withPermission(PermBoardsView, handler.listBoards)))
	mux.Handle("POST /v1/tasks/boards", middleware.RequireAuth(handler.withPermission(PermBoardsManage, handler.createBoard)))
	mux.Handle("GET /v1/tasks/boards/{boardId}", middleware.RequireAuth(handler.withPermission(PermBoardsView, handler.getBoard)))
	mux.Handle("PATCH /v1/tasks/boards/{boardId}", middleware.RequireAuth(handler.withPermission(PermBoardsManage, handler.updateBoard)))
	mux.Handle("POST /v1/tasks/boards/{boardId}/columns", middleware.RequireAuth(handler.withPermission(PermBoardsManage, handler.createColumn)))
	mux.Handle("PATCH /v1/tasks/columns/{columnId}", middleware.RequireAuth(handler.withPermission(PermBoardsManage, handler.updateColumn)))
	mux.Handle("DELETE /v1/tasks/columns/{columnId}", middleware.RequireAuth(handler.withPermission(PermBoardsManage, handler.deleteColumn)))
	mux.Handle("POST /v1/tasks/boards/{boardId}/fields", middleware.RequireAuth(handler.withPermission(PermBoardsManage, handler.createField)))
	mux.Handle("GET /v1/tasks/boards/{boardId}/tasks", middleware.RequireAuth(handler.withPermission(PermTasksView, handler.listTasks)))
	mux.Handle("POST /v1/tasks/boards/{boardId}/tasks", middleware.RequireAuth(handler.withPermission(PermTasksCreate, handler.createTask)))
	mux.Handle("GET /v1/tasks/{taskId}", middleware.RequireAuth(handler.withPermission(PermTasksView, handler.getTask)))
	mux.Handle("PATCH /v1/tasks/{taskId}", middleware.RequireAuth(handler.withPermission(PermTasksEdit, handler.updateTask)))
	mux.Handle("DELETE /v1/tasks/{taskId}", middleware.RequireAuth(handler.withPermission(PermTasksDelete, handler.archiveTask)))
	mux.Handle("POST /v1/tasks/{taskId}/move", middleware.RequireAuth(handler.withPermission(PermTasksEdit, handler.moveTask)))
	mux.Handle("GET /v1/tasks/{taskId}/comments", middleware.RequireAuth(handler.withPermission(PermTasksView, handler.listComments)))
	mux.Handle("POST /v1/tasks/{taskId}/comments", middleware.RequireAuth(handler.withPermission(PermTasksComment, handler.addComment)))
	mux.Handle("POST /v1/tasks/{taskId}/shares", middleware.RequireAuth(handler.withPermission(PermSharesManage, handler.addShare)))
	mux.Handle("GET /v1/tasks/{taskId}/relations", middleware.RequireAuth(handler.withPermission(PermTasksView, handler.listRelations)))
	mux.Handle("POST /v1/tasks/{taskId}/relations", middleware.RequireAuth(handler.withPermission(PermRelationsManage, handler.addRelation)))
	mux.Handle("GET /v1/tasks/{taskId}/relations:expand", middleware.RequireAuth(handler.withPermission(PermTasksView, handler.expandRelations)))
	mux.Handle("GET /v1/tasks/{taskId}/audit", middleware.RequireAuth(handler.withPermission(PermBoardsManage, handler.listAudit)))

	handler.registerTrackingRoutes(mux, middleware)
}

type taskHTTPContext struct {
	Access AccessContext
}

func (handler *HTTPHandler) withPermission(permission string, next func(http.ResponseWriter, *http.Request, taskHTTPContext)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		accountID := strings.TrimSpace(r.Header.Get("X-Account-Id"))
		if accountID == "" {
			accountID = strings.TrimSpace(r.URL.Query().Get("accountId"))
		}
		if accountID == "" {
			accountID = strings.TrimSpace(principal.TenantID)
		}

		access, err := handler.service.ResolveAccessContext(r.Context(), principal, accountID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		if !access.Has(permission) {
			writeServiceError(w, r, ErrForbidden)
			return
		}

		next(w, r, taskHTTPContext{Access: access})
	})
}

func (handler *HTTPHandler) listBoards(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	boards, err := handler.service.ListBoards(r.Context(), ctx.Access)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"boards": boards})
}

func (handler *HTTPHandler) createBoard(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	var input CreateBoardInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
		return
	}
	board, err := handler.service.CreateBoard(r.Context(), ctx.Access, input)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"board": board})
}

func (handler *HTTPHandler) getBoard(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	board, err := handler.service.GetBoard(r.Context(), ctx.Access, r.PathValue("boardId"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"board": board})
}

func (handler *HTTPHandler) updateBoard(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	var input UpdateBoardInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
		return
	}
	input.ID = strings.TrimSpace(r.PathValue("boardId"))
	board, err := handler.service.UpdateBoard(r.Context(), ctx.Access, input)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"board": board})
}

func (handler *HTTPHandler) createColumn(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	var input CreateColumnInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
		return
	}
	input.BoardID = strings.TrimSpace(r.PathValue("boardId"))
	column, err := handler.service.CreateColumn(r.Context(), ctx.Access, input)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"column": column})
}

func (handler *HTTPHandler) updateColumn(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	var input UpdateColumnInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
		return
	}
	input.ID = strings.TrimSpace(r.PathValue("columnId"))
	column, err := handler.service.UpdateColumn(r.Context(), ctx.Access, input)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"column": column})
}

func (handler *HTTPHandler) deleteColumn(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	var input DeleteColumnInput
	if r.Body != nil && r.ContentLength != 0 {
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}
	}
	input.ID = strings.TrimSpace(r.PathValue("columnId"))
	if err := handler.service.DeleteColumn(r.Context(), ctx.Access, input); err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler *HTTPHandler) createField(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	var input CreateFieldInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
		return
	}
	input.BoardID = strings.TrimSpace(r.PathValue("boardId"))
	field, err := handler.service.CreateField(r.Context(), ctx.Access, input)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"field": field})
}

func (handler *HTTPHandler) listTasks(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	limit, err := parseOptionalInt(r.URL.Query().Get("limit"))
	if err != nil {
		writeServiceError(w, r, ErrValidation)
		return
	}
	includeArchived, err := parseOptionalBool(r.URL.Query().Get("archived"))
	if err != nil {
		writeServiceError(w, r, ErrValidation)
		return
	}
	tasks, err := handler.service.ListTasks(r.Context(), ctx.Access, ListTasksInput{
		BoardID:         strings.TrimSpace(r.PathValue("boardId")),
		Limit:           limit,
		Cursor:          strings.TrimSpace(r.URL.Query().Get("cursor")),
		IncludeArchived: includeArchived,
	})
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"tasks": tasks})
}

func (handler *HTTPHandler) createTask(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	var input CreateTaskInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
		return
	}
	input.BoardID = strings.TrimSpace(r.PathValue("boardId"))
	task, err := handler.service.CreateTask(r.Context(), ctx.Access, input)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"task": task})
}

func (handler *HTTPHandler) getTask(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	task, err := handler.service.GetTask(r.Context(), ctx.Access, r.PathValue("taskId"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"task": task})
}

func (handler *HTTPHandler) updateTask(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	var input UpdateTaskInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
		return
	}
	input.ID = strings.TrimSpace(r.PathValue("taskId"))
	input.ExpectedVersion = parseIfMatch(r.Header.Get("If-Match"))
	task, err := handler.service.UpdateTask(r.Context(), ctx.Access, input)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"task": task})
}

func (handler *HTTPHandler) moveTask(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	var input MoveTaskInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
		return
	}
	input.ID = strings.TrimSpace(r.PathValue("taskId"))
	input.ExpectedVersion = parseIfMatch(r.Header.Get("If-Match"))
	task, err := handler.service.MoveTask(r.Context(), ctx.Access, input)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"task": task})
}

func (handler *HTTPHandler) archiveTask(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	if err := handler.service.ArchiveTask(r.Context(), ctx.Access, r.PathValue("taskId")); err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler *HTTPHandler) listComments(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	comments, err := handler.service.ListComments(r.Context(), ctx.Access, r.PathValue("taskId"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"comments": comments})
}

func (handler *HTTPHandler) addComment(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	var input AddCommentInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
		return
	}
	input.TaskID = strings.TrimSpace(r.PathValue("taskId"))
	comment, err := handler.service.AddComment(r.Context(), ctx.Access, input)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"comment": comment})
}

func (handler *HTTPHandler) addShare(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	var input AddShareInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
		return
	}
	input.TaskID = strings.TrimSpace(r.PathValue("taskId"))
	share, err := handler.service.AddShare(r.Context(), ctx.Access, input)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"share": share})
}

func (handler *HTTPHandler) listRelations(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	relations, err := handler.service.ListRelations(r.Context(), ctx.Access, r.PathValue("taskId"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"relations": relations})
}

func (handler *HTTPHandler) addRelation(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	var input AddRelationInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
		return
	}
	input.TaskID = strings.TrimSpace(r.PathValue("taskId"))
	relation, err := handler.service.AddRelation(r.Context(), ctx.Access, input)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"relation": relation})
}

func (handler *HTTPHandler) listAudit(w http.ResponseWriter, r *http.Request, ctx taskHTTPContext) {
	entries, err := handler.service.ListAudit(r.Context(), ctx.Access, r.PathValue("taskId"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"audit": entries})
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar este recurso.")
	case errors.Is(err, ErrAccountRequired), errors.Is(err, ErrValidation):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Verifique os dados enviados.")
	case errors.Is(err, ErrAccountNotFound), errors.Is(err, ErrBoardNotFound), errors.Is(err, ErrColumnNotFound), errors.Is(err, ErrFieldNotFound), errors.Is(err, ErrTaskNotFound), errors.Is(err, ErrShareRequired):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Recurso nao encontrado.")
	case errors.Is(err, ErrVersionConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "version_conflict", "O recurso foi alterado por outra pessoa.")
	case errors.Is(err, ErrTimeEntryNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "tracking_not_found", "Tracking ativo nao encontrado.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao processar tasks.")
	}
}

func parseOptionalInt(raw string) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
}

func parseOptionalBool(raw string) (bool, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return false, nil
	}
	return strconv.ParseBool(value)
}

func parseIfMatch(raw string) *int {
	value := strings.Trim(strings.TrimSpace(raw), "\"")
	if value == "" {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	return &parsed
}
