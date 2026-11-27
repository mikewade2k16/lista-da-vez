package tools

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

type scriptRepositoryFake struct {
	account string
	input   scriptInput
	err     error
	calls   int
	search  string
	client  string
	removed []string
	patched string
}

func (f *scriptRepositoryFake) clients(_ context.Context, account string) ([]scriptClient, error) {
	f.account = account
	return nil, f.err
}

func (f *scriptRepositoryFake) list(_ context.Context, account, _ string, filter scriptListFilter) ([]scriptDocument, error) {
	f.account = account
	f.search = filter.Search
	f.client = filter.Client
	return nil, f.err
}
func (f *scriptRepositoryFake) get(_ context.Context, account, _ string) (scriptDocument, error) {
	f.account = account
	return scriptDocument{}, f.err
}
func (f *scriptRepositoryFake) save(_ context.Context, account, _ string, input scriptInput, canRemove func(string) bool) (scriptDocument, error) {
	f.account = account
	f.input = input
	f.calls++
	for _, removed := range f.removed {
		if canRemove != nil && !canRemove(removed) {
			return scriptDocument{}, errScriptLocked
		}
	}
	return scriptDocument{Title: input.Title}, f.err
}
func (f *scriptRepositoryFake) patchCell(_ context.Context, account, _, field, rowID, value string) (string, string, error) {
	f.account = account
	f.calls++
	f.patched = field + "|" + rowID + "|" + value
	return "2026-11-12T10:00:00Z", "", f.err
}
func (f *scriptRepositoryFake) delete(_ context.Context, account, _ string) error {
	f.account = account
	return f.err
}
func TestScriptSaveValidation(t *testing.T) {
	account := "00000000-0000-4000-8000-000000000001"
	cases := []struct {
		name    string
		account string
		id      string
		input   scriptInput
	}{
		{name: "missing account", input: scriptInput{Title: "Título"}},
		{name: "blank title", account: account, input: scriptInput{Title: "  "}},
		{name: "long title", account: account, input: scriptInput{Title: strings.Repeat("á", 201)}},
		{name: "too many rows", account: account, input: scriptInput{Title: "Título", Rows: make([]scriptRow, 201)}},
		{name: "oversized cell", account: account, input: scriptInput{Title: "Título", Rows: []scriptRow{{Audio: strings.Repeat("a", 200001)}}}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			repository := &scriptRepositoryFake{}
			_, err := newScriptService(repository).save(context.Background(), test.account, "", test.id, test.input)
			if !errors.Is(err, errInvalidScript) || repository.calls != 0 {
				t.Fatalf("invalid save reached repository: %v, calls=%d", err, repository.calls)
			}
		})
	}
}
func TestScriptSaveNormalizesAndPreservesScope(t *testing.T) {
	repository := &scriptRepositoryFake{}
	account := "00000000-0000-4000-8000-000000000001"
	_, err := newScriptService(repository).save(context.Background(), account, "", "", scriptInput{Title: "  Roteiro  ",
		Rows: []scriptRow{{ID: "bad"}, {}, {ID: "00000000-0000-4000-8000-000000000009"}, {ID: "00000000-0000-4000-8000-000000000009"}}})
	if err != nil || repository.account != account || repository.input.Title != "Roteiro" {
		t.Fatalf("invalid persisted input: %#v, %v", repository, err)
	}
	seen := map[string]bool{}
	for _, row := range repository.input.Rows {
		if !isUUID(row.ID) || seen[row.ID] {
			t.Fatalf("row ids not normalized: %#v", repository.input.Rows)
		}
		seen[row.ID] = true
	}
}
func TestScriptErrorDoesNotExposeExistenceOutsideScope(t *testing.T) {
	for _, test := range []struct {
		err    error
		status int
	}{{errScriptNotFound, 404}, {scriptLockedError{holder: "Ana"}, 409}, {errInvalidScript, 400}, {errors.New("private database details"), 500}} {
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(context.Background(), "GET", "/v1/tools/scripts/id", nil)
		scriptError(response, request, test.err)
		if response.Code != test.status || strings.Contains(response.Body.String(), "private database") {
			t.Fatalf("unsafe response: %d %s", response.Code, response.Body.String())
		}
	}
}

