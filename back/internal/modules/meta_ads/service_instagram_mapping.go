package metaads

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

var (
	ErrInvalidInstagramIdentity = errors.New("meta_ads: identidade Instagram invalida")
	metaAdsGraphIDRe            = regexp.MustCompile(`^[0-9]{1,64}$`)
)

// ListInstagramIdentities devolve identidades atuais da Graph com os vinculos
// persistidos. Somente a agencia dona de uma conexao direta pode administrar a
// lista; clientes que reutilizam a conexao central recebem 404.
func (s *Service) ListInstagramIdentities(ctx context.Context, accountID string) ([]InstagramIdentityView, error) {
	if err := s.requireDirectAgencyConnection(ctx, accountID); err != nil {
		return nil, err
	}
	accounts, err := s.InstagramAccounts(ctx, accountID)
	if err != nil {
		return nil, err
	}
	mappings, err := s.store.ListInstagramIdentityMappings(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return instagramIdentityViews(accounts, mappings), nil
}

// SetInstagramIdentityClient valida a identidade contra a Graph no mesmo turno
// e deriva page_id da resposta viva. O browser escolhe apenas cliente + igUserId.
func (s *Service) SetInstagramIdentityClient(
	ctx context.Context, accountID, igUserID, clientAccountID string,
) (InstagramIdentityView, error) {
	accountID = strings.TrimSpace(accountID)
	igUserID = strings.TrimSpace(igUserID)
	clientAccountID = strings.TrimSpace(clientAccountID)
	if !metaAdsGraphIDRe.MatchString(igUserID) ||
		(clientAccountID != "" && !metaAdsUUIDRe.MatchString(clientAccountID)) {
		return InstagramIdentityView{}, ErrInvalidInstagramIdentity
	}
	if err := s.requireDirectAgencyConnection(ctx, accountID); err != nil {
		return InstagramIdentityView{}, err
	}
	accounts, err := s.InstagramAccounts(ctx, accountID)
	if err != nil {
		return InstagramIdentityView{}, err
	}
	identity, ok := findInstagramAccount(accounts, igUserID)
	if !ok || !metaAdsGraphIDRe.MatchString(strings.TrimSpace(identity.PageID)) {
		return InstagramIdentityView{}, pgx.ErrNoRows
	}
	if clientAccountID == "" {
		if err = s.store.DeleteInstagramIdentityMapping(ctx, accountID, identity.IGUserID, identity.PageID); err != nil {
			return InstagramIdentityView{}, err
		}
		return toInstagramIdentityView(identity, nil), nil
	}
	allowed, err := s.store.AgencyCanAssignClient(ctx, accountID, clientAccountID)
	if err != nil {
		return InstagramIdentityView{}, err
	}
	if !allowed {
		return InstagramIdentityView{}, pgx.ErrNoRows
	}
	mapping, err := s.store.SetInstagramIdentityClient(
		ctx, accountID, clientAccountID, identity.IGUserID, identity.PageID,
	)
	if err != nil {
		return InstagramIdentityView{}, err
	}
	return toInstagramIdentityView(identity, &mapping.ClientAccountID), nil
}

func (s *Service) requireDirectAgencyConnection(ctx context.Context, accountID string) error {
	connection, err := s.store.GetConnection(ctx, strings.TrimSpace(accountID))
	if noRows(err) {
		return ErrNotConnected
	}
	if err != nil {
		return err
	}
	if connection.Status != connectionActive {
		return ErrNotConnected
	}
	isAgency, err := s.store.AccountIsAgency(ctx, accountID)
	if err != nil {
		return err
	}
	if !isAgency {
		return pgx.ErrNoRows
	}
	return nil
}

func instagramIdentityViews(
	accounts []InstagramAccountView, mappings []InstagramIdentityClientMapping,
) []InstagramIdentityView {
	byIdentity := instagramMappingByIdentity(mappings)
	out := make([]InstagramIdentityView, 0, len(accounts))
	for _, account := range accounts {
		var clientAccountID *string
		if mapping, ok := byIdentity[instagramIdentityKey(account.IGUserID, account.PageID)]; ok {
			clientID := mapping.ClientAccountID
			clientAccountID = &clientID
		}
		out = append(out, toInstagramIdentityView(account, clientAccountID))
	}
	return out
}

func toInstagramIdentityView(account InstagramAccountView, clientAccountID *string) InstagramIdentityView {
	return InstagramIdentityView{
		IGUserID: account.IGUserID, Username: account.Username,
		PageID: account.PageID, PageName: account.PageName,
		ClientAccountID: clientAccountID,
	}
}

func findInstagramAccount(accounts []InstagramAccountView, igUserID string) (InstagramAccountView, bool) {
	for _, account := range accounts {
		if strings.TrimSpace(account.IGUserID) == strings.TrimSpace(igUserID) {
			return account, true
		}
	}
	return InstagramAccountView{}, false
}

func instagramMappingByIdentity(mappings []InstagramIdentityClientMapping) map[string]InstagramIdentityClientMapping {
	out := make(map[string]InstagramIdentityClientMapping, len(mappings))
	for _, mapping := range mappings {
		out[instagramIdentityKey(mapping.IGUserID, mapping.PageID)] = mapping
	}
	return out
}

func instagramIdentityKey(igUserID, pageID string) string {
	return strings.TrimSpace(igUserID) + "\x00" + strings.TrimSpace(pageID)
}
