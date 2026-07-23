package omnichannel

import (
	"context"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

const maxContactAIRestriction = 10 * 365 * 24 * time.Hour

// requireConversationPrivacyManage nao usa requirePermission: essa operacao sensivel
// exige grant RBAC explicito e, por contrato, nao herda bypass de platform_admin.
func (s *Service) requireConversationPrivacyManage(ctx context.Context, accountID string, p auth.Principal) error {
	if accountID == "" || p.UserID == "" {
		return ErrForbidden
	}
	ok, err := s.store.hasEffectivePermission(ctx, accountID, p.UserID, conversationPrivacyManagePermission)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

func (s *Service) HideContactConversation(ctx context.Context, accountID string, p auth.Principal, conversationID string, in HideContactInput) (HiddenContactView, error) {
	if err := s.requireConversationPrivacyManage(ctx, accountID, p); err != nil {
		return HiddenContactView{}, err
	}
	if !validPrivacyID(conversationID) {
		return HiddenContactView{}, ErrNotFound
	}
	row, err := s.store.HideContactByConversation(ctx, accountID, conversationID, p.UserID, in.ClearHistory)
	if err != nil {
		return HiddenContactView{}, privacyNotFound(err)
	}
	return row.HiddenContactView, nil
}

func (s *Service) ListHiddenContacts(ctx context.Context, accountID string, p auth.Principal) ([]HiddenContactView, error) {
	if err := s.requireConversationPrivacyManage(ctx, accountID, p); err != nil {
		return nil, err
	}
	rows, err := s.store.ListHiddenContacts(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]HiddenContactView, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.HiddenContactView)
	}
	return out, nil
}

func (s *Service) RestoreHiddenContact(ctx context.Context, accountID string, p auth.Principal, contactID string) error {
	if err := s.requireConversationPrivacyManage(ctx, accountID, p); err != nil {
		return err
	}
	if !validPrivacyID(contactID) {
		return ErrNotFound
	}
	return privacyNotFound(s.store.RestoreHiddenContact(ctx, accountID, contactID, p.UserID))
}

func (s *Service) GetContactAIRestriction(ctx context.Context, accountID string, p auth.Principal, conversationID string) (ContactAIRestrictionView, error) {
	if err := s.requireConversationPrivacyManage(ctx, accountID, p); err != nil {
		return ContactAIRestrictionView{}, err
	}
	if !validPrivacyID(conversationID) {
		return ContactAIRestrictionView{}, ErrNotFound
	}
	view, err := s.store.GetContactAIRestrictionByConversation(ctx, accountID, conversationID)
	if err != nil {
		return ContactAIRestrictionView{}, privacyNotFound(err)
	}
	return view, nil
}

func (s *Service) UpdateContactAIRestriction(ctx context.Context, accountID string, p auth.Principal, conversationID string, in ContactAIRestrictionInput) (ContactAIRestrictionView, error) {
	if err := s.requireConversationPrivacyManage(ctx, accountID, p); err != nil {
		return ContactAIRestrictionView{}, err
	}
	if !validPrivacyID(conversationID) {
		return ContactAIRestrictionView{}, ErrNotFound
	}
	in.Mode = strings.ToLower(strings.TrimSpace(in.Mode))
	switch in.Mode {
	case "allow", "indefinite":
		in.BlockedUntil = nil
	case "until":
		if in.BlockedUntil == nil {
			return ContactAIRestrictionView{}, ErrInvalidBody
		}
		until := in.BlockedUntil.UTC()
		if !until.After(time.Now().UTC()) || until.After(time.Now().UTC().Add(maxContactAIRestriction)) {
			return ContactAIRestrictionView{}, ErrInvalidBody
		}
		in.BlockedUntil = &until
	default:
		return ContactAIRestrictionView{}, ErrInvalidBody
	}
	view, err := s.store.SetContactAIRestriction(ctx, accountID, conversationID, p.UserID, in)
	if err != nil {
		return ContactAIRestrictionView{}, privacyNotFound(err)
	}
	return view, nil
}