func (f *scriptRepositoryFake) reorder(_ context.Context, account, _ string, _ []string) error {
	f.account = account
	f.calls++
	return f.err
}

func (f *scriptRepositoryFake) addImage(_ context.Context, account string, _ scriptImage) error {
	f.account = account
	f.calls++
	return f.err
}
func (f *scriptRepositoryFake) getImage(_ context.Context, account, _ string) (scriptImage, error) {
	f.account = account
	f.calls++
	return scriptImage{}, f.err
}

func TestScriptListSearch(t *testing.T) {
	store := &scriptRepositoryFake{}
	svc := newScriptService(store)
	account := "00000000-0000-4000-8000-000000000001"
	if _, err := svc.list(context.Background(), account, "", scriptListFilter{Search: "  café  ", Client: " none "}); err != nil || store.search != "café" || store.client != "none" {
		t.Fatalf("search = %q, client = %q, err = %v", store.search, store.client, err)
	}
	for _, filter := range []scriptListFilter{{Search: strings.Repeat("a", maxScriptSearch+1)}, {Client: "duby"}} {
		store.search = "untouched"
		if _, err := svc.list(context.Background(), account, "", filter); !errors.Is(err, errInvalidScript) || store.search != "untouched" {
			t.Fatalf("filter %+v err = %v, reached store = %q", filter, err, store.search)
		}
	}
}

