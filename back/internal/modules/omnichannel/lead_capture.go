package omnichannel

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

type LeadSourceView struct {
	ID              string    `json:"id"`
	Slug            string    `json:"slug"`
	Name            string    `json:"name"`
	Domain          string    `json:"domain"`
	AllowedOrigins  []string  `json:"allowedOrigins"`
	IsActive        bool      `json:"isActive"`
	HasCaptureToken bool      `json:"hasCaptureToken"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type LeadSourceInput struct {
	Slug           string   `json:"slug"`
	Name           string   `json:"name"`
	Domain         string   `json:"domain"`
	AllowedOrigins []string `json:"allowedOrigins"`
	IsActive       *bool    `json:"isActive"`
}

type LeadSourcePatch struct {
	Name           *string   `json:"name"`
	Domain         *string   `json:"domain"`
	AllowedOrigins *[]string `json:"allowedOrigins"`
	IsActive       *bool     `json:"isActive"`
	RotateToken    bool      `json:"rotateToken"`
}

type LeadSourceWriteView struct {
	LeadSourceView
	CaptureToken string `json:"captureToken"`
}

const (
	leadCaptureMaxBody = 64 << 10
	leadCaptureRate    = 60
	leadCaptureWindow  = time.Minute
)

type LeadCaptureInput struct {
	ExternalID    string          `json:"externalId"`
	Name          string          `json:"name"`
	Phone         string          `json:"phone"`
	Email         string          `json:"email"`
	SourceRef     string          `json:"sourceRef"`
	LandingPageID string          `json:"landingPageId"`
	CampaignID    string          `json:"campaignId"`
	UTMSource     string          `json:"utmSource"`
	UTMMedium     string          `json:"utmMedium"`
	UTMCampaign   string          `json:"utmCampaign"`
	UTMTerm       string          `json:"utmTerm"`
	UTMContent    string          `json:"utmContent"`
	Metadata      json.RawMessage `json:"metadata"`
}

type LeadCaptureResult struct {
	Status       string `json:"status"`
	ContactID    string `json:"contactId"`
	TouchpointID string `json:"touchpointId"`
}

func registerLeadCaptureRoutes(mux *http.ServeMux, store *Store, limiter *rateLimiter) {
	mux.HandleFunc("POST /v1/public/omnichannel/leads/{sourceSlug}", handleLeadCapture(store, limiter))
}

func handleLeadCapture(store *Store, limiter *rateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := strings.TrimSpace(r.PathValue("sourceSlug"))
		if slug == "" || len([]rune(slug)) > 80 {
			writeLeadCaptureError(w, r, ErrNotFound)
			return
		}
		if !limiter.allow("lead:"+slug, clientIP(r), leadCaptureRate, leadCaptureWindow) {
			httpapi.WriteError(w, r, http.StatusTooManyRequests, "rate_limited", "Muitas requisicoes. Tente novamente em instantes.")
			return
		}
		if !isJSONContentType(r.Header.Get("Content-Type")) {
			httpapi.WriteError(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type deve ser application/json.")
			return
		}
		var in LeadCaptureInput
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, leadCaptureMaxBody)).Decode(&in); err != nil {
			writeLeadCaptureError(w, r, ErrInvalidBody)
			return
		}
		out, err := store.CaptureLead(r.Context(), slug, strings.TrimSpace(r.Header.Get("X-Omni-Capture-Token")), strings.TrimSpace(r.Header.Get("Origin")), in)
		if err != nil {
			writeLeadCaptureError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusAccepted, out)
	}
}

func (s *Store) CaptureLead(ctx context.Context, slug, token, origin string, in LeadCaptureInput) (LeadCaptureResult, error) {
	slug = strings.TrimSpace(slug)
	token = strings.TrimSpace(token)
	if slug == "" || token == "" {
		return LeadCaptureResult{}, ErrUnauthorized
	}
	if len([]rune(in.ExternalID)) == 0 || len([]rune(in.ExternalID)) > 180 {
		return LeadCaptureResult{}, ErrInvalidBody
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Phone = normalizePhoneDigits(in.Phone)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if in.Phone == "" && in.Email == "" {
		return LeadCaptureResult{}, ErrInvalidBody
	}
	if len([]rune(in.Name)) > 160 || len(in.Phone) > 24 || len([]rune(in.Email)) > 320 {
		return LeadCaptureResult{}, ErrInvalidBody
	}
	if in.Email != "" {
		if _, err := mailParseAddress(in.Email); err != nil {
			return LeadCaptureResult{}, ErrInvalidBody
		}
	}
	metadata, err := validateLeadMetadata(in.Metadata)
	if err != nil {
		return LeadCaptureResult{}, err
	}
	tokenSum := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(tokenSum[:])
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return LeadCaptureResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var accountID, sourceID string
	var allowedOrigins []string
	err = tx.QueryRow(ctx, `select account_id::text, id::text, allowed_origins from messaging.lead_sources where slug=$1 and capture_token_hash=$2 and is_active`, slug, tokenHash).Scan(&accountID, &sourceID, &allowedOrigins)
	if errors.Is(err, pgx.ErrNoRows) {
		return LeadCaptureResult{}, ErrUnauthorized
	}
	if err != nil {
		return LeadCaptureResult{}, err
	}
	if !leadOriginAllowed(origin, allowedOrigins) {
		return LeadCaptureResult{}, ErrUnauthorized
	}
	var existingID, existingContactID string
	err = tx.QueryRow(ctx, `select id::text, contact_id::text from messaging.contact_touchpoints where account_id=$1::uuid and provider='landing_page' and external_event_id=$2`, accountID, in.ExternalID).Scan(&existingID, &existingContactID)
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return LeadCaptureResult{}, err
		}
		return LeadCaptureResult{Status: "duplicate", ContactID: existingContactID, TouchpointID: existingID}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return LeadCaptureResult{}, err
	}
	var contactID string
	lookup := `select id::text from messaging.contacts where account_id=$1::uuid and ((nullif($2,'') is not null and phone=$2) or (nullif($3,'') is not null and lower(primary_email)=lower($3))) order by updated_at desc limit 1 for update`
	err = tx.QueryRow(ctx, lookup, accountID, in.Phone, in.Email).Scan(&contactID)
	if errors.Is(err, pgx.ErrNoRows) {
		name := in.Name
		if name == "" {
			name = firstNonEmpty(in.Phone, in.Email)
		}
		err = tx.QueryRow(ctx, `insert into messaging.contacts (account_id,name,phone,primary_email,source,relationship_status,classification_source,classification_confidence,first_seen_at,last_seen_at,first_channel,last_channel) values ($1::uuid,$2,nullif($3,''),nullif($4,''),'landing:'||$5,'new_lead','rule',1,now(),now(),'LANDING_PAGE','LANDING_PAGE') returning id::text`, accountID, name, in.Phone, in.Email, slug).Scan(&contactID)
	} else if err == nil {
		_, err = tx.Exec(ctx, `update messaging.contacts set name=case when nullif($2,'') is not null then $2 else name end, phone=coalesce(phone,nullif($3,'')), primary_email=coalesce(primary_email,nullif($4,'')), source='landing:'||$5, relationship_status=case when relationship_status in ('customer','inactive') then relationship_status else 'new_lead' end, last_seen_at=now(), last_channel='LANDING_PAGE', updated_at=now() where account_id=$1::uuid and id=$6::uuid`, accountID, in.Name, in.Phone, in.Email, slug, contactID)
	}
	if err != nil {
		return LeadCaptureResult{}, err
	}
	var touchpointID string
	err = tx.QueryRow(ctx, `insert into messaging.contact_touchpoints (account_id,contact_id,channel,provider,external_event_id,source_kind,source_ref,landing_page_id,landing_source_id,campaign_id,utm_source,utm_medium,utm_campaign,utm_term,utm_content,referrer_host,metadata,occurred_at) values ($1::uuid,$2::uuid,'LANDING_PAGE','landing_page',$3,'landing_page',nullif($4,''),nullif($5,''),$6::uuid,nullif($7,''),nullif($8,''),nullif($9,''),nullif($10,''),nullif($11,''),nullif($12,''),nullif($13,''),$14::jsonb,now()) returning id::text`, accountID, contactID, in.ExternalID, in.SourceRef, in.LandingPageID, sourceID, in.CampaignID, in.UTMSource, in.UTMMedium, in.UTMCampaign, in.UTMTerm, in.UTMContent, originHost(origin), metadata).Scan(&touchpointID)
	if err != nil {
		return LeadCaptureResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return LeadCaptureResult{}, err
	}
	return LeadCaptureResult{Status: "accepted", ContactID: contactID, TouchpointID: touchpointID}, nil
}

func (s *Store) ListLeadSources(ctx context.Context, accountID string) ([]LeadSourceView, error) {
	rows, err := s.pool.Query(ctx, `select id::text, slug, name, domain, allowed_origins, is_active,
		capture_token_hash <> '', created_at, updated_at from messaging.lead_sources where account_id=$1::uuid order by name, id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]LeadSourceView, 0)
	for rows.Next() {
		var item LeadSourceView
		if err := rows.Scan(&item.ID, &item.Slug, &item.Name, &item.Domain, &item.AllowedOrigins, &item.IsActive, &item.HasCaptureToken, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) CreateLeadSource(ctx context.Context, accountID string, in LeadSourceInput) (LeadSourceWriteView, error) {
	token, hash, err := newCaptureToken()
	if err != nil {
		return LeadSourceWriteView{}, err
	}
	var out LeadSourceWriteView
	err = s.pool.QueryRow(ctx, `insert into messaging.lead_sources (account_id,slug,name,domain,allowed_origins,is_active,capture_token_hash) values ($1::uuid,$2,$3,$4,$5,$6,$7) returning id::text, slug, name, domain, allowed_origins, is_active, true, created_at, updated_at`, accountID, in.Slug, in.Name, in.Domain, in.AllowedOrigins, boolPtrValue(in.IsActive, true), hash).Scan(&out.ID, &out.Slug, &out.Name, &out.Domain, &out.AllowedOrigins, &out.IsActive, &out.HasCaptureToken, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return LeadSourceWriteView{}, ErrConflict
		}
		return LeadSourceWriteView{}, err
	}
	out.CaptureToken = token
	return out, nil
}

