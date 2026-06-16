package site

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// perolaFixture e um payload real-like da API da Perola: envelope {data, meta}
// com categories/campaigns como TEXTO contendo JSON-array, price/stock/fator
// numericos e um item com deleted_at.
const perolaFixture = `{
  "data": [
    {
      "id": 101,
      "name": "Anel Solitario",
      "code": "AN-101",
      "categories": "[\"Aneis\",\"Ouro\"]",
      "campaigns": "[\"Natal\"]",
      "image": "/uploads/anel.jpg",
      "status": "active",
      "stock": 7,
      "fator": 2.5,
      "price": 1299.9,
      "deleted_at": null
    },
    {
      "id": 102,
      "name": "Brinco Perola",
      "code": "BR-102",
      "categories": "[]",
      "campaigns": "[]",
      "image": "https://cdn.example.com/brinco.png",
      "status": "desactive",
      "stock": 0,
      "fator": 0,
      "price": 0,
      "deleted_at": "2026-06-01 10:00:00"
    }
  ],
  "meta": { "page": 0, "limit": 100, "count": 2, "total": 2, "has_more": false }
}`

func TestParsePerolaEnvelope(t *testing.T) {
	env, err := parsePerolaEnvelope([]byte(perolaFixture))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(env.Data) != 2 {
		t.Fatalf("data len = %d, want 2", len(env.Data))
	}
	if env.Meta.HasMore {
		t.Fatalf("meta.has_more = true, want false")
	}
	if env.Meta.Total != 2 {
		t.Fatalf("meta.total = %d, want 2", env.Meta.Total)
	}
}

func TestMapPerolaProduct(t *testing.T) {
	env, err := parsePerolaEnvelope([]byte(perolaFixture))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	a := mapPerolaProduct(env.Data[0])
	if a.ExternalID != "101" || a.Source != "perola" {
		t.Fatalf("key = (%q,%q), want (101, perola)", a.ExternalID, a.Source)
	}
	if a.Name != "Anel Solitario" || a.Code != "AN-101" {
		t.Fatalf("name/code wrong: %q %q", a.Name, a.Code)
	}
	if a.Image != "https://perolajoias.com/uploads/anel.jpg" {
		t.Fatalf("image not absolutized: %q", a.Image)
	}
	if !reflect.DeepEqual(a.Categories, []string{"Aneis", "Ouro"}) {
		t.Fatalf("categories = %v", a.Categories)
	}
	if !reflect.DeepEqual(a.Campaigns, []string{"Natal"}) {
		t.Fatalf("campaigns = %v", a.Campaigns)
	}
	if a.Price != 1299.9 {
		t.Fatalf("price = %v, want 1299.9", a.Price)
	}
	if a.Fator != 2.5 {
		t.Fatalf("fator = %v, want 2.5", a.Fator)
	}
	if a.Stock != 7 {
		t.Fatalf("stock = %d, want 7", a.Stock)
	}
	if a.Status != "active" {
		t.Fatalf("status = %q, want active", a.Status)
	}
	if a.Deleted {
		t.Fatalf("item 101 should not be deleted")
	}

	b := mapPerolaProduct(env.Data[1])
	if b.Status != "inactive" {
		t.Fatalf("desactive should map to inactive, got %q", b.Status)
	}
	if !b.Deleted {
		t.Fatalf("item 102 should be deleted (deleted_at set)")
	}
	if b.Image != "https://cdn.example.com/brinco.png" {
		t.Fatalf("absolute image should pass through: %q", b.Image)
	}
	if b.Fator != 1 {
		t.Fatalf("fator 0 should default to 1, got %v", b.Fator)
	}
	if len(b.Categories) != 0 || len(b.Campaigns) != 0 {
		t.Fatalf("empty arrays should map to empty slices")
	}
}

