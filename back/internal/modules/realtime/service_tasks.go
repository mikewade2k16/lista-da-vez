package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	tasksmodule "github.com/mikewade2k16/lista-da-vez/back/internal/modules/tasks"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

const (
	tasksSubscriptionBuffer    = 16
	realtimeClientRateLimit    = 30
	realtimeClientRateInterval = time.Second
)

var (
	errRealtimeValidation  = errors.New("realtime: validation")
	errRealtimeForbidden   = errors.New("realtime: forbidden")
	errRealtimeNotFound    = errors.New("realtime: not found")
	errRealtimeUnavailable = errors.New("realtime: unavailable")
)

type realtimeSubscription struct {
	topic     string
	accountID string
	boardID   string
	taskID    string
	userID    string
}

type presenceClientMessage struct {
	Type     string `json:"type"`
	FieldKey string `json:"fieldKey"`
	LockID   string `json:"lockId"`
}

type socketRateLimiter struct {
	limit       int
	interval    time.Duration
	windowStart time.Time
	count       int
}

func newSocketRateLimiter(limit int, interval time.Duration) *socketRateLimiter {
	return &socketRateLimiter{
		limit:       limit,
		interval:    interval,
		windowStart: time.Now(),
	}
}

func (limiter *socketRateLimiter) Allow(now time.Time) bool {
	if now.Sub(limiter.windowStart) >= limiter.interval {
		limiter.windowStart = now
		limiter.count = 0
	}

	limiter.count++
	return limiter.count <= limiter.limit
}

func (service *Service) PublishTaskEvent(_ context.Context, event tasksmodule.TaskEvent) {
	if service.hub == nil {
		return
	}

	eventType := strings.TrimSpace(event.Type)
	accountID := strings.TrimSpace(event.AccountID)
	boardID := strings.TrimSpace(event.BoardID)
	taskID := strings.TrimSpace(event.TaskID)
	if eventType == "" {
		return
	}

	realtimeEvent := Event{
		Type:      eventType,
		AccountID: accountID,
		BoardID:   boardID,
		TaskID:    taskID,
		Version:   event.Version,
		SavedAt:   time.Now().UTC(),
	}

	if accountID != "" {
		service.hub.Publish(tasksAccountTopic(accountID), realtimeEvent)
	}
	if boardID != "" {
		service.hub.Publish(tasksBoardTopic(boardID), realtimeEvent)
	}
	if taskID != "" {
		service.hub.Publish(tasksTaskTopic(taskID), realtimeEvent)
	}
}

func (service *Service) PublishBoardEvent(_ context.Context, event tasksmodule.BoardEvent) {
	if service.hub == nil {
		return
	}

	eventType := strings.TrimSpace(event.Type)
	accountID := strings.TrimSpace(event.AccountID)
	boardID := strings.TrimSpace(event.BoardID)
	if eventType == "" {
		return
	}

	realtimeEvent := Event{
		Type:      eventType,
		AccountID: accountID,
		BoardID:   boardID,
		SavedAt:   time.Now().UTC(),
	}

	if accountID != "" {
		service.hub.Publish(tasksAccountTopic(accountID), realtimeEvent)
	}
	if boardID != "" {
		service.hub.Publish(tasksBoardTopic(boardID), realtimeEvent)
	}
}

func (service *Service) PublishPresenceEvent(_ context.Context, event tasksmodule.PresenceEvent) {
	if service.hub == nil {
		return
	}

	eventType := strings.TrimSpace(event.Type)
	boardID := strings.TrimSpace(event.BoardID)
	taskID := strings.TrimSpace(event.TaskID)
	if eventType == "" || (boardID == "" && taskID == "") {
		return
	}

	realtimeEvent := Event{
		Type:        eventType,
		AccountID:   strings.TrimSpace(event.AccountID),
		BoardID:     boardID,
		TaskID:      taskID,
		UserID:      strings.TrimSpace(event.UserID),
		DisplayName: strings.TrimSpace(event.DisplayName),
		AvatarPath:  strings.TrimSpace(event.AvatarPath),
		FieldKey:    strings.TrimSpace(event.FieldKey),
		LockID:      strings.TrimSpace(event.LockID),
		SavedAt:     time.Now().UTC(),
	}

	if taskID != "" {
		service.hub.Publish(presenceTaskTopic(taskID), realtimeEvent)
		return
	}
	service.hub.Publish(presenceBoardTopic(boardID), realtimeEvent)
}

