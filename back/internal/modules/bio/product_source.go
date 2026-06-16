package bio

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ProductSource e a abstracao de uma fonte de produtos para o slideTop. A 1a
// implementacao (SiteProductsSource) le o schema site.products; ERP/API externa
// do cliente entram depois sem mexer no resto (D1).
//
// Facets devolve os valores distintos (categorias/campanhas/tipos) da account
// para popular os selects do editor. Resolve aplica o filtro e o limite e
// devolve os slides ja prontos (src/title/href).
type ProductSource interface {
	Facets(ctx context.Context, accountID string) (SourceFacets, error)
	Resolve(ctx context.Context, accountID string, filter SourceFilter, link, whatsapp string) ([]ResolvedSlide, error)
}

// SiteProductsSource le produtos ativos de site.products por account. Acessa o
// schema `site` cross-schema pelo MESMO pool (apenas SELECT) — NAO importa o
// modulo site; depende so do shape estavel da tabela (documentado no AGENT.md).
type SiteProductsSource struct {
	pool *pgxpool.Pool
}

// NewSiteProductsSource cria a fonte. pool nil => Facets/Resolve devolvem vazio
// (a fonte simplesmente nao resolve nada; cai no fallback manual).
func NewSiteProductsSource(pool *pgxpool.Pool) *SiteProductsSource {
	return &SiteProductsSource{pool: pool}
}

// activeProductsWhere e a condicao de "produto ativo" reutilizada nas duas
// queries: status ativo E nao soft-deletado.
const activeProductsWhere = "p.account_id = $1::uuid and p.is_active = true and p.status = 'active'"

// Facets devolve categorias/campanhas/tipos distintos (ordem alfabetica estavel)
// dos produtos ativos da account. categories/campaigns sao jsonb arrays de
// texto em site.products; o jsonb_array_elements_text desaninha sem N+1.
func (s *SiteProductsSource) Facets(ctx context.Context, accountID string) (SourceFacets, error) {
	out := SourceFacets{Categories: []string{}, Campaigns: []string{}, Tipos: []string{}}
	if s.pool == nil || strings.TrimSpace(accountID) == "" {
		return out, nil
	}

	categories, err := s.distinctFromArray(ctx, accountID, "categories")
	if err != nil {
		return SourceFacets{}, err
	}
	campaigns, err := s.distinctFromArray(ctx, accountID, "campaigns")
	if err != nil {
		return SourceFacets{}, err
	}
	tipos, err := s.distinctTipos(ctx, accountID)
	if err != nil {
		return SourceFacets{}, err
	}
	out.Categories = categories
	out.Campaigns = campaigns
	out.Tipos = tipos
	return out, nil
}

// distinctFromArray devolve os valores distintos nao-vazios de uma coluna jsonb
// array de texto (categories/campaigns). O nome da coluna NAO vem do usuario:
// e uma constante interna ("categories"/"campaigns"), sem superficie de injecao.
func (s *SiteProductsSource) distinctFromArray(ctx context.Context, accountID, column string) ([]string, error) {
	query := `
		select distinct elem
		from site.products p
		cross join lateral jsonb_array_elements_text(coalesce(p.` + column + `, '[]'::jsonb)) as elem
		where ` + activeProductsWhere + `
		  and trim(elem) <> ''
		order by elem asc`
	return s.queryStrings(ctx, query, accountID)
}

// distinctTipos devolve os tipos distintos nao-vazios (coluna text simples).
func (s *SiteProductsSource) distinctTipos(ctx context.Context, accountID string) ([]string, error) {
	const query = `
		select distinct p.tipo
		from site.products p
		where ` + activeProductsWhere + `
		  and trim(p.tipo) <> ''
		order by p.tipo asc`
	return s.queryStrings(ctx, query, accountID)
}

