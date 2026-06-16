package bio

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// ============================================================================
// deepMerge
// ============================================================================

func TestDeepMergeObjectMerges(t *testing.T) {
	base := json.RawMessage(`{"a":1,"nested":{"x":1,"y":2}}`)
	override := json.RawMessage(`{"b":2,"nested":{"y":9,"z":3}}`)

	out, err := deepMerge(base, override)
	if err != nil {
		t.Fatalf("deepMerge err: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["a"].(float64) != 1 || got["b"].(float64) != 2 {
		t.Fatalf("top-level keys not merged: %v", got)
	}
	nested, ok := got["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested not an object: %v", got["nested"])
	}
	// x preservado do base, y sobrescrito, z adicionado.
	if nested["x"].(float64) != 1 || nested["y"].(float64) != 9 || nested["z"].(float64) != 3 {
		t.Fatalf("nested object merge wrong: %v", nested)
	}
}

func TestDeepMergeArrayReplaces(t *testing.T) {
	base := json.RawMessage(`{"links":[1,2,3]}`)
	override := json.RawMessage(`{"links":[9]}`)

	out, err := deepMerge(base, override)
	if err != nil {
		t.Fatalf("deepMerge err: %v", err)
	}

	var got map[string][]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got["links"]) != 1 || got["links"][0].(float64) != 9 {
		t.Fatalf("array should be replaced wholesale, got: %v", got["links"])
	}
}