func (service *Service) PublishNotificationEvent(_ context.Context, userID string, eventType string, notificationID string, payload map[string]any) {
	if service.hub == nil {
		return
	}

	userID = strings.TrimSpace(userID)
	eventType = strings.TrimSpace(eventType)
	if userID == "" || eventType == "" {
		return
	}

	service.hub.Publish(notificationsUserTopic(userID), Event{
		Type:           eventType,
		UserID:         userID,
		NotificationID: strings.TrimSpace(notificationID),
		Payload:        payload,
		SavedAt:        time.Now().UTC(),
	})
}

func (service *Service) HandleTasksSocket(w http.ResponseWriter, r *http.Request) {
	principal, ok := service.authenticateRealtimeRequest(w, r)
	if !ok {
		return
	}

	subscription, err := service.resolveTasksSubscription(r.Context(), principal, r)
	if err != nil {
		service.writeRealtimeAccessError(w, r, err, "Canal de tasks invalido.", "Canal de tasks nao encontrado.")
		return
	}

	service.serveSubscriptionSocket(w, r, subscription.topic, tasksSubscriptionBuffer, Event{
		Type:      EventTypeConnected,
		AccountID: subscription.accountID,
		BoardID:   subscription.boardID,
		TaskID:    subscription.taskID,
		SavedAt:   time.Now().UTC(),
	}, nil, nil, service.readPumpWithRateLimit)
}

func (service *Service) HandlePresenceSocket(w http.ResponseWriter, r *http.Request) {
	principal, ok := service.authenticateRealtimeRequest(w, r)
	if !ok {
		return
	}

	if service.presence == nil {
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "realtime_unavailable", "Realtime de presenca indisponivel.")
		return
	}

	subscription, err := service.resolvePresenceSubscription(r.Context(), principal, r)
	if err != nil {
		service.writeRealtimeAccessError(w, r, err, "Canal de presenca invalido.", "Canal de presenca nao encontrado.")
		return
	}

	user := PresenceUser{
		UserID:      strings.TrimSpace(principal.UserID),
		DisplayName: strings.TrimSpace(principal.DisplayName),
	}

	onConnected := func() []Event {
		snapshot := service.presence.Join(subscription.topic, user)
		return []Event{{
			Type:         EventTypePresenceSnapshot,
			AccountID:    subscription.accountID,
			BoardID:      subscription.boardID,
			TaskID:       subscription.taskID,
			Participants: snapshot,
			SavedAt:      time.Now().UTC(),
		}}
	}

	onClose := func() {
		service.presence.Leave(subscription.topic, user.UserID)
	}

	readPump := func(connection *websocket.Conn, done chan<- struct{}) {
		service.readPresencePump(connection, done, subscription.topic, user)
	}

	service.serveSubscriptionSocket(w, r, subscription.topic, tasksSubscriptionBuffer, Event{
		Type:      EventTypeConnected,
		AccountID: subscription.accountID,
		BoardID:   subscription.boardID,
		TaskID:    subscription.taskID,
		UserID:    user.UserID,
		SavedAt:   time.Now().UTC(),
	}, onConnected, onClose, readPump)
}

func (service *Service) HandleNotificationsSocket(w http.ResponseWriter, r *http.Request) {
	principal, ok := service.authenticateRealtimeRequest(w, r)
	if !ok {
		return
	}

	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	if userID == "" {
		userID = strings.TrimSpace(principal.UserID)
	}
	if userID == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Usuario obrigatorio.")
		return
	}
	if userID != principal.UserID && principal.Role != auth.RolePlatformAdmin {
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar este recurso.")
		return
	}

	accountID := strings.TrimSpace(r.URL.Query().Get("accountId"))
	if principal.Role != auth.RolePlatformAdmin || accountID != "" {
		resolvedAccountID, err := service.resolveRealtimeAccountID(principal, accountID)
		if err != nil {
			service.writeRealtimeAccessError(w, r, err, "Account invalida para notificacoes.", "Account nao encontrada.")
			return
		}
		if resolvedAccountID != "" {
			if err := service.authorizeTasksAccount(r.Context(), principal, resolvedAccountID); err != nil {
				service.writeRealtimeAccessError(w, r, err, "Account invalida para notificacoes.", "Account nao encontrada.")
				return
			}
			accountID = resolvedAccountID
		}
	}

	service.serveSubscriptionSocket(w, r, notificationsUserTopic(userID), tasksSubscriptionBuffer, Event{
		Type:      EventTypeConnected,
		AccountID: accountID,
		UserID:    userID,
		SavedAt:   time.Now().UTC(),
	}, nil, nil, service.readPumpWithRateLimit)
}

