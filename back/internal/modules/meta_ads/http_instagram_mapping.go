package metaads

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

func registerInstagramIdentityRoutes(
	mux *http.ServeMux,
	svc *Service,
	wrap func(string, http.HandlerFunc) http.Handler,
) {
	mux.Handle(
		"GET /v1/meta-ads/instagram-identities",
		wrap("meta_ads.view", handleInstagramIdentitiesList(svc)),
	)
	mux.Handle(
		"PATCH /v1/meta-ads/instagram-identities/{igUserId}/client",
		wrap("meta_ads.manage", handleInstagramIdentityClientUpdate(svc)),
	)
}

func handleInstagramIdentitiesList(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		identities, err := svc.ListInstagramIdentities(r.Context(), accountID)
		if err != nil {
			writeServiceError(w, r, err, "Falha ao listar Paginas e identidades do Instagram.")
			return
		}
		if identities == nil {
			identities = []InstagramIdentityView{}
		}
		httpapi.WriteJSON(w, http.StatusOK, identities)
	}
}

func handleInstagramIdentityClientUpdate(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		body, ok := readInstagramIdentityClientBody(w, r)
		if !ok {
			return
		}
		view, err := svc.SetInstagramIdentityClient(
			r.Context(), accountID, strings.TrimSpace(r.PathValue("igUserId")), *body.ClientAccountID,
		)
		if errors.Is(err, ErrInvalidInstagramIdentity) {
			httpapi.WriteError(
				w, r, http.StatusBadRequest, "invalid_instagram_identity",
				"Identidade do Instagram ou cliente invalido.",
			)
			return
		}
		if err != nil {
			writeServiceError(w, r, err, "Falha ao vincular a identidade do Instagram ao cliente.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

type instagramIdentityClientBody struct {
	ClientAccountID *string `json:"clientAccountId"`
}

func readInstagramIdentityClientBody(
	w http.ResponseWriter, r *http.Request,
) (instagramIdentityClientBody, bool) {
	var body instagramIdentityClientBody
	if err := decodeInstagramIdentityClientBody(w, r, &body); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
		return instagramIdentityClientBody{}, false
	}
	return body, true
}

func decodeInstagramIdentityClientBody(
	w http.ResponseWriter, r *http.Request, destination *instagramIdentityClientBody,
) error {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil || len(bytes.TrimSpace(raw)) == 0 {
		return ErrInvalidInstagramIdentity
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(destination); err != nil {
		return err
	}
	if err = decoder.Decode(&struct{}{}); err != io.EOF {
		return ErrInvalidInstagramIdentity
	}
	if destination.ClientAccountID == nil {
		return ErrInvalidInstagramIdentity
	}
	return nil
}