func (s *Store) UpdateLeadSource(ctx context.Context, accountID, id string, patch LeadSourcePatch) (LeadSourceWriteView, error) {
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(id)) {
		return LeadSourceWriteView{}, ErrNotFound
	}
	var token, hash string
	if patch.RotateToken {
		var err error
		token, hash, err = newCaptureToken()
		if err != nil {
			return LeadSourceWriteView{}, err
		}
	}
	var originsArg any
	if patch.AllowedOrigins != nil {
		originsArg = *patch.AllowedOrigins
	}
	var out LeadSourceWriteView
	err := s.pool.QueryRow(ctx, `update messaging.lead_sources set name=coalesce($3,name), domain=coalesce($4,domain), allowed_origins=coalesce($5,allowed_origins), is_active=coalesce($6,is_active), capture_token_hash=case when $7 then $8 else capture_token_hash end, updated_at=now() where account_id=$1::uuid and id=$2::uuid returning id::text, slug, name, domain, allowed_origins, is_active, capture_token_hash <> '', created_at, updated_at`, accountID, id, patch.Name, patch.Domain, originsArg, patch.IsActive, patch.RotateToken, hash).Scan(&out.ID, &out.Slug, &out.Name, &out.Domain, &out.AllowedOrigins, &out.IsActive, &out.HasCaptureToken, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return LeadSourceWriteView{}, ErrNotFound
	}
	if err != nil {
		return LeadSourceWriteView{}, err
	}
	out.CaptureToken = token
	return out, nil
}

