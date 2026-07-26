package customerdata

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

const maxJSONBodyBytes = 1 << 20

func RegisterRoutes(
	mux *http.ServeMux,
	service *Service,
	middleware *auth.Middleware,
	modulesGuard *httpapi.AccountModulesGuard,
) {
	registerCustomerDataHandlers(mux, service, func(handler http.Handler) http.Handler {
		wrapped := middleware.RequireAuthWithAccount(handler)
		if modulesGuard != nil {
			wrapped = modulesGuard.RequireModule(ModuleID)(wrapped)
		}
		return wrapped
	})
}

func registerCustomerDataHandlers(
	mux *http.ServeMux,
	service *Service,
	wrap func(http.Handler) http.Handler,
) {
	registerSubjectRoutes(mux, service, wrap)
	registerIdentityRoutes(mux, service, wrap)
	registerNoteConsentRoutes(mux, service, wrap)
	registerOfflineRoutes(mux, service, wrap)
	registerMatchingRoutes(mux, service, wrap)
	registerSegmentRoutes(mux, service, wrap)
	registerControlStateRoutes(mux, service, wrap)
}

func route(mux *http.ServeMux, pattern string, wrap func(http.Handler) http.Handler, handler http.HandlerFunc) {
	mux.Handle(pattern, wrap(handler))
}

func requestPrincipal(w http.ResponseWriter, r *http.Request) (auth.Principal, bool) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok || principal.AccountID == "" {
		writeCustomerDataError(w, ErrForbidden)
		return auth.Principal{}, false
	}
	return principal, true
}

func decodeStrictJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeCustomerDataError(w, invalid("body", "invalid_json"))
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		writeCustomerDataError(w, invalid("body", "trailing_json"))
		return false
	}
	return true
}

func writeCustomerDataJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeCustomerDataError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	message := "não foi possível concluir a operação"
	switch {
	case errors.Is(err, ErrValidation):
		status, code, message = http.StatusBadRequest, "invalid_request", "requisição inválida"
	case errors.Is(err, ErrNotFound):
		status, code, message = http.StatusNotFound, "not_found", "recurso não encontrado"
	case errors.Is(err, ErrForbidden):
		status, code, message = http.StatusForbidden, "forbidden", "acesso negado"
	case errors.Is(err, ErrConflict):
		status, code, message = http.StatusConflict, "conflict", "estado ou revisão conflitante"
	case errors.Is(err, ErrCapabilityDisabled):
		status, code, message = http.StatusConflict, "capability_disabled", "capability desativada"
	case errors.Is(err, ErrWriterInactive):
		status, code, message = http.StatusConflict, "writer_inactive", "writer novo ainda não é autoritativo"
	case errors.Is(err, ErrIdentityProtectionUnavailable):
		status, code, message = http.StatusServiceUnavailable, "identity_protection_unavailable", "proteção de dados indisponível"
	}
	writeCustomerDataJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

func queryLimit(r *http.Request) int {
	value, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	return value
}

func queryExpectedRevision(r *http.Request) (int64, error) {
	value, err := strconv.ParseInt(r.URL.Query().Get("expectedRevision"), 10, 64)
	if err != nil || value <= 0 {
		return 0, invalid("expectedRevision", "must_be_positive")
	}
	return value, nil
}

func queryOptionalTime(r *http.Request, key string) (*time.Time, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return nil, nil
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, invalid(key, "invalid_datetime")
	}
	return &value, nil
}

func queryOptionalBool(r *http.Request, key string) (*bool, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, invalid(key, "invalid_boolean")
	}
	return &value, nil
}
