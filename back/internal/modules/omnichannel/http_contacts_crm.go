package omnichannel

import (
	"net/http"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// RegisterCRMContactRoutes exposes the additive CRM surface. The legacy contact
// endpoints remain untouched for compatibility with the inbox.
func RegisterCRMContactRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler { return middleware.RequireAuthWithAccount(h) }
	mux.Handle("GET /v1/omnichannel/contacts/crm", wrap(handleListCRMContacts(svc)))
	mux.Handle("GET /v1/omnichannel/contacts/{id}/profile", wrap(handleGetCRMContactProfile(svc)))
	mux.Handle("PATCH /v1/omnichannel/contacts/{id}/crm", wrap(handleUpdateCRMContact(svc)))
	mux.Handle("GET /v1/omnichannel/contacts/{id}/notes", wrap(handleListCRMContactNotes(svc)))
	mux.Handle("POST /v1/omnichannel/contacts/{id}/notes", wrap(handleCreateCRMContactNote(svc)))
	mux.Handle("POST /v1/omnichannel/contacts/{id}/merge", wrap(handleMergeCRMContacts(svc)))
	mux.Handle("POST /v1/omnichannel/contacts/merges/{eventId}/undo", wrap(handleUndoCRMContactMerge(svc)))
	mux.Handle("GET /v1/omnichannel/settings/lead-sources", wrap(handleListLeadSources(svc)))
	mux.Handle("POST /v1/omnichannel/settings/lead-sources", wrap(handleCreateLeadSource(svc)))
	mux.Handle("PATCH /v1/omnichannel/settings/lead-sources/{id}", wrap(handleUpdateLeadSource(svc)))
	mux.Handle("GET /v1/omnichannel/settings/contact-segments", wrap(handleListContactSegments(svc)))
	mux.Handle("POST /v1/omnichannel/settings/contact-segments", wrap(handleCreateContactSegment(svc)))
	mux.Handle("PATCH /v1/omnichannel/settings/contact-segments/{id}", wrap(handleUpdateContactSegment(svc)))
}

func handleMergeCRMContacts(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in ContactMergeInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.MergeCRMContacts(r.Context(), p.AccountID, p, r.PathValue("id"), in)
		writeDomainResult(w, r, http.StatusCreated, out, err)
	}
}

func handleUndoCRMContactMerge(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.UndoCRMContactMerge(r.Context(), p.AccountID, p, r.PathValue("eventId"))
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

func handleListLeadSources(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListLeadSources(r.Context(), p.AccountID, p)
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

func handleCreateLeadSource(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in LeadSourceInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.CreateLeadSource(r.Context(), p.AccountID, p, in)
		writeDomainResult(w, r, http.StatusCreated, out, err)
	}
}

func handleUpdateLeadSource(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var patch LeadSourcePatch
		if err := decodeJSONBody(w, r, &patch); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.UpdateLeadSource(r.Context(), p.AccountID, p, r.PathValue("id"), patch)
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

func handleListContactSegments(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		out, err := svc.ListContactSegments(r.Context(), p.AccountID, p)
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

func handleCreateContactSegment(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in ContactSegmentInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.CreateContactSegment(r.Context(), p.AccountID, p, in)
		writeDomainResult(w, r, http.StatusCreated, out, err)
	}
}

func handleUpdateContactSegment(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var patch ContactSegmentPatch
		if err := decodeJSONBody(w, r, &patch); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.UpdateContactSegment(r.Context(), p.AccountID, p, r.PathValue("id"), patch)
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

func handleListCRMContacts(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		q := r.URL.Query()
		f := CRMContactFilter{Limit: parseLimit(q.Get("limit")), BeforeCursor: strings.TrimSpace(q.Get("before")),
			Search: firstNonEmpty(q.Get("q"), q.Get("search")), Channel: q.Get("channel"), Status: q.Get("status"),
			Tag: q.Get("tag"), OwnerID: firstNonEmpty(q.Get("ownerId"), q.Get("ownerUserId")), Source: q.Get("source")}
		var err error
		f.LastSeenAfter, err = parseCRMTime(q.Get("lastSeenAfter"))
		if err != nil {
			writeInvalidBody(w, r)
			return
		}
		f.LastSeenBefore, err = parseCRMTime(q.Get("lastSeenBefore"))
		if err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.ListCRMContacts(r.Context(), p.AccountID, p, f)
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

func handleGetCRMContactProfile(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		q := r.URL.Query()
		out, err := svc.GetCRMContactProfile(r.Context(), p.AccountID, p, r.PathValue("id"), strings.TrimSpace(q.Get("touchpointsBefore")), strings.TrimSpace(q.Get("notesBefore")), parseLimit(q.Get("limit")))
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

func handleUpdateCRMContact(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var patch CRMContactPatch
		if err := decodeJSONBody(w, r, &patch); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.UpdateCRMContact(r.Context(), p.AccountID, p, r.PathValue("id"), patch)
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

func handleListCRMContactNotes(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		q := r.URL.Query()
		out, err := svc.ListCRMContactNotes(r.Context(), p.AccountID, p, r.PathValue("id"), strings.TrimSpace(q.Get("before")), parseLimit(q.Get("limit")))
		writeDomainResult(w, r, http.StatusOK, out, err)
	}
}

func handleCreateCRMContactNote(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := domainScope(w, r)
		if !ok {
			return
		}
		var in ContactNoteInput
		if err := decodeJSONBody(w, r, &in); err != nil {
			writeInvalidBody(w, r)
			return
		}
		out, err := svc.CreateCRMContactNote(r.Context(), p.AccountID, p, r.PathValue("id"), in)
		writeDomainResult(w, r, http.StatusCreated, out, err)
	}
}

func parseCRMTime(raw string) (*time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil {
		return nil, ErrInvalidBody
	}
	return &t, nil
}
