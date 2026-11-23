package tools

import (
	"context"
	"crypto/rand"
	"regexp"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/stringsx"
)

// ShortLinkStore abstrai a persistencia de links curtos.
type ShortLinkStore interface {
	// List: accountID "" = todas as contas (so alcancado por platform_admin).
	List(ctx context.Context, accountID string, f ListFilter) ([]ShortLinkItem, int, error)
	Create(ctx context.Context, accountID, slug, targetURL string) (ShortLinkItem, error)
	// Get: accountID "" = por id (admin); senao id + account_id (isolamento).
	Get(ctx context.Context, id, accountID string) (ShortLinkItem, error)
	// Update: patch parcial. accountID "" = por id (admin); senao id + account_id.
	Update(ctx context.Context, id, accountID string, p ShortLinkPatch) (ShortLinkItem, error)
	// Delete: accountID "" = por id (admin); senao id + account_id (isolamento).
	Delete(ctx context.Context, id, accountID string) error
	// Resolve devolve o destino do redirect publico /s/{slug} se o link esta
	// ativo; count=true tambem incrementa hits/last_hit_at.
	Resolve(ctx context.Context, slug string, count bool) (string, error)
}

// QrCodeStore abstrai a persistencia de QR codes.
type QrCodeStore interface {
	List(ctx context.Context, accountID string, f ListFilter) ([]QrCodeItem, int, error)
	Create(ctx context.Context, accountID string, rec QrCodeRecord) (QrCodeItem, error)
	Get(ctx context.Context, id, accountID string) (QrCodeItem, error)
	// Update aplica o patch; com ExpectFill/ExpectBack, so grava se as cores
	// atuais ainda forem essas (senao ErrConcurrentChange).
	Update(ctx context.Context, id, accountID string, p QrCodePatch) (QrCodeItem, error)
	Delete(ctx context.Context, id, accountID string) error
	// Resolve devolve o destino se is_active (redirect publico /q/{slug});
	// count=true tambem incrementa scan_count/last_scanned_at.
	Resolve(ctx context.Context, slug string, count bool) (string, error)
}

// Service concentra as regras de negocio das duas ferramentas.
type Service struct {
	shortLinks ShortLinkStore
	qrCodes    QrCodeStore
	publicBase string // base absoluta dos links /s e /q (sem barra final)
}

// NewService injeta os stores e a base publica dos redirects.
func NewService(shortLinks ShortLinkStore, qrCodes QrCodeStore, publicBase string) *Service {
	return &Service{
		shortLinks: shortLinks,
		qrCodes:    qrCodes,
		publicBase: strings.TrimRight(strings.TrimSpace(publicBase), "/"),
	}
}

// ============================================================================
// Normalizacao (espelha o slugify/normalize do mock antigo)
// ============================================================================

var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// normalizeSlug deriva um slug valido; vazio quando a entrada nao tem base (o
// store gera um codigo aleatorio nesse caso). Max 80.
func normalizeSlug(raw string) string {
	s := stringsx.Slugify(raw)
	if len([]rune(s)) > 80 {
		s = string([]rune(s)[:80])
	}
	return s
}

// normalizeTargetURL faz trim e prefixa https:// quando falta esquema. Vazio
// devolve "" (o service rejeita com ErrInvalidTargetURL).
func normalizeTargetURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if len([]rune(s)) > 2000 {
		s = string([]rune(s)[:2000])
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return s
	}
	return "https://" + s
}

// normalizeColor valida #rrggbb (lower). Vazio usa o fallback (so no create);
// qualquer outro valor invalido e erro — nunca default silencioso.
func normalizeColor(raw, fallback string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" && fallback != "" {
		return fallback, nil
	}
	if colorPattern.MatchString(s) {
		return strings.ToLower(s), nil
	}
	return "", ErrInvalidColor
}

// normalizeSize limita 120..1000; invalido/zero cai no fallback.
func normalizeSize(v, fallback int) int {
	if v == 0 {
		return fallback
	}
	if v < 120 {
		return 120
	}
	if v > 1000 {
		return 1000
	}
	return v
}

// randomCode gera um codigo curto [a-z0-9] (fallback de slug) com crypto/rand.
func randomCode(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand nao deve falhar; fallback deterministico so por seguranca.
		return strings.Repeat("x", n)
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

// clampPage normaliza page/limit da listagem.
func clampPage(f *ListFilter) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 {
		f.Limit = 120
	}
	if f.Limit > 500 {
		f.Limit = 500
	}
}

// buildMeta calcula o bloco de paginacao.
func buildMeta(f ListFilter, total int) ListMeta {
	totalPages := 0
	if f.Limit > 0 {
		totalPages = (total + f.Limit - 1) / f.Limit
	}
	return ListMeta{
		Page:       f.Page,
		Limit:      f.Limit,
		Total:      total,
		TotalPages: totalPages,
		HasMore:    f.Page < totalPages,
	}
}

