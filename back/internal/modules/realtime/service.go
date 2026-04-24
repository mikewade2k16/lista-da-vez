package realtime

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/operations"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/stores"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/tenants"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024
)

type TokenAuthenticator interface {
	AuthenticateToken(ctx context.Context, token string) (auth.Principal, error)
}

type StoreFinder interface {
	FindAccessible(ctx context.Context, principal auth.Principal, storeID string) (stores.StoreView, error)
}

type TenantLister interface {
	ListAccessible(ctx context.Context, principal auth.Principal) ([]tenants.TenantView, error)
}

type Service struct {
	authenticator  TokenAuthenticator
	storeFinder    StoreFinder
	tenantLister   TenantLister
	allowedOrigins []string
	hub            *Hub
	upgrader       websocket.Upgrader
}

func NewService(authenticator TokenAuthenticator, storeFinder StoreFinder, tenantLister TenantLister, allowedOrigins []string, hub *Hub) *Service {
	service := &Service{
		authenticator:  authenticator,
		storeFinder:    storeFinder,
		tenantLister:   tenantLister,
		allowedOrigins: append([]string{}, allowedOrigins...),
		hub:            hub,
	}

	service.upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin == "" {
				return true
			}

			return httpapi.OriginAllowed(origin, service.allowedOrigins)
		},
	}

	return service
}

func (service *Service) SetStoreFinder(storeFinder StoreFinder) {
	service.storeFinder = storeFinder
}

func (service *Service) PublishOperationEvent(ctx context.Context, event operations.PublishedEvent) {
	normalizedStoreID := strings.TrimSpace(event.StoreID)
	if normalizedStoreID == "" {
		return
	}

	service.hub.Publish(operationTopic(normalizedStoreID), Event{
		Type:     EventTypeOperationUpdated,
		StoreID:  normalizedStoreID,
		Action:   strings.TrimSpace(event.Action),
		PersonID: strings.TrimSpace(event.PersonID),
		SavedAt:  event.SavedAt.UTC(),
	})
}

func (service *Service) PublishContextEvent(_ context.Context, tenantID string, resource string, action string, resourceID string, savedAt time.Time) {
	normalizedTenantID := strings.TrimSpace(tenantID)
	if normalizedTenantID == "" {
		return
	}

	service.hub.Publish(contextTopic(normalizedTenantID), Event{
		Type:       EventTypeContextUpdated,
		TenantID:   normalizedTenantID,
		Resource:   strings.TrimSpace(resource),
		Action:     strings.TrimSpace(action),
		ResourceID: strings.TrimSpace(resourceID),
		SavedAt:    savedAt.UTC(),
	})
}

func (service *Service) HandleOperationSocket(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("access_token"))
	if token == "" {
		authorizationHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if authorizationHeader != "" {
			bearerToken, err := auth.ExtractBearerToken(authorizationHeader)
			if err == nil {
				token = bearerToken
			}
		}
	}

	principal, err := service.authenticator.AuthenticateToken(r.Context(), token)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUnauthorized), errors.Is(err, auth.ErrUserInactive):
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
		default:
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao validar a sessao.")
		}
		return
	}

	if !operations.CanAccessOperationsRole(string(principal.Role)) {
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar este recurso.")
		return
	}

	storeID := strings.TrimSpace(r.URL.Query().Get("storeId"))
	if storeID == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Loja obrigatoria.")
		return
	}

	if _, err := service.storeFinder.FindAccessible(r.Context(), principal, storeID); err != nil {
		switch {
		case errors.Is(err, stores.ErrForbidden):
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar este recurso.")
		case errors.Is(err, stores.ErrStoreNotFound):
			httpapi.WriteError(w, r, http.StatusNotFound, "store_not_found", "Loja nao encontrada.")
		default:
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao validar a loja.")
		}
		return
	}

	connection, err := service.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer connection.Close()

	connection.SetReadLimit(maxMessageSize)
	connection.SetReadDeadline(time.Now().Add(pongWait))
	connection.SetPongHandler(func(string) error {
		connection.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	subscription := service.hub.Subscribe(operationTopic(storeID), 1)
	defer subscription.Close()

	done := make(chan struct{})
	go service.readPump(connection, done)

	if err := service.writeEvent(connection, Event{
		Type:    EventTypeConnected,
		StoreID: storeID,
		SavedAt: time.Now().UTC(),
	}); err != nil {
		return
	}

	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-r.Context().Done():
			return
		case <-ticker.C:
			connection.SetWriteDeadline(time.Now().Add(writeWait))
			if err := connection.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case event, ok := <-subscription.Events():
			if !ok {
				return
			}

			if err := service.writeEvent(connection, event); err != nil {
				return
			}
		}
	}
}

