package cardapio

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

// Limites do layout (Fase 3 / Opcao B). Validacao ESTRUTURAL apenas; gating por
// plano e sanitizacao pesada de props/theme sao Fase 4.
const (
	maxLayoutPages   = 20
	maxBlocksPerPage = 100
	maxLayoutBytes   = 256 * 1024
)

// PublicLayout devolve o layout PUBLICADO de um restaurante (por slug). ErrNotFound
// se o restaurante nao existe OU nao ha layout publicado (o site cai no fallback).
func (s *Service) PublicLayout(ctx context.Context, slug string) (json.RawMessage, int64, error) {
	restaurant, accountID, err := s.loadPublicRestaurant(ctx, slug)
	if err != nil {
		return nil, 0, err
	}
	published, version, err := s.store.GetPublishedLayout(ctx, accountID, restaurant.ID)
	if err != nil {
		return nil, 0, mapStoreErr(err) // ErrNoRows -> ErrNotFound
	}
	if isEmptyLayout(published) {
		return nil, 0, ErrNotFound
	}
	return published, version, nil
}

// GetDraftLayout devolve o rascunho do layout (painel). Sem rascunho => layout
// vazio + version 0 (o editor comeca em branco / usa o default do front).
func (s *Service) GetDraftLayout(ctx context.Context, accountID, restaurantID string) (json.RawMessage, int64, error) {
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return nil, 0, mapStoreErr(err)
	}
	draft, version, found, err := s.store.GetDraftLayout(ctx, accountID, restaurantID)
	if err != nil {
		return nil, 0, mapStoreErr(err)
	}
	if !found || isEmptyLayout(draft) {
		return json.RawMessage(`{"pages":{}}`), 0, nil
	}
	return draft, version, nil
}

// SaveDraftLayout valida a estrutura e grava o rascunho (painel). ifMatch (opcional)
// e a version esperada (concorrencia otimista).
func (s *Service) SaveDraftLayout(ctx context.Context, accountID, restaurantID string, body json.RawMessage, ifMatch *int64) (json.RawMessage, int64, error) {
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return nil, 0, mapStoreErr(err)
	}
	canonical, err := validateSiteLayout(body)
	if err != nil {
		return nil, 0, err
	}
	draft, version, err := s.store.PutDraftLayout(ctx, accountID, restaurantID, canonical, ifMatch)
	return draft, version, mapStoreErr(err)
}

// PublishLayout promove o rascunho para publicado (painel). ErrNotFound se nao ha
// rascunho; ErrValidation se o rascunho esta vazio.
func (s *Service) PublishLayout(ctx context.Context, accountID, restaurantID string) (json.RawMessage, int64, error) {
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return nil, 0, mapStoreErr(err)
	}
	published, version, err := s.store.PublishLayout(ctx, accountID, restaurantID)
	if err != nil {
		return nil, 0, mapStoreErr(err) // ErrNoRows -> ErrNotFound (nada para publicar)
	}
	if isEmptyLayout(published) {
		return nil, 0, ErrValidation
	}
	return published, version, nil
}

func isEmptyLayout(raw json.RawMessage) bool {
	t := strings.TrimSpace(string(raw))
	return t == "" || t == "{}" || t == "null"
}

// validateSiteLayout valida a estrutura do layout e devolve a forma canonica (com
// updatedAt do servidor e ids gerados onde faltam). Lenient com chaves extras
// (descartadas no re-marshal). Sanitizacao pesada de props/theme = Fase 4.
func validateSiteLayout(body json.RawMessage) (json.RawMessage, error) {
	if len(body) == 0 || len(body) > maxLayoutBytes {
		return nil, ErrValidation
	}
	var layout SiteLayout
	if err := json.Unmarshal(body, &layout); err != nil {
		return nil, ErrValidation
	}
	if layout.Pages == nil || len(layout.Pages) > maxLayoutPages {
		return nil, ErrValidation
	}
	for name, page := range layout.Pages {
		if strings.TrimSpace(name) == "" {
			return nil, ErrValidation
		}
		if page.Page == "" {
			page.Page = name
		}
		if len(page.Blocks) > maxBlocksPerPage {
			return nil, ErrValidation
		}
		seen := make(map[string]struct{}, len(page.Blocks))
		for i := range page.Blocks {
			b := &page.Blocks[i]
			if strings.TrimSpace(b.Type) == "" {
				return nil, ErrValidation
			}
			if strings.TrimSpace(b.ID) == "" {
				b.ID = genBlockID()
			}
			if _, dup := seen[b.ID]; dup {
				return nil, ErrValidation
			}
			seen[b.ID] = struct{}{}
		}
		layout.Pages[name] = page
	}
	layout.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	out, err := json.Marshal(layout)
	if err != nil {
		return nil, ErrValidation
	}
	return out, nil
}

// genBlockID gera um id de bloco quando o cliente envia vazio.
func genBlockID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "blk"
	}
	return "blk_" + hex.EncodeToString(buf)
}
