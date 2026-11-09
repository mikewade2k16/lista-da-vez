package nfse

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/accounting"
	"io"
	"net/http"
	"time"
)

func compressXML(raw []byte) (string, error) {
	var out bytes.Buffer
	w := gzip.NewWriter(&out)
	if _, e := w.Write(raw); e != nil {
		return "", failure("compression_failed")
	}
	if e := w.Close(); e != nil {
		return "", failure("compression_failed")
	}
	return base64.StdEncoding.EncodeToString(out.Bytes()), nil
}
func expandXML(encoded string) ([]byte, error) {
	if len(encoded) > maxResponseBytes {
		return nil, failure("response_size_invalid")
	}
	compressed, e := base64.StdEncoding.DecodeString(encoded)
	if e != nil {
		return nil, failure("response_encoding_invalid")
	}
	reader, e := gzip.NewReader(bytes.NewReader(compressed))
	if e != nil {
		return nil, failure("response_encoding_invalid")
	}
	defer func() { _ = reader.Close() }()
	raw, e := io.ReadAll(io.LimitReader(reader, MaxXMLBytes+1))
	if e != nil || len(raw) > MaxXMLBytes {
		return nil, failure("response_size_invalid")
	}
	return raw, nil
}
func (c *Connector) request(ctx context.Context, cfg Config, credential Credential, method, path string, body []byte) ([]byte, int, error) {
	environment := cfg.Environment
	if c.simulated && environment == "fake_local" {
		environment = "homologation"
	}
	base, e := baseURL(environment)
	if e != nil {
		return nil, 0, e
	}
	req, e := http.NewRequestWithContext(ctx, method, base+path, bytes.NewReader(body))
	if e != nil {
		return nil, 0, failure("request_invalid")
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	transport := c.transport
	if transport == nil {
		transport = &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{credential.Certificate}, RootCAs: credential.Roots}, DisableCompression: true, MaxConnsPerHost: 2, ResponseHeaderTimeout: 20 * time.Second}
		defer transport.(*http.Transport).CloseIdleConnections()
	}
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, e := client.Do(req)
	if e != nil {
		return nil, 0, failure("transport_result_unknown")
	}
	defer func() { _ = response.Body.Close() }()
	raw, e := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if e != nil || len(raw) > maxResponseBytes {
		return nil, response.StatusCode, failure("response_size_invalid")
	}
	return raw, response.StatusCode, nil
}

type wireResponse struct {
	DPSID  string `json:"idDps"`
	Key    string `json:"chaveAcesso"`
	XML    string `json:"nfseXmlGZipB64"`
	Errors []struct {
		Code        string `json:"Codigo"`
		Description string `json:"Descricao"`
	} `json:"erros"`
}

func decodeWire(raw []byte) (wireResponse, error) {
	var result wireResponse
	if len(raw) == 0 || json.Unmarshal(raw, &result) != nil {
		return result, failure("response_invalid")
	}
	return result, nil
}
func (c *Connector) validatedResult(ctx context.Context, cfg Config, m Manifest, d accounting.Document, response wireResponse) ([]byte, error) {
	if !accessKeyPattern.MatchString(response.Key) || (response.DPSID != "" && response.DPSID != m.DPSID) || response.XML == "" {
		return nil, failure("authorization_unconfirmed")
	}
	raw, e := expandXML(response.XML)
	if e != nil {
		return nil, e
	}
	if c.ports.ValidateSchema == nil || c.ports.VerifySignature == nil {
		return nil, failure("validator_unavailable")
	}
	if e = c.ports.ValidateSchema(ctx, "nfse", cfg.SchemaVersion, raw); e != nil {
		return nil, failure("nfse_schema_invalid")
	}
	if e = c.ports.VerifySignature(ctx, "nfse", raw); e != nil {
		return nil, failure("nfse_signature_invalid")
	}
	projection, e := inspectXML(raw)
	if e != nil {
		return nil, e
	}
	if projection.Root != "NFSe" || projection.Key != response.Key {
		return nil, failure("authorization_key_mismatch")
	}
	if e = matchesDocument(projection, cfg, m, d); e != nil {
		return nil, e
	}
	return raw, nil
}
