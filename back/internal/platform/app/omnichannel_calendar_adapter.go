package app

import (
	"context"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/calendar"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/tenants"
)

// omnichannelClientCatalogAdapter mantem a tela nova na mesma fonte de clientes do
// Calendario (/v1/tenants), sem duplicar a query permission-scoped no Omnichannel.
type omnichannelClientCatalogAdapter struct {
	service *tenants.Service
}

func (a omnichannelClientCatalogAdapter) ListAccessible(ctx context.Context, principal auth.Principal) ([]omnichannel.AutomationClientRef, error) {
	items, err := a.service.ListAccessible(ctx, principal, tenants.ListInput{ModuleID: "omnichannel"})
	if err != nil {
		return nil, err
	}
	out := make([]omnichannel.AutomationClientRef, 0, len(items))
	for _, item := range items {
		out = append(out, omnichannel.AutomationClientRef{ID: item.ID, Slug: item.Slug, Name: item.Name})
	}
	return out, nil
}

// omnichannelCalendarContextAdapter e somente traducao de contrato. A implementacao
// concreta continua no Calendar e o Omnichannel nao importa nem consulta calendar.*.
type omnichannelCalendarContextAdapter struct {
	service func() *calendar.Service
}

func (a omnichannelCalendarContextAdapter) Load(ctx context.Context, accountID, clientID string) (omnichannel.AutomationBusinessContext, bool, error) {
	service := a.service()
	if service == nil {
		return omnichannel.AutomationBusinessContext{ClientID: clientID}, false, nil
	}
	profile, err := service.GetClientProfile(ctx, accountID, clientID)
	if err != nil {
		return omnichannel.AutomationBusinessContext{}, false, err
	}
	return omnichannel.AutomationBusinessContext{
		ClientID: profile.ClientID, Segment: profile.Segment, Positioning: profile.Positioning,
		Description: profile.Description, History: profile.History, SiteURL: profile.SiteURL,
		Instagram: profile.Instagram, Address: profile.Address, Objectives: profile.Objectives,
		BrandVoice: profile.BrandVoice,
		Extra: omnichannel.AutomationBusinessExtra{
			Audience: profile.Extra.Audience, Offer: profile.Extra.Offer, Pillars: profile.Extra.Pillars,
			Cadence: profile.Extra.Cadence, Restrictions: profile.Extra.Restrictions,
			Performance: profile.Extra.Performance, Assets: profile.Extra.Assets,
		},
		UpdatedAt: profile.UpdatedAt,
	}, true, nil
}
