package site

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// perolaBaseHost absolutiza imagens relativas vindas da API da Perola (default
// online). Para a fonte LOCAL (XAMPP), o host vem do base_url via perolaSiteRoot.
const perolaBaseHost = "https://perolajoias.com"

// dockerInternalHost e o nome que o container usa p/ alcancar o host (XAMPP).
const dockerInternalHost = "host.docker.internal"

// perolaPageLimit e o limite por pagina aceito pela API (1..100).
const perolaPageLimit = 100

// perolaMaxPages e um teto de seguranca contra loop infinito caso meta.has_more
// venha sempre true por bug da origem.
const perolaMaxPages = 500

// perolaProduct e o shape de cada item retornado por GET /api/products/.
// categories/campaigns chegam como TEXTO contendo um JSON-array (ex.: "[\"Aneis\"]").
type perolaProduct struct {
	ID         json.Number `json:"id"`
	Name       string      `json:"name"`
	Code       string      `json:"code"`
	Categories string      `json:"categories"`
	Campaigns  string      `json:"campaigns"`
	Image      string      `json:"image"`
	// ImagePath e o caminho relativo REAL da imagem resolvido pela API local
	// (campo aditivo image_path; ex.: "assets/images/products/namorados_26/x.avif").
	// Quando presente, dispensa a heuristica de pasta. Online ainda nao manda.
	ImagePath string      `json:"image_path"`
	Status    string      `json:"status"` // 'active' | 'desactive'
	Stock     json.Number `json:"stock"`
	Fator     json.Number `json:"fator"`
	Price     json.Number `json:"price"`
	DeletedAt *string     `json:"deleted_at"`
}

// perolaMeta e o bloco de paginacao do envelope.
type perolaMeta struct {
	Page    int  `json:"page"`
	Limit   int  `json:"limit"`
	Count   int  `json:"count"`
	Total   int  `json:"total"`
	HasMore bool `json:"has_more"`
}

// perolaEnvelope e o body de GET /api/products/.
type perolaEnvelope struct {
	Data []perolaProduct `json:"data"`
	Meta perolaMeta      `json:"meta"`
}

// ProductSourceClient busca produtos de uma fonte externa e os mapeia para o
// shape de upsert de site.products.
type ProductSourceClient struct {
	httpClient *http.Client
}

// NewProductSourceClient cria o client com timeout sao por request.
func NewProductSourceClient() *ProductSourceClient {
	return &ProductSourceClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// FetchAll percorre todas as paginas de baseURL (limit=100, seguindo
// meta.has_more) e devolve os produtos ja mapeados para upsert.
func (c *ProductSourceClient) FetchAll(ctx context.Context, baseURL string) ([]ProductUpsertItem, error) {
	// O host das imagens segue o base_url da fonte (toggle local<->online): online
	// = https://perolajoias.com; local = http://host.docker.internal/painel-perola.
	host := perolaSiteRoot(baseURL)
	out := make([]ProductUpsertItem, 0)
	for page := 0; page < perolaMaxPages; page++ {
		env, err := c.fetchPage(ctx, baseURL, page)
		if err != nil {
			return nil, err
		}
		for i := range env.Data {
			out = append(out, mapPerolaProductAt(env.Data[i], host))
		}
		if !env.Meta.HasMore || len(env.Data) == 0 {
			break
		}
	}
	return out, nil
}

// perolaSiteRoot deriva a raiz do site (scheme://host[/prefixo]) a partir do
// base_url da API, removendo o sufixo /api/...; e o host usado p/ montar as URLs
// de imagem. Permite alternar a fonte entre online e o XAMPP local so trocando o
// base_url. base_url invalido/vazio -> perolaBaseHost (online, default).
func perolaSiteRoot(baseURL string) string {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return perolaBaseHost
	}
	p := u.Path
	if i := strings.Index(p, "/api"); i >= 0 {
		p = p[:i]
	}
	root := u.Scheme + "://" + u.Host + strings.TrimRight(p, "/")
	return strings.TrimRight(root, "/")
}

