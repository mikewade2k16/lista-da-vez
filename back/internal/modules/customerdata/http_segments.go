package customerdata

import "net/http"

func registerSegmentRoutes(mux *http.ServeMux, service *Service, wrap func(http.Handler) http.Handler) {
	route(mux, "GET /v1/customer-data/segment-fields", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		out, err := service.SegmentFields(r.Context(), principal, r.URL.Query().Get("clientAccountId"))
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, out)
	})
	route(mux, "GET /v1/customer-data/segments", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		items, cursor, err := service.ListSegments(
			r.Context(), principal, r.URL.Query().Get("clientAccountId"),
			r.URL.Query().Get("status"), r.URL.Query().Get("cursor"), queryLimit(r),
		)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, map[string]any{"items": items, "nextCursor": cursor})
	})
	route(mux, "POST /v1/customer-data/segments", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		var input CreateSegmentInput
		if !decodeStrictJSON(w, r, &input) {
			return
		}
		out, err := service.CreateSegment(r.Context(), principal, input)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusCreated, out)
	})
	route(mux, "GET /v1/customer-data/segments/{segmentId}", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		out, err := service.GetSegment(r.Context(), principal, r.PathValue("segmentId"))
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, out)
	})
	route(mux, "PATCH /v1/customer-data/segments/{segmentId}", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		var patch SegmentPatch
		if !decodeStrictJSON(w, r, &patch) {
			return
		}
		out, err := service.UpdateSegment(r.Context(), principal, r.PathValue("segmentId"), patch)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, out)
	})
	route(mux, "POST /v1/customer-data/segments/{segmentId}/archive", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		revision, err := queryExpectedRevision(r)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		out, err := service.ArchiveSegment(r.Context(), principal, r.PathValue("segmentId"), revision)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, out)
	})
	route(mux, "GET /v1/customer-data/segments/{segmentId}/versions", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		out, err := service.ListSegmentVersions(r.Context(), principal, r.PathValue("segmentId"))
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, map[string]any{"items": out})
	})
	route(mux, "POST /v1/customer-data/segments/{segmentId}/versions", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		var input CreateSegmentVersionInput
		if !decodeStrictJSON(w, r, &input) {
			return
		}
		out, replayed, err := service.CreateSegmentVersion(r.Context(), principal, r.PathValue("segmentId"), input)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusCreated, map[string]any{"version": out, "replayed": replayed})
	})
	route(mux, "PATCH /v1/customer-data/segment-versions/{versionId}", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		var patch SegmentVersionPatch
		if !decodeStrictJSON(w, r, &patch) {
			return
		}
		out, err := service.UpdateSegmentVersion(r.Context(), principal, r.PathValue("versionId"), patch)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, out)
	})
	route(mux, "POST /v1/customer-data/segment-versions/{versionId}/validate", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		out, err := service.ValidateSegmentVersion(r.Context(), principal, r.PathValue("versionId"))
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, out)
	})
	route(mux, "POST /v1/customer-data/segment-versions/{versionId}/preview", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		var input SegmentEvaluationRequest
		if !decodeStrictJSON(w, r, &input) {
			return
		}
		out, err := service.RequestSegmentPreview(r.Context(), principal, r.PathValue("versionId"), input)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusAccepted, out)
	})
	route(mux, "POST /v1/customer-data/segment-versions/{versionId}/publish", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		var input PublishSegmentVersionInput
		if !decodeStrictJSON(w, r, &input) {
			return
		}
		out, replayed, err := service.PublishSegmentVersion(r.Context(), principal, r.PathValue("versionId"), input)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, map[string]any{"version": out, "replayed": replayed})
	})
	route(mux, "POST /v1/customer-data/segments/{segmentId}/rollback", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		var input RollbackSegmentInput
		if !decodeStrictJSON(w, r, &input) {
			return
		}
		out, replayed, err := service.RollbackSegment(r.Context(), principal, r.PathValue("segmentId"), input)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, map[string]any{"segment": out, "replayed": replayed})
	})
	route(mux, "POST /v1/customer-data/segments/{segmentId}/materializations", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		var input SegmentEvaluationRequest
		if !decodeStrictJSON(w, r, &input) {
			return
		}
		input.Mode = "materialize"
		out, err := service.RequestSegmentEvaluation(r.Context(), principal, r.PathValue("segmentId"), input)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusAccepted, out)
	})
	route(mux, "GET /v1/customer-data/segment-evaluation-runs/{runId}", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		out, err := service.GetEvaluationRun(r.Context(), principal, r.PathValue("runId"))
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, out)
	})
	route(mux, "GET /v1/customer-data/segments/{segmentId}/materializations", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		out, err := service.ListMaterializations(r.Context(), principal, r.PathValue("segmentId"), queryLimit(r))
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, map[string]any{"items": out})
	})
	route(mux, "GET /v1/customer-data/segment-materializations/{materializationId}/members", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		items, cursor, err := service.ListMaterializationMembers(
			r.Context(), principal, r.PathValue("materializationId"),
			r.URL.Query().Get("cursor"), queryLimit(r),
		)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, map[string]any{"items": items, "nextCursor": cursor})
	})
}