// withShortURL preenche ShortURL absoluto a partir do slug.
func (s *Service) withShortURL(it ShortLinkItem) ShortLinkItem {
	it.ShortURL = s.publicBase + "/s/" + it.Slug
	return it
}

// withQrURL preenche QrURL absoluto a partir do slug.
func (s *Service) withQrURL(it QrCodeItem) QrCodeItem {
	it.QrURL = s.publicBase + "/q/" + it.Slug
	return it
}

// ============================================================================
// Short links
// ============================================================================

// ListShortLinks lista os links da account (ou de todas, quando accountID "").
func (s *Service) ListShortLinks(ctx context.Context, accountID string, f ListFilter) ([]ShortLinkItem, ListMeta, error) {
	clampPage(&f)
	f.Q = strings.TrimSpace(f.Q)
	f.Status = strings.ToLower(strings.TrimSpace(f.Status))
	items, total, err := s.shortLinks.List(ctx, accountID, f)
	if err != nil {
		return nil, ListMeta{}, err
	}
	for i := range items {
		items[i] = s.withShortURL(items[i])
	}
	return items, buildMeta(f, total), nil
}

// CreateShortLink normaliza e cria um link na account informada.
func (s *Service) CreateShortLink(ctx context.Context, accountID string, in ShortLinkInput) (ShortLinkItem, error) {
	if strings.TrimSpace(accountID) == "" {
		return ShortLinkItem{}, ErrAccountRequired
	}
	targetURL := ""
	if in.TargetURL != nil {
		targetURL = normalizeTargetURL(*in.TargetURL)
	}
	if targetURL == "" {
		return ShortLinkItem{}, ErrInvalidTargetURL
	}
	slug := ""
	if in.Slug != nil {
		slug = normalizeSlug(*in.Slug)
	}
	item, err := s.shortLinks.Create(ctx, accountID, slug, targetURL)
	if err != nil {
		return ShortLinkItem{}, err
	}
	return s.withShortURL(item), nil
}

// UpdateShortLink aplica um patch parcial (404 se fora do escopo).
//   - slug igual ao atual = no-op; vazio = ErrInvalidSlug;
//   - slug diferente exige confirmSlugChange=true (o link antigo para de
//     funcionar) — senao ErrSlugChangeNotConfirmed. Colisao ganha sufixo -2/-3.
//   - isActive liga/desliga o redirect sem excluir.
func (s *Service) UpdateShortLink(ctx context.Context, id, accountID string, in ShortLinkInput) (ShortLinkItem, error) {
	p := ShortLinkPatch{IsActive: in.IsActive}
	if in.TargetURL != nil {
		target := normalizeTargetURL(*in.TargetURL)
		if target == "" {
			return ShortLinkItem{}, ErrInvalidTargetURL
		}
		p.TargetURL = &target
	}
	if in.Slug != nil {
		slug := normalizeSlug(*in.Slug)
		if slug == "" {
			return ShortLinkItem{}, ErrInvalidSlug
		}
		current, err := s.shortLinks.Get(ctx, id, accountID)
		if err != nil {
			return ShortLinkItem{}, err
		}
		if slug != current.Slug {
			if in.ConfirmSlugChange == nil || !*in.ConfirmSlugChange {
				return ShortLinkItem{}, ErrSlugChangeNotConfirmed
			}
			p.Slug = &slug
		}
	}
	item, err := s.shortLinks.Update(ctx, id, accountID, p)
	if err != nil {
		return ShortLinkItem{}, err
	}
	return s.withShortURL(item), nil
}

// DeleteShortLink remove o link (404 se fora do escopo).
func (s *Service) DeleteShortLink(ctx context.Context, id, accountID string) error {
	return s.shortLinks.Delete(ctx, id, accountID)
}

// ResolveShortLink devolve o destino do redirect publico; count=true conta o
// clique (o handler decide: robo de previa/HEAD nao conta).
func (s *Service) ResolveShortLink(ctx context.Context, slug string, count bool) (string, error) {
	slug = normalizeSlug(slug)
	if slug == "" {
		return "", ErrShortLinkNotFound
	}
	return s.shortLinks.Resolve(ctx, slug, count)
}

// ============================================================================
// QR codes
// ============================================================================

// ListQrCodes lista os QR da account (ou de todas, quando accountID "").
func (s *Service) ListQrCodes(ctx context.Context, accountID string, f ListFilter) ([]QrCodeItem, ListMeta, error) {
	clampPage(&f)
	f.Q = strings.TrimSpace(f.Q)
	f.Status = strings.ToLower(strings.TrimSpace(f.Status))
	items, total, err := s.qrCodes.List(ctx, accountID, f)
	if err != nil {
		return nil, ListMeta{}, err
	}
	for i := range items {
		items[i] = s.withQrURL(items[i])
	}
	return items, buildMeta(f, total), nil
}

