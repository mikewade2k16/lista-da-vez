package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Curadoria do acervo público (CT-15 fase C). Somente platform_admin chega aqui
// (gate na rota). Coleta gera rascunhos; publicar exige aprovação do dono registrada.

type PublicReviewSummary struct {
	Owner      string `json:"owner"`      // approved | changes_requested | rejected | pending
	AI         int    `json:"aiApproved"` // revisões de IA aprovadas
	AIConcerns int    `json:"aiConcerns"`
	Accountant string `json:"accountant"` // approved | changes_requested | rejected | pending
}

type PublicReviewInput struct {
	ReviewerKind string `json:"reviewerKind"` // owner | ai | accountant
	ReviewerName string `json:"reviewerName"`
	Verdict      string `json:"verdict"` // approved | changes_requested | rejected
	Notes        string `json:"notes"`
}

type PublicReview struct {
	ID           string `json:"id"`
	ReviewerKind string `json:"reviewerKind"`
	ReviewerName string `json:"reviewerName"`
	Verdict      string `json:"verdict"`
	Notes        string `json:"notes"`
	ActorID      string `json:"actorId"`
	CreatedAt    string `json:"createdAt"`
}

type PublicCollection struct {
	ID          string              `json:"id"`
	CatalogKey  string              `json:"catalogKey"`
	Version     int                 `json:"version"`
	Title       string              `json:"title"`
	SourceRef   string              `json:"sourceRef"`
	ContentType string              `json:"contentType"`
	SHA256      string              `json:"sha256"`
	ByteSize    int64               `json:"byteSize"`
	Pages       *int                `json:"pages,omitempty"`
	ChunkCount  int                 `json:"chunkCount"`
	Excluded    int                 `json:"excluded"`
	Status      string              `json:"status"`
	CollectedAt string              `json:"collectedAt"`
	PublishedAt *string             `json:"publishedAt,omitempty"`
	Review      PublicReviewSummary `json:"review"`
}

type PublicCatalogStatus struct {
	Entry       PublicCatalogEntry `json:"entry"`
	LastAttempt *PublicAttempt     `json:"lastAttempt,omitempty"`
	Latest      *PublicCollection  `json:"latest,omitempty"`
	Published   *PublicCollection  `json:"published,omitempty"`
}

type PublicAttempt struct {
	Outcome      string `json:"outcome"` // collected | unchanged | failed
	ErrorCode    string `json:"errorCode,omitempty"`
	CollectionID string `json:"collectionId,omitempty"`
	CreatedAt    string `json:"createdAt"`
}

type PublicChunk struct {
	ID      string `json:"id"`
	Index   int    `json:"index"`
	Page    int    `json:"page"`
	Heading string `json:"heading"`
	Title   string `json:"title"`
	Text    string `json:"text"`
	Status  string `json:"status"`
}

var pdfSoftBreak = regexp.MustCompile(`([^\n])\n([^\n])`)

func publicKindVerdictValid(in PublicReviewInput) bool {
	kind := in.ReviewerKind == "owner" || in.ReviewerKind == "ai" || in.ReviewerKind == "accountant"
	verdict := in.Verdict == "approved" || in.Verdict == "changes_requested" || in.Verdict == "rejected"
	name := strings.TrimSpace(in.ReviewerName)
	return kind && verdict && name != "" && len(name) <= 120 && len(in.Notes) <= 4000
}

// CollectPublic baixa a fonte do catálogo, extrai e fatia o texto e grava uma nova
// versão em rascunho. Mesmo conteúdo (hash) não cria versão. Toda tentativa fica
// registrada, inclusive falhas, com código seguro.
func (s *Service) CollectPublic(ctx context.Context, actor, key string) (PublicAttempt, error) {
	entry, ok := publicCatalogEntry(key)
	if !ok || !idPattern.MatchString(actor) {
		return PublicAttempt{}, fail("not_found", 404)
	}
	attempt := func(outcome, code, collection string) (PublicAttempt, error) {
		return s.store.recordPublicAttempt(ctx, key, outcome, code, collection, actor)
	}
	raw, charset, err := s.fetchPublic(ctx, entry)
	if err != nil {
		return attempt("failed", errorCode(err, "source_fetch_unavailable"), "")
	}
	sum := sha256.Sum256(raw)
	checksum := hex.EncodeToString(sum[:])
	if existing, found, e := s.store.latestPublicByHash(ctx, key, checksum); e != nil {
		return PublicAttempt{}, e
	} else if found {
		return attempt("unchanged", "", existing)
	}
	pages, pageCount, err := s.extractPublic(ctx, entry, raw, charset)
	if err != nil {
		return attempt("failed", errorCode(err, "extraction_failed"), "")
	}
	chunks := chunkPublicPages(pages)
	switch {
	case len(chunks) == 0:
		return attempt("failed", "extraction_empty", "")
	case len(chunks) > publicChunkLimit:
		return attempt("failed", "too_many_chunks", "")
	}
	id, err := s.store.insertPublicCollection(ctx, entry, actor, checksum, int64(len(raw)), pageCount, chunks, time.Now().UTC())
	if err != nil {
		return PublicAttempt{}, err
	}
	return attempt("collected", "", id)
}

func errorCode(err error, fallback string) string {
	var domain *Error
	if errors.As(err, &domain) {
		return domain.Code
	}
	return fallback
}

