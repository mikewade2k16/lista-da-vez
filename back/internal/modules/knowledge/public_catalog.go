package knowledge

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PublicCatalogEntry é uma fonte oficial que o coletor pode baixar. A lista é fechada
// no servidor: o painel escolhe a chave, nunca envia URL. Incluir fonte nova exige
// conferir que é oficial, pública (sem login) e aceita download automático.
type PublicCatalogEntry struct {
	Key          string `json:"key"`
	Title        string `json:"title"`
	URL          string `json:"url"`
	ContentType  string `json:"contentType"`
	Authority    string `json:"authority"`
	Jurisdiction string `json:"jurisdiction"`
	Regime       string `json:"regime"`
	Notes        string `json:"notes"`
}

// PublicCatalog é revisado em código (CT-15 fase C, 2026-11-09). Fontes conhecidas e
// fora da lista: FAQ NFS-e v1.2 (exige login gov.br), Resolução CGSN 140 em
// normas.receita (texto carregado por script) e ISS de Aracaju (sem fonte oficial
// pública localizada). Ver docs/accounting/BASE_PUBLICA.md.
func PublicCatalog() []PublicCatalogEntry {
	return []PublicCatalogEntry{
		{Key: "simples-perguntao-mei", Title: "Perguntas e Respostas MEI e SIMEI (Receita Federal)", URL: "https://www8.receita.fazenda.gov.br/SimplesNacional/Arquivos/manual/PerguntaoMEI.pdf", ContentType: "application/pdf", Authority: "Receita Federal / CGSN", Jurisdiction: "BR", Regime: "MEI", Notes: "Documento orientativo; não dispensa a legislação."},
		{Key: "simples-perguntao-sn", Title: "Perguntas e Respostas Simples Nacional (Receita Federal)", URL: "https://www8.receita.fazenda.gov.br/SimplesNacional/Arquivos/manual/perguntaosn.pdf", ContentType: "application/pdf", Authority: "Receita Federal / CGSN", Jurisdiction: "BR", Regime: "Simples Nacional", Notes: "Pode estar em atualização pela Reforma Tributária; conferir data da versão."},
		{Key: "lc-123-2006", Title: "Lei Complementar nº 123/2006 (Estatuto da ME e EPP) — texto do Planalto", URL: "https://www.planalto.gov.br/ccivil_03/leis/lcp/lcp123.htm", ContentType: "text/html", Authority: "Presidência da República", Jurisdiction: "BR", Regime: "Simples Nacional / MEI", Notes: "Texto compilado do Planalto; conferir alterações recentes."},
		{Key: "nfse-guia-emissor-web", Title: "Guia do Emissor Público Nacional Web da NFS-e", URL: "https://www.gov.br/nfse/pt-br/biblioteca/documentacao-tecnica/documentacao-atual/guia-emissorpubliconacionalweb_snnfse-ern-v1-3.pdf", ContentType: "application/pdf", Authority: "Receita Federal / NFS-e", Jurisdiction: "BR", Regime: "NFS-e", Notes: "Guia de uso do emissor web; não cobre regras municipais."},
		{Key: "nfse-manual-api", Title: "Manual de contribuintes — API do Sistema Nacional NFS-e", URL: "https://www.gov.br/nfse/pt-br/biblioteca/documentacao-tecnica/documentacao-atual/manual-contribuintes-emissor-publico-api-sistema-nacional-nfs-e-v1-2-out2025.pdf", ContentType: "application/pdf", Authority: "Receita Federal / NFS-e", Jurisdiction: "BR", Regime: "NFS-e", Notes: "Documentação técnica da API."},
		{Key: "mei-dasn-servico", Title: "Serviço gov.br: declarar receita bruta anual do MEI (DASN-SIMEI)", URL: "https://www.gov.br/pt-br/servicos/declarar-receita-bruta-anual-para-o-mei", ContentType: "text/html", Authority: "GOV.BR", Jurisdiction: "BR", Regime: "MEI", Notes: "Página de serviço; prazos podem ser prorrogados."},
		{Key: "mei-teto-memp", Title: "MEMP: teto do MEI", URL: "https://www.gov.br/memp/pt-br/teto-do-mei", ContentType: "text/html", Authority: "Ministério do Empreendedorismo", Jurisdiction: "BR", Regime: "MEI", Notes: "Conferir data e tramitação legislativa."},
	}
}

func publicCatalogEntry(key string) (PublicCatalogEntry, bool) {
	for _, entry := range PublicCatalog() {
		if entry.Key == key {
			return entry, true
		}
	}
	return PublicCatalogEntry{}, false
}

const publicFetchMaxBytes = 16 << 20

type publicFetched struct {
	Body    []byte
	Charset string
}

// fetchPublicSource reaproveita o dial do coletor CT-12 (HTTPS 443, IP público fixado,
// sem proxy/redirect/compressão) com limites próprios de documentos longos e
// identificação honesta do cliente. Bloqueio (403/login) é falha visível, sem contorno.
func fetchPublicSource(ctx context.Context, entry PublicCatalogEntry) (publicFetched, error) {
	u, err := url.Parse(entry.URL)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.RawQuery != "" || u.Fragment != "" || u.Host == "" {
		return publicFetched{}, fail("source_not_allowed", 422)
	}
	client := newIngestionHTTPClient()
	client.Timeout = 0
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, entry.URL, nil)
	if err != nil {
		return publicFetched{}, fail("source_not_allowed", 422)
	}
	req.Header.Set("Accept", entry.ContentType)
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("User-Agent", "OmniKnowledgeCollector/1.0 (coleta de fontes publicas oficiais)")
	res, err := client.Do(req)
	if err != nil {
		var blocked *Error
		if errors.As(err, &blocked) {
			return publicFetched{}, blocked
		}
		return publicFetched{}, fail("source_fetch_unavailable", 503)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		if res.StatusCode == 429 || res.StatusCode >= 500 {
			return publicFetched{}, fail("source_fetch_unavailable", 503)
		}
		return publicFetched{}, fail("source_http_rejected", 422)
	}
	kind, params, err := mime.ParseMediaType(res.Header.Get("Content-Type"))
	if err != nil || kind != entry.ContentType || res.Header.Get("Content-Encoding") != "" || res.ContentLength > publicFetchMaxBytes {
		return publicFetched{}, fail("source_type_or_size_rejected", 422)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, publicFetchMaxBytes+1))
	if err != nil {
		return publicFetched{}, fail("source_fetch_unavailable", 503)
	}
	if len(raw) == 0 || len(raw) > publicFetchMaxBytes || (res.ContentLength >= 0 && int64(len(raw)) != res.ContentLength) {
		return publicFetched{}, fail("source_type_or_size_rejected", 422)
	}
	return publicFetched{Body: raw, Charset: strings.ToLower(params["charset"])}, nil
}