func TestPerolaImageURL(t *testing.T) {
	// Casos com path/http/vazio: campaigns/categories nao influem.
	base := map[string]string{
		"":                    "",
		"/uploads/a.jpg":      "https://perolajoias.com/uploads/a.jpg",
		"uploads/a.jpg":       "https://perolajoias.com/uploads/a.jpg",
		"https://x.com/a.jpg": "https://x.com/a.jpg",
		"http://x.com/a.jpg":  "http://x.com/a.jpg",
	}
	for in, want := range base {
		if got := perolaImageURL(in, nil, nil); got != want {
			t.Fatalf("perolaImageURL(%q) = %q, want %q", in, got, want)
		}
	}

	// So o nome do arquivo: monta assets/images/products/{segmento}/{arquivo},
	// segmento = 1a campanha (ou 1a categoria como fallback; sem ambas, sem segmento).
	const host = "https://perolajoias.com/assets/images/products/"
	if got := perolaImageURL("368252.avif", []string{"namorados_26"}, []string{"Aneis"}); got != host+"namorados_26/368252.avif" {
		t.Fatalf("campanha como segmento: got %q", got)
	}
	if got := perolaImageURL("368252.avif", nil, []string{"Aneis"}); got != host+"Aneis/368252.avif" {
		t.Fatalf("categoria como fallback: got %q", got)
	}
	if got := perolaImageURL("368252.avif", nil, nil); got != host+"368252.avif" {
		t.Fatalf("sem segmento: got %q", got)
	}
}

func TestPerolaImageCandidates(t *testing.T) {
	const base = "https://perolajoias.com/assets/images/products"

	// http/path: passthrough como candidata unica (igual a perolaImageURL).
	if got := perolaImageCandidates("https://x.com/a.jpg", nil, nil); !reflect.DeepEqual(got, []string{"https://x.com/a.jpg"}) {
		t.Fatalf("http passthrough: %v", got)
	}
	if got := perolaImageCandidates("/uploads/a.jpg", nil, nil); !reflect.DeepEqual(got, []string{"https://perolajoias.com/uploads/a.jpg"}) {
		t.Fatalf("path passthrough: %v", got)
	}
	if got := perolaImageCandidates("", nil, nil); got != nil {
		t.Fatalf("vazio deveria ser nil: %v", got)
	}

	// So o nome do arquivo, SEM campanha/categoria: precisa cair em /default/ (o
	// bug original montava /products/0278091.webp sem pasta e dava 404).
	cands := perolaImageCandidates("0278091.webp", nil, nil)
	if len(cands) == 0 || cands[0] != base+"/default/0278091.webp" {
		t.Fatalf("1a candidata sem segmento deveria ser default/original: %v", cands)
	}
	if !containsStr(cands, base+"/default/0278091_sm.avif") {
		t.Fatalf("deveria conter a thumb _sm.avif em default: %v", cands)
	}
	if !containsStr(cands, base+"/default/0278091.avif") || !containsStr(cands, base+"/default/0278091.jpg") {
		t.Fatalf("deveria conter variantes .avif e .jpg: %v", cands)
	}

	// Com campanha: o segmento da campanha (slug em underscore) vem ANTES do
	// default; categoria entra como fallback intermediario.
	cands2 := perolaImageCandidates("368252.avif", []string{"Dia dos Pais"}, []string{"Aneis"})
	if cands2[0] != base+"/dia_dos_pais/368252.avif" {
		t.Fatalf("campanha deveria virar o 1o segmento (slug): %v", cands2[0])
	}
	if !containsStr(cands2, base+"/aneis/368252.avif") {
		t.Fatalf("categoria deveria entrar como segmento: %v", cands2)
	}
	if !containsStr(cands2, base+"/default/368252.avif") {
		t.Fatalf("default deveria fechar a lista: %v", cands2)
	}
}

