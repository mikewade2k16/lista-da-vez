package customerintelligence

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func retentionPoliciesGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		items, err := service.RetentionPolicies(r.Context(), principal.AccountID)
		writeResult(w, r, http.StatusOK, items, err)
	}
}

func retentionPolicyDraftPost(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input RetentionPolicyDraftInput
		if !decodeJSON(w, r, &input) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		item, err := service.CreateRetentionPolicyDraft(
			r.Context(),
			principal.AccountID,
			principal.UserID,
			r.PathValue("policyKey"),
			input,
		)
		writeResult(w, r, http.StatusCreated, item, err)
	}
}

func retentionPolicyPublishPost(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input PublishRetentionPolicyInput
		if !decodeJSON(w, r, &input) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		item, err := service.PublishRetentionPolicy(
			r.Context(),
			principal.AccountID,
			principal.UserID,
			r.PathValue("id"),
			input,
		)
		writeResult(w, r, http.StatusOK, item, err)
	}
}
