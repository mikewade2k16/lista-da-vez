package customerdata

import (
	"net/http"
)

func registerSubjectRoutes(mux *http.ServeMux, service *Service, wrap func(http.Handler) http.Handler) {
	route(mux, "GET /v1/customer-data/subjects", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		archived, err := queryOptionalBool(r, "archived")
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		updatedAfter, err := queryOptionalTime(r, "updatedAfter")
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		out, err := service.ListSubjects(r.Context(), principal, SubjectFilter{
			ClientAccountID: r.URL.Query().Get("clientAccountId"),
			Query:           r.URL.Query().Get("q"), SubjectType: r.URL.Query().Get("subjectType"),
			LifecycleStatus: r.URL.Query().Get("lifecycleStatus"), Tag: r.URL.Query().Get("tag"),
			OwnerUserID: r.URL.Query().Get("ownerUserId"), Archived: archived,
			UpdatedAfter: updatedAfter, Cursor: r.URL.Query().Get("cursor"), Limit: queryLimit(r),
		})
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, out)
	})

	route(mux, "POST /v1/customer-data/subjects", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		var input CreateSubjectInput
		if !decodeStrictJSON(w, r, &input) {
			return
		}
		out, err := service.CreateSubject(r.Context(), principal, input)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusCreated, out)
	})

	route(mux, "GET /v1/customer-data/relationships/{relationshipId}/profile", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		out, err := service.GetProfile(r.Context(), principal, r.PathValue("relationshipId"))
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, out)
	})

	route(mux, "PATCH /v1/customer-data/subjects/{subjectId}", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		var patch SubjectPatch
		if !decodeStrictJSON(w, r, &patch) {
			return
		}
		out, err := service.UpdateSubject(
			r.Context(), principal, r.PathValue("subjectId"),
			r.URL.Query().Get("clientAccountId"), patch,
		)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, out)
	})

	route(mux, "PATCH /v1/customer-data/relationships/{relationshipId}", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		var patch RelationshipPatch
		if !decodeStrictJSON(w, r, &patch) {
			return
		}
		out, err := service.UpdateRelationship(r.Context(), principal, r.PathValue("relationshipId"), patch)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, out)
	})
}
