package omnichannel

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func RegisterChannelClientBindingRoutes(
	mux *http.ServeMux,
	svc *ChannelClientBindingService,
	middleware *auth.Middleware,
) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuthWithAccount(h)
	}
	mux.Handle("GET /v1/omnichannel/channel-client-bindings", wrap(handleListChannelClientBindings(svc)))
	mux.Handle("POST /v1/omnichannel/channel-client-bindings", wrap(handleCreateChannelClientBinding(svc)))
	mux.Handle("POST /v1/omnichannel/channel-client-bindings/{id}/reassign", wrap(handleReassignChannelClientBinding(svc)))
	mux.Handle("POST /v1/omnichannel/channel-client-bindings/{id}/end", wrap(handleEndChannelClientBinding(svc)))
	mux.Handle("GET /v1/omnichannel/channel-client-binding-exceptions", wrap(handleListChannelClientBindingExceptions(svc)))
	mux.Handle("POST /v1/omnichannel/channel-client-binding-exceptions/resolve", wrap(handleResolveChannelClientBindingException(svc)))
	mux.Handle("GET /v1/omnichannel/channel-client-binding-policy", wrap(handleGetChannelClientBindingPolicy(svc)))
	mux.Handle("PUT /v1/omnichannel/channel-client-binding-policy", wrap(handleUpdateChannelClientBindingPolicy(svc)))
	mux.Handle("POST /v1/omnichannel/channel-client-binding-repair-previews", wrap(handleCreateChannelClientBindingRepairPreview(svc)))
	mux.Handle("POST /v1/omnichannel/channel-client-binding-repair-jobs", wrap(handleApplyChannelClientBindingRepair(svc)))
	mux.Handle("GET /v1/omnichannel/channel-client-binding-repair-jobs/{id}", wrap(handleGetChannelClientBindingRepairJob(svc)))
}

func handleGetChannelClientBindingPolicy(svc *ChannelClientBindingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.GetPolicy(r.Context(), p.AccountID, p)
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

func handleUpdateChannelClientBindingPolicy(svc *ChannelClientBindingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in ChannelClientBindingPolicyInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.UpdatePolicy(r.Context(), p.AccountID, p, in)
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

func handleListChannelClientBindings(svc *ChannelClientBindingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.List(r.Context(), p.AccountID, p, ChannelClientBindingFilter{
			ClientAccountID: r.URL.Query().Get("clientAccountId"),
			Channel:         r.URL.Query().Get("channel"),
			State:           r.URL.Query().Get("state"),
			Cursor:          r.URL.Query().Get("cursor"),
			Limit:           parseLimit(r.URL.Query().Get("limit")),
		})
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

func handleCreateChannelClientBinding(svc *ChannelClientBindingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in CreateChannelClientBindingInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.Create(r.Context(), p.AccountID, p, in)
		writeDomainResult(w, r, http.StatusCreated, out, err)
	}
}

func handleReassignChannelClientBinding(svc *ChannelClientBindingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in ReassignChannelClientBindingInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.Reassign(r.Context(), p.AccountID, r.PathValue("id"), p, in)
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

func handleEndChannelClientBinding(svc *ChannelClientBindingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in EndChannelClientBindingInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.End(r.Context(), p.AccountID, r.PathValue("id"), p, in)
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

func handleListChannelClientBindingExceptions(svc *ChannelClientBindingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.Exceptions(r.Context(), p.AccountID, p)
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

func handleResolveChannelClientBindingException(svc *ChannelClientBindingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in ResolveChannelClientBindingExceptionInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.ResolveException(r.Context(), p.AccountID, p, in)
		writeDomainResult(w, r, http.StatusCreated, out, err)
	}
}

func handleCreateChannelClientBindingRepairPreview(svc *ChannelClientBindingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in ChannelClientBindingRepairPreviewInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.CreateRepairPreview(r.Context(), p.AccountID, p, in)
		writeDomainResult(w, r, http.StatusCreated, out, err)
	}
}

func handleApplyChannelClientBindingRepair(svc *ChannelClientBindingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in ChannelClientBindingRepairApplyInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.ApplyRepair(r.Context(), p.AccountID, p, in)
		writeDomainResult(w, r, http.StatusAccepted, out, err)
	}
}

func handleGetChannelClientBindingRepairJob(svc *ChannelClientBindingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.GetRepairJob(r.Context(), p.AccountID, r.PathValue("id"), p)
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}
