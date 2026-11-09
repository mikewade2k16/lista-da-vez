package nfse

import (
	"context"
	"encoding/json"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/accounting"
)

func (c *Connector) Submit(ctx context.Context, sc accounting.Scope, sub accounting.Submission) (accounting.Outcome, error) {
	unknown := accounting.Outcome{State: "unknown"}
	cfg, _, e := c.configuration(ctx, sc)
	if e != nil {
		return unknown, e
	}
	cap, e := c.Capability(ctx, sc)
	if e != nil || !cap.CanSubmit || sub.Environment != cfg.Environment {
		return unknown, failure("submission_unavailable")
	}
	if !uuidPattern.MatchString(sub.ExecutionID) || sub.RemoteIdentity != sub.ExecutionID || !checksumPattern.MatchString(sub.SnapshotHash) || (sub.Document.State != "submitting" && sub.Document.State != "authorized_for_submission") {
		return unknown, failure("motor_authorization_required")
	}
	manifest, e := c.prepared(ctx, sc, cfg, sub.Document)
	if e != nil {
		return unknown, e
	}
	xml, e := c.ports.ReadSignedXML(ctx, sc, manifest)
	if e != nil {
		return unknown, failure("signed_dps_unavailable")
	}
	if len(xml) == 0 || len(xml) > MaxXMLBytes {
		return unknown, failure("signed_dps_size_invalid")
	}
	if sha(xml) != manifest.SHA256 {
		return unknown, failure("signed_dps_hash_mismatch")
	}
	if e = c.ports.ValidateSchema(ctx, "dps", cfg.SchemaVersion, xml); e != nil {
		return unknown, failure("dps_schema_invalid")
	}
	if e = c.ports.VerifySignature(ctx, "dps", xml); e != nil {
		return unknown, failure("dps_signature_invalid")
	}
	projection, e := inspectXML(xml)
	if e != nil {
		return unknown, e
	}
	if projection.Root != "DPS" {
		return unknown, failure("dps_root_invalid")
	}
	if e = matchesDocument(projection, cfg, manifest, sub.Document); e != nil {
		return unknown, e
	}
	compressed, e := compressXML(xml)
	if e != nil {
		return unknown, e
	}
	body, e := json.Marshal(struct {
		XML string `json:"dpsXmlGZipB64"`
	}{compressed})
	if e != nil {
		return unknown, failure("payload_invalid")
	}
	latest, activeCredential, refreshError := c.configuration(ctx, sc)
	if refreshError != nil || latest != cfg {
		return unknown, failure("configuration_changed_before_submit")
	}
	credential := activeCredential
	raw, status, e := c.request(ctx, cfg, credential, "POST", "/nfse", body)
	if e != nil {
		return unknown, e
	}
	response, e := decodeWire(raw)
	if e != nil {
		return unknown, e
	}
	if status == 400 || status == 422 {
		if response.Key == "" && response.XML == "" && len(response.Errors) > 0 {
			for _, problem := range response.Errors {
				if problem.Code == "" {
					return unknown, failure("rejection_unconfirmed")
				}
			}
			return accounting.Outcome{State: "rejected"}, nil
		}
	}
	if status < 200 || status >= 300 {
		return unknown, failure("remote_result_unknown")
	}
	if _, e = c.validatedResult(ctx, cfg, manifest, sub.Document, response); e != nil {
		return unknown, e
	}
	return accounting.Outcome{State: "authorized", RemoteReference: &response.Key}, nil
}
func (c *Connector) lookup(ctx context.Context, sc accounting.Scope, sub accounting.Submission) (wireResponse, []byte, error) {
	var response wireResponse
	cfg, credential, e := c.configuration(ctx, sc)
	if e != nil {
		return response, nil, e
	}
	if sub.Environment != cfg.Environment {
		return response, nil, failure("environment_mismatch")
	}
	manifest, e := c.prepared(ctx, sc, cfg, sub.Document)
	if e != nil {
		return response, nil, e
	}
	// Even 404 is inconclusive: a previous POST may still be completing.
	raw, status, e := c.request(ctx, cfg, credential, "GET", "/dps/"+manifest.DPSID, nil)
	if e != nil {
		return response, nil, e
	}
	if status != 200 {
		return response, nil, failure("lookup_inconclusive")
	}
	response, e = decodeWire(raw)
	if e != nil || !accessKeyPattern.MatchString(response.Key) {
		return response, nil, failure("lookup_inconclusive")
	}
	key := response.Key
	raw, status, e = c.request(ctx, cfg, credential, "GET", "/nfse/"+key, nil)
	if e != nil {
		return response, nil, e
	}
	if status != 200 {
		return response, nil, failure("lookup_inconclusive")
	}
	response, e = decodeWire(raw)
	if e != nil {
		return response, nil, e
	}
	if response.Key != key {
		return response, nil, failure("lookup_key_mismatch")
	}
	xml, e := c.validatedResult(ctx, cfg, manifest, sub.Document, response)
	return response, xml, e
}
func (c *Connector) Lookup(ctx context.Context, sc accounting.Scope, sub accounting.Submission) (accounting.Outcome, error) {
	response, _, e := c.lookup(ctx, sc, sub)
	if e != nil {
		return accounting.Outcome{State: "unknown"}, e
	}
	return accounting.Outcome{State: "authorized", RemoteReference: &response.Key}, nil
}
func (c *Connector) FetchArtifacts(ctx context.Context, sc accounting.Scope, sub accounting.Submission) ([]accounting.Artifact, error) {
	response, xml, e := c.lookup(ctx, sc, sub)
	if e != nil {
		return nil, e
	}
	// No invented fiscal protocol/PDF. Receipt is explicitly a local technical reference.
	receipt, e := json.Marshal(map[string]string{"kind": "sefin_technical_receipt", "source": "sefin_nacional", "dpsId": response.DPSID, "accessKey": response.Key, "environment": sub.Environment})
	if e != nil {
		return nil, failure("receipt_invalid")
	}
	return []accounting.Artifact{{Kind: "xml", Content: xml, SHA256: sha(xml)}, {Kind: "protocol", Content: receipt, SHA256: sha(receipt)}}, nil
}
