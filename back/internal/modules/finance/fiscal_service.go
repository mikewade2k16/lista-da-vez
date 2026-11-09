package finance

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

type FiscalStore interface {
	GetProfile(context.Context, string, string) (FiscalProfileView, error)
	SaveProfile(context.Context, string, string, FiscalProfileInput) (FiscalProfileView, error)
	SaveCredentials(context.Context, string, string, string, string) error
	DeleteCredentials(context.Context, string, string, string) error
	ListTemplates(context.Context, string, string) ([]FiscalTemplate, error)
	SaveTemplate(context.Context, string, string, string, FiscalTemplateInput) (FiscalTemplate, error)
	DeleteTemplate(context.Context, string, string, string, string) error
}

type FiscalService struct {
	store            FiscalStore
	box              *secretbox.Box
	portalProbe      func(context.Context, portalCredentials) PortalCheck
	portalSlots      chan struct{}
	portalHTTPClient func(http.CookieJar) *http.Client
}

func NewFiscalService(store FiscalStore, box *secretbox.Box) *FiscalService {
	return &FiscalService{store: store, box: box, portalProbe: probeNationalPortal, portalSlots: make(chan struct{}, 2), portalHTTPClient: newPortalHTTPClient}
}

func (s *FiscalService) GetProfile(ctx context.Context, account, user string) (FiscalProfileView, error) {
	view, err := s.store.GetProfile(ctx, account, user)
	view.MissingFields = fiscalMissingFields(view.Profile)
	return view, err
}
func (s *FiscalService) SaveProfile(ctx context.Context, account, user string, in FiscalProfileInput) (FiscalProfileView, error) {
	if err := normalizeFiscalProfile(&in.Profile); err != nil {
		return FiscalProfileView{}, err
	}
	view, err := s.store.SaveProfile(ctx, account, user, in)
	view.MissingFields = fiscalMissingFields(view.Profile)
	return view, err
}

func invalidFiscal(message string) error { return fmt.Errorf("%w: %s", ErrFiscalInvalid, message) }

var fiscalDocumentPattern = regexp.MustCompile(`^(?:[0-9]{11}|[A-Z0-9]{12}[0-9]{2})$`)
var fiscalDigitsPattern = regexp.MustCompile(`^[0-9]+$`)

