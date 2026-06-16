package bio

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// ============================================================================
// Fake ProductSource (sem banco) para cobrir Sources/Facets/resolucao publica
// ============================================================================

// fakeSource implementa ProductSource em memoria. Registra os argumentos da
// ultima chamada de Resolve para os testes inspecionarem o filtro propagado.
type fakeSource struct {
	facets     SourceFacets
	slides     []ResolvedSlide
	resolveErr error

	lastFilter   SourceFilter
	lastLink     string
	lastWhatsapp string
	lastAccount  string
}

func (f *fakeSource) Facets(_ context.Context, _ string) (SourceFacets, error) {
	return f.facets, nil
}

func (f *fakeSource) Resolve(_ context.Context, accountID string, filter SourceFilter, link, whatsapp string) ([]ResolvedSlide, error) {
	f.lastAccount = accountID
	f.lastFilter = filter
	f.lastLink = link
	f.lastWhatsapp = whatsapp
	if f.resolveErr != nil {
		return nil, f.resolveErr
	}
	return f.slides, nil
}

// ============================================================================
// Sources / Facets
// ============================================================================

func TestSourcesListsRegistered(t *testing.T) {
	svc := NewService(newFakeStore(), "")
	svc.RegisterSource(SourceTypeSiteProducts, &fakeSource{})

	sources := svc.Sources(context.Background(), "acc-1")
	if len(sources) != 1 {
		t.Fatalf("esperava 1 fonte, got: %d", len(sources))
	}
	if sources[0].Type != SourceTypeSiteProducts || !sources[0].Available {
		t.Fatalf("fonte inesperada: %+v", sources[0])
	}
	if sources[0].Label != "Produtos do site" {
		t.Fatalf("label inesperado: %q", sources[0].Label)
	}
}

func TestFacetsDelegatesToSource(t *testing.T) {
	svc := NewService(newFakeStore(), "")
	want := SourceFacets{
		Categories: []string{"Relogios", "Joias"},
		Campaigns:  []string{"Natal"},
		Tipos:      []string{"masculino"},
	}
	svc.RegisterSource(SourceTypeSiteProducts, &fakeSource{facets: want})

	got, err := svc.Facets(context.Background(), "acc-1", SourceTypeSiteProducts)
	if err != nil {
		t.Fatalf("Facets err: %v", err)
	}
	if len(got.Categories) != 2 || got.Campaigns[0] != "Natal" || got.Tipos[0] != "masculino" {
		t.Fatalf("facets inesperados: %+v", got)
	}
}

func TestFacetsUnknownSourceNotFound(t *testing.T) {
	svc := NewService(newFakeStore(), "")
	_, err := svc.Facets(context.Background(), "acc-1", "erp_xpto")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("fonte desconhecida deveria ser ErrNotFound, got: %v", err)
	}
}

// ============================================================================
// Public — resolucao da fonte (injeta slides) + fallback manual
// ============================================================================

// publicStore embute o fakeStore mas serve um data_published com o slideTop
// pedido pelo teste, para exercitar Service.Public ponta a ponta.
type publicStore struct {
	*fakeStore
	published json.RawMessage
	account   string
}

func (p *publicStore) PublicLookup(context.Context, string) (json.RawMessage, string, error) {
	return p.published, p.account, nil
}

func newPublicService(published string, account string) *Service {
	store := &publicStore{
		fakeStore: newFakeStore(),
		published: json.RawMessage(published),
		account:   account,
	}
	return NewService(store, "")
}

