package finance

import (
	"context"
	"strings"
)

func (s *AccessConversationService) saveConversationIssuerField(ctx context.Context, account, user, field, value string) error {
	view, err := s.fiscal.GetProfile(ctx, account, user)
	if err != nil {
		return err
	}
	p := view.Profile
	switch field {
	case "Nome/razão social":
		p.LegalName = value
	case "CPF/CNPJ":
		p.Document = value
	case "Tipo de emitente":
		switch normalizedDocumentLabel(value) {
		case "mei":
			p.IssuerType = "mei"
		case "empresa", "nao mei", "company":
			p.IssuerType = "company"
		case "pessoa fisica", "individual":
			p.IssuerType = "individual"
		default:
			return invalidFiscal("Informe MEI, empresa ou pessoa física.")
		}
	case "Regime tributário":
		p.TaxRegime = value
	case "Município":
		p.Municipality = value
	case "UF":
		p.State = strings.ToUpper(value)
	case "Endereço":
		p.Street = value
	case "Número":
		p.Number = value
	case "Bairro":
		p.District = value
	case "CEP":
		p.PostalCode = value
	case "Portal de emissão":
		p.PortalURL = value
	default:
		return invalidFiscal("Campo cadastral indisponível para edição na conversa.")
	}
	_, err = s.fiscal.SaveProfile(ctx, account, user, FiscalProfileInput{Profile: p, ExpectedUpdatedAt: view.UpdatedAt})
	return err
}
