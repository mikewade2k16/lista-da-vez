package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
	instagram "github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel/instagram"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

const InstagramActionJobKind = "omnichannel.instagram_action"

type InstagramService struct {
	store    *Store
	registry *channel.Registry
	box      *secretbox.Box
}

func NewInstagramService(store *Store, registry *channel.Registry, box *secretbox.Box) *InstagramService {
	return &InstagramService{store: store, registry: registry, box: box}
}

func (s *InstagramService) Configure(ctx context.Context, accountID string, caller Caller, in InstagramAccountInput) (InstagramAccountView, error) {
	if !caller.IsAdmin || strings.TrimSpace(in.IGUserID) == "" || strings.TrimSpace(in.GraphVersion) == "" || strings.TrimSpace(in.AccessToken) == "" || strings.TrimSpace(in.AppSecret) == "" || strings.TrimSpace(in.VerifyToken) == "" {
		return InstagramAccountView{}, ErrInvalidBody
	}
	if !instagram.ValidateGraphVersion(in.GraphVersion) {
		return InstagramAccountView{}, ErrInvalidBody
	}
	if s.box == nil {
		return InstagramAccountView{}, errors.New("omnichannel: secretbox nao inicializado")
	}
	secret, _ := json.Marshal(map[string]string{"accessToken": strings.TrimSpace(in.AccessToken), "appSecret": strings.TrimSpace(in.AppSecret), "verifyToken": strings.TrimSpace(in.VerifyToken)})
	cipher, err := s.box.Encrypt(string(secret))
	if err != nil {
		return InstagramAccountView{}, err
	}
	config := map[string]string{"igUserId": strings.TrimSpace(in.IGUserID), "graphVersion": strings.TrimSpace(in.GraphVersion), "username": strings.TrimSpace(in.Username), "pageId": strings.TrimSpace(in.PageID)}
	return s.store.UpsertInstagramAccount(ctx, accountID, in, config, cipher)
}
func (s *InstagramService) Accounts(ctx context.Context, accountID string, caller Caller) ([]InstagramAccountView, error) {
	if !caller.IsAdmin {
		return nil, ErrForbidden
	}
	return s.store.ListInstagramAccounts(ctx, accountID)
}
func (s *InstagramService) Comments(ctx context.Context, accountID string, caller Caller, accountRef string) ([]InstagramCommentView, error) {
	if !caller.IsAdmin {
		return nil, ErrForbidden
	}
	return s.store.ListInstagramComments(ctx, accountID, accountRef, 100)
}
func (s *InstagramService) Actions(ctx context.Context, accountID string, caller Caller, commentID string) ([]InstagramCommentActionView, error) {
	if !caller.IsAdmin {
		return nil, ErrForbidden
	}
	return s.store.ListInstagramActions(ctx, accountID, commentID)
}
func (s *InstagramService) DecideAction(ctx context.Context, accountID string, caller Caller, commentID, actionID string, in InstagramActionDecisionInput) (InstagramCommentActionView, error) {
	if !caller.IsAdmin {
		return InstagramCommentActionView{}, ErrForbidden
	}
	out, err := s.store.DecideInstagramAction(ctx, accountID, commentID, actionID, caller.UserID, in)
	if err != nil {
		return InstagramCommentActionView{}, err
	}
	if out.Status == "approved" {
		if err := s.store.EnqueueInstagramAction(ctx, accountID, commentID, out.ID); err != nil {
			return InstagramCommentActionView{}, err
		}
	}
	return out, nil
}

type instagramActionJobPayload struct {
	CommentID string `json:"commentId"`
	ActionID  string `json:"actionId"`
}

type InstagramActionHandler struct {
	store    *Store
	registry *channel.Registry
	box      *secretbox.Box
}

func NewInstagramActionHandler(store *Store, registry *channel.Registry, box *secretbox.Box) *InstagramActionHandler {
	return &InstagramActionHandler{store: store, registry: registry, box: box}
}
func (h *InstagramActionHandler) Handle(ctx context.Context, job jobs.Job) error {
	var payload instagramActionJobPayload
	if json.Unmarshal(job.Payload, &payload) != nil || payload.CommentID == "" || payload.ActionID == "" {
		return &jobs.StatusError{Unrecoverable: true, Err: errors.New("instagram action payload invalid")}
	}
	data, err := h.store.ClaimInstagramAction(ctx, job.AccountID, payload.CommentID, payload.ActionID)
	if err != nil {
		return err
	}
	if data.Status == "sent" || data.Status == "ignored" {
		return nil
	}
	provider, config, cipher, err := h.loadCredentialForComment(ctx, job.AccountID, payload.CommentID)
	if err != nil {
		return err
	}
	if h.box == nil || cipher == "" {
		return &jobs.StatusError{StatusCode: 422, Unrecoverable: true, Err: errors.New("instagram credential missing")}
	}
	token, err := h.box.Decrypt(cipher)
	if err != nil {
		return err
	}
	actionProvider, ok := provider.(channel.SocialActionProvider)
	if !ok {
		return &jobs.StatusError{StatusCode: 422, Unrecoverable: true, Err: errors.New("instagram action adapter unavailable")}
	}
	result, err := actionProvider.SendSocialAction(ctx, channel.Credentials{Token: token, Config: config}, channel.SocialAction{Kind: data.ActionKind, ContentID: data.ExternalCommentID, Text: data.Text})
	if err != nil {
		return err
	}
	return h.store.MarkInstagramActionSent(ctx, job.AccountID, payload.ActionID, result.ExternalMessageID)
}

type claimedInstagramAction struct{ Status, ActionKind, ExternalCommentID, Text string }

func (h *InstagramActionHandler) loadCredentialForComment(ctx context.Context, accountID, commentID string) (channel.Provider, map[string]string, string, error) {
	var igID, configRaw, cipher string
	err := h.store.pool.QueryRow(ctx, `select ia.ig_user_id,ia.provider_config,coalesce(ia.credentials_ciphertext,'') from messaging.instagram_comments c join messaging.instagram_accounts ia on ia.account_id=c.account_id and ia.id=c.instagram_account_id where c.account_id=$1::uuid and c.id=$2::uuid`, accountID, commentID).Scan(&igID, &configRaw, &cipher)
	if err != nil {
		return nil, nil, "", err
	}
	p, err := h.registry.Get("instagram")
	return p, decodeStringMap([]byte(configRaw)), cipher, err
}