// CreateQrCode normaliza e cria um QR na account informada.
func (s *Service) CreateQrCode(ctx context.Context, accountID string, in QrCodeInput) (QrCodeItem, error) {
	if strings.TrimSpace(accountID) == "" {
		return QrCodeItem{}, ErrAccountRequired
	}
	target := ""
	if in.TargetURL != nil {
		target = normalizeTargetURL(*in.TargetURL)
	}
	if target == "" {
		return QrCodeItem{}, ErrInvalidTargetURL
	}
	slug := ""
	if in.Slug != nil {
		slug = normalizeSlug(*in.Slug)
	}
	isActive := true
	if in.IsActive != nil {
		isActive = *in.IsActive
	}
	fill, err := normalizeColor(strPtr(in.FillColor), "#000000")
	if err != nil {
		return QrCodeItem{}, err
	}
	back, err := normalizeColor(strPtr(in.BackColor), "#ffffff")
	if err != nil {
		return QrCodeItem{}, err
	}
	if err := validateQrColors(fill, back); err != nil {
		return QrCodeItem{}, err
	}
	rec := QrCodeRecord{
		Slug:      slug,
		TargetURL: target,
		FillColor: fill,
		BackColor: back,
		Size:      normalizeSize(intPtr(in.Size), 220),
		IsActive:  isActive,
	}
	item, err := s.qrCodes.Create(ctx, accountID, rec)
	if err != nil {
		return QrCodeItem{}, err
	}
	return s.withQrURL(item), nil
}

// UpdateQrCode aplica um patch parcial (404 se fora do escopo).
//   - slug e imutavel: o QR impresso codifica /q/{slug}. Slug igual ao atual e
//     aceito (no-op, telas antigas mandam o form inteiro); diferente/vazio =
//     ErrQrSlugImmutable.
//   - cores: valida o par efetivo (patch + valor atual) e grava so se as cores
//     atuais nao mudaram no meio (ErrConcurrentChange).
func (s *Service) UpdateQrCode(ctx context.Context, id, accountID string, in QrCodeInput) (QrCodeItem, error) {
	p := QrCodePatch{IsActive: in.IsActive}
	if in.TargetURL != nil {
		target := normalizeTargetURL(*in.TargetURL)
		if target == "" {
			return QrCodeItem{}, ErrInvalidTargetURL
		}
		p.TargetURL = &target
	}
	colorsTouched := in.FillColor != nil || in.BackColor != nil
	if in.Slug != nil || colorsTouched {
		current, err := s.qrCodes.Get(ctx, id, accountID)
		if err != nil {
			return QrCodeItem{}, err
		}
		if in.Slug != nil && normalizeSlug(*in.Slug) != current.Slug {
			return QrCodeItem{}, ErrQrSlugImmutable
		}
		if colorsTouched {
			fill, back := current.FillColor, current.BackColor
			if in.FillColor != nil {
				if fill, err = normalizeColor(*in.FillColor, ""); err != nil {
					return QrCodeItem{}, err
				}
			}
			if in.BackColor != nil {
				if back, err = normalizeColor(*in.BackColor, ""); err != nil {
					return QrCodeItem{}, err
				}
			}
			if err := validateQrColors(fill, back); err != nil {
				return QrCodeItem{}, err
			}
			p.FillColor, p.BackColor = &fill, &back
			p.ExpectFill, p.ExpectBack = current.FillColor, current.BackColor
		}
	}
	if in.Size != nil {
		size := normalizeSize(*in.Size, 220)
		p.Size = &size
	}
	item, err := s.qrCodes.Update(ctx, id, accountID, p)
	if err != nil {
		return QrCodeItem{}, err
	}
	return s.withQrURL(item), nil
}

// DeleteQrCode remove o QR (404 se fora do escopo).
func (s *Service) DeleteQrCode(ctx context.Context, id, accountID string) error {
	return s.qrCodes.Delete(ctx, id, accountID)
}

// ResolveQrCode devolve o destino do redirect publico se ativo; count=true
// conta o scan.
func (s *Service) ResolveQrCode(ctx context.Context, slug string, count bool) (string, error) {
	slug = normalizeSlug(slug)
	if slug == "" {
		return "", ErrQrCodeNotFound
	}
	return s.qrCodes.Resolve(ctx, slug, count)
}

// strPtr desreferencia *string opcional para "".
func strPtr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// intPtr desreferencia *int opcional para 0.
func intPtr(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
