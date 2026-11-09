package nfse

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/accounting"
	"io"
	"math/big"
	"regexp"
	"strings"
)

type projection struct {
	Root, DPSID, Key string
	values           map[string]string
}

var xmlMoney = regexp.MustCompile(`^(0|[1-9][0-9]{0,17})(\.[0-9]{1,2})?$`)
var xmlFields = map[string]string{
	"infDPS/tpAmb": "environment", "infDPS/dCompet": "competenceDate", "infDPS/cLocEmi": "municipality",
	"infDPS/prest/CNPJ": "issuer", "infDPS/prest/CPF": "issuer", "infDPS/toma/CNPJ": "customer", "infDPS/toma/CPF": "customer",
	"infDPS/serv/cServ/xDescServ": "description", "infDPS/serv/cServ/cTribNac": "serviceCode", "infDPS/valores/vServPrest/vServ": "amount",
}

func inspectXML(raw []byte) (projection, error) {
	out := projection{values: map[string]string{}}
	if len(raw) == 0 || len(raw) > MaxXMLBytes {
		return out, failure("xml_size_invalid")
	}
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	stack := []string{}
	counts := map[string]int{}
	elements := 0
	roots := 0
	for {
		token, e := decoder.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return out, failure("xml_invalid")
		}
		switch t := token.(type) {
		case xml.Directive:
			return out, failure("xml_directive_forbidden")
		case xml.ProcInst:
			if t.Target != "xml" {
				return out, failure("xml_instruction_forbidden")
			}
		case xml.StartElement:
			elements++
			if elements > 10000 || len(stack) > 32 {
				return out, failure("xml_complexity_invalid")
			}
			if len(stack) == 0 {
				roots++
				out.Root = t.Name.Local
				if roots != 1 || t.Name.Space != Namespace || (out.Root != "DPS" && out.Root != "NFSe") {
					return out, failure("xml_root_invalid")
				}
			}
			if t.Name.Space != Namespace && t.Name.Space != "http://www.w3.org/2000/09/xmldsig#" {
				return out, failure("xml_namespace_invalid")
			}
			stack = append(stack, t.Name.Local)
			path := strings.Join(stack, "/")
			for suffix, key := range xmlFields {
				if strings.HasSuffix(path, "/"+suffix) {
					counts[key]++
					if counts[key] > 1 {
						return out, failure("xml_duplicate_field")
					}
				}
			}
			if t.Name.Space == Namespace && (t.Name.Local == "infDPS" || t.Name.Local == "infNFSe") {
				for _, a := range t.Attr {
					if a.Name.Local == "Id" {
						if t.Name.Local == "infDPS" {
							if out.DPSID != "" {
								return out, failure("xml_duplicate_dps")
							}
							out.DPSID = a.Value
						} else {
							if out.Key != "" || !strings.HasPrefix(a.Value, "NFS") {
								return out, failure("xml_key_invalid")
							}
							out.Key = strings.TrimPrefix(a.Value, "NFS")
						}
					}
				}
			}
		case xml.CharData:
			if len(stack) == 0 {
				if strings.TrimSpace(string(t)) != "" {
					return out, failure("xml_trailing_data")
				}
				continue
			}
			path := strings.Join(stack, "/")
			for suffix, key := range xmlFields {
				if strings.HasSuffix(path, "/"+suffix) {
					out.values[key] += string(t)
				}
			}
		case xml.EndElement:
			if len(stack) == 0 {
				return out, failure("xml_invalid")
			}
			stack = stack[:len(stack)-1]
		}
	}
	if roots != 1 || len(stack) != 0 || !dpsIDPattern.MatchString(out.DPSID) {
		return out, failure("xml_dps_identity_invalid")
	}
	return out, nil
}
func matchesDocument(p projection, cfg Config, m Manifest, d accounting.Document) error {
	if p.DPSID != m.DPSID || p.values["issuer"] != cfg.IssuerDocument || p.values["municipality"] != cfg.MunicipalityCode {
		return failure("xml_issuer_mismatch")
	}
	environment := "2"
	if cfg.Environment == "production" {
		environment = "1"
	}
	if strings.TrimSpace(p.values["environment"]) != environment {
		return failure("xml_environment_mismatch")
	}
	if d.Fields.CompetenceDate == nil || p.values["competenceDate"] != *d.Fields.CompetenceDate || d.Fields.Customer == nil || d.Fields.Customer.Document == nil || p.values["customer"] != *d.Fields.Customer.Document || d.Fields.Service == nil || d.Fields.Service.Description == nil || p.values["description"] != *d.Fields.Service.Description || d.Fields.Service.ServiceCode == nil || p.values["serviceCode"] != *d.Fields.Service.ServiceCode {
		return failure("xml_document_mismatch")
	}
	if !xmlMoney.MatchString(p.values["amount"]) {
		return failure("xml_amount_mismatch")
	}
	if d.Fields.Amount == nil {
		return failure("amount_missing")
	}
	a, ok := new(big.Rat).SetString(p.values["amount"])
	b, bok := new(big.Rat).SetString(*d.Fields.Amount)
	if !ok || !bok || a.Sign() <= 0 || a.Cmp(b) != 0 {
		return failure("xml_amount_mismatch")
	}
	return nil
}
func sha(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
