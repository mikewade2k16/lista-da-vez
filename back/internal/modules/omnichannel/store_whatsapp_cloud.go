package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
)

const whatsappTemplateCols = `t.id::text, t.instance_id::text, t.meta_template_id, t.name,
	t.language, t.category, t.status, t.components, t.quality_rating, t.last_synced_at,
	t.created_at, t.updated_at`

func scanWhatsAppTemplate(row rowScanner) (WhatsAppTemplateView, error) {
	var out WhatsAppTemplateView
	err := row.Scan(&out.ID, &out.InstanceID, &out.ExternalID, &out.Name, &out.Language,
		&out.Category, &out.Status, &out.Components, &out.QualityRating, &out.LastSyncedAt,
		&out.CreatedAt, &out.UpdatedAt)
	if len(out.Components) == 0 {
		out.Components = json.RawMessage(`[]`)
	}
	return out, err
}

func (s *Store) ListWhatsAppTemplates(ctx context.Context, accountID, instanceID string) ([]WhatsAppTemplateView, error) {
	rows, err := s.pool.Query(ctx, `select `+whatsappTemplateCols+` from messaging.whatsapp_templates t
		where t.account_id=$1::uuid and t.instance_id=$2::uuid order by t.name,t.language`, accountID, instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]WhatsAppTemplateView, 0)
	for rows.Next() {
		item, scanErr := scanWhatsAppTemplate(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) UpsertWhatsAppTemplates(ctx context.Context, accountID, instanceID string, items []channel.Template) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, item := range items {
		if strings.TrimSpace(item.ExternalID) == "" || strings.TrimSpace(item.Name) == "" || strings.TrimSpace(item.Language) == "" {
			continue
		}
		status := strings.ToUpper(strings.TrimSpace(item.Status))
		if status == "" {
			status = "UNKNOWN"
		}
		switch status {
		case "APPROVED", "PENDING", "REJECTED", "PAUSED", "DISABLED", "UNKNOWN":
		default:
			status = "UNKNOWN"
		}
		components := item.Components
		if len(components) == 0 {
			components = json.RawMessage(`[]`)
		}
		if _, err := tx.Exec(ctx, `insert into messaging.whatsapp_templates
			(account_id,instance_id,meta_template_id,name,language,category,status,components,quality_rating,last_synced_at)
			values ($1::uuid,$2::uuid,$3,$4,$5,$6,$7,$8::jsonb,nullif($9,''),now())
			on conflict (account_id,instance_id,name,language) do update set
				meta_template_id=excluded.meta_template_id, category=excluded.category, status=excluded.status,
				components=excluded.components, quality_rating=excluded.quality_rating, last_synced_at=now(), updated_at=now()`,
			accountID, instanceID, strings.TrimSpace(item.ExternalID), strings.TrimSpace(item.Name), strings.TrimSpace(item.Language),
			strings.ToUpper(strings.TrimSpace(item.Category)), status, components, strings.TrimSpace(item.Quality)); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) SetMetaCloudConfig(ctx context.Context, accountID, instanceID string, config map[string]string, ciphertext string) error {
	raw, err := json.Marshal(config)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `update messaging.whatsapp_instances set provider='meta_whatsapp_cloud',
		provider_config = provider_config || $3::jsonb, credentials_ciphertext = nullif($4,''), updated_at=now()
		where account_id=$1::uuid and id=$2::uuid`, accountID, instanceID, raw, ciphertext)
	return err
}

func (s *Store) GetMetaCloudConfigSafe(ctx context.Context, accountID, instanceID string) (MetaCloudConfigView, error) {
	var out MetaCloudConfigView
	var raw []byte
	var ciphertext *string
	err := s.pool.QueryRow(ctx, `select id::text,provider,provider_config,credentials_ciphertext,updated_at
		from messaging.whatsapp_instances where account_id=$1::uuid and id=$2::uuid and provider='meta_whatsapp_cloud'`, accountID, instanceID).
		Scan(&out.InstanceID, &out.Provider, &raw, &ciphertext, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return MetaCloudConfigView{}, ErrNotFound
	}
	if err != nil {
		return MetaCloudConfigView{}, err
	}
	values := decodeStringMap(raw)
	out.WABAID, out.PhoneNumberID = values["wabaId"], values["phoneNumberId"]
	out.BusinessPortfolioID, out.AppID = values["businessPortfolioId"], values["appId"]
	out.GraphVersion, out.WebhookMode = values["graphVersion"], values["webhookMode"]
	out.CredentialsSet = ciphertext != nil && strings.TrimSpace(*ciphertext) != ""
	return out, nil
}
