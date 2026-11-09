package nfse

import (
	"bytes"
	"encoding/xml"
	"regexp"
	"strings"
	"time"
)

// CT-15 pacote 7 (P15-09): deterministic DPS producer for layout 1.01.
// Every fiscal code is an explicit input from reviewed configuration; nothing is
// inferred from payment, profile name or defaults. Missing or malformed values are
// returned as pending fields and no XML is produced. The DPS identifier comes from
// the canonical sequencer (not implemented yet); only its XSD format is checked.
// Output is unsigned and structurally checked against the official XSD in tests.
// It is not registered in the runtime and does not prove fiscal conformance.

type DPSParty struct {
	CNPJ, CPF, Name string
}

type DPSInput struct {
	Environment         string // TSTipoAmbiente
	DPSID               string // TSIdDPS, from the canonical sequencer
	IssuedAt            time.Time
	AppVersion          string
	Series, Number      string
	CompetenceDate      string // YYYY-MM-DD
	EmitterType         string // TSEmitenteDPS
	IssueMunicipality   string // IBGE, 7 digits
	Provider            DPSParty
	SimplesOption       string // TSOpSimpNac
	SpecialRegime       string // TSRegEspTrib
	Customer            *DPSParty
	ServiceMunicipality string // IBGE, 7 digits
	NationalTaxCode     string // TSCodTribNac, 6 digits
	Description         string
	Amount              string // exact decimal, e.g. "1500.00"
	ISSQNTaxation       string // TSTribISSQN
	ISSQNRetention      string // TSTipoRetISSQN
	TotalTaxIndicator   string // TSTipoIndTotTrib; one of the totTrib options is required
	SimplesTotalPercent string // TSDec2V2
}

var (
	dpsEnum = map[string]map[string]bool{
		"tpAmb": {"1": true, "2": true}, "tpEmit": {"1": true, "2": true, "3": true},
		"opSimpNac": {"1": true, "2": true, "3": true}, "regEspTrib": {"0": true, "1": true, "2": true, "3": true, "4": true, "5": true, "6": true, "9": true},
		"tribISSQN": {"1": true, "2": true, "3": true, "4": true}, "tpRetISSQN": {"1": true, "2": true, "3": true}, "indTotTrib": {"0": true},
	}
	dpsSeries   = regexp.MustCompile(`^0{0,4}\d{1,5}$`)
	dpsNumber   = regexp.MustCompile(`^[1-9][0-9]{0,14}$`)
	dpsMun      = regexp.MustCompile(`^[0-9]{7}$`)
	dpsTribNac  = regexp.MustCompile(`^[0-9]{6}$`)
	dpsCNPJ     = regexp.MustCompile(`^[0-9]{14}$`)
	dpsCPF      = regexp.MustCompile(`^[0-9]{11}$`)
	dpsDec15V2  = regexp.MustCompile(`^(0|0\.[0-9]{2}|[1-9][0-9]{0,14}(\.[0-9]{2})?)$`)
	dpsDec2V2   = regexp.MustCompile(`^(0|0\.[0-9]{2}|[1-9][0-9]?(\.[0-9]{2})?)$`)
	dpsDate     = regexp.MustCompile(`^20[0-9]{2}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])$`)
	dpsIDFormat = regexp.MustCompile(`^DPS[0-9]{42}$`)
)

type dpsXML struct {
	XMLName xml.Name   `xml:"DPS"`
	Xmlns   string     `xml:"xmlns,attr"`
	Versao  string     `xml:"versao,attr"`
	Inf     dpsInfoXML `xml:"infDPS"`
}
type dpsInfoXML struct {
	ID       string        `xml:"Id,attr"`
	TpAmb    string        `xml:"tpAmb"`
	DhEmi    string        `xml:"dhEmi"`
	VerAplic string        `xml:"verAplic"`
	Serie    string        `xml:"serie"`
	NDPS     string        `xml:"nDPS"`
	DCompet  string        `xml:"dCompet"`
	TpEmit   string        `xml:"tpEmit"`
	CLocEmi  string        `xml:"cLocEmi"`
	Prest    dpsPrestXML   `xml:"prest"`
	Toma     *dpsPessoaXML `xml:"toma,omitempty"`
	Serv     dpsServXML    `xml:"serv"`
	Valores  dpsValoresXML `xml:"valores"`
}
type dpsPrestXML struct {
	CNPJ    string `xml:"CNPJ,omitempty"`
	CPF     string `xml:"CPF,omitempty"`
	RegTrib struct {
		OpSimpNac  string `xml:"opSimpNac"`
		RegEspTrib string `xml:"regEspTrib"`
	} `xml:"regTrib"`
}
type dpsPessoaXML struct {
	CNPJ  string `xml:"CNPJ,omitempty"`
	CPF   string `xml:"CPF,omitempty"`
	XNome string `xml:"xNome"`
}
type dpsServXML struct {
	LocPrest struct {
		CLocPrestacao string `xml:"cLocPrestacao"`
	} `xml:"locPrest"`
	CServ struct {
		CTribNac  string `xml:"cTribNac"`
		XDescServ string `xml:"xDescServ"`
	} `xml:"cServ"`
}
type dpsValoresXML struct {
	VServPrest struct {
		VServ string `xml:"vServ"`
	} `xml:"vServPrest"`
	Trib struct {
		TribMun struct {
			TribISSQN  string `xml:"tribISSQN"`
			TpRetISSQN string `xml:"tpRetISSQN"`
		} `xml:"tribMun"`
		TotTrib struct {
			IndTotTrib string `xml:"indTotTrib,omitempty"`
			PTotTribSN string `xml:"pTotTribSN,omitempty"`
		} `xml:"totTrib"`
	} `xml:"trib"`
}