func slidesOf(t *testing.T, raw json.RawMessage) []map[string]any {
	t.Helper()
	var doc struct {
		SlideTop struct {
			Slides []map[string]any `json:"slides"`
		} `json:"slideTop"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal public: %v", err)
	}
	return doc.SlideTop.Slides
}

func TestPublicResolvesSiteProductsSource(t *testing.T) {
	published := `{
		"slideTop": {
			"source": {"type":"site_products","category":"Relogios","limit":2,"link":"whatsapp"},
			"slides": [{"src":"/manual.png","title":"manual"}]
		},
		"lightbox": {"whatsappNumber":"+55 11 98888-7777"}
	}`
	svc := newPublicService(published, "acc-1")
	src := &fakeSource{slides: []ResolvedSlide{
		{Src: "/uploads/p1.png", Title: "Produto 1", Desc: "Anel de ouro", Price: "R$ 1.234,56", Href: "https://wa.me/5511988887777"},
		{Src: "/uploads/p2.png", Title: "Produto 2"},
	}}
	svc.RegisterSource(SourceTypeSiteProducts, src)

	out, err := svc.Public(context.Background(), "minha-bio")
	if err != nil {
		t.Fatalf("Public err: %v", err)
	}

	slides := slidesOf(t, out)
	if len(slides) != 2 {
		t.Fatalf("esperava 2 slides resolvidos, got: %d (%s)", len(slides), out)
	}
	if slides[0]["title"] != "Produto 1" || slides[0]["src"] != "/uploads/p1.png" {
		t.Fatalf("slide 0 inesperado: %+v", slides[0])
	}
	// desc/price chegam ao slide resolvido (Lightbox do front exibe).
	if slides[0]["desc"] != "Anel de ouro" || slides[0]["price"] != "R$ 1.234,56" {
		t.Fatalf("desc/price nao propagados: %+v", slides[0])
	}
	// Slide sem desc/price nao carrega campos vazios (omitempty).
	if _, ok := slides[1]["desc"]; ok {
		t.Fatalf("slide 1 nao deveria ter desc: %+v", slides[1])
	}
	if _, ok := slides[1]["price"]; ok {
		t.Fatalf("slide 1 nao deveria ter price: %+v", slides[1])
	}
	if slides[0]["href"] != "https://wa.me/5511988887777" {
		t.Fatalf("href do slide 0 inesperado: %+v", slides[0]["href"])
	}
	// Filtro propagado para a fonte: categoria, limit e account.
	if src.lastFilter.Category != "Relogios" || src.lastFilter.Limit != 2 {
		t.Fatalf("filtro nao propagado: %+v", src.lastFilter)
	}
	if src.lastAccount != "acc-1" {
		t.Fatalf("account nao propagado: %q", src.lastAccount)
	}
	if src.lastLink != "whatsapp" || src.lastWhatsapp != "+55 11 98888-7777" {
		t.Fatalf("link/whatsapp nao propagados: %q / %q", src.lastLink, src.lastWhatsapp)
	}
}

func TestPublicManualSourceKeepsManualSlides(t *testing.T) {
	published := `{"slideTop":{"source":{"type":"manual"},"slides":[{"src":"/m.png","title":"manual"}]}}`
	svc := newPublicService(published, "acc-1")
	svc.RegisterSource(SourceTypeSiteProducts, &fakeSource{slides: []ResolvedSlide{{Src: "/x.png"}}})

	out, err := svc.Public(context.Background(), "minha-bio")
	if err != nil {
		t.Fatalf("Public err: %v", err)
	}
	slides := slidesOf(t, out)
	if len(slides) != 1 || slides[0]["src"] != "/m.png" {
		t.Fatalf("source manual deveria manter os slides manuais, got: %+v", slides)
	}
}

func TestPublicNoSourceKeepsManualSlides(t *testing.T) {
	published := `{"slideTop":{"slides":[{"src":"/m.png"}]}}`
	svc := newPublicService(published, "acc-1")
	svc.RegisterSource(SourceTypeSiteProducts, &fakeSource{slides: []ResolvedSlide{{Src: "/x.png"}}})

	out, err := svc.Public(context.Background(), "minha-bio")
	if err != nil {
		t.Fatalf("Public err: %v", err)
	}
	slides := slidesOf(t, out)
	if len(slides) != 1 || slides[0]["src"] != "/m.png" {
		t.Fatalf("sem source deveria manter os slides manuais, got: %+v", slides)
	}
}

func TestPublicSourceErrorFallsBackToManual(t *testing.T) {
	published := `{"slideTop":{"source":{"type":"site_products"},"slides":[{"src":"/m.png"}]}}`
	svc := newPublicService(published, "acc-1")
	svc.RegisterSource(SourceTypeSiteProducts, &fakeSource{resolveErr: errors.New("db down")})

	out, err := svc.Public(context.Background(), "minha-bio")
	if err != nil {
		t.Fatalf("Public nao deveria propagar erro da fonte: %v", err)
	}
	slides := slidesOf(t, out)
	if len(slides) != 1 || slides[0]["src"] != "/m.png" {
		t.Fatalf("erro da fonte deveria cair nos slides manuais, got: %+v", slides)
	}
}

func TestPublicEmptySourceFallsBackToManual(t *testing.T) {
	published := `{"slideTop":{"source":{"type":"site_products"},"slides":[{"src":"/m.png"}]}}`
	svc := newPublicService(published, "acc-1")
	svc.RegisterSource(SourceTypeSiteProducts, &fakeSource{slides: nil})

	out, err := svc.Public(context.Background(), "minha-bio")
	if err != nil {
		t.Fatalf("Public err: %v", err)
	}
	slides := slidesOf(t, out)
	if len(slides) != 1 || slides[0]["src"] != "/m.png" {
		t.Fatalf("fonte vazia deveria cair nos slides manuais, got: %+v", slides)
	}
}

func TestPublicUnknownSourceTypeFallsBackToManual(t *testing.T) {
	published := `{"slideTop":{"source":{"type":"erp_externo"},"slides":[{"src":"/m.png"}]}}`
	svc := newPublicService(published, "acc-1")
	svc.RegisterSource(SourceTypeSiteProducts, &fakeSource{slides: []ResolvedSlide{{Src: "/x.png"}}})

	out, err := svc.Public(context.Background(), "minha-bio")
	if err != nil {
		t.Fatalf("Public err: %v", err)
	}
	slides := slidesOf(t, out)
	if len(slides) != 1 || slides[0]["src"] != "/m.png" {
		t.Fatalf("type desconhecido deveria cair nos slides manuais, got: %+v", slides)
	}
}

// ============================================================================
// Helpers de href (whatsapp/produto)
// ============================================================================

func TestResolveProductHref(t *testing.T) {
	if got := resolveProductHref(ProductLinkNone, "5511999998888", "X", "C1"); got != "" {
		t.Fatalf("link none deveria ser vazio, got: %q", got)
	}
	if got := resolveProductHref(ProductLinkProduct, "5511999998888", "X", "C1"); got != "" {
		t.Fatalf("link product (sem URL na fonte) deveria ser vazio, got: %q", got)
	}
	got := resolveProductHref(ProductLinkWhatsApp, "+55 (11) 99999-8888", "Relogio", "C1")
	if got == "" || got[:len("https://wa.me/5511999998888")] != "https://wa.me/5511999998888" {
		t.Fatalf("whatsapp href inesperado: %q", got)
	}
	if got := resolveProductHref(ProductLinkWhatsApp, "", "X", "C1"); got != "" {
		t.Fatalf("whatsapp sem numero deveria ser vazio, got: %q", got)
	}
}

func TestFormatPriceBRL(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, ""},
		{-5, ""},
		{9.9, "R$ 9,90"},
		{1234.56, "R$ 1.234,56"},
		{1000000, "R$ 1.000.000,00"},
		{99.5, "R$ 99,50"},
	}
	for _, c := range cases {
		if got := formatPriceBRL(c.in); got != c.want {
			t.Fatalf("formatPriceBRL(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDigitsOnly(t *testing.T) {
	if got := digitsOnly("+55 (11) 99999-8888"); got != "5511999998888" {
		t.Fatalf("digitsOnly inesperado: %q", got)
	}
}
