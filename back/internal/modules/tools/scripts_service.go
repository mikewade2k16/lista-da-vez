package tools

import (
	"context"
	"errors"
	"strings"
	"time"
)

var errInvalidScript = errors.New("invalid script")

const maxScriptSearch = 100

type scriptRepository interface {
	list(ctx context.Context, account, user string, filter scriptListFilter) ([]scriptDocument, error)
	clients(ctx context.Context, account string) ([]scriptClient, error)
	reorder(context.Context, string, string, []string) error
	get(context.Context, string, string) (scriptDocument, error)
	save(context.Context, string, string, scriptInput) (scriptDocument, error)
	delete(context.Context, string, string) error
	addImage(context.Context, string, scriptImage) error
	getImage(context.Context, string, string) (scriptImage, error)
}
type scriptService struct {
	store     scriptRepository
	imageRoot string
	publisher ScriptPublisher
}

func newScriptService(store scriptRepository) *scriptService {
	return &scriptService{store: store, publisher: noopScriptPublisher{}}
}

// visibleClient returns the client account that currently reads the script, if any.
func visibleClient(item scriptDocument) string {
	if item.ClientVisible {
		return item.ClientAccountID
	}
	return ""
}
func (s *scriptService) publish(ctx context.Context, evt ScriptEvent, clients ...string) {
	for _, client := range clients {
		if client != "" && client != evt.AccountID {
			evt.ClientAccountIDs = append(evt.ClientAccountIDs, client)
		}
	}
	s.publisher.PublishScriptEvent(ctx, evt)
}
func (s *scriptService) list(ctx context.Context, account, user string, filter scriptListFilter) ([]scriptDocument, error) {
	filter.Search = strings.TrimSpace(filter.Search)
	filter.Client = strings.TrimSpace(filter.Client)
	if len([]rune(filter.Search)) > maxScriptSearch || (filter.Client != "" && filter.Client != "none" && !isUUID(filter.Client)) {
		return nil, errInvalidScript
	}
	return s.store.list(ctx, account, user, filter)
}
func (s *scriptService) clients(ctx context.Context, account string) ([]scriptClient, error) {
	if !isUUID(account) {
		return nil, errInvalidScript
	}
	return s.store.clients(ctx, account)
}
func (s *scriptService) save(ctx context.Context, account, id string, input scriptInput) (scriptDocument, error) {
	if !isUUID(account) {
		return scriptDocument{}, errInvalidScript
	}
	if id != "" {
		if _, err := time.Parse(time.RFC3339Nano, input.UpdatedAt); err != nil {
			return scriptDocument{}, errInvalidScript
		}
	}
	input.Title = strings.TrimSpace(input.Title)
	input.ClientAccountID = strings.TrimSpace(input.ClientAccountID)
	if input.ClientAccountID != "" && (!isUUID(input.ClientAccountID) || input.ClientAccountID == account) {
		return scriptDocument{}, errInvalidScriptClient
	}
	if input.Title == "" || len([]rune(input.Title)) > 200 || len(input.Rows) > 200 || len(input.Description) > 200000 {
		return scriptDocument{}, errInvalidScript
	}
	total := len(input.Description)
	for _, row := range input.Rows {
		total += len(row.Audio) + len(row.Video)
	}
	if total > 800000 {
		return scriptDocument{}, errInvalidScript
	}
	if input.Rows == nil {
		input.Rows = []scriptRow{}
	}
	for _, row := range input.Rows {
		if len(row.Audio) > 200000 || len(row.Video) > 200000 {
			return scriptDocument{}, errInvalidScript
		}
	}
	// Cliente anterior também é avisado: ao trocar/ocultar, o card some da tela dele.
	previous := ""
	if id != "" {
		if before, err := s.store.get(ctx, account, id); err == nil && !before.Shared {
			previous = visibleClient(before)
		}
	}
	item, err := s.store.save(ctx, account, id, input)
	if err != nil {
		return item, err
	}
	s.publish(ctx, ScriptEvent{Type: ScriptEventSaved, AccountID: account, ResourceID: item.ID, UpdatedAt: item.UpdatedAt},
		visibleClient(item), previous)
	return item, nil
}
func (s *scriptService) delete(ctx context.Context, account, id string) error {
	before, getErr := s.store.get(ctx, account, id)
	if err := s.store.delete(ctx, account, id); err != nil {
		return err
	}
	client := ""
	if getErr == nil {
		client = visibleClient(before)
	}
	s.publish(ctx, ScriptEvent{Type: ScriptEventDeleted, AccountID: account, ResourceID: id}, client)
	return nil
}

func (s *scriptService) get(ctx context.Context, account, id string) (scriptDocument, error) {
	return s.store.get(ctx, account, id)
}

func (s *scriptService) reorder(ctx context.Context, account, user string, ids []string) error {
	if !isUUID(account) || !isUUID(user) || len(ids) > 10000 || len(ids) == 0 {
		return errInvalidScript
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !isUUID(id) || seen[id] {
			return errInvalidScript
		}
		seen[id] = true
	}
	if err := s.store.reorder(ctx, account, user, ids); err != nil {
		return err
	}
	s.publish(ctx, ScriptEvent{Type: ScriptEventReordered, AccountID: account, UserID: user})
	return nil
}