func normalizeFiscalProfile(p *FiscalProfile) error {
	for _, field := range []*string{&p.LegalName, &p.MunicipalRegistration, &p.Municipality, &p.TaxRegime, &p.Street, &p.Number, &p.Complement, &p.District} {
		*field = strings.TrimSpace(*field)
		if len([]rune(*field)) > 240 {
			return invalidFiscal("Campo fiscal excede 240 caracteres.")
		}
	}
	p.Document = strings.ToUpper(strings.NewReplacer(".", "", "/", "", "-", "", " ", "").Replace(strings.TrimSpace(p.Document)))
	if p.Document != "" && !fiscalDocumentPattern.MatchString(p.Document) {
		return invalidFiscal("Informe CPF com 11 dígitos ou CNPJ com 14 caracteres.")
	}
	p.IssuerType = strings.TrimSpace(p.IssuerType)
	switch p.IssuerType {
	case "", "mei", "company", "individual":
	default:
		return invalidFiscal("Tipo de emitente inválido.")
	}
	p.State = strings.ToUpper(strings.TrimSpace(p.State))
	states := " AC AL AP AM BA CE DF ES GO MA MT MS MG PA PB PR PE PI RJ RN RS RO RR SC SP SE TO "
	if p.State != "" && (len(p.State) != 2 || !strings.Contains(states, " "+p.State+" ")) {
		return invalidFiscal("UF inválida.")
	}
	p.MunicipalityCode = strings.TrimSpace(p.MunicipalityCode)
	if p.MunicipalityCode != "" && (len(p.MunicipalityCode) != 7 || !fiscalDigitsPattern.MatchString(p.MunicipalityCode)) {
		return invalidFiscal("Código do município deve ter 7 dígitos.")
	}
	p.PostalCode = strings.ReplaceAll(strings.TrimSpace(p.PostalCode), "-", "")
	if p.PostalCode != "" && (len(p.PostalCode) != 8 || !fiscalDigitsPattern.MatchString(p.PostalCode)) {
		return invalidFiscal("CEP deve ter 8 dígitos.")
	}
	p.Email = strings.TrimSpace(p.Email)
	if p.Email != "" {
		address, err := mail.ParseAddress(p.Email)
		if err != nil || address.Address != p.Email || len(p.Email) > 254 {
			return invalidFiscal("E-mail fiscal inválido.")
		}
	}
	switch p.PortalLoginMethod {
	case "", "portal", "govbr":
	default:
		return invalidFiscal("Forma de acesso ao portal inválida.")
	}
	p.PortalURL = strings.TrimSpace(p.PortalURL)
	if p.PortalURL != "" {
		u, err := url.Parse(p.PortalURL)
		if err == nil && strings.EqualFold(u.Hostname(), "www.nfse.gov.br") && strings.EqualFold(u.Path, "/EmissorNacional/Login") && len(u.Query()) == 1 && strings.EqualFold(u.Query().Get("ReturnUrl"), "/emissornacional") {
			u.RawQuery = ""
			p.PortalURL = u.String()
		}

		if err != nil || len(p.PortalURL) > 1000 || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Port() != "" || net.ParseIP(u.Hostname()) != nil || !strings.Contains(u.Hostname(), ".") || strings.HasSuffix(u.Hostname(), ".local") {
			return invalidFiscal("Informe a URL HTTPS pública do portal, sem credenciais ou parâmetros.")
		}
	}
	return nil
}
func fiscalMissingFields(p FiscalProfile) []string {
	missing := []string{}
	for _, field := range []struct{ name, value string }{{"Nome/razão social", p.LegalName}, {"CPF/CNPJ", p.Document}, {"Tipo de emitente", p.IssuerType}, {"Regime tributário", p.TaxRegime}, {"Município", p.Municipality}, {"UF", p.State}, {"Endereço", p.Street}, {"Número", p.Number}, {"Bairro", p.District}, {"CEP", p.PostalCode}, {"Portal de emissão", p.PortalURL}} {
		if field.value == "" {
			missing = append(missing, field.name)
		}
	}
	return missing
}
func (s *FiscalService) SaveCredentials(ctx context.Context, account, user string, in FiscalCredentialInput) (FiscalProfileView, error) {
	if s.box == nil {
		return FiscalProfileView{}, ErrFiscalVaultUnavailable
	}
	in.Username = strings.TrimSpace(in.Username)
	if in.Username == "" || in.Password == "" || len(in.Username) > 320 || len(in.Password) > 4096 {
		return FiscalProfileView{}, invalidFiscal("Informe usuário e senha do portal nos limites permitidos.")
	}
	raw, err := json.Marshal(struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{in.Username, in.Password})
	if err != nil {
		return FiscalProfileView{}, err
	}
	encrypted, err := s.box.Encrypt(string(raw))
	if err != nil {
		return FiscalProfileView{}, ErrFiscalVaultUnavailable
	}
	if err = s.store.SaveCredentials(ctx, account, user, in.ExpectedUpdatedAt, encrypted); err != nil {
		return FiscalProfileView{}, err
	}
	return s.GetProfile(ctx, account, user)
}
func (s *FiscalService) DeleteCredentials(ctx context.Context, account, user, expected string) (FiscalProfileView, error) {
	if err := s.store.DeleteCredentials(ctx, account, user, expected); err != nil {
		return FiscalProfileView{}, err
	}
	return s.GetProfile(ctx, account, user)
}
func (s *FiscalService) ListTemplates(ctx context.Context, account, user string) ([]FiscalTemplate, error) {
	return s.store.ListTemplates(ctx, account, user)
}
func (s *FiscalService) SaveTemplate(ctx context.Context, account, user, id string, in FiscalTemplateInput) (FiscalTemplate, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len([]rune(in.Name)) > 100 {
		return FiscalTemplate{}, invalidFiscal("Informe um nome de template com até 100 caracteres.")
	}
	in.Fields.ServiceDescription = strings.TrimSpace(in.Fields.ServiceDescription)
	if in.Fields.ServiceDescription == "" || len([]rune(in.Fields.ServiceDescription)) > 4000 {
		return FiscalTemplate{}, invalidFiscal("Informe a descrição do serviço com até 4000 caracteres.")
	}
	for _, field := range []*string{&in.Fields.ServiceCode, &in.Fields.CNAE, &in.Fields.NBS} {
		*field = strings.TrimSpace(*field)
		if len(*field) > 60 {
			return FiscalTemplate{}, invalidFiscal("Código de serviço excede 60 caracteres.")
		}
	}
	in.Fields.Notes = strings.TrimSpace(in.Fields.Notes)
	if len([]rune(in.Fields.Notes)) > 2000 {
		return FiscalTemplate{}, invalidFiscal("Observação excede 2000 caracteres.")
	}
	if in.Fields.ISSRate != nil && (*in.Fields.ISSRate < 0 || *in.Fields.ISSRate > 100) {
		return FiscalTemplate{}, invalidFiscal("Percentual deve estar entre 0 e 100.")
	}
	if id != "" && !isUUID(id) {
		return FiscalTemplate{}, ErrFiscalNotFound
	}
	return s.store.SaveTemplate(ctx, account, user, id, in)
}
func (s *FiscalService) DeleteTemplate(ctx context.Context, account, user, id, expected string) error {
	if !isUUID(id) {
		return ErrFiscalNotFound
	}
	return s.store.DeleteTemplate(ctx, account, user, id, expected)
}