func TestDeepMergePrimitiveReplaces(t *testing.T) {
	base := json.RawMessage(`{"title":"old","keep":"base"}`)
	override := json.RawMessage(`{"title":"new"}`)

	out, err := deepMerge(base, override)
	if err != nil {
		t.Fatalf("deepMerge err: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["title"] != "new" {
		t.Fatalf("primitive should be replaced, got: %v", got["title"])
	}
	if got["keep"] != "base" {
		t.Fatalf("untouched base key should survive, got: %v", got["keep"])
	}
}

func TestDeepMergeEmptyOverrideKeepsBase(t *testing.T) {
	base := json.RawMessage(`{"a":1}`)
	out, err := deepMerge(base, nil)
	if err != nil {
		t.Fatalf("deepMerge err: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["a"].(float64) != 1 {
		t.Fatalf("empty override should keep base, got: %v", got)
	}
}

// ============================================================================
// absolutizeUploads
// ============================================================================

func TestAbsolutizeUploadsRewritesPaths(t *testing.T) {
	in := json.RawMessage(`{"logo":"/uploads/bio/acc/logo.png","ext":"https://x/y.png","plain":"hello"}`)
	out, err := absolutizeUploads(in, "https://api.omni.dev/")
	if err != nil {
		t.Fatalf("absolutizeUploads err: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["logo"] != "https://api.omni.dev/uploads/bio/acc/logo.png" {
		t.Fatalf("upload path not absolutized: %v", got["logo"])
	}
	if got["ext"] != "https://x/y.png" {
		t.Fatalf("external url should be untouched: %v", got["ext"])
	}
	if got["plain"] != "hello" {
		t.Fatalf("plain string should be untouched: %v", got["plain"])
	}
}

func TestAbsolutizeUploadsNoBaseKeepsRelative(t *testing.T) {
	in := json.RawMessage(`{"logo":"/uploads/bio/acc/logo.png"}`)
	out, err := absolutizeUploads(in, "")
	if err != nil {
		t.Fatalf("absolutizeUploads err: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["logo"] != "/uploads/bio/acc/logo.png" {
		t.Fatalf("without base, path should stay relative: %v", got["logo"])
	}
}

func TestAbsolutizeUploadsNestedAndArrays(t *testing.T) {
	in := json.RawMessage(`{"slides":[{"img":"/uploads/a.png"},{"img":"/uploads/b.png"}]}`)
	out, err := absolutizeUploads(in, "https://api.dev")
	if err != nil {
		t.Fatalf("absolutizeUploads err: %v", err)
	}
	var got struct {
		Slides []struct {
			Img string `json:"img"`
		} `json:"slides"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Slides[0].Img != "https://api.dev/uploads/a.png" || got.Slides[1].Img != "https://api.dev/uploads/b.png" {
		t.Fatalf("nested array paths not absolutized: %+v", got.Slides)
	}
}

// ============================================================================
// resolveScope
// ============================================================================

func TestResolveScopeNonAdminOtherAccountNotFound(t *testing.T) {
	_, err := resolveScope(false, "acc-1", "acc-2")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("non-admin requesting other account should be ErrNotFound, got: %v", err)
	}
}

func TestResolveScopeNonAdminOwnAccount(t *testing.T) {
	scope, err := resolveScope(false, "acc-1", "acc-1")
	if err != nil {
		t.Fatalf("own account should be allowed: %v", err)
	}
	if scope != "acc-1" {
		t.Fatalf("scope should be own account, got: %q", scope)
	}
}

func TestResolveScopeNonAdminEmptyRequestUsesContext(t *testing.T) {
	scope, err := resolveScope(false, "acc-1", "")
	if err != nil {
		t.Fatalf("empty request should fall back to context: %v", err)
	}
	if scope != "acc-1" {
		t.Fatalf("scope should be context account, got: %q", scope)
	}
}

func TestResolveScopeNonAdminNoContextNotFound(t *testing.T) {
	_, err := resolveScope(false, "", "")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("non-admin without account should be ErrNotFound, got: %v", err)
	}
}

func TestResolveScopeAdminFilters(t *testing.T) {
	scope, err := resolveScope(true, "", "acc-9")
	if err != nil {
		t.Fatalf("admin should filter freely: %v", err)
	}
	if scope != "acc-9" {
		t.Fatalf("admin scope should match requested, got: %q", scope)
	}

	all, err := resolveScope(true, "", "")
	if err != nil {
		t.Fatalf("admin global list: %v", err)
	}
	if all != "" {
		t.Fatalf("admin without filter should be unscoped, got: %q", all)
	}
}

// scopeForLookup: admin sem filtro de account, nao-admin com o contexto.
func TestScopeForLookup(t *testing.T) {
	if got := scopeForLookup(true, "acc-1"); got != "" {
		t.Fatalf("admin lookup scope should be empty, got: %q", got)
	}
	if got := scopeForLookup(false, "acc-1"); got != "acc-1" {
		t.Fatalf("non-admin lookup scope should be context, got: %q", got)
	}
}

// ============================================================================
// publish validation (campos minimos no JSON mesclado)
// ============================================================================

func TestPublishMinimumsPresent(t *testing.T) {
	merged := json.RawMessage(`{"branding":{"logo":{"srcMobile":"/uploads/l.png"}},"video":{"bgVideo":"/uploads/v.mp4"}}`)
	if !jsonHasNonEmptyPath(merged, "branding", "logo", "srcMobile") {
		t.Fatal("expected branding.logo.srcMobile present")
	}
	if !jsonHasNonEmptyPath(merged, "video", "bgVideo") {
		t.Fatal("expected video.bgVideo present")
	}
}

func TestPublishMinimumsMissing(t *testing.T) {
	// logo presente mas video vazio.
	merged := json.RawMessage(`{"branding":{"logo":{"srcMobile":"/uploads/l.png"}},"video":{"bgVideo":""}}`)
	if jsonHasNonEmptyPath(merged, "video", "bgVideo") {
		t.Fatal("empty bgVideo should fail the minimum check")
	}
	// path ausente por completo.
	if jsonHasNonEmptyPath(merged, "branding", "logo", "missing") {
		t.Fatal("missing path should fail")
	}
}

// ============================================================================
// slug validation
// ============================================================================

func TestNormalizeSlug(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"Minha-Bio", "minha-bio", false},
		{"  ABC-123  ", "abc-123", false},
		{"with space", "", true},
		{"under_score", "", true},
		{"acentuação", "", true},
		{"", "", true},
	}
	for _, c := range cases {
		got, err := normalizeSlug(c.in)
		if c.wantErr {
			if !errors.Is(err, ErrInvalidSlug) {
				t.Fatalf("normalizeSlug(%q) expected ErrInvalidSlug, got %v / %q", c.in, err, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("normalizeSlug(%q) unexpected err: %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("normalizeSlug(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Minha Bio", "minha-bio"},
		{"  Loja  da   Esquina  ", "loja-da-esquina"},
		{"Acentuação É Legal", "acentuacao-e-legal"},
		{"Pérola@RioMar!", "perola-riomar"},
		{"___", ""},
		{"já-ok-123", "ja-ok-123"},
	}
	for _, c := range cases {
		if got := slugify(c.in); got != c.want {
			t.Fatalf("slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ============================================================================
// Fake store (sem banco) para cobrir Create/Patch/Duplicate
// ============================================================================

// fakeStore implementa bioStore em memoria. So os metodos exercitados pelos
// testes de Create/Patch/Duplicate tem logica real; o resto e stub.
type fakeStore struct {
	bios          map[string]*Bio // por id
	slugs         map[string]bool // lower(slug) ocupado
	accounts      map[string]bool // accounts existentes
	moduleEnabled map[string]bool // EnsureBioModuleEnabled chamado por account
	seq           int
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		bios:          map[string]*Bio{},
		slugs:         map[string]bool{},
		accounts:      map[string]bool{},
		moduleEnabled: map[string]bool{},
	}
}

func (f *fakeStore) seed(b Bio) {
	f.bios[b.ID] = &b
	f.slugs[strings.ToLower(b.Slug)] = true
	f.accounts[b.AccountID] = true
}

func (f *fakeStore) GetByID(_ context.Context, id, accountID string) (Bio, error) {
	b, ok := f.bios[id]
	if !ok {
		return Bio{}, ErrNoRowsSentinel
	}
	if strings.TrimSpace(accountID) != "" && b.AccountID != accountID {
		return Bio{}, ErrNoRowsSentinel
	}
	return *b, nil
}

func (f *fakeStore) Create(ctx context.Context, accountID, slug, name string) (Bio, error) {
	return f.CreateWithDraft(ctx, accountID, slug, name, json.RawMessage("{}"))
}

func (f *fakeStore) CreateWithDraft(_ context.Context, accountID, slug, name string, draft json.RawMessage) (Bio, error) {
	f.seq++
	now := time.Now()
	b := Bio{
		ID: "bio-" + strings.ToLower(slug), AccountID: accountID, Slug: slug,
		Name: name, Status: "draft", DataDraft: normalizeRaw(draft),
		CreatedAt: now, UpdatedAt: now,
	}
	f.bios[b.ID] = &b
	f.slugs[strings.ToLower(slug)] = true
	f.accounts[accountID] = true
	return b, nil
}

func (f *fakeStore) Patch(_ context.Context, id string, name, slug, accountID *string, draft *json.RawMessage) (Bio, error) {
	b, ok := f.bios[id]
	if !ok {
		return Bio{}, ErrNoRowsSentinel
	}
	if name != nil {
		b.Name = *name
	}
	if slug != nil {
		delete(f.slugs, strings.ToLower(b.Slug))
		b.Slug = *slug
		f.slugs[strings.ToLower(*slug)] = true
	}
	if accountID != nil {
		b.AccountID = *accountID
	}
	if draft != nil {
		b.DataDraft = normalizeRaw(*draft)
	}
	return *b, nil
}

func (f *fakeStore) SlugExists(_ context.Context, slug, _ string) (bool, error) {
	return f.slugs[strings.ToLower(slug)], nil
}

func (f *fakeStore) EnsureBioModuleEnabled(_ context.Context, accountID string) error {
	f.moduleEnabled[accountID] = true
	return nil
}

func (f *fakeStore) AccountExists(_ context.Context, accountID string) (bool, error) {
	return f.accounts[accountID], nil
}

// Stubs nao exercitados pelos testes deste arquivo.
func (f *fakeStore) List(context.Context, ListFilter) ([]BioSummary, error)        { return nil, nil }
func (f *fakeStore) Publish(context.Context, string, json.RawMessage) (Bio, error) { return Bio{}, nil }
func (f *fakeStore) Unpublish(context.Context, string) (Bio, error)                { return Bio{}, nil }
func (f *fakeStore) Delete(context.Context, string, string) error                  { return nil }
func (f *fakeStore) PublicLookup(context.Context, string) (json.RawMessage, string, error) {
	return nil, "", ErrNoRowsSentinel
}
func (f *fakeStore) GetDefaults(context.Context) (BioDefaults, error) {
	return BioDefaults{Data: json.RawMessage("{}")}, nil
}
func (f *fakeStore) PutDefaults(_ context.Context, data json.RawMessage) (BioDefaults, error) {
	return BioDefaults{Data: data}, nil
}
func (f *fakeStore) InsertMedia(context.Context, string, string, string, string, string, int64) (Media, error) {
	return Media{}, nil
}

// ErrNoRowsSentinel e o pgx.ErrNoRows que o fake devolve em "nao encontrado" —
// mapNotFound do service o colapsa em ErrNotFound (404), espelhando o store real.
var ErrNoRowsSentinel = pgx.ErrNoRows

// ============================================================================
// Create — cliente opcional (usa contexto) + slug derivado
// ============================================================================

func TestCreateAdminWithoutAccountUsesContext(t *testing.T) {
	store := newFakeStore()
	store.accounts["ag-1"] = true
	svc := NewService(store, "")

	view, err := svc.Create(context.Background(), true, "ag-1", CreateRequest{Name: "Minha Bio"})
	if err != nil {
		t.Fatalf("Create err: %v", err)
	}
	if view.AccountID != "ag-1" {
		t.Fatalf("admin sem accountId deveria usar o contexto, got: %q", view.AccountID)
	}
	if view.Slug != "minha-bio" {
		t.Fatalf("slug vazio deveria derivar do name, got: %q", view.Slug)
	}
	if !store.moduleEnabled["ag-1"] {
		t.Fatal("Create deveria garantir o modulo bio na account")
	}
}

func TestCreateDerivedSlugCollisionGetsSuffix(t *testing.T) {
	store := newFakeStore()
	store.slugs["minha-bio"] = true // ja existe
	svc := NewService(store, "")

	view, err := svc.Create(context.Background(), false, "acc-1", CreateRequest{Name: "Minha Bio"})
	if err != nil {
		t.Fatalf("Create err: %v", err)
	}
	if view.Slug != "minha-bio-2" {
		t.Fatalf("slug derivado colidindo deveria virar minha-bio-2, got: %q", view.Slug)
	}
}

func TestCreateNonAdminIgnoresAccountID(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store, "")

	view, err := svc.Create(context.Background(), false, "acc-1", CreateRequest{Name: "X", AccountID: "outra"})
	if err != nil {
		t.Fatalf("Create err: %v", err)
	}
	if view.AccountID != "acc-1" {
		t.Fatalf("nao-admin deveria ignorar accountId do body, got: %q", view.AccountID)
	}
}

// ============================================================================
// Duplicate
// ============================================================================

func seedSource(store *fakeStore) {
	store.seed(Bio{
		ID: "src", AccountID: "acc-1", Slug: "perola", Name: "Perola",
		Status:        "published",
		DataDraft:     json.RawMessage(`{"title":"rascunho"}`),
		DataPublished: json.RawMessage(`{"title":"publicado"}`),
	})
}

func TestDuplicateCopiesDraftNotPublished(t *testing.T) {
	store := newFakeStore()
	seedSource(store)
	svc := NewService(store, "")

	view, err := svc.Duplicate(context.Background(), false, "acc-1", "src", DuplicateRequest{})
	if err != nil {
		t.Fatalf("Duplicate err: %v", err)
	}
	if view.Status != "draft" {
		t.Fatalf("copia deveria nascer draft, got: %q", view.Status)
	}
	if view.Name != "Copia de Perola" {
		t.Fatalf("name da copia inesperado: %q", view.Name)
	}
	if view.Slug != "perola-copia" {
		t.Fatalf("slug da copia inesperado: %q", view.Slug)
	}
	if string(view.DataDraft) != `{"title":"rascunho"}` {
		t.Fatalf("copia deveria copiar o DRAFT da origem, got: %s", view.DataDraft)
	}
	if len(view.DataPublished) != 0 {
		t.Fatalf("copia nao deveria herdar o published, got: %s", view.DataPublished)
	}
	if view.AccountID != "acc-1" {
		t.Fatalf("copia deveria ficar na mesma account, got: %q", view.AccountID)
	}
}

func TestDuplicateUniqueSlugSuffix(t *testing.T) {
	store := newFakeStore()
	seedSource(store)
	store.slugs["perola-copia"] = true // ja existe uma copia
	svc := NewService(store, "")

	view, err := svc.Duplicate(context.Background(), true, "", "src", DuplicateRequest{})
	if err != nil {
		t.Fatalf("Duplicate err: %v", err)
	}
	if view.Slug != "perola-copia-2" {
		t.Fatalf("slug da copia deveria escapar a colisao, got: %q", view.Slug)
	}
}

func TestDuplicateOutOfScopeNotFound(t *testing.T) {
	store := newFakeStore()
	seedSource(store)
	svc := NewService(store, "")

	_, err := svc.Duplicate(context.Background(), false, "acc-2", "src", DuplicateRequest{})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("duplicar bio de outra account deveria ser ErrNotFound, got: %v", err)
	}
}

func TestDuplicateAdminToOtherAccount(t *testing.T) {
	store := newFakeStore()
	seedSource(store)
	store.accounts["acc-9"] = true
	svc := NewService(store, "")

	view, err := svc.Duplicate(context.Background(), true, "", "src", DuplicateRequest{AccountID: "acc-9"})
	if err != nil {
		t.Fatalf("Duplicate err: %v", err)
	}
	if view.AccountID != "acc-9" {
		t.Fatalf("admin deveria duplicar para a account destino, got: %q", view.AccountID)
	}
	if !store.moduleEnabled["acc-9"] {
		t.Fatal("Duplicate deveria habilitar o modulo na account destino")
	}
}

func TestDuplicateAdminToMissingAccountNotFound(t *testing.T) {
	store := newFakeStore()
	seedSource(store)
	svc := NewService(store, "")

	_, err := svc.Duplicate(context.Background(), true, "", "src", DuplicateRequest{AccountID: "nao-existe"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("account destino inexistente deveria ser ErrNotFound, got: %v", err)
	}
}

// ============================================================================
// Patch — mover de account so admin
// ============================================================================

func TestPatchAdminMovesAccount(t *testing.T) {
	store := newFakeStore()
	seedSource(store)
	store.accounts["acc-9"] = true
	svc := NewService(store, "")

	target := "acc-9"
	view, err := svc.Patch(context.Background(), true, "", "src", PatchRequest{AccountID: &target})
	if err != nil {
		t.Fatalf("Patch err: %v", err)
	}
	if view.AccountID != "acc-9" {
		t.Fatalf("admin deveria mover a bio de account, got: %q", view.AccountID)
	}
}

func TestPatchNonAdminIgnoresAccount(t *testing.T) {
	store := newFakeStore()
	seedSource(store)
	store.accounts["acc-9"] = true
	svc := NewService(store, "")

	target := "acc-9"
	view, err := svc.Patch(context.Background(), false, "acc-1", "src", PatchRequest{AccountID: &target})
	if err != nil {
		t.Fatalf("Patch err: %v", err)
	}
	if view.AccountID != "acc-1" {
		t.Fatalf("nao-admin NUNCA troca de account, got: %q", view.AccountID)
	}
}

func TestPatchAdminMissingAccountNotFound(t *testing.T) {
	store := newFakeStore()
	seedSource(store)
	svc := NewService(store, "")

	target := "nao-existe"
	_, err := svc.Patch(context.Background(), true, "", "src", PatchRequest{AccountID: &target})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("mover para account inexistente deveria ser ErrNotFound, got: %v", err)
	}
}