func (s *SiteProductsSource) queryStrings(ctx context.Context, query, accountID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, query, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Resolve devolve os slides dos produtos ativos da account que casam o filtro,
// respeitando o limite (0 = todos; senao N). Ordem estavel por nome (e id como
// desempate) para a mesma entrada produzir sempre a mesma saida.
func (s *SiteProductsSource) Resolve(ctx context.Context, accountID string, filter SourceFilter, link, whatsapp string) ([]ResolvedSlide, error) {
	if s.pool == nil || strings.TrimSpace(accountID) == "" {
		return nil, nil
	}

	args := []any{accountID}
	conds := []string{activeProductsWhere}
	n := 2

	if c := strings.TrimSpace(filter.Category); c != "" {
		conds = append(conds, "p.categories @> to_jsonb(array["+paramN(n)+"::text])")
		args = append(args, c)
		n++
	}
	if t := strings.TrimSpace(filter.Tipo); t != "" {
		conds = append(conds, "p.tipo = "+paramN(n))
		args = append(args, t)
		n++
	}
	if camps := trimNonEmpty(filter.Campaigns); len(camps) > 0 {
		// Casa se o produto tem QUALQUER uma das campanhas pedidas (overlap).
		// pgx serializa []string como text[]; o operador jsonb ?| testa
		// existencia de qualquer elemento do array no array de campanhas.
		conds = append(conds, "p.campaigns ?| "+paramN(n)+"::text[]")
		args = append(args, camps)
		n++
	}

	query := `
		select p.id, p.name, p.image, p.code, p.description, p.price
		from site.products p
		where ` + strings.Join(conds, " and ") + `
		order by lower(p.name) asc, p.id asc`
	if filter.Limit > 0 {
		query += " limit " + paramN(n)
		args = append(args, filter.Limit)
	}

	return s.scanSlides(ctx, query, args, link, whatsapp)
}

func (s *SiteProductsSource) scanSlides(ctx context.Context, query string, args []any, link, whatsapp string) ([]ResolvedSlide, error) {
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ResolvedSlide{}
	for rows.Next() {
		var id, name, image, code, description string
		var price float64
		if err := rows.Scan(&id, &name, &image, &code, &description, &price); err != nil {
			return nil, err
		}
		out = append(out, ResolvedSlide{
			Src:   strings.TrimSpace(image),
			Title: strings.TrimSpace(name),
			Desc:  strings.TrimSpace(description),
			Price: formatPriceBRL(price),
			Href:  resolveProductHref(link, whatsapp, name, code),
		})
	}
	return out, rows.Err()
}

// resolveProductHref monta o href do slide conforme a opcao de link:
//   - product : sem URL de produto por enquanto em site.products -> sem link
//     (placeholder ate a fonte trazer a URL; nunca quebra).
//   - whatsapp: wa.me do numero da bio/lightbox com mensagem do produto.
//   - none / desconhecido: "" (sem link).
func resolveProductHref(link, whatsapp, name, code string) string {
	switch strings.TrimSpace(link) {
	case ProductLinkWhatsApp:
		return whatsappHref(whatsapp, name, code)
	default:
		// product/none/desconhecido: site.products nao tem coluna de URL do
		// produto hoje; sem link (o botao abaixo do carrossel cobre o "ver
		// colecao"). Quando a fonte trouxer a URL, e so popular aqui.
		return ""
	}
}

// whatsappHref monta https://wa.me/<numero>?text=... O numero so mantem digitos;
// vazio => sem link. A mensagem cita o produto (nome/codigo) quando houver.
func whatsappHref(whatsapp, name, code string) string {
	digits := digitsOnly(whatsapp)
	if digits == "" {
		return ""
	}
	href := "https://wa.me/" + digits
	if msg := productMessage(name, code); msg != "" {
		href += "?text=" + url.QueryEscape(msg)
	}
	return href
}

func productMessage(name, code string) string {
	name = strings.TrimSpace(name)
	code = strings.TrimSpace(code)
	switch {
	case name != "" && code != "":
		return "Ola! Tenho interesse no produto " + name + " (" + code + ")."
	case name != "":
		return "Ola! Tenho interesse no produto " + name + "."
	default:
		return ""
	}
}

// formatPriceBRL formata o preco em Real (pt-BR: "R$ 1.234,56"). Preco <= 0 =>
// "" (sem preco; o Lightbox so mostra o bloco quando ha valor). Espelha o
// formato do BioSlide manual e do admin de produtos do site.
func formatPriceBRL(price float64) string {
	if price <= 0 {
		return ""
	}
	// Duas casas, separador de milhar "." e decimal ",".
	whole := int64(price)
	cents := int64(price*100+0.5) - whole*100
	intPart := strconv.FormatInt(whole, 10)
	var b strings.Builder
	for i, r := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return "R$ " + b.String() + "," + fmt.Sprintf("%02d", cents)
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func trimNonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if t := strings.TrimSpace(v); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func paramN(n int) string { return "$" + strconv.Itoa(n) }