func TestLikePatternEscapesWildcards(t *testing.T) {
	cases := map[string]string{"": "", "50%": `%50\%%`, "a_b": `%a\_b%`, `c\d`: `%c\\d%`}
	for in, want := range cases {
		if got := likePattern(in); got != want {
			t.Fatalf("likePattern(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestScriptSaveRejectsInvalidClient(t *testing.T) {
	account := "00000000-0000-4000-8000-000000000001"
	for _, client := range []string{"duby", account} {
		store := &scriptRepositoryFake{}
		_, err := newScriptService(store).save(context.Background(), account, "", "", scriptInput{Title: "Roteiro", ClientAccountID: client})
		if !errors.Is(err, errInvalidScriptClient) || store.calls != 0 {
			t.Fatalf("client %q: err = %v, store calls = %d", client, err, store.calls)
		}
	}
	response := httptest.NewRecorder()
	scriptError(response, httptest.NewRequestWithContext(context.Background(), "POST", "/v1/tools/scripts", nil), errInvalidScriptClient)
	if response.Code != 400 || !strings.Contains(response.Body.String(), "invalid_client") {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
	}
}

type scriptPublisherFake struct{ events []ScriptEvent }

func (f *scriptPublisherFake) PublishScriptEvent(_ context.Context, evt ScriptEvent) {
	f.events = append(f.events, evt)
}

func TestScriptMutationsPublishInvalidation(t *testing.T) {
	account := "00000000-0000-4000-8000-000000000001"
	store := &scriptRepositoryFake{}
	publisher := &scriptPublisherFake{}
	svc := newScriptService(store)
	svc.publisher = publisher
	if _, err := svc.save(context.Background(), account, "", "", scriptInput{Title: "Roteiro"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.delete(context.Background(), account, "", "id"); err != nil {
		t.Fatal(err)
	}
	if len(publisher.events) != 2 || publisher.events[0].Type != ScriptEventSaved || publisher.events[1].Type != ScriptEventDeleted {
		t.Fatalf("unexpected events: %+v", publisher.events)
	}
	store.err = errors.New("db down")
	_, _ = svc.save(context.Background(), account, "", "", scriptInput{Title: "Roteiro"})
	if len(publisher.events) != 2 {
		t.Fatalf("failed save must not publish: %+v", publisher.events)
	}
}

func TestScriptPublishSkipsOwnAndEmptyClients(t *testing.T) {
	publisher := &scriptPublisherFake{}
	svc := newScriptService(&scriptRepositoryFake{})
	svc.publisher = publisher
	svc.publish(context.Background(), ScriptEvent{AccountID: "agency"}, "", "agency", "client")
	if got := publisher.events[0].ClientAccountIDs; len(got) != 1 || got[0] != "client" {
		t.Fatalf("client scopes = %v", got)
	}
	if visibleClient(scriptDocument{ClientAccountID: "client", ClientVisible: false}) != "" {
		t.Fatal("hidden client must not receive events")
	}
}

type scriptLocksFake struct{ fields map[string]string }

func (f scriptLocksFake) ScriptFieldLocker(_, fieldKey, _ string) string { return f.fields[fieldKey] }
func (f scriptLocksFake) ScriptAnyLocker(string, string) string {
	for _, holder := range f.fields {
		return holder
	}
	return ""
}

func TestScriptCellPatchRespectsLocksAndPublishesContent(t *testing.T) {
	account := "00000000-0000-4000-8000-000000000001"
	script := "00000000-0000-4000-8000-000000000002"
	row := "00000000-0000-4000-8000-000000000003"
	store := &scriptRepositoryFake{}
	publisher := &scriptPublisherFake{}
	svc := newScriptService(store)
	svc.publisher = publisher
	svc.locks = scriptLocksFake{fields: map[string]string{ScriptCellKey("video", row): "Ana"}}

	if _, err := svc.patchCell(context.Background(), account, "user", script, "audio", row, "<p>oi</p>"); err != nil {
		t.Fatal(err)
	}
	if store.patched != "audio|"+row+"|<p>oi</p>" || len(publisher.events) != 1 ||
		publisher.events[0].Type != ScriptEventCell || publisher.events[0].Value != "<p>oi</p>" || publisher.events[0].RowID != row {
		t.Fatalf("unexpected patch/event: %q %+v", store.patched, publisher.events)
	}
	_, err := svc.patchCell(context.Background(), account, "user", script, "video", row, "x")
	var locked scriptLockedError
	if !errors.As(err, &locked) || locked.holder != "Ana" || len(publisher.events) != 1 {
		t.Fatalf("locked cell must be refused without publishing: %v", err)
	}
	for _, bad := range []struct{ field, row string }{{"audio", "x"}, {"rows", row}, {"title", ""}} {
		if _, err := svc.patchCell(context.Background(), account, "user", script, bad.field, bad.row, ""); !errors.Is(err, errInvalidScript) {
			t.Fatalf("%+v must be invalid: %v", bad, err)
		}
	}
}

func TestScriptDeleteAndRowRemovalBlockedWhileSomeoneEdits(t *testing.T) {
	account := "00000000-0000-4000-8000-000000000001"
	row := "00000000-0000-4000-8000-000000000003"
	store := &scriptRepositoryFake{removed: []string{row}}
	svc := newScriptService(store)
	svc.locks = scriptLocksFake{fields: map[string]string{ScriptCellKey("audio", row): "Ana"}}
	if err := svc.delete(context.Background(), account, "user", "script"); !errors.Is(err, errScriptLocked) {
		t.Fatalf("delete while editing: %v", err)
	}
	_, err := svc.save(context.Background(), account, "user", "00000000-0000-4000-8000-000000000002", scriptInput{Title: "Roteiro"})
	var locked scriptLockedError
	if !errors.As(err, &locked) || locked.holder != "Ana" {
		t.Fatalf("removing a locked row: %v", err)
	}
}

// Regressão: o front envia o ScriptDraft inteiro (inclui updatedAt). O decoder recusa campos
// desconhecidos, então tirar um campo do scriptInput quebra "+ Linha" e a troca de cliente.
func TestScriptInputAcceptsFrontendDraft(t *testing.T) {
	body := `{"title":"Novo roteiro","rows":[{"id":"r1","audio":"","video":""}],"description":"",` +
		`"updatedAt":"2026-11-27T12:00:00.123456Z","clientAccountId":"","clientVisible":false}`
	request := httptest.NewRequestWithContext(context.Background(), "PUT", "/v1/tools/scripts/id", strings.NewReader(body))
	var in scriptInput
	if err := httpapi.ReadJSON(request, &in); err != nil {
		t.Fatalf("payload do front recusado: %v", err)
	}
	if len(in.Rows) != 1 || in.Rows[0].ID != "r1" {
		t.Fatalf("rows: %+v", in.Rows)
	}
}
