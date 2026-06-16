package bio

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Erros de dominio do modulo. Fora-do-escopo e nao-encontrado colapsam em
// ErrNotFound (404) para nao vazar existencia de recurso de outra account.
var (
	ErrNotFound     = errors.New("bio: not found")
	ErrInvalidSlug  = errors.New("bio: invalid slug")
	ErrSlugTaken    = errors.New("bio: slug already in use")
	ErrInvalidName  = errors.New("bio: invalid name")
	ErrPublishEmpty = errors.New("bio: publish missing required fields")
	ErrForbidden    = errors.New("bio: forbidden")
)

// bioStore e a fatia da persistencia que o Service consome. *Store satisfaz a
// interface; testes injetam um fake sem banco.
type bioStore interface {
	List(ctx context.Context, f ListFilter) ([]BioSummary, error)
	GetByID(ctx context.Context, id, accountID string) (Bio, error)
	Create(ctx context.Context, accountID, slug, name string) (Bio, error)
	CreateWithDraft(ctx context.Context, accountID, slug, name string, draft json.RawMessage) (Bio, error)
	Patch(ctx context.Context, id string, name, slug, accountID *string, draft *json.RawMessage) (Bio, error)
	Publish(ctx context.Context, id string, published json.RawMessage) (Bio, error)
	Unpublish(ctx context.Context, id string) (Bio, error)
	Delete(ctx context.Context, id, accountID string) error
	SlugExists(ctx context.Context, slug, excludeID string) (bool, error)
	EnsureBioModuleEnabled(ctx context.Context, accountID string) error
	AccountExists(ctx context.Context, accountID string) (bool, error)
	PublicLookup(ctx context.Context, slug string) (data json.RawMessage, accountID string, err error)
	GetDefaults(ctx context.Context) (BioDefaults, error)
	PutDefaults(ctx context.Context, data json.RawMessage) (BioDefaults, error)
	InsertMedia(ctx context.Context, accountID, bioID, kind, path, mime string, size int64) (Media, error)
}

// Service implementa as regras do modulo bio.
type Service struct {
	store      bioStore
	publicBase string                   // PUBLIC_API_BASE_URL — absolutiza midia no endpoint publico
	sources    map[string]ProductSource // fontes de produto plugaveis (B7), por type
}

// NewService cria o Service. publicBase pode ser vazio (midia sai relativa). As
// fontes de produto sao registradas depois via RegisterSource.
func NewService(store bioStore, publicBase string) *Service {
	return &Service{
		store:      store,
		publicBase: strings.TrimSpace(publicBase),
		sources:    map[string]ProductSource{},
	}
}

// RegisterSource registra uma fonte de produtos plugavel sob o seu type
// (ex.: SourceTypeSiteProducts). type/src vazios sao ignorados.
func (s *Service) RegisterSource(sourceType string, src ProductSource) {
	sourceType = strings.TrimSpace(sourceType)
	if sourceType == "" || src == nil {
		return
	}
	s.sources[sourceType] = src
}

// ============================================================================
// Escopo
// ============================================================================

// resolveScope devolve o accountID efetivo do filtro. Para nao-admin, qualquer
// accountID de query diferente do contexto e rejeitado como ErrNotFound (nao
// vaza existencia). Para admin, requested vazio = todas; requested preenchido =
// filtro pela account pedida.
func resolveScope(isAdmin bool, contextAccountID, requestedAccountID string) (string, error) {
	requested := strings.TrimSpace(requestedAccountID)
	ctxAccount := strings.TrimSpace(contextAccountID)

	if isAdmin {
		return requested, nil
	}
	if ctxAccount == "" {
		return "", ErrNotFound
	}
	if requested != "" && requested != ctxAccount {
		return "", ErrNotFound
	}
	return ctxAccount, nil
}

// ============================================================================
// Listagem e leitura
// ============================================================================

// List devolve a projecao lean das bios visiveis para o Principal.
func (s *Service) List(ctx context.Context, isAdmin bool, contextAccountID string, f ListFilter) ([]BioSummary, error) {
	scope, err := resolveScope(isAdmin, contextAccountID, f.AccountID)
	if err != nil {
		return nil, err
	}
	f.AccountID = scope
	return s.store.List(ctx, f)
}

// Get devolve o detalhe completo de uma bio dentro do escopo permitido.
func (s *Service) Get(ctx context.Context, isAdmin bool, contextAccountID, id string) (BioView, error) {
	scope := scopeForLookup(isAdmin, contextAccountID)
	b, err := s.store.GetByID(ctx, id, scope)
	if err != nil {
		return BioView{}, mapNotFound(err)
	}
	return b.view(), nil
}

// Preview devolve o JSON mesclado do DRAFT (defaults + data_draft) para previa
// no painel. Midia NAO e absolutizada (painel usa relativo).
func (s *Service) Preview(ctx context.Context, isAdmin bool, contextAccountID, id string) (json.RawMessage, error) {
	scope := scopeForLookup(isAdmin, contextAccountID)
	b, err := s.store.GetByID(ctx, id, scope)
	if err != nil {
		return nil, mapNotFound(err)
	}
	defaults, err := s.store.GetDefaults(ctx)
	if err != nil {
		return nil, err
	}
	return deepMerge(defaults.Data, normalizeRaw(b.DataDraft))
}