func (service *Service) HandleContextSocket(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("access_token"))
	if token == "" {
		authorizationHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if authorizationHeader != "" {
			bearerToken, err := auth.ExtractBearerToken(authorizationHeader)
			if err == nil {
				token = bearerToken
			}
		}
	}

	principal, err := service.authenticator.AuthenticateToken(r.Context(), token)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUnauthorized), errors.Is(err, auth.ErrUserInactive):
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
		default:
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao validar a sessao.")
		}
		return
	}

	tenantID, err := service.resolveContextTenantID(r.Context(), principal, strings.TrimSpace(r.URL.Query().Get("tenantId")))
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrForbidden):
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar este recurso.")
		default:
			httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Tenant invalido para sincronizacao.")
		}
		return
	}

	connection, err := service.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer connection.Close()

	connection.SetReadLimit(maxMessageSize)
	connection.SetReadDeadline(time.Now().Add(pongWait))
	connection.SetPongHandler(func(string) error {
		connection.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	subscription := service.hub.Subscribe(contextTopic(tenantID), 1)
	defer subscription.Close()

	done := make(chan struct{})
	go service.readPump(connection, done)

	if err := service.writeEvent(connection, Event{
		Type:     EventTypeConnected,
		TenantID: tenantID,
		SavedAt:  time.Now().UTC(),
	}); err != nil {
		return
	}

	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-r.Context().Done():
			return
		case <-ticker.C:
			connection.SetWriteDeadline(time.Now().Add(writeWait))
			if err := connection.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case event, ok := <-subscription.Events():
			if !ok {
				return
			}

			if err := service.writeEvent(connection, event); err != nil {
				return
			}
		}
	}
}

func (service *Service) resolveContextTenantID(ctx context.Context, principal auth.Principal, requestedTenantID string) (string, error) {
	normalizedTenantID := strings.TrimSpace(requestedTenantID)

	if principal.Role != auth.RolePlatformAdmin {
		if principal.TenantID == "" {
			return "", auth.ErrForbidden
		}

		if normalizedTenantID != "" && normalizedTenantID != principal.TenantID {
			return "", auth.ErrForbidden
		}

		return principal.TenantID, nil
	}

	accessibleTenants, err := service.tenantLister.ListAccessible(ctx, principal)
	if err != nil {
		return "", err
	}

	if normalizedTenantID == "" {
		if len(accessibleTenants) == 1 {
			return strings.TrimSpace(accessibleTenants[0].ID), nil
		}

		return "", auth.ErrForbidden
	}

	for _, tenantView := range accessibleTenants {
		if strings.TrimSpace(tenantView.ID) == normalizedTenantID {
			return normalizedTenantID, nil
		}
	}

	return "", auth.ErrForbidden
}

func (service *Service) readPump(connection *websocket.Conn, done chan<- struct{}) {
	defer close(done)

	for {
		if _, _, err := connection.ReadMessage(); err != nil {
			return
		}
	}
}

func (service *Service) writeEvent(connection *websocket.Conn, event Event) error {
	connection.SetWriteDeadline(time.Now().Add(writeWait))
	return connection.WriteJSON(event)
}
