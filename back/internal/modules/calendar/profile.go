package calendar

import (
	"context"
	"strings"
	"time"
)

// ClientProfile e o perfil estrategico de um cliente (schema
// calendar.client_profiles). PK composta (account_id, client_id): a account e a
// dona do calendario (Principal); o clientId e a conta-cliente. Insumo da IA.
// Contrato C3 (docs/CALENDARIO_SPECS.md). Campos livres do brief no `Extra`.
type ClientProfile struct {
	ClientID    string
	Segment     string
	Positioning string
	Description string
	History     string
	SiteURL     string
	Instagram   string
	Address     string
	Objectives  string
	BrandVoice  string
	Extra       ProfileExtra
	UpdatedAt   time.Time
}

// ProfileExtra sao os campos livres do brief (chaves fixas do contrato C3).
type ProfileExtra struct {
	Audience     string `json:"audience"`
	Offer        string `json:"offer"`
	Pillars      string `json:"pillars"`
	Cadence      string `json:"cadence"`
	Restrictions string `json:"restrictions"`
	Performance  string `json:"performance"`
	Assets       string `json:"assets"`
}

// ProfileView e a projecao JSON do perfil (chaves batem 1:1 com o front). Perfil
// inexistente => view com defaults vazios + UpdatedAt zero (o front decide o "vazio").
type ProfileView struct {
	ClientID    string       `json:"clientId"`
	Segment     string       `json:"segment"`
	Positioning string       `json:"positioning"`
	Description string       `json:"description"`
	History     string       `json:"history"`
	SiteURL     string       `json:"siteUrl"`
	Instagram   string       `json:"instagram"`
	Address     string       `json:"address"`
	Objectives  string       `json:"objectives"`
	BrandVoice  string       `json:"brandVoice"`
	Extra       ProfileExtra `json:"extra"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

// view converte para a projecao JSON. ClientProfile e ProfileView tem os mesmos
// campos na mesma ordem (ProfileView so adiciona as tags json), logo a conversao
// direta e valida e evita repetir campo a campo.
func (p ClientProfile) view() ProfileView {
	return ProfileView(p)
}

// ProfileInput e o body do PUT (upsert full-replace). O clientId vem da query, nao
// do body (o body pode traze-lo, mas a query e a autoridade — igual ao account_id).
type ProfileInput struct {
	Segment     string       `json:"segment"`
	Positioning string       `json:"positioning"`
	Description string       `json:"description"`
	History     string       `json:"history"`
	SiteURL     string       `json:"siteUrl"`
	Instagram   string       `json:"instagram"`
	Address     string       `json:"address"`
	Objectives  string       `json:"objectives"`
	BrandVoice  string       `json:"brandVoice"`
	Extra       ProfileExtra `json:"extra"`
}

// ProfileIndexItem e a linha lean do indice de perfis (lista por account).
type ProfileIndexItem struct {
	ClientID  string    `json:"clientId"`
	Filled    bool      `json:"filled"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// emptyProfileView devolve o perfil vazio (perfil inexistente => 200 com defaults,
// nao 404 — perfil e opcional por design, ver C3).
func emptyProfileView(clientID string) ProfileView {
	return ProfileView{ClientID: clientID}
}

// profileStore e a fatia da persistencia que o Service de perfil consome.
type profileStore interface {
	GetClientProfile(ctx context.Context, accountID, clientID string) (ClientProfile, bool, error)
	PutClientProfile(ctx context.Context, accountID string, p ClientProfile, updatedBy string) (ClientProfile, error)
	ListClientProfiles(ctx context.Context, accountID string) ([]ProfileIndexItem, error)
}

// GetClientProfile devolve o perfil do cliente no escopo da account. Cliente sem
// perfil => view vazia (200). clientId invalido ja e barrado no handler.
func (s *Service) GetClientProfile(ctx context.Context, accountID, clientID string) (ProfileView, error) {
	account := strings.TrimSpace(accountID)
	client := normalizeUUID(clientID)
	if client == "" {
		return ProfileView{}, ErrInvalidClient
	}
	p, found, err := s.store.GetClientProfile(ctx, account, client)
	if err != nil {
		return ProfileView{}, err
	}
	if !found {
		return emptyProfileView(client), nil
	}
	return p.view(), nil
}

// PutClientProfile faz upsert (full replace) do perfil. updatedBy = principalLabel.
func (s *Service) PutClientProfile(ctx context.Context, accountID, clientID string, in ProfileInput, updatedBy string) (ProfileView, error) {
	account := strings.TrimSpace(accountID)
	client := normalizeUUID(clientID)
	if client == "" {
		return ProfileView{}, ErrInvalidClient
	}
	p := ClientProfile{
		ClientID:    client,
		Segment:     strings.TrimSpace(in.Segment),
		Positioning: strings.TrimSpace(in.Positioning),
		Description: strings.TrimSpace(in.Description),
		History:     strings.TrimSpace(in.History),
		SiteURL:     strings.TrimSpace(in.SiteURL),
		Instagram:   strings.TrimSpace(in.Instagram),
		Address:     strings.TrimSpace(in.Address),
		Objectives:  strings.TrimSpace(in.Objectives),
		BrandVoice:  strings.TrimSpace(in.BrandVoice),
		Extra:       trimExtra(in.Extra),
	}
	saved, err := s.store.PutClientProfile(ctx, account, p, strings.TrimSpace(updatedBy))
	if err != nil {
		return ProfileView{}, err
	}
	return saved.view(), nil
}

// ListClientProfiles devolve o indice lean dos perfis da account.
func (s *Service) ListClientProfiles(ctx context.Context, accountID string) ([]ProfileIndexItem, error) {
	return s.store.ListClientProfiles(ctx, strings.TrimSpace(accountID))
}

// trimExtra normaliza (trim) os campos livres do brief.
func trimExtra(e ProfileExtra) ProfileExtra {
	return ProfileExtra{
		Audience:     strings.TrimSpace(e.Audience),
		Offer:        strings.TrimSpace(e.Offer),
		Pillars:      strings.TrimSpace(e.Pillars),
		Cadence:      strings.TrimSpace(e.Cadence),
		Restrictions: strings.TrimSpace(e.Restrictions),
		Performance:  strings.TrimSpace(e.Performance),
		Assets:       strings.TrimSpace(e.Assets),
	}
}

// profileFilled indica se algum campo estavel do perfil esta preenchido (define o
// badge "preenchido/vazio" do indice). Extra (brief) NAO conta — sao anotacoes.
func profileFilled(p ClientProfile) bool {
	fields := []string{p.Segment, p.Positioning, p.Description, p.History,
		p.SiteURL, p.Instagram, p.Address, p.Objectives, p.BrandVoice}
	for _, f := range fields {
		if strings.TrimSpace(f) != "" {
			return true
		}
	}
	return false
}