// ============================================================================
// Mutacoes
// ============================================================================

// Create cria uma bio. Cliente OPCIONAL: para nao-admin, ignora req.AccountID e
// usa o contexto; para admin sem accountId, usa tambem o contexto (a agencia).
// Slug vazio e derivado do name (com sufixo numerico se colidir).
func (s *Service) Create(ctx context.Context, isAdmin bool, contextAccountID string, req CreateRequest) (BioView, error) {
	accountID := strings.TrimSpace(contextAccountID)
	if isAdmin && strings.TrimSpace(req.AccountID) != "" {
		accountID = strings.TrimSpace(req.AccountID)
	}
	if accountID == "" {
		return BioView{}, ErrForbidden
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return BioView{}, ErrInvalidName
	}

	slug, err := s.resolveCreateSlug(ctx, req.Slug, name)
	if err != nil {
		return BioView{}, err
	}

	// Garante o modulo bio habilitado na account ANTES de criar — senao a bio
	// publicada nunca apareceria no endpoint publico (que exige o modulo).
	if err := s.store.EnsureBioModuleEnabled(ctx, accountID); err != nil {
		return BioView{}, err
	}

	b, err := s.store.Create(ctx, accountID, slug, name)
	if err != nil {
		return BioView{}, err
	}
	return b.view(), nil
}

// resolveCreateSlug decide o slug do Create: se o request traz slug, valida e
// exige unicidade (erro se colidir). Se vazio, deriva do name e garante
// unicidade com sufixo numerico.
func (s *Service) resolveCreateSlug(ctx context.Context, rawSlug, name string) (string, error) {
	if strings.TrimSpace(rawSlug) != "" {
		slug, err := normalizeSlug(rawSlug)
		if err != nil {
			return "", err
		}
		taken, err := s.store.SlugExists(ctx, slug, "")
		if err != nil {
			return "", err
		}
		if taken {
			return "", ErrSlugTaken
		}
		return slug, nil
	}
	base := slugify(name)
	if base == "" {
		return "", ErrInvalidSlug
	}
	return s.uniqueSlug(ctx, base)
}

// Duplicate cria uma copia (status draft) da bio origem dentro do escopo
// permitido, copiando o data_draft (NAO o published). name = "Copia de {name}",
// slug unico derivado de "{slug}-copia". Admin pode duplicar para outra account
// via req.AccountID; vazio = mesma account da origem.
func (s *Service) Duplicate(ctx context.Context, isAdmin bool, contextAccountID, id string, req DuplicateRequest) (BioView, error) {
	scope := scopeForLookup(isAdmin, contextAccountID)
	src, err := s.store.GetByID(ctx, id, scope)
	if err != nil {
		return BioView{}, mapNotFound(err)
	}

	targetAccount := src.AccountID
	if isAdmin && strings.TrimSpace(req.AccountID) != "" {
		targetAccount = strings.TrimSpace(req.AccountID)
		if err := s.requireAccountExists(ctx, targetAccount); err != nil {
			return BioView{}, err
		}
	}

	slug, err := s.uniqueSlug(ctx, src.Slug+"-copia")
	if err != nil {
		return BioView{}, err
	}
	name := "Copia de " + src.Name

	if err := s.store.EnsureBioModuleEnabled(ctx, targetAccount); err != nil {
		return BioView{}, err
	}

	b, err := s.store.CreateWithDraft(ctx, targetAccount, slug, name, normalizeRaw(src.DataDraft))
	if err != nil {
		return BioView{}, err
	}
	return b.view(), nil
}

// Patch atualiza nome, slug, draft e/ou account de uma bio no escopo permitido.
// Mover de account (req.AccountID) so e honrado para platform_admin; nao-admin
// que mandar accountId tem o campo IGNORADO (nunca troca de account).
func (s *Service) Patch(ctx context.Context, isAdmin bool, contextAccountID, id string, req PatchRequest) (BioView, error) {
	scope := scopeForLookup(isAdmin, contextAccountID)
	current, err := s.store.GetByID(ctx, id, scope)
	if err != nil {
		return BioView{}, mapNotFound(err)
	}

	namePtr, err := validatePatchName(req.Name)
	if err != nil {
		return BioView{}, err
	}
	slugPtr, err := s.validatePatchSlug(ctx, req.Slug, current.ID)
	if err != nil {
		return BioView{}, err
	}
	accountPtr, err := s.validatePatchAccount(ctx, isAdmin, req.AccountID, current.AccountID)
	if err != nil {
		return BioView{}, err
	}

	b, err := s.store.Patch(ctx, current.ID, namePtr, slugPtr, accountPtr, req.DataDraft)
	if err != nil {
		return BioView{}, mapNotFound(err)
	}
	return b.view(), nil
}

