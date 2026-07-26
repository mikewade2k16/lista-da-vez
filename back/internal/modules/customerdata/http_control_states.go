package customerdata

import "net/http"

func registerControlStateRoutes(mux *http.ServeMux, service *Service, wrap func(http.Handler) http.Handler) {
	route(mux, "GET /v1/customer-data/control-state", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		out, err := service.GetControlState(r.Context(), principal, r.URL.Query().Get("clientAccountId"))
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, out)
	})
	route(mux, "PUT /v1/customer-data/capabilities/{capabilityKey}", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		var input CapabilityStateInput
		if !decodeStrictJSON(w, r, &input) {
			return
		}
		out, replayed, err := service.SetCapabilityState(
			r.Context(), principal, r.URL.Query().Get("clientAccountId"),
			r.PathValue("capabilityKey"), input,
		)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, map[string]any{"capability": out, "replayed": replayed})
	})
	route(mux, "PUT /v1/customer-data/writer-states/{entityKey}", wrap, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requestPrincipal(w, r)
		if !ok {
			return
		}
		var input WriterStateInput
		if !decodeStrictJSON(w, r, &input) {
			return
		}
		out, replayed, err := service.SetWriterState(
			r.Context(), principal, r.URL.Query().Get("clientAccountId"),
			r.PathValue("entityKey"), input,
		)
		if err != nil {
			writeCustomerDataError(w, err)
			return
		}
		writeCustomerDataJSON(w, http.StatusOK, map[string]any{"writerState": out, "replayed": replayed})
	})
}
