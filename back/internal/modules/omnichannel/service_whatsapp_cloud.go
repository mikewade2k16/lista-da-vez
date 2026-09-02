package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
	meta_whatsapp "github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel/meta_whatsapp"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

type WhatsAppCloudService struct {
	store    *Store
	registry *channel.Registry
	box      *secretbox.Box
}

func NewWhatsAppCloudService(store *Store, registry *channel.Registry, box *secretbox.Box) *WhatsAppCloudService {
	return &WhatsAppCloudService{store: store, registry: registry, box: box}
}

func (s *WhatsAppCloudService) Configure(ctx context.Context, accountID string, caller Caller, instanceID string, in MetaCloudConfigInput) (MetaCloudConfigView, error) {
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(instanceID)) {
		return MetaCloudConfigView{}, ErrForbidden
	}
	if _, err := s.store.RequireInstanceAccess(ctx, accountID, caller.UserID, instanceID,
		"omnichannel.instances.manage", InstanceGrantManage); err != nil {
		return MetaCloudConfigView{}, err
	}
	if strings.TrimSpace(in.WABAID) == "" || strings.TrimSpace(in.PhoneNumberID) == "" ||
		strings.TrimSpace(in.AppID) == "" || !meta_whatsapp.ValidateGraphVersion(in.GraphVersion) ||
		strings.TrimSpace(in.AccessToken) == "" || strings.TrimSpace(in.AppSecret) == "" || strings.TrimSpace(in.VerifyToken) == "" {
		return MetaCloudConfigView{}, ErrInvalidBody
	}
	webhookMode := strings.TrimSpace(in.WebhookMode)
	if webhookMode == "" {
		webhookMode = "waba_override"
	}
	if webhookMode != "waba_override" && webhookMode != "account_callback" {
		return MetaCloudConfigView{}, ErrInvalidBody
	}
	var provider string
	if err := s.store.pool.QueryRow(ctx, `select provider from messaging.whatsapp_instances where account_id=$1::uuid and id=$2::uuid`, accountID, instanceID).Scan(&provider); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MetaCloudConfigView{}, ErrNotFound
		}
		return MetaCloudConfigView{}, err
	}
	if s.box == nil {
		return MetaCloudConfigView{}, errors.New("omnichannel: secretbox nao inicializado")
	}
	secret, err := json.Marshal(map[string]string{"accessToken": strings.TrimSpace(in.AccessToken), "appSecret": strings.TrimSpace(in.AppSecret), "verifyToken": strings.TrimSpace(in.VerifyToken)})
	if err != nil {
		return MetaCloudConfigView{}, err
	}
	ciphertext, err := s.box.Encrypt(string(secret))
	if err != nil {
		return MetaCloudConfigView{}, err
	}
	config := map[string]string{"wabaId": strings.TrimSpace(in.WABAID), "phoneNumberId": strings.TrimSpace(in.PhoneNumberID),
		"businessPortfolioId": strings.TrimSpace(in.BusinessPortfolioID), "appId": strings.TrimSpace(in.AppID),
		"graphVersion": strings.TrimSpace(in.GraphVersion), "webhookMode": webhookMode}
	if err := s.store.SetMetaCloudConfig(ctx, accountID, instanceID, config, ciphertext); err != nil {
		return MetaCloudConfigView{}, err
	}
	return s.store.GetMetaCloudConfigSafe(ctx, accountID, instanceID)
}

func (s *WhatsAppCloudService) GetConfig(ctx context.Context, accountID string, caller Caller, instanceID string) (MetaCloudConfigView, error) {
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(instanceID)) {
		return MetaCloudConfigView{}, ErrForbidden
	}
	if _, err := s.store.RequireInstanceAccess(ctx, accountID, caller.UserID, instanceID,
		"omnichannel.instances.manage", InstanceGrantManage); err != nil {
		return MetaCloudConfigView{}, err
	}
	return s.store.GetMetaCloudConfigSafe(ctx, accountID, instanceID)
}

func (s *WhatsAppCloudService) Templates(ctx context.Context, accountID string, caller Caller, instanceID string) ([]WhatsAppTemplateView, error) {
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(instanceID)) {
		return nil, ErrForbidden
	}
	if _, err := s.store.RequireInstanceAccess(ctx, accountID, caller.UserID, instanceID,
		"omnichannel.instances.manage", InstanceGrantManage); err != nil {
		return nil, err
	}
	return s.store.ListWhatsAppTemplates(ctx, accountID, instanceID)
}

func (s *WhatsAppCloudService) SyncTemplates(ctx context.Context, accountID string, caller Caller, instanceID string) ([]WhatsAppTemplateView, error) {
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(instanceID)) {
		return nil, ErrForbidden
	}
	if _, err := s.store.RequireInstanceAccess(ctx, accountID, caller.UserID, instanceID,
		"omnichannel.instances.manage", InstanceGrantManage); err != nil {
		return nil, err
	}
	provider, config, ciphertext, err := s.loadCredential(ctx, accountID, instanceID)
	if err != nil {
		return nil, err
	}
	templateProvider, ok := provider.(channel.TemplateProvider)
	if !ok {
		return nil, ErrProviderUnsupported
	}
	if s.box == nil || strings.TrimSpace(ciphertext) == "" {
		return nil, ErrInvalidBody
	}
	token, err := s.box.Decrypt(ciphertext)
	if err != nil {
		return nil, err
	}
	items, err := templateProvider.ListTemplates(ctx, channel.Credentials{Token: token, Config: config})
	if err != nil {
		return nil, err
	}
	if err := s.store.UpsertWhatsAppTemplates(ctx, accountID, instanceID, items); err != nil {
		return nil, err
	}
	return s.store.ListWhatsAppTemplates(ctx, accountID, instanceID)
}

func (s *WhatsAppCloudService) loadCredential(ctx context.Context, accountID, instanceID string) (channel.Provider, map[string]string, string, error) {
	var providerKey, ciphertext string
	var raw []byte
	if err := s.store.pool.QueryRow(ctx, `select provider,provider_config,coalesce(credentials_ciphertext,'')
		from messaging.whatsapp_instances where account_id=$1::uuid and id=$2::uuid`, accountID, instanceID).Scan(&providerKey, &raw, &ciphertext); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, "", ErrNotFound
		}
		return nil, nil, "", err
	}
	provider, err := s.registry.Get(providerKey)
	if err != nil {
		return nil, nil, "", ErrProviderUnsupported
	}
	return provider, decodeStringMap(raw), ciphertext, nil
}