// Publish copia draft->published apos validar campos minimos no JSON mesclado
// com defaults.
func (s *Service) Publish(ctx context.Context, isAdmin bool, contextAccountID, id string) (BioView, error) {
	scope := scopeForLookup(isAdmin, contextAccountID)
	current, err := s.store.GetByID(ctx, id, scope)
	if err != nil {
		return BioView{}, mapNotFound(err)
	}

	defaults, err := s.store.GetDefaults(ctx)
	if err != nil {
		return BioView{}, err
	}
	merged, err := deepMerge(defaults.Data, normalizeRaw(current.DataDraft))
	if err != nil {
		return BioView{}, err
	}
	// Minimos para publicar: logo + um fundo (video OU imagem).
	hasLogo := jsonHasNonEmptyPath(merged, "branding", "logo", "srcMobile")
	hasBackground := jsonHasNonEmptyPath(merged, "video", "bgVideo") ||
		jsonHasNonEmptyPath(merged, "video", "bgImage")
	if !hasLogo || !hasBackground {
		return BioView{}, ErrPublishEmpty
	}

	b, err := s.store.Publish(ctx, current.ID, normalizeRaw(current.DataDraft))
	if err != nil {
		return BioView{}, mapNotFound(err)
	}
	return b.view(), nil
}

// Unpublish volta a bio para draft.
func (s *Service) Unpublish(ctx context.Context, isAdmin bool, contextAccountID, id string) (BioView, error) {
	scope := scopeForLookup(isAdmin, contextAccountID)
	current, err := s.store.GetByID(ctx, id, scope)
	if err != nil {
		return BioView{}, mapNotFound(err)
	}
	b, err := s.store.Unpublish(ctx, current.ID)
	if err != nil {
		return BioView{}, mapNotFound(err)
	}
	return b.view(), nil
}

// Delete remove a bio no escopo permitido.
func (s *Service) Delete(ctx context.Context, isAdmin bool, contextAccountID, id string) error {
	scope := scopeForLookup(isAdmin, contextAccountID)
	if err := s.store.Delete(ctx, id, scope); err != nil {
		return mapNotFound(err)
	}
	return nil
}

// ============================================================================
// Defaults
// ============================================================================

// Defaults retorna a config global (so platform_admin no handler).
func (s *Service) Defaults(ctx context.Context) (BioDefaults, error) {
	return s.store.GetDefaults(ctx)
}

// SaveDefaults faz upsert da config global.
func (s *Service) SaveDefaults(ctx context.Context, data json.RawMessage) (BioDefaults, error) {
	if !json.Valid(data) {
		return BioDefaults{}, ErrInvalidName
	}
	return s.store.PutDefaults(ctx, data)
}

// ============================================================================
// Midia
// ============================================================================

// RegisterMedia grava o metadado do upload em bio.media.
func (s *Service) RegisterMedia(ctx context.Context, accountID, bioID, kind, path, mime string, size int64) error {
	_, err := s.store.InsertMedia(ctx, accountID, bioID, kind, path, mime, size)
	return err
}

// ============================================================================
// Helpers
// ============================================================================

// scopeForLookup devolve o accountID usado como filtro nas queries por id. Admin
// passa "" (sem filtro de account); nao-admin passa o contexto (defesa em
// profundidade — query nao casa bio de outra account => ErrNoRows => 404).
func scopeForLookup(isAdmin bool, contextAccountID string) string {
	if isAdmin {
		return ""
	}
	return strings.TrimSpace(contextAccountID)
}

func validatePatchName(name *string) (*string, error) {
	if name == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*name)
	if trimmed == "" {
		return nil, ErrInvalidName
	}
	return &trimmed, nil
}

func (s *Service) validatePatchSlug(ctx context.Context, slug *string, bioID string) (*string, error) {
	if slug == nil {
		return nil, nil
	}
	normalized, err := normalizeSlug(*slug)
	if err != nil {
		return nil, err
	}
	taken, err := s.store.SlugExists(ctx, normalized, bioID)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrSlugTaken
	}
	return &normalized, nil
}

// validatePatchAccount resolve o ponteiro de account para o Patch. Apenas
// platform_admin pode mover de account; nao-admin (ou admin sem accountId, ou
// account igual a atual) => nil (nao altera). Valida a existencia do destino.
func (s *Service) validatePatchAccount(ctx context.Context, isAdmin bool, accountID *string, currentAccountID string) (*string, error) {
	if !isAdmin || accountID == nil {
		return nil, nil
	}
	target := strings.TrimSpace(*accountID)
	if target == "" || target == currentAccountID {
		return nil, nil
	}
	if err := s.requireAccountExists(ctx, target); err != nil {
		return nil, err
	}
	return &target, nil
}

// requireAccountExists devolve ErrNotFound quando a account destino nao existe
// (a FK ja protege; este check da um 404 limpo antes do update).
func (s *Service) requireAccountExists(ctx context.Context, accountID string) error {
	exists, err := s.store.AccountExists(ctx, accountID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}

// mapNotFound colapsa pgx.ErrNoRows em ErrNotFound (404). Outros erros passam.
func mapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