// fetchPage faz um GET de uma pagina e decodifica o envelope.
func (c *ProductSourceClient) fetchPage(ctx context.Context, baseURL string, page int) (perolaEnvelope, error) {
	reqURL, err := buildPageURL(baseURL, page, perolaPageLimit)
	if err != nil {
		return perolaEnvelope{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return perolaEnvelope{}, err
	}
	req.Header.Set("Accept", "application/json")
	// Alguns sites de cliente (ex.: Perola) tem WAF que bloqueia o User-Agent
	// padrao do Go ("Go-http-client/...") com 406. Mandamos um UA descritivo.
	req.Header.Set("User-Agent", "OmniSync/1.0 (+https://omni)")
	// Fonte local (XAMPP via host.docker.internal): o index.php da Perola escolhe
	// o banco OFFLINE so quando HTTP_HOST contem "localhost". Forcamos o Host p/ o
	// container alcancar o host e ainda assim a API usar o banco local.
	if u, perr := url.Parse(reqURL); perr == nil && u.Hostname() == dockerInternalHost {
		req.Host = "localhost"
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return perolaEnvelope{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20)) // 8 MB por pagina
	if err != nil {
		return perolaEnvelope{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return perolaEnvelope{}, fmt.Errorf("site: product source returned status %d", resp.StatusCode)
	}
	return parsePerolaEnvelope(body)
}

// parsePerolaEnvelope decodifica o JSON {data:[...], meta:{...}}.
func parsePerolaEnvelope(body []byte) (perolaEnvelope, error) {
	var env perolaEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return perolaEnvelope{}, fmt.Errorf("site: invalid product source payload: %w", err)
	}
	return env, nil
}

// buildPageURL anexa page/limit ao base_url preservando query existente.
func buildPageURL(baseURL string, page, limit int) (string, error) {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", fmt.Errorf("site: invalid product source base_url: %w", err)
	}
	q := u.Query()
	q.Set("page", fmt.Sprintf("%d", page))
	q.Set("limit", fmt.Sprintf("%d", limit))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// mapPerolaProduct converte o shape da Perola para o item de upsert (host online
// default). Wrapper de mapPerolaProductAt usado nos testes.
func mapPerolaProduct(p perolaProduct) ProductUpsertItem {
	return mapPerolaProductAt(p, perolaBaseHost)
}

// mapPerolaProductAt e como mapPerolaProduct, mas com o host das imagens explicito
// (online ou XAMPP local), derivado do base_url da fonte.
func mapPerolaProductAt(p perolaProduct, host string) ProductUpsertItem {
	categories := parseJSONArray(p.Categories)
	campaigns := parseJSONArray(p.Campaigns)
	// Prefere o caminho real (image_path) quando a API resolve no disco; senao cai
	// no nome cru + heuristica de pasta. imgSrc com "/" vira candidata unica.
	imgSrc := p.Image
	if strings.TrimSpace(p.ImagePath) != "" {
		imgSrc = p.ImagePath
	}
	return ProductUpsertItem{
		ExternalID: strings.TrimSpace(p.ID.String()),
		Source:     "perola",
		Name:       p.Name,
		Code:       p.Code,
		Image:      perolaImageURLAt(imgSrc, campaigns, categories, host),
		// Lista ordenada que o cache do sync tenta ANTES de desistir: cobre a
		// pasta `default`, variantes _sm/.avif/.jpg, etc. Espelha a heuristica do
		// painel-perola (crow-notion) que funciona — so que do lado servidor, sem
		// hotlink no browser (a Perola bloqueia o IP via Cloudflare).
		ImageCandidates: perolaImageCandidatesAt(imgSrc, campaigns, categories, host),
		Categories:      categories,
		Campaigns:       campaigns,
		Price:           numberToFloat(p.Price),
		Fator:           fatorOrDefault(p.Fator),
		Stock:           numberToInt(p.Stock),
		Status:          mapPerolaStatus(p.Status),
		Deleted:         p.DeletedAt != nil && strings.TrimSpace(*p.DeletedAt) != "",
	}
}

// perolaImageURL monta a URL absoluta da imagem do produto.
//
// A API da Perola devolve so o NOME do arquivo (ex.: "368252.avif", ja com a
// extensao). No site, a imagem fica em assets/images/products/{segmento}/{arquivo},
// onde segmento = 1a campanha (ou 1a categoria). Heuristica especifica da Perola;
// o ideal e a API devolver a URL pronta (ver painel-perola/docs/AGENT.md).
func perolaImageURL(image string, campaigns, categories []string) string {
	return perolaImageURLAt(image, campaigns, categories, perolaBaseHost)
}

// perolaImageURLAt e como perolaImageURL, mas com o host explicito (online/local).
func perolaImageURLAt(image string, campaigns, categories []string, host string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return ""
	}
	if strings.HasPrefix(image, "http://") || strings.HasPrefix(image, "https://") {
		return image
	}
	// Ja veio com path (contem barra): so absolutiza o host.
	if strings.Contains(image, "/") {
		if strings.HasPrefix(image, "/") {
			return host + image
		}
		return host + "/" + image
	}
	// So o nome do arquivo: monta o caminho do site da Perola.
	segment := firstNonEmptyString(campaigns, categories)
	if segment == "" {
		return host + "/assets/images/products/" + image
	}
	return host + "/assets/images/products/" + segment + "/" + image
}

