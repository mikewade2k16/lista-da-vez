package customerintelligence

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func candidateClaimsGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		items, err := service.CandidateClaims(
			r.Context(),
			requestScope(r, principal),
			r.PathValue("relationshipId"),
			r.URL.Query().Get("status"),
			requestLimit(r),
		)
		writeResult(w, r, http.StatusOK, items, err)
	}
}

func candidateClaimReview(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input ClaimReviewInput
		if !decodeJSON(w, r, &input) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		item, err := service.ReviewCandidateClaim(
			r.Context(),
			requestScope(r, principal),
			principal.UserID,
			r.PathValue("id"),
			input,
		)
		writeResult(w, r, http.StatusOK, item, err)
	}
}