func newCaptureToken() (string, string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(buffer)
	sum := sha256.Sum256([]byte(token))
	return token, hex.EncodeToString(sum[:]), nil
}

func boolPtrValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func leadOriginAllowed(origin string, allowed []string) bool {
	origin = strings.TrimSpace(origin)
	if len(allowed) == 0 {
		return origin == ""
	}
	for _, candidate := range allowed {
		if strings.TrimSpace(candidate) == origin {
			return true
		}
	}
	return false
}

func originHost(raw string) string {
	if raw == "" {
		return ""
	}
	if i := strings.Index(raw, "://"); i >= 0 {
		raw = raw[i+3:]
	}
	if i := strings.IndexByte(raw, '/'); i >= 0 {
		raw = raw[:i]
	}
	return strings.ToLower(raw)
}

func validateLeadMetadata(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage(`{}`), nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil || len(object) > 32 {
		return nil, ErrInvalidBody
	}
	for key, value := range object {
		lower := strings.ToLower(key)
		for _, denied := range []string{"password", "token", "secret", "cpf", "credit", "phone", "email"} {
			if strings.Contains(lower, denied) {
				return nil, ErrInvalidBody
			}
		}
		if len(value) > 2048 {
			return nil, ErrInvalidBody
		}
	}
	encoded, err := json.Marshal(object)
	return encoded, err
}

func mailParseAddress(value string) (string, error) {
	if !strings.Contains(value, "@") || strings.ContainsAny(value, "\r\n") {
		return "", errors.New("invalid email")
	}
	return value, nil
}

func writeLeadCaptureError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrUnauthorized):
		httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Token de captura invalido.")
	case errors.Is(err, ErrInvalidBody):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Payload invalido.")
	case errors.Is(err, ErrConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "conflict", "Captura ja processada com outro contato.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao processar a captura.")
	}
}