func (service *Service) authenticateRealtimeRequest(w http.ResponseWriter, r *http.Request) (auth.Principal, bool) {
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
		return auth.Principal{}, false
	}

	return principal, true
}

func (service *Service) resolveTasksSubscription(ctx context.Context, principal auth.Principal, r *http.Request) (realtimeSubscription, error) {
	query := r.URL.Query()
	topic := strings.TrimSpace(query.Get("topic"))
	scope := strings.TrimSpace(query.Get("scope"))
	accountID := strings.TrimSpace(query.Get("accountId"))
	boardID := strings.TrimSpace(query.Get("boardId"))
	taskID := strings.TrimSpace(query.Get("taskId"))

	if topic != "" {
		switch {
		case strings.HasPrefix(topic, "tasks:account:"):
			scope = "account"
			var err error
			accountID, err = mergeTopicID(accountID, strings.TrimPrefix(topic, "tasks:account:"))
			if err != nil {
				return realtimeSubscription{}, err
			}
		case strings.HasPrefix(topic, "tasks:board:"):
			scope = "board"
			var err error
			boardID, err = mergeTopicID(boardID, strings.TrimPrefix(topic, "tasks:board:"))
			if err != nil {
				return realtimeSubscription{}, err
			}
		case strings.HasPrefix(topic, "tasks:task:"):
			scope = "task"
			var err error
			taskID, err = mergeTopicID(taskID, strings.TrimPrefix(topic, "tasks:task:"))
			if err != nil {
				return realtimeSubscription{}, err
			}
		default:
			return realtimeSubscription{}, errRealtimeValidation
		}
	}

	if scope == "" {
		switch {
		case taskID != "":
			scope = "task"
		case boardID != "":
			scope = "board"
		default:
			scope = "account"
		}
	}

	switch scope {
	case "account":
		resolvedAccountID, err := service.resolveRealtimeAccountID(principal, accountID)
		if err != nil {
			return realtimeSubscription{}, err
		}
		if resolvedAccountID == "" {
			return realtimeSubscription{}, errRealtimeValidation
		}
		if err := service.authorizeTasksAccount(ctx, principal, resolvedAccountID); err != nil {
			return realtimeSubscription{}, err
		}
		return realtimeSubscription{topic: tasksAccountTopic(resolvedAccountID), accountID: resolvedAccountID}, nil
	case "board":
		resolvedAccountID, err := service.resolveRealtimeAccountID(principal, accountID)
		if err != nil {
			return realtimeSubscription{}, err
		}
		authorized, err := service.authorizeTasksBoard(ctx, principal, resolvedAccountID, boardID)
		if err != nil {
			return realtimeSubscription{}, err
		}
		return realtimeSubscription{topic: tasksBoardTopic(authorized.boardID), accountID: authorized.accountID, boardID: authorized.boardID}, nil
	case "task":
		resolvedAccountID, err := service.resolveRealtimeAccountID(principal, accountID)
		if err != nil {
			return realtimeSubscription{}, err
		}
		authorized, err := service.authorizeTasksTask(ctx, principal, resolvedAccountID, taskID)
		if err != nil {
			return realtimeSubscription{}, err
		}
		return realtimeSubscription{topic: tasksTaskTopic(authorized.taskID), accountID: authorized.accountID, boardID: authorized.boardID, taskID: authorized.taskID}, nil
	default:
		return realtimeSubscription{}, errRealtimeValidation
	}
}

