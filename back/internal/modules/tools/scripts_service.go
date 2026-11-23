package tools

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
)

var errInvalidScript = errors.New("invalid script")

const maxScriptSearch = 100

type scriptRepository interface {
	list(ctx context.Context, account, user string, filter scriptListFilter) ([]scriptDocument, error)
	clients(ctx context.Context, account string) ([]scriptClient, error)
	reorder(context.Context, string, string, []string) error
	get(context.Context, string, string) (scriptDocument, error)
	save(ctx context.Context, account, id string, input scriptInput, canRemove func(string) bool) (scriptDocument, error)
	patchCell(ctx context.Context, account, id, field, rowID, value string) (updatedAt, client string, err error)
	delete(context.Context, string, string) error
	addImage(context.Context, string, scriptImage) error
	getImage(context.Context, string, string) (scriptImage, error)
}
type scriptService struct {
	store     scriptRepository
	imageRoot string
	publisher ScriptPublisher
	locks     ScriptLocks
}

func newScriptService(store scriptRepository) *scriptService {
	return &scriptService{store: store, publisher: noopScriptPublisher{}, locks: noopScriptLocks{}}
}

// scriptLockedError carries who holds the lock so the API can say "Fulano está editando".
type scriptLockedError struct{ holder string }

func (e scriptLockedError) Error() string        { return "script locked by " + e.holder }
func (e scriptLockedError) Is(target error) bool { return target == errScriptLocked }

// normalizeRowIDs keeps valid unique row ids and assigns new ones where missing.
func normalizeRowIDs(rows []scriptRow) error {
	seen := make(map[string]bool, len(rows))
	for i := range rows {
		if !isUUID(rows[i].ID) || seen[rows[i].ID] {
			id, err := newScriptUUID()
			if err != nil {
				return err
			}
			rows[i].ID = id
		}
		seen[rows[i].ID] = true
	}
	return nil
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

// save creates a script (id == "") or applies a STRUCTURAL update (row order, added/removed
// rows, client fields). Cell content is edited only through patchCell, so a structural save
// never overwrites what someone else is typing. Removing a row locked by another user fails.
func (s *scriptService) save(ctx context.Context, account, user, id string, input scriptInput) (scriptDocument, error) {
	if !isUUID(account) {
		return scriptDocument{}, errInvalidScript
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
	if err := normalizeRowIDs(input.Rows); err != nil {
		return scriptDocument{}, err
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
	var holder string
	canRemove := func(rowID string) bool {
		for _, field := range []string{"audio", "video"} {
			if holder = s.locks.ScriptFieldLocker(id, ScriptCellKey(field, rowID), user); holder != "" {
				return false
			}
		}
		return true
	}
	item, err := s.store.save(ctx, account, id, input, canRemove)
	if errors.Is(err, errScriptLocked) {
		return item, scriptLockedError{holder: holder}
	}
	if err != nil {
		return item, err
	}
	s.publish(ctx, ScriptEvent{Type: ScriptEventSaved, AccountID: account, ResourceID: item.ID, UpdatedAt: item.UpdatedAt},
		visibleClient(item), previous)
	return item, nil
}

// patchCell saves one cell ("title", "description" or a row's "audio"/"video") unless another
// user holds that cell's lock, and broadcasts the new content to whoever can read the script.
func (s *scriptService) patchCell(ctx context.Context, account, user, id, field, rowID, value string) (string, error) {
	if !isUUID(account) || !isUUID(id) {
		return "", errScriptNotFound
	}
	switch field {
	case "title":
		value = strings.TrimSpace(value)
		if value == "" || len([]rune(value)) > 200 {
			return "", errInvalidScript
		}
		rowID = ""
	case "description":
		rowID = ""
	case "audio", "video":
		if !isUUID(rowID) {
			return "", errInvalidScript
		}
	default:
		return "", errInvalidScript
	}
	if len(value) > 200000 {
		return "", errInvalidScript
	}
	if holder := s.locks.ScriptFieldLocker(id, ScriptCellKey(field, rowID), user); holder != "" {
		return "", scriptLockedError{holder: holder}
	}
	updatedAt, client, err := s.store.patchCell(ctx, account, id, field, rowID, value)
	if err != nil {
		return "", err
	}
	s.publish(ctx, ScriptEvent{Type: ScriptEventCell, AccountID: account, ResourceID: id, UpdatedAt: updatedAt,
		UserID: user, Field: field, RowID: rowID, Value: value}, client)
	return updatedAt, nil
}

// delete removes a script unless someone else is editing it right now.
func (s *scriptService) delete(ctx context.Context, account, user, id string) error {
	if holder := s.locks.ScriptAnyLocker(id, user); holder != "" {
		return scriptLockedError{holder: holder}
	}
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

func newScriptUUID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[:4], raw[4:6], raw[6:8], raw[8:10], raw[10:]), nil
}