// firstNonEmptyString devolve o 1o elemento nao-vazio das listas, na ordem dada.
func firstNonEmptyString(lists ...[]string) string {
	for _, list := range lists {
		for _, v := range list {
			if s := strings.TrimSpace(v); s != "" {
				return s
			}
		}
	}
	return ""
}

// perolaImageCandidates monta a lista ORDENADA de URLs candidatas para a imagem
// de um produto, espelhando a heuristica do painel-perola (crow-notion) que
// funciona: a imagem vive em assets/images/products/{segmento}/{arquivo}, com
// segmento = campanha/categoria (ou "default" quando nao ha nenhuma) e o arquivo
// em variantes (_sm.avif, .avif, _sm.<ext>, .jpg...). O cache do sync tenta cada
// uma e fica com a 1a que responder 200 — assim NUNCA dependemos de hotlink no
// browser (a Perola bloqueia o IP do cliente via Cloudflare; ver image_cache.go).
func perolaImageCandidates(image string, campaigns, categories []string) []string {
	return perolaImageCandidatesAt(image, campaigns, categories, perolaBaseHost)
}

// perolaImageCandidatesAt e como perolaImageCandidates, mas com o host explicito
// (online ou XAMPP local), derivado do base_url da fonte.
func perolaImageCandidatesAt(image string, campaigns, categories []string, host string) []string {
	image = strings.TrimSpace(image)
	if image == "" {
		return nil
	}
	// Ja resolvida (http/https) ou veio com path: uma unica candidata absoluta.
	if strings.HasPrefix(image, "http://") || strings.HasPrefix(image, "https://") ||
		strings.Contains(image, "/") {
		return []string{perolaImageURLAt(image, campaigns, categories, host)}
	}
	base := host + "/assets/images/products"
	segments := perolaSegments(campaigns, categories)
	files := perolaFileVariants(image)
	out := make([]string, 0, len(segments)*len(files))
	seen := map[string]struct{}{}
	for _, seg := range segments {
		for _, f := range files {
			u := base + "/" + seg + "/" + f
			if _, ok := seen[u]; !ok {
				seen[u] = struct{}{}
				out = append(out, u)
			}
		}
	}
	return out
}