func (service *Service) resolvePresenceSubscription(ctx context.Context, principal auth.Principal, r *http.Request) (realtimeSubscription, error) {
	query := r.URL.Query()
	topic := strings.TrimSpace(query.Get("topic"))
	scope := strings.TrimSpace(query.Get("scope"))
	accountID := strings.TrimSpace(query.Get("accountId"))
	boardID := strings.TrimSpace(query.Get("boardId"))
	taskID := strings.TrimSpace(query.Get("taskId"))

	if topic != "" {
		switch {
		case strings.HasPrefix(topic, "presence:board:"):
			scope = "board"
			var err error
			boardID, err = mergeTopicID(boardID, strings.TrimPrefix(topic, "presence:board:"))
			if err != nil {
				return realtimeSubscription{}, err
			}
		case strings.HasPrefix(topic, "presence:task:"):
			scope = "task"
			var err error
			taskID, err = mergeTopicID(taskID, strings.TrimPrefix(topic, "presence:task:"))
			if err != nil {
				return realtimeSubscription{}, err
			}
		default:
			return realtimeSubscription{}, errRealtimeValidation
		}
	}

	if scope == "" {
		if taskID != "" {
			scope = "task"
		} else {
			scope = "board"
		}
	}

	resolvedAccountID, err := service.resolveRealtimeAccountID(principal, accountID)
	if err != nil {
		return realtimeSubscription{}, err
	}

	switch scope {
	case "board":
		authorized, err := service.authorizeTasksBoard(ctx, principal, resolvedAccountID, boardID)
		if err != nil {
			return realtimeSubscription{}, err
		}
		return realtimeSubscription{topic: presenceBoardTopic(authorized.boardID), accountID: authorized.accountID, boardID: authorized.boardID}, nil
	case "task":
		authorized, err := service.authorizeTasksTask(ctx, principal, resolvedAccountID, taskID)
		if err != nil {
			return realtimeSubscription{}, err
		}
		return realtimeSubscription{topic: presenceTaskTopic(authorized.taskID), accountID: authorized.accountID, boardID: authorized.boardID, taskID: authorized.taskID}, nil
	default:
		return realtimeSubscription{}, errRealtimeValidation
	}
}

func (service *Service) resolveRealtimeAccountID(principal auth.Principal, requestedAccountID string) (string, error) {
	requestedAccountID = strings.TrimSpace(requestedAccountID)
	if principal.Role == auth.RolePlatformAdmin {
		return requestedAccountID, nil
	}

	principalAccountID := strings.TrimSpace(principal.TenantID)
	if principalAccountID == "" {
		return "", errRealtimeForbidden
	}
	if requestedAccountID != "" && requestedAccountID != principalAccountID {
		return "", errRealtimeForbidden
	}

	return principalAccountID, nil
}

func (service *Service) authorizeTasksAccount(ctx context.Context, principal auth.Principal, accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return errRealtimeValidation
	}
	if service.pool == nil {
		return errRealtimeUnavailable
	}

	var exists bool
	if err := service.pool.QueryRow(ctx, `
		select exists (
			select 1 from core.accounts where id = $1::uuid and is_active = true
		)
	`, accountID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return errRealtimeNotFound
	}

	if principal.Role == auth.RolePlatformAdmin {
		return nil
	}

	var member bool
	if err := service.pool.QueryRow(ctx, `
		select exists (
			select 1
			from core.account_users
			where account_id = $1::uuid and user_id = $2::uuid and is_active = true
		)
	`, accountID, principal.UserID).Scan(&member); err != nil {
		return err
	}
	if !member {
		return errRealtimeForbidden
	}

	hasPermission, err := service.hasAnyCoreTaskPermission(ctx, accountID, principal.UserID, []string{
		tasksmodule.PermTasksView,
		tasksmodule.PermClientView,
	})
	if err != nil {
		return err
	}
	if hasPermission {
		return nil
	}

	if principal.PermissionsResolved && hasAnyString(principal.Permissions, tasksmodule.PermTasksView, tasksmodule.PermClientView) {
		return nil
	}

	return errRealtimeForbidden
}