// latin1Text mirrors the XSD TSString range: printable Latin-1 without edge spaces.
func latin1Text(v string, max int) bool {
	if v == "" || v != strings.TrimSpace(v) || len([]rune(v)) > max {
		return false
	}
	for _, r := range v {
		if (r < ' ' && r != '\n') || r > 'ÿ' || (r > '~' && r < 0xA0) {
			return false
		}
	}
	return true
}

func partyPending(prefix string, p DPSParty, needName bool) []string {
	out := []string{}
	if (p.CNPJ == "") == (p.CPF == "") || (p.CNPJ != "" && !dpsCNPJ.MatchString(p.CNPJ)) || (p.CPF != "" && !dpsCPF.MatchString(p.CPF)) {
		out = append(out, prefix+": CNPJ ou CPF (somente dígitos, um deles)")
	}
	if needName && !latin1Text(p.Name, 300) {
		out = append(out, prefix+": nome (até 300 caracteres Latin-1)")
	}
	return out
}

// DPSPending lists every missing or malformed input; empty means BuildDPS can run.
func DPSPending(in DPSInput) []string {
	pending := []string{}
	need := func(ok bool, field string) {
		if !ok {
			pending = append(pending, field)
		}
	}
	need(dpsEnum["tpAmb"][in.Environment], "Ambiente (tpAmb)")
	need(dpsIDFormat.MatchString(in.DPSID), "Identificador da DPS do sequenciador")
	need(!in.IssuedAt.IsZero(), "Data/hora de emissão")
	need(latin1Text(in.AppVersion, 20), "Versão do aplicativo")
	need(dpsSeries.MatchString(in.Series), "Série da DPS")
	need(dpsNumber.MatchString(in.Number), "Número da DPS")
	need(dpsDate.MatchString(in.CompetenceDate), "Competência (aaaa-mm-dd)")
	need(dpsEnum["tpEmit"][in.EmitterType], "Tipo de emitente (tpEmit)")
	need(dpsMun.MatchString(in.IssueMunicipality), "Município emissor (IBGE)")
	pending = append(pending, partyPending("Prestador", in.Provider, false)...)
	need(dpsEnum["opSimpNac"][in.SimplesOption], "Situação no Simples Nacional (opSimpNac)")
	need(dpsEnum["regEspTrib"][in.SpecialRegime], "Regime especial (regEspTrib)")
	if in.Customer != nil {
		pending = append(pending, partyPending("Tomador", *in.Customer, true)...)
	}
	need(dpsMun.MatchString(in.ServiceMunicipality), "Município da prestação (IBGE)")
	need(dpsTribNac.MatchString(in.NationalTaxCode), "Código de tributação nacional")
	need(latin1Text(in.Description, 2000), "Descrição do serviço (até 2000 caracteres Latin-1)")
	need(dpsDec15V2.MatchString(in.Amount) && in.Amount != "0" && in.Amount != "0.00", "Valor do serviço (decimal com ponto)")
	need(dpsEnum["tribISSQN"][in.ISSQNTaxation], "Tributação do ISSQN (tribISSQN)")
	need(dpsEnum["tpRetISSQN"][in.ISSQNRetention], "Retenção do ISSQN (tpRetISSQN)")
	totals := 0
	if in.TotalTaxIndicator != "" {
		totals++
		need(dpsEnum["indTotTrib"][in.TotalTaxIndicator], "Indicador de tributos totais")
	}
	if in.SimplesTotalPercent != "" {
		totals++
		need(dpsDec2V2.MatchString(in.SimplesTotalPercent), "Percentual de tributos do Simples")
	}
	need(totals == 1, "Uma informação de tributos totais (totTrib)")
	return pending
}

// BuildDPS returns canonical unsigned XML or the pending fields.
func BuildDPS(in DPSInput) ([]byte, []string, error) {
	if pending := DPSPending(in); len(pending) > 0 {
		return nil, pending, nil
	}
	doc := dpsXML{Xmlns: Namespace, Versao: "1.01", Inf: dpsInfoXML{
		ID: in.DPSID, TpAmb: in.Environment, DhEmi: in.IssuedAt.Format("2006-01-02T15:04:05-07:00"), VerAplic: in.AppVersion,
		Serie: in.Series, NDPS: in.Number, DCompet: in.CompetenceDate, TpEmit: in.EmitterType, CLocEmi: in.IssueMunicipality,
	}}
	doc.Inf.Prest.CNPJ, doc.Inf.Prest.CPF = in.Provider.CNPJ, in.Provider.CPF
	doc.Inf.Prest.RegTrib.OpSimpNac, doc.Inf.Prest.RegTrib.RegEspTrib = in.SimplesOption, in.SpecialRegime
	if in.Customer != nil {
		doc.Inf.Toma = &dpsPessoaXML{CNPJ: in.Customer.CNPJ, CPF: in.Customer.CPF, XNome: in.Customer.Name}
	}
	doc.Inf.Serv.LocPrest.CLocPrestacao = in.ServiceMunicipality
	doc.Inf.Serv.CServ.CTribNac, doc.Inf.Serv.CServ.XDescServ = in.NationalTaxCode, in.Description
	doc.Inf.Valores.VServPrest.VServ = in.Amount
	doc.Inf.Valores.Trib.TribMun.TribISSQN, doc.Inf.Valores.Trib.TribMun.TpRetISSQN = in.ISSQNTaxation, in.ISSQNRetention
	doc.Inf.Valores.Trib.TotTrib.IndTotTrib, doc.Inf.Valores.Trib.TotTrib.PTotTribSN = in.TotalTaxIndicator, in.SimplesTotalPercent
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	if err := xml.NewEncoder(&buf).Encode(doc); err != nil {
		return nil, nil, err
	}
	return buf.Bytes(), nil, nil
}