// perolaSegments devolve as pastas a tentar, na ordem: cada campanha, cada
// categoria e por fim "default" (igual ao fallback do crow-notion quando o
// produto nao tem campanha/categoria). Slug em underscore, sem acentos.
func perolaSegments(campaigns, categories []string) []string {
	out := make([]string, 0, len(campaigns)+len(categories)+1)
	seen := map[string]struct{}{}
	push := func(v string) {
		s := perolaSlug(v)
		if s == "" {
			return
		}
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	for _, c := range campaigns {
		push(c)
	}
	for _, c := range categories {
		push(c)
	}
	push("default")
	return out
}

// perolaFileVariants gera, na ordem de prioridade, os nomes de arquivo a tentar
// para uma imagem cujo nome veio cru da API (ex.: "0278091.webp"). Prioriza a
// thumbnail _sm/.avif (mais leve e o que de fato existe no site da Perola),
// mantendo o nome original e os fallbacks .jpg/.JPG. Espelha buildImageFileVariants
// (modo thumb) do crow-notion.
func perolaFileVariants(file string) []string {
	file = strings.TrimSpace(file)
	if i := strings.IndexAny(file, "?#"); i >= 0 {
		file = file[:i]
	}
	if file == "" {
		return nil
	}
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(file), "."))
	stem := perolaStripSize(strings.TrimSuffix(file, path.Ext(file)))

	out := make([]string, 0, 8)
	seen := map[string]struct{}{}
	push := func(v string) {
		if v == "" {
			return
		}
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	push(file) // exatamente como veio da API
	if stem != "" {
		push(stem + "_sm.avif")
		push(stem + ".avif")
		if ext != "" && ext != "avif" {
			push(stem + "_sm." + ext)
			push(stem + "." + ext)
		}
		push(stem + "_lg.avif")
		push(stem + ".jpg")
		push(stem + ".JPG")
	}
	return out
}

// perolaStripSize remove o sufixo de tamanho (_sm/_md/_lg) do nome base.
func perolaStripSize(name string) string {
	low := strings.ToLower(name)
	for _, suf := range []string{"_sm", "_md", "_lg"} {
		if strings.HasSuffix(low, suf) {
			return name[:len(name)-len(suf)]
		}
	}
	return name
}

// perolaSlug normaliza um segmento como o slug_us do crow-notion: sem acentos,
// espaco/hifen -> "_", colapsa repetidos, minusculo.
func perolaSlug(txt string) string {
	txt = perolaRemoveAccents(strings.TrimSpace(txt))
	txt = strings.Map(func(r rune) rune {
		if r == ' ' || r == '-' {
			return '_'
		}
		return r
	}, txt)
	for strings.Contains(txt, "__") {
		txt = strings.ReplaceAll(txt, "__", "_")
	}
	return strings.ToLower(txt)
}

// perolaRemoveAccents troca os acentos pt-BR pela letra base (mesmo conjunto do
// removerAcentos do crow-notion).
func perolaRemoveAccents(s string) string {
	return perolaAccentReplacer.Replace(s)
}

var perolaAccentReplacer = strings.NewReplacer(
	"á", "a", "à", "a", "ã", "a", "â", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "õ", "o", "ô", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n",
	"Á", "A", "À", "A", "Ã", "A", "Â", "A", "Ä", "A",
	"É", "E", "È", "E", "Ê", "E", "Ë", "E",
	"Í", "I", "Ì", "I", "Î", "I", "Ï", "I",
	"Ó", "O", "Ò", "O", "Õ", "O", "Ô", "O", "Ö", "O",
	"Ú", "U", "Ù", "U", "Û", "U", "Ü", "U",
	"Ç", "C", "Ñ", "N",
)

// parseJSONArray decodifica um texto que contem um JSON-array de strings.
// Tolera valor vazio, JSON invalido (devolve vazio) e itens nao-string.
func parseJSONArray(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return []string{}
	}
	var arr []any
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return []string{}
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		s := strings.TrimSpace(fmt.Sprintf("%v", item))
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// mapPerolaStatus normaliza 'active'/'desactive' para 'active'/'inactive'.
func mapPerolaStatus(status string) string {
	if strings.EqualFold(strings.TrimSpace(status), "active") {
		return string(ProductStatusActive)
	}
	return string(ProductStatusInactive)
}

func numberToFloat(n json.Number) float64 {
	if n == "" {
		return 0
	}
	f, err := n.Float64()
	if err != nil {
		return 0
	}
	return f
}

func numberToInt(n json.Number) int {
	if n == "" {
		return 0
	}
	f, err := n.Float64()
	if err != nil {
		return 0
	}
	return int(math.Round(f))
}

func fatorOrDefault(n json.Number) float64 {
	f := numberToFloat(n)
	if f == 0 {
		return 1
	}
	return f
}