func (service *Service) authorizeTasksBoard(ctx context.Context, principal auth.Principal, accountID string, boardID string) (realtimeSubscription, error) {
	boardID = strings.TrimSpace(boardID)
	accountID = strings.TrimSpace(accountID)
	if boardID == "" {
		return realtimeSubscription{}, errRealtimeValidation
	}
	if service.pool == nil {
		return realtimeSubscription{}, errRealtimeUnavailable
	}

	var ownerAccountID string
	if accountID == "" {
		if principal.Role != auth.RolePlatformAdmin {
			return realtimeSubscription{}, errRealtimeForbidden
		}
		err := service.pool.QueryRow(ctx, `
			select account_id::text
			from tasks.boards
			where id = $1::uuid and archived = false
		`, boardID).Scan(&ownerAccountID)
		if errors.Is(err, pgx.ErrNoRows) {
			return realtimeSubscription{}, errRealtimeNotFound
		}
		if err != nil {
			return realtimeSubscription{}, err
		}
		accountID = ownerAccountID
	} else {
		err := service.pool.QueryRow(ctx, `
			select b.account_id::text
			from tasks.boards b
			where b.id = $1::uuid
			  and b.archived = false
			  and (
			    b.account_id = $2::uuid
			    or exists (
			      select 1
			      from tasks.tasks t
			      join tasks.task_shares s on s.task_id = t.id
			      where t.board_id = b.id
			        and t.archived = false
			        and s.client_account_id = $2::uuid
			        and s.revoked_at is null
			    )
			  )
		`, boardID, accountID).Scan(&ownerAccountID)
		if errors.Is(err, pgx.ErrNoRows) {
			return realtimeSubscription{}, errRealtimeNotFound
		}
		if err != nil {
			return realtimeSubscription{}, err
		}
	}

	if err := service.authorizeTasksAccount(ctx, principal, accountID); err != nil {
		return realtimeSubscription{}, err
	}

	return realtimeSubscription{accountID: accountID, boardID: boardID}, nil
}

func (service *Service) authorizeTasksTask(ctx context.Context, principal auth.Principal, accountID string, taskID string) (realtimeSubscription, error) {
	taskID = strings.TrimSpace(taskID)
	accountID = strings.TrimSpace(accountID)
	if taskID == "" {
		return realtimeSubscription{}, errRealtimeValidation
	}
	if service.pool == nil {
		return realtimeSubscription{}, errRealtimeUnavailable
	}

	var ownerAccountID string
	var boardID string
	if accountID == "" {
		if principal.Role != auth.RolePlatformAdmin {
			return realtimeSubscription{}, errRealtimeForbidden
		}
		err := service.pool.QueryRow(ctx, `
			select t.account_id::text, t.board_id::text
			from tasks.tasks t
			join tasks.boards b on b.id = t.board_id
			where t.id = $1::uuid and t.archived = false and b.archived = false
		`, taskID).Scan(&ownerAccountID, &boardID)
		if errors.Is(err, pgx.ErrNoRows) {
			return realtimeSubscription{}, errRealtimeNotFound
		}
		if err != nil {
			return realtimeSubscription{}, err
		}
		accountID = ownerAccountID
	} else {
		err := service.pool.QueryRow(ctx, `
			select t.account_id::text, t.board_id::text
			from tasks.tasks t
			join tasks.boards b on b.id = t.board_id
			where t.id = $1::uuid
			  and t.archived = false
			  and b.archived = false
			  and (
			    t.account_id = $2::uuid
			    or t.client_account_id = $2::uuid
			    or exists (
			      select 1
			      from tasks.task_shares s
			      where s.task_id = t.id
			        and s.client_account_id = $2::uuid
			        and s.revoked_at is null
			    )
			  )
		`, taskID, accountID).Scan(&ownerAccountID, &boardID)
		if errors.Is(err, pgx.ErrNoRows) {
			return realtimeSubscription{}, errRealtimeNotFound
		}
		if err != nil {
			return realtimeSubscription{}, err
		}
	}

	if err := service.authorizeTasksAccount(ctx, principal, accountID); err != nil {
		return realtimeSubscription{}, err
	}

	return realtimeSubscription{accountID: accountID, boardID: boardID, taskID: taskID}, nil
}

func (service *Service) hasAnyCoreTaskPermission(ctx context.Context, accountID string, userID string, permissionKeys []string) (bool, error) {
	var hasPermission bool
	err := service.pool.QueryRow(ctx, `
		select exists (
			select 1
			from (
				select rp.permission_key
				from core.user_role_assignments ura
				join core.role_permissions rp on rp.role_id = ura.role_id
				join core.permissions p on p.key = rp.permission_key and p.deprecated_at is null
				where ura.account_id = $1::uuid and ura.user_id = $2::uuid

				union

				select permission_key
				from core.user_permission_overrides
				where account_id = $1::uuid and user_id = $2::uuid
				  and effect = 'allow' and is_active = true

				except

				select permission_key
				from core.user_permission_overrides
				where account_id = $1::uuid and user_id = $2::uuid
				  and effect = 'deny' and is_active = true
			) effective
			where effective.permission_key = any($3::text[])
		)
	`, accountID, userID, permissionKeys).Scan(&hasPermission)
	return hasPermission, err
}

