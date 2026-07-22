package omnichannel

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

// MediaAnalysisGateway serves a private media stream to the configured
// orchestrator. It never returns a storage path and never accepts a message ID
// that is not bound to the encrypted claim's analysis.
type MediaAnalysisGateway struct {
	box   *secretbox.Box
	store *Store
	media *DiskMediaStorage
}

type mediaStreamClaims struct {
	Version    string `json:"v"`
	ExpiresAt  int64  `json:"exp"`
	AccountID  string `json:"accountId"`
	MessageID  string `json:"messageId"`
	AnalysisID string `json:"analysisId"`
}

func newMediaAnalysisGateway(box *secretbox.Box, store *Store, media *DiskMediaStorage) *MediaAnalysisGateway {
	if box == nil || store == nil || media == nil {
		return nil
	}
	return &MediaAnalysisGateway{box: box, store: store, media: media}
}

// IssueMediaStreamToken creates a short-lived opaque token for one analysis and
// one message. The plaintext claims never enter logs or workflow exports.
func IssueMediaStreamToken(box *secretbox.Box, accountID, messageID, analysisID string, ttl time.Duration) (string, error) {
	if box == nil || !omnichannelUUIDPattern.MatchString(strings.TrimSpace(accountID)) ||
		!omnichannelUUIDPattern.MatchString(strings.TrimSpace(messageID)) ||
		!omnichannelUUIDPattern.MatchString(strings.TrimSpace(analysisID)) {
		return "", ErrMediaAnalysisInvalid
	}
	if ttl <= 0 || ttl > 5*time.Minute {
		ttl = 2 * time.Minute
	}
	return encryptMediaStreamClaims(box, mediaStreamClaims{
		Version: "media-stream.v1", ExpiresAt: time.Now().Add(ttl).Unix(),
		AccountID: accountID, MessageID: messageID, AnalysisID: analysisID,
	})
}

func encryptMediaStreamClaims(box *secretbox.Box, claims mediaStreamClaims) (string, error) {
	if box == nil || claims.Version != "media-stream.v1" || claims.ExpiresAt <= time.Now().Unix() {
		return "", ErrMediaAnalysisInvalid
	}
	if !omnichannelUUIDPattern.MatchString(claims.AccountID) || !omnichannelUUIDPattern.MatchString(claims.MessageID) || !omnichannelUUIDPattern.MatchString(claims.AnalysisID) {
		return "", ErrMediaAnalysisInvalid
	}
	encoded, err := json.Marshal(claims)
	if err != nil {
		return "", ErrMediaAnalysisInvalid
	}
	return box.Encrypt(string(encoded))
}

func registerMediaAnalysisRoutes(mux *http.ServeMux, gateway *MediaAnalysisGateway) {
	if gateway != nil {
		mux.Handle("GET /v1/runtime/omnichannel/media/{messageId}", http.HandlerFunc(gateway.handle))
	}
}

func (g *MediaAnalysisGateway) handle(w http.ResponseWriter, r *http.Request) {
	claims, ok := g.claims(r)
	if !ok || claims.MessageID != strings.TrimSpace(r.PathValue("messageId")) {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "media_gateway_unauthorized", "Token interno invalido.")
		return
	}
	descriptor, status, err := g.store.GetMediaDescriptorForAnalysis(r.Context(), claims.AccountID, claims.AnalysisID, claims.MessageID)
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrMediaAnalysisInvalid) {
			httpapi.WriteError(w, r, http.StatusNotFound, "media_not_found", "Midia indisponivel.")
			return
		}
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "media_gateway_unavailable", "Midia indisponivel.")
		return
	}
	if status != MediaAnalysisStatusQueued && status != MediaAnalysisStatusProcessing {
		httpapi.WriteError(w, r, http.StatusNotFound, "media_not_found", "Midia indisponivel.")
		return
	}
	key := deref(descriptor.StorageKey)
	if key == "" || !strings.EqualFold(deref(descriptor.SourceKind), "disk") {
		httpapi.WriteError(w, r, http.StatusNotFound, "media_not_ready", "Midia ainda nao esta pronta.")
		return
	}
	file, info, err := g.media.Open(key)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusNotFound, "media_not_found", "Midia indisponivel.")
		return
	}
	defer func() { _ = file.Close() }()
	mime := deref(descriptor.MimeType)
	if mime == "" {
		mime = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Length", formatInt(info.Size()))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.CopyN(w, file, info.Size())
}

func (g *MediaAnalysisGateway) claims(r *http.Request) (mediaStreamClaims, bool) {
	if g == nil || g.box == nil || g.store == nil || g.media == nil {
		return mediaStreamClaims{}, false
	}
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if token == "" {
		return mediaStreamClaims{}, false
	}
	plaintext, err := g.box.Decrypt(token)
	if err != nil {
		return mediaStreamClaims{}, false
	}
	var claims mediaStreamClaims
	if json.Unmarshal([]byte(plaintext), &claims) != nil || claims.Version != "media-stream.v1" ||
		claims.ExpiresAt < time.Now().Unix() || !omnichannelUUIDPattern.MatchString(claims.AccountID) ||
		!omnichannelUUIDPattern.MatchString(claims.MessageID) || !omnichannelUUIDPattern.MatchString(claims.AnalysisID) {
		return mediaStreamClaims{}, false
	}
	return claims, true
}

var formatInt = func(value int64) string { return strconv.FormatInt(value, 10) }