func (s *Service) fetchPublic(ctx context.Context, entry PublicCatalogEntry) ([]byte, string, error) {
	if s.ports.FetchPublic != nil {
		return s.ports.FetchPublic(ctx, entry)
	}
	got, err := fetchPublicSource(ctx, entry)
	return got.Body, got.Charset, err
}

func (s *Service) extractPublic(ctx context.Context, entry PublicCatalogEntry, raw []byte, charset string) ([]PublicPage, *int, error) {
	if entry.ContentType == "text/html" {
		text := htmlToText(decodeHTMLBytes(raw, charset))
		if len(text) > 4<<20 {
			return nil, nil, fail("extraction_too_large", 422)
		}
		return []PublicPage{{Page: 0, Text: text}}, nil, nil
	}
	if len(raw) < 5 || string(raw[:5]) != "%PDF-" || s.ports.ExtractPublicPDF == nil {
		return nil, nil, fail("extraction_unavailable", 503)
	}
	texts, err := s.ports.ExtractPublicPDF(ctx, raw)
	if err != nil {
		return nil, nil, fail("extraction_failed", 422)
	}
	pages, empty := []PublicPage{}, 0
	for i, text := range texts {
		if strings.TrimSpace(text) == "" {
			empty++
			continue
		}
		pages = append(pages, PublicPage{Page: i + 1, Text: pdfSoftBreak.ReplaceAllString(text, "$1 $2")})
	}
	// Documento digitalizado (sem texto na maioria das páginas) não vira publicação parcial.
	if len(texts) == 0 || empty*2 > len(texts) {
		return nil, nil, fail("extraction_incomplete", 422)
	}
	count := len(texts)
	return pages, &count, nil
}

func (s *Service) PublicCatalogStatus(ctx context.Context) ([]PublicCatalogStatus, error) {
	return s.store.publicCatalogStatus(ctx, PublicCatalog())
}

func (s *Service) PublicChunks(ctx context.Context, collection string, page, limit int) ([]PublicChunk, error) {
	if !idPattern.MatchString(collection) || page < 1 || page > 1000 || limit < 1 || limit > 200 {
		return nil, fail("invalid_input", 400)
	}
	return s.store.publicChunks(ctx, collection, page, limit)
}

func (s *Service) ExcludePublicChunk(ctx context.Context, collection, chunk string) error {
	if !idPattern.MatchString(collection) || !idPattern.MatchString(chunk) {
		return fail("not_found", 404)
	}
	return s.store.excludePublicChunk(ctx, collection, chunk)
}

func (s *Service) ReviewPublicCollection(ctx context.Context, actor, collection string, in PublicReviewInput) (PublicReview, error) {
	if !idPattern.MatchString(collection) || !idPattern.MatchString(actor) {
		return PublicReview{}, fail("not_found", 404)
	}
	if !publicKindVerdictValid(in) {
		return PublicReview{}, fail("invalid_review", 400)
	}
	in.ReviewerName, in.Notes = strings.TrimSpace(in.ReviewerName), strings.TrimSpace(in.Notes)
	return s.store.insertPublicReview(ctx, actor, collection, in)
}

func (s *Service) PublicReviews(ctx context.Context, collection string) ([]PublicReview, error) {
	if !idPattern.MatchString(collection) {
		return nil, fail("not_found", 404)
	}
	return s.store.publicReviews(ctx, collection)
}

// PublishPublicCollection publica os trechos não excluídos e retira a versão
// anterior da mesma fonte na mesma transação. Exige aprovação do dono como última
// decisão do dono; IA e contador ficam registrados e aparecem na resposta do chat.
func (s *Service) PublishPublicCollection(ctx context.Context, actor, collection string) (PublicCollection, error) {
	if !idPattern.MatchString(collection) || !idPattern.MatchString(actor) {
		return PublicCollection{}, fail("not_found", 404)
	}
	return s.store.publishPublicCollection(ctx, actor, collection)
}

func (s *Service) DiscardPublicCollection(ctx context.Context, collection string) error {
	if !idPattern.MatchString(collection) {
		return fail("not_found", 404)
	}
	return s.store.discardPublicCollection(ctx, collection)
}

// ExportPublicCollection gera texto simples para revisão externa (outras IAs,
// contador): cabeçalho com origem/hash e cada trecho numerado com página.
func (s *Service) ExportPublicCollection(ctx context.Context, collection string) (string, error) {
	c, err := s.store.publicCollection(ctx, collection)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s (versão %d)\n\nOrigem: %s\nSHA-256 do original: %s\nColetado em: %s\nTrechos: %d (excluídos: %d)\n\n", c.Title, c.Version, c.SourceRef, c.SHA256, c.CollectedAt, c.ChunkCount, c.Excluded)
	b.WriteString("Instrução ao revisor: compare cada trecho com a fonte original. Aponte texto truncado, fora de ordem, desatualizado ou que não deveria orientar contribuintes. Não é necessário reescrever.\n\n")
	for page := 1; ; page++ {
		chunks, err := s.store.publicChunks(ctx, collection, page, 200)
		if err != nil {
			return "", err
		}
		for _, chunk := range chunks {
			if chunk.Status == "withdrawn" {
				continue
			}
			location := ""
			if chunk.Page > 0 {
				location = fmt.Sprintf(" — página %d", chunk.Page)
			}
			if chunk.Heading != "" {
				location += " — " + chunk.Heading
			}
			fmt.Fprintf(&b, "## Trecho %d%s\n\n%s\n\n", chunk.Index+1, location, chunk.Text)
		}
		if len(chunks) < 200 {
			break
		}
	}
	return b.String(), nil
}