func (service *Service) serveSubscriptionSocket(
	w http.ResponseWriter,
	r *http.Request,
	topic string,
	bufferSize int,
	connected Event,
	onConnected func() []Event,
	onClose func(),
	readPump func(*websocket.Conn, chan<- struct{}),
) {
	connection, err := service.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer connection.Close()
	if onClose != nil {
		defer onClose()
	}

	connection.SetReadLimit(maxMessageSize)
	connection.SetReadDeadline(time.Now().Add(pongWait))
	connection.SetPongHandler(func(string) error {
		connection.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	subscription := service.hub.Subscribe(topic, bufferSize)
	defer subscription.Close()

	done := make(chan struct{})
	if readPump == nil {
		readPump = service.readPumpWithRateLimit
	}
	go readPump(connection, done)

	if err := service.writeEvent(connection, connected); err != nil {
		return
	}
	if onConnected != nil {
		for _, event := range onConnected() {
			if err := service.writeEvent(connection, event); err != nil {
				return
			}
		}
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

func (service *Service) readPumpWithRateLimit(connection *websocket.Conn, done chan<- struct{}) {
	defer close(done)

	limiter := newSocketRateLimiter(realtimeClientRateLimit, realtimeClientRateInterval)
	for {
		if _, _, err := connection.ReadMessage(); err != nil {
			return
		}
		if !limiter.Allow(time.Now()) {
			closeRateLimitedConnection(connection)
			return
		}
	}
}

func (service *Service) readPresencePump(connection *websocket.Conn, done chan<- struct{}, topic string, user PresenceUser) {
	defer close(done)

	limiter := newSocketRateLimiter(realtimeClientRateLimit, realtimeClientRateInterval)
	for {
		_, message, err := connection.ReadMessage()
		if err != nil {
			return
		}
		if !limiter.Allow(time.Now()) {
			closeRateLimitedConnection(connection)
			return
		}

		var payload presenceClientMessage
		if err := json.Unmarshal(message, &payload); err != nil {
			continue
		}

		switch normalizeClientEventType(payload.Type) {
		case "presence.heartbeat":
			service.presence.Heartbeat(topic, user)
		case "presence.field_focus":
			service.presence.LockField(topic, user, payload.FieldKey, payload.LockID)
		case "presence.field_blur":
			service.presence.UnlockField(topic, user.UserID, payload.FieldKey)
		}
	}
}

func closeRateLimitedConnection(connection *websocket.Conn) {
	_ = connection.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "rate limit exceeded"),
		time.Now().Add(writeWait),
	)
}

func normalizeClientEventType(eventType string) string {
	eventType = strings.TrimSpace(eventType)
	switch eventType {
	case "heartbeat":
		return "presence.heartbeat"
	case "field_focus":
		return "presence.field_focus"
	case "field_blur":
		return "presence.field_blur"
	default:
		return eventType
	}
}

func mergeTopicID(queryID string, topicID string) (string, error) {
	queryID = strings.TrimSpace(queryID)
	topicID = strings.TrimSpace(topicID)
	if queryID != "" && topicID != "" && queryID != topicID {
		return "", errRealtimeValidation
	}
	if topicID != "" {
		return topicID, nil
	}
	return queryID, nil
}

func hasAnyString(values []string, targets ...string) bool {
	for _, value := range values {
		for _, target := range targets {
			if strings.TrimSpace(value) == target {
				return true
			}
		}
	}
	return false
}

func (service *Service) writeRealtimeAccessError(w http.ResponseWriter, r *http.Request, err error, validationMessage string, notFoundMessage string) {
	switch {
	case errors.Is(err, errRealtimeValidation), isInvalidUUIDError(err):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", validationMessage)
	case errors.Is(err, errRealtimeForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para acessar este recurso.")
	case errors.Is(err, errRealtimeNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", notFoundMessage)
	case errors.Is(err, errRealtimeUnavailable):
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "realtime_unavailable", "Realtime indisponivel.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao validar o canal realtime.")
	}
}

func isInvalidUUIDError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22P02"
}
