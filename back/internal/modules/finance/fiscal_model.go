package finance

import "errors"

var (
	ErrFiscalInvalid          = errors.New("finance: invalid fiscal input")
	ErrFiscalConflict         = errors.New("finance: fiscal record changed")
	ErrFiscalNotFound         = errors.New("finance: fiscal record not found")
	ErrFiscalVaultUnavailable = errors.New("finance: fiscal vault unavailable")
)

type FiscalProfile struct {
	LegalName             string `json:"legalName"`
	Document              string `json:"document"`
	IssuerType            string `json:"issuerType"`
	TaxRegime             string `json:"taxRegime"`
	MunicipalRegistration string `json:"municipalRegistration"`
	Municipality          string `json:"municipality"`
	MunicipalityCode      string `json:"municipalityCode"`
	State                 string `json:"state"`
	Email                 string `json:"email"`
	PostalCode            string `json:"postalCode"`
	Street                string `json:"street"`
	Number                string `json:"number"`
	Complement            string `json:"complement"`
	District              string `json:"district"`
	PortalURL             string `json:"portalUrl"`
	PortalLoginMethod     string `json:"portalLoginMethod"`
}

type FiscalProfileView struct {
	Profile              FiscalProfile `json:"profile"`
	UpdatedAt            string        `json:"updatedAt"`
	CredentialsSet       bool          `json:"credentialsSet"`
	CredentialsUpdatedAt string        `json:"credentialsUpdatedAt"`
	MissingFields        []string      `json:"missingFields"`
}

type FiscalProfileInput struct {
	Profile           FiscalProfile `json:"profile"`
	ExpectedUpdatedAt string        `json:"expectedUpdatedAt"`
}

type FiscalCredentialInput struct {
	Username          string `json:"username"`
	Password          string `json:"password"`
	ExpectedUpdatedAt string `json:"expectedUpdatedAt"`
}

type FiscalTemplateFields struct {
	ServiceDescription string   `json:"serviceDescription"`
	ServiceCode        string   `json:"serviceCode"`
	CNAE               string   `json:"cnae"`
	NBS                string   `json:"nbs"`
	ISSRate            *float64 `json:"issRate"`
	ISSWithheld        *bool    `json:"issWithheld"`
	Notes              string   `json:"notes"`
}

type FiscalTemplate struct {
	ID        string               `json:"id"`
	Name      string               `json:"name"`
	Fields    FiscalTemplateFields `json:"fields"`
	IsDefault bool                 `json:"isDefault"`
	UpdatedAt string               `json:"updatedAt"`
}

type FiscalTemplateInput struct {
	Name              string               `json:"name"`
	Fields            FiscalTemplateFields `json:"fields"`
	IsDefault         bool                 `json:"isDefault"`
	ExpectedUpdatedAt string               `json:"expectedUpdatedAt"`
}