func containsStr(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func TestParseJSONArrayTolerant(t *testing.T) {
	if got := parseJSONArray(""); len(got) != 0 {
		t.Fatalf("empty -> %v", got)
	}
	if got := parseJSONArray("null"); len(got) != 0 {
		t.Fatalf("null -> %v", got)
	}
	if got := parseJSONArray("nao-json"); len(got) != 0 {
		t.Fatalf("invalid -> %v", got)
	}
	got := parseJSONArray(`["A"," B ",""]`)
	if !reflect.DeepEqual(got, []string{"A", "B"}) {
		t.Fatalf("trim/drop-empty failed: %v", got)
	}
}

// ============================================================================
// Sync idempotente (fake fetcher + fake repo que simula o ON CONFLICT)
// ============================================================================

type fakeFetcher struct {
	items []ProductUpsertItem
	calls int
}

func (f *fakeFetcher) FetchAll(_ context.Context, _ string) ([]ProductUpsertItem, error) {
	f.calls++
	return f.items, nil
}

// fakeSourceRepo simula site.product_sources + o upsert por (account, source,
// external_id): primeira gravacao de uma chave conta como inserted, repeticoes
// como updated.
type fakeSourceRepo struct {
	sources []ProductSource
	seen    map[string]bool
}

func (r *fakeSourceRepo) ListByAccount(_ context.Context, _ string) ([]ProductSource, error) {
	return r.sources, nil
}

func (r *fakeSourceRepo) UpsertProducts(_ context.Context, accountID string, items []ProductUpsertItem) (ProductSyncResult, error) {
	if r.seen == nil {
		r.seen = map[string]bool{}
	}
	res := ProductSyncResult{}
	for _, it := range items {
		key := accountID + "|" + it.Source + "|" + it.ExternalID
		if r.seen[key] {
			res.Updated++
		} else {
			r.seen[key] = true
			res.Inserted++
		}
	}
	return res, nil
}

func (r *fakeSourceRepo) GetAccountSource(_ context.Context, _ string) (ProductSource, error) {
	for i := range r.sources {
		if r.sources[i].Type == productSourceType {
			return r.sources[i], nil
		}
	}
	return ProductSource{}, ErrNoProductSource
}

func (r *fakeSourceRepo) SetAccountSourceBaseURL(_ context.Context, _, baseURL string) error {
	for i := range r.sources {
		if r.sources[i].Type == productSourceType {
			r.sources[i].BaseURL = baseURL
			return nil
		}
	}
	return ErrNoProductSource
}

func newSyncService(fetcher productSourceFetcher, repo ProductSourceRepository) *Service {
	return NewService(nil, nil, nil, nil).WithProductSync(repo, fetcher)
}

func TestProductSourceModeFromBaseURL(t *testing.T) {
	cases := map[string]string{
		productSourceURLOnline:            productSourceModeOnline,
		productSourceURLLocal:             productSourceModeLocal,
		"https://outro.com/api/products/": productSourceModeCustom,
		"":                                productSourceModeCustom,
	}
	for baseURL, want := range cases {
		if got := productSourceModeFromBaseURL(baseURL); got != want {
			t.Fatalf("productSourceModeFromBaseURL(%q) = %q, want %q", baseURL, got, want)
		}
	}
}

func TestGetProductSourceNoSource(t *testing.T) {
	svc := newSyncService(&fakeFetcher{}, &fakeSourceRepo{})
	view, err := svc.GetProductSource(context.Background(), "acc-1")
	if err != nil {
		t.Fatalf("GetProductSource: %v", err)
	}
	if view.Mode != productSourceModeOnline || view.BaseURL != "" {
		t.Fatalf("GetProductSource (no source) = %+v, want {online, ''}", view)
	}
}

func TestSetProductSourceMode(t *testing.T) {
	repo := &fakeSourceRepo{
		sources: []ProductSource{{BaseURL: productSourceURLOnline, Enabled: true, Type: productSourceType}},
	}
	svc := newSyncService(&fakeFetcher{}, repo)

	view, err := svc.SetProductSourceMode(context.Background(), "acc-1", productSourceModeLocal)
	if err != nil {
		t.Fatalf("SetProductSourceMode(local): %v", err)
	}
	if view.Mode != productSourceModeLocal || view.BaseURL != productSourceURLLocal {
		t.Fatalf("SetProductSourceMode(local) = %+v, want {local, %q}", view, productSourceURLLocal)
	}

	// O GET subsequente deve refletir o novo base_url (local).
	got, err := svc.GetProductSource(context.Background(), "acc-1")
	if err != nil {
		t.Fatalf("GetProductSource after set: %v", err)
	}
	if got.Mode != productSourceModeLocal || got.BaseURL != productSourceURLLocal {
		t.Fatalf("GetProductSource after set = %+v, want {local, %q}", got, productSourceURLLocal)
	}

	if _, err := svc.SetProductSourceMode(context.Background(), "acc-1", "bogus"); !errors.Is(err, ErrInvalidProductSourceMode) {
		t.Fatalf("SetProductSourceMode(bogus) err = %v, want ErrInvalidProductSourceMode", err)
	}
}

func TestSyncProductsIdempotent(t *testing.T) {
	items := []ProductUpsertItem{
		{ExternalID: "101", Source: "perola", Name: "Anel"},
		{ExternalID: "102", Source: "perola", Name: "Brinco"},
	}
	fetcher := &fakeFetcher{items: items}
	repo := &fakeSourceRepo{
		sources: []ProductSource{{BaseURL: "https://perolajoias.com/api/products/", Enabled: true, Type: "external_api"}},
	}
	svc := newSyncService(fetcher, repo)

	first, err := svc.SyncProducts(context.Background(), "acc-1")
	if err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if first.Inserted != 2 || first.Updated != 0 {
		t.Fatalf("first sync = %+v, want inserted=2 updated=0", first)
	}

	second, err := svc.SyncProducts(context.Background(), "acc-1")
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if second.Inserted != 0 || second.Updated != 2 {
		t.Fatalf("second sync = %+v, want inserted=0 updated=2 (idempotente)", second)
	}
	if fetcher.calls != 2 {
		t.Fatalf("fetcher calls = %d, want 2", fetcher.calls)
	}
}

func TestSyncProductsNoSource(t *testing.T) {
	svc := newSyncService(&fakeFetcher{}, &fakeSourceRepo{sources: nil})
	_, err := svc.SyncProducts(context.Background(), "acc-1")
	if err != ErrNoProductSource {
		t.Fatalf("err = %v, want ErrNoProductSource", err)
	}
}

func TestSyncProductsDisabledSourceSkipped(t *testing.T) {
	repo := &fakeSourceRepo{sources: []ProductSource{{BaseURL: "x", Enabled: false}}}
	svc := newSyncService(&fakeFetcher{}, repo)
	_, err := svc.SyncProducts(context.Background(), "acc-1")
	if err != ErrNoProductSource {
		t.Fatalf("disabled-only should yield ErrNoProductSource, got %v", err)
	}
}

func TestSyncProductsUnavailable(t *testing.T) {
	svc := NewService(nil, nil, nil, nil) // sem WithProductSync
	_, err := svc.SyncProducts(context.Background(), "acc-1")
	if err != ErrProductSyncUnavailable {
		t.Fatalf("err = %v, want ErrProductSyncUnavailable", err)
	}
}

// TestPerolaSiteRoot: o host das imagens segue o base_url (toggle local/online).
func TestPerolaSiteRoot(t *testing.T) {
	cases := map[string]string{
		"https://perolajoias.com/api/products/":                   "https://perolajoias.com",
		"https://perolajoias.com/api/products":                    "https://perolajoias.com",
		"http://host.docker.internal/painel-perola/api/products/": "http://host.docker.internal/painel-perola",
		"":            perolaBaseHost,
		"nao-e-url":   perolaBaseHost,
		"ftp://x/api": "ftp://x",
	}
	for in, want := range cases {
		if got := perolaSiteRoot(in); got != want {
			t.Fatalf("perolaSiteRoot(%q) = %q, want %q", in, got, want)
		}
	}

	// A URL de imagem realmente troca de host conforme a raiz.
	if got := perolaImageCandidatesAt("368252.avif", []string{"namorados_26"}, nil, "http://host.docker.internal/painel-perola"); len(got) == 0 ||
		got[0] != "http://host.docker.internal/painel-perola/assets/images/products/namorados_26/368252.avif" {
		t.Fatalf("candidata local com host derivado errada: %v", got)
	}
}
