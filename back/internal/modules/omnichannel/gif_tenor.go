package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client Tenor v2 (F12 C2). A chave e da PLATAFORMA e vem do painel/banco (secrets_gif.go),
// NUNCA de env — aqui ela chega ja resolvida (server-side apenas). O mapeamento media_formats
// -> shape do front prefere mp4 a gif (C2). O default do User-Agent do Go e barrado por WAF
// (registro de falhas nº7), entao usamos um proprio. NUNCA logar a URL montada (tem key=) nem
// o payload do Tenor (canonico §10).

const (
	// tenorDefaultBaseURL e a base v2 usada quando o painel nao define uma custom.
	tenorDefaultBaseURL = "https://tenor.googleapis.com/v2"
	// tenorTimeout limita a ida ao Tenor (padrao calendar/ai_models.go). Endpoint travado
	// nao pode segurar o handler.
	tenorTimeout = 10 * time.Second
	// tenorUserAgent identifica o cliente ao Tenor (WAF barra o default do Go).
	tenorUserAgent = "Omni-Omnichannel/1.0 (+https://omni.crowvisuals.com.br)"
	// tenorMediaFilter e o conjunto de formatos pedidos ao Tenor (C2, verbatim).
	tenorMediaFilter = "gif,tinygif,mp4,tinymp4,nanomp4,nanogif"
	// tenorLocale prioriza resultados em pt-BR.
	tenorLocale = "pt_BR"
	// tenorMaxBody limita o corpo decodificado da resposta (defesa contra payload gigante).
	tenorMaxBody = 4 << 20 // 4 MiB
)

// ErrTenorUpstream: o Tenor respondeu erro/rede/timeout, ou o payload veio invalido. O
// handler mapeia para o SOFT-ERROR (HTTP 200 + items:[] + error), nunca 4xx/5xx (C2). O erro
// NUNCA carrega a URL montada (tem key=).
var ErrTenorUpstream = errors.New("omnichannel: consulta ao provedor de GIF falhou")

// GifItem e o shape que o front espera de cada resultado (C2, useInboxChatGifAssets.ts).
type GifItem struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	PreviewURL    string `json:"previewUrl"`
	MediaURL      string `json:"mediaUrl"`
	MimeType      string `json:"mimeType"`
	SourcePageURL string `json:"sourcePageUrl"`
}

// tenorClient chama o Tenor v2. Usa um http.Client comum (timeout): o baseURL e controlado
// pelo platform_admin (nao pelo cliente final), mesmo padrao do calendar/ai_models.go; a
// guarda anti-SSRF fica no proxy /gif/media, que recebe URL do cliente.
type tenorClient struct {
	http *http.Client
}

func newTenorClient() *tenorClient {
	return &tenorClient{http: &http.Client{Timeout: tenorTimeout}}
}

// tenorSearchResponse e a fatia da resposta do Tenor que consumimos.
type tenorSearchResponse struct {
	Results []tenorResult `json:"results"`
}

type tenorResult struct {
	ID                 string                 `json:"id"`
	Title              string                 `json:"title"`
	ContentDescription string                 `json:"content_description"`
	ItemURL            string                 `json:"itemurl"`
	MediaFormats       map[string]tenorFormat `json:"media_formats"`
}

type tenorFormat struct {
	URL string `json:"url"`
}

// Ordens de preferencia (C2). mediaUrl: mp4 antes de gif. previewUrl: tinygif primeiro.
var (
	tenorMP4Order     = []string{"mp4", "tinymp4", "nanomp4", "loopedmp4"}
	tenorGIFOrder     = []string{"gif", "tinygif", "nanogif", "mediumgif"}
	tenorPreviewOrder = []string{"tinygif", "nanogif", "gifpreview", "gif", "mp4"}
)

// search consulta {base}/search do Tenor e mapeia para []GifItem. baseURL vazio => default.
// Falha de rede/timeout/status/payload => ErrTenorUpstream (o handler responde soft-error 200).
// SEGURANCA: nunca loga o endpoint (contem key=) nem o corpo.
func (c *tenorClient) search(ctx context.Context, baseURL, apiKey, query string, limit int) ([]GifItem, error) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = tenorDefaultBaseURL
	}
	params := url.Values{}
	params.Set("q", query)
	params.Set("key", apiKey)
	params.Set("limit", strconv.Itoa(limit))
	params.Set("media_filter", tenorMediaFilter)
	params.Set("locale", tenorLocale)
	endpoint := base + "/search?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		// Nao envolver err: poderia serializar o endpoint (com key=) em log do caller.
		return nil, ErrTenorUpstream
	}
	req.Header.Set("User-Agent", tenorUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, ErrTenorUpstream
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, ErrTenorUpstream
	}

	var payload tenorSearchResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, tenorMaxBody)).Decode(&payload); err != nil {
		return nil, ErrTenorUpstream
	}

	items := make([]GifItem, 0, len(payload.Results))
	for _, res := range payload.Results {
		if item, ok := tenorMapResult(res); ok {
			items = append(items, item)
		}
	}
	return items, nil
}

// tenorMapResult projeta um resultado do Tenor no shape do front. Sem mediaUrl (nenhum
// formato utilizavel) => descartado (ok=false), como o legado (search.get.ts).
func tenorMapResult(res tenorResult) (GifItem, bool) {
	mediaURL, mime := tenorPickMedia(res.MediaFormats)
	if mediaURL == "" {
		return GifItem{}, false
	}
	title := strings.TrimSpace(res.ContentDescription)
	if title == "" {
		title = "GIF"
	}
	return GifItem{
		ID:            strings.TrimSpace(res.ID),
		Title:         title,
		PreviewURL:    tenorPickFormat(res.MediaFormats, tenorPreviewOrder),
		MediaURL:      mediaURL,
		MimeType:      mime,
		SourcePageURL: strings.TrimSpace(res.ItemURL),
	}, true
}

// tenorPickMedia escolhe a mediaUrl preferindo mp4 (mime video/mp4); sem mp4, cai no gif
// (mime image/gif). Sem nenhum => ("", "") e o item e descartado.
func tenorPickMedia(formats map[string]tenorFormat) (string, string) {
	if u := tenorPickFormat(formats, tenorMP4Order); u != "" {
		return u, "video/mp4"
	}
	if u := tenorPickFormat(formats, tenorGIFOrder); u != "" {
		return u, "image/gif"
	}
	return "", ""
}

// tenorPickFormat devolve a 1a URL nao-vazia na ordem dada; "" se nenhuma existe.
func tenorPickFormat(formats map[string]tenorFormat, order []string) string {
	for _, key := range order {
		if f, ok := formats[key]; ok {
			if u := strings.TrimSpace(f.URL); u != "" {
				return u
			}
		}
	}
	return ""
}
