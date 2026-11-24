package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

var queryUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ErrInvalidQuery indica parametro de query invalido; a mensagem ja e amigavel.
var ErrInvalidQuery = errors.New("invalid query parameter")

// IsUUID informa se o texto (sem espacos nas pontas) e um UUID canonico.
func IsUUID(value string) bool {
	return queryUUIDPattern.MatchString(strings.TrimSpace(value))
}

// QueryUUID le um UUID opcional da query. Vazio retorna "" sem erro.
func QueryUUID(r *http.Request, name string) (string, error) {
	value := strings.TrimSpace(r.URL.Query().Get(name))
	if value == "" {
		return "", nil
	}
	if !IsUUID(value) {
		return "", fmt.Errorf("%w: O parametro %q deve ser um UUID valido", ErrInvalidQuery, name)
	}
	return value, nil
}

// QueryInt le um inteiro opcional da query, com limites opcionais (min<=max).
func QueryInt(r *http.Request, name string, min, max int) (int, error) {
	value := strings.TrimSpace(r.URL.Query().Get(name))
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || (max >= min && (parsed < min || parsed > max)) {
		return 0, fmt.Errorf("%w: O parametro %q deve ser um numero inteiro valido", ErrInvalidQuery, name)
	}
	return parsed, nil
}

// QueryUUIDs le varios UUIDs opcionais; devolve na mesma ordem dos nomes.
func QueryUUIDs(r *http.Request, names ...string) ([]string, error) {
	out := make([]string, 0, len(names))
	for _, name := range names {
		value, err := QueryUUID(r, name)
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}

// WriteInvalidQuery responde 400 validation_error se err for ErrInvalidQuery.
// Retorna true quando respondeu.
func WriteInvalidQuery(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil || !errors.Is(err, ErrInvalidQuery) {
		return false
	}
	message := strings.TrimPrefix(err.Error(), ErrInvalidQuery.Error()+": ")
	WriteError(w, r, http.StatusBadRequest, "validation_error", message+".")
	return true
}
