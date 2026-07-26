package customerintelligence

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

const maxRequestBytes = 1 << 20

func RegisterRoutes(
	mux *http.ServeMux,
	service *Service,
	middleware *auth.Middleware,
	modulesGuard *httpapi.AccountModulesGuard,
	gate *permissionGate,
) {
	wrap := func(permission string, handler http.HandlerFunc) http.Handler {
		authorized := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := auth.PrincipalFromContext(r.Context())
			if !ok || principal.AccountID == "" {
				httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
				return
			}
			if err := gate.Authorize(r.Context(), principal, permission); err != nil {
				if errors.Is(err, ErrForbidden) {
					httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para esta acao.")
				} else {
					httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao validar permissao.")
				}
				return
			}
			handler.ServeHTTP(w, r)
		})
		wrapped := middleware.RequireAuthWithAccount(authorized)
		if modulesGuard != nil {
			wrapped = modulesGuard.RequireModule(ModuleID)(wrapped)
		}
		return wrapped
	}
	route := func(pattern, permission string, handler http.HandlerFunc) {
		mux.Handle(pattern, wrap(permission, handler))
	}

	route("GET /v1/customer-intelligence/capabilities/{key}", PermissionProfileView, capabilityGet(service))
	route("GET /v1/customer-intelligence/capabilities", PermissionProfileView, capabilitiesGet(service))
	route("PUT /v1/customer-intelligence/capabilities/{key}", PermissionProfileManage, capabilityPut(service))
	route("GET /v1/customer-intelligence/sources/catalog", PermissionSourcesView, sourceCatalogGet())
	route("GET /v1/customer-intelligence/sources", PermissionSourcesView, sourcesGet(service))
	route("PUT /v1/customer-intelligence/sources", PermissionSourcesManage, sourcePut(service))
	route("POST /v1/customer-intelligence/sources", PermissionSourcesManage, sourcePut(service))
	route("POST /v1/customer-intelligence/sources/{id}/sync", PermissionSourcesManage, sourceSync(service))
	route("GET /v1/customer-intelligence/sources/{id}/runs", PermissionSourcesView, sourceRunsGet(service))
	route("GET /v1/customer-intelligence/retention-policies", PermissionSourcesView, retentionPoliciesGet(service))
	route("POST /v1/customer-intelligence/retention-policies/{policyKey}/drafts", PermissionSourcesManage, retentionPolicyDraftPost(service))
	route("POST /v1/customer-intelligence/retention-policy-versions/{id}/publish", PermissionSourcesManage, retentionPolicyPublishPost(service))
	route("POST /v1/customer-intelligence/relationships/{relationshipId}/facts/manual", PermissionProfileManage, manualFactPost(service))
	route("GET /v1/customer-intelligence/relationships/{relationshipId}/facts", PermissionProfileView, factsGet(service))
	route("GET /v1/customer-intelligence/relationships/{relationshipId}/observations", PermissionProfileView, observationsGet(service))
	route("GET /v1/customer-intelligence/observations/{id}", PermissionAuditView, observationGet(service))
	route("POST /v1/customer-intelligence/observations/{id}/reveal", PermissionAuditView, observationReveal(service))
	route("GET /v1/customer-intelligence/relationships/{relationshipId}/claims", PermissionProfileView, candidateClaimsGet(service))
	route("POST /v1/customer-intelligence/claims/{id}/review", PermissionProfileManage, candidateClaimReview(service))
	route("GET /v1/customer-intelligence/relationships/{relationshipId}/profile", PermissionProfileView, profileGet(service))
	route("POST /v1/customer-intelligence/relationships/{relationshipId}/refresh", PermissionProfileManage, relationshipRefreshPost(service))
	route("GET /v1/customer-intelligence/relationships/{relationshipId}/recommendations", PermissionProfileView, recommendationsGet(service))
	route("POST /v1/customer-intelligence/recommendations/{id}/review", PermissionProfileManage, recommendationReview(service))
	route("GET /v1/customer-intelligence/relationships/{relationshipId}/source-suggestions", PermissionProfileView, sourceSuggestionsGet(service))
	route("POST /v1/customer-intelligence/source-suggestions/{id}/review", PermissionSourcesManage, sourceSuggestionReview(service))

	route("GET /v1/customer-intelligence/processes", PermissionPromptsView, processesGet(service))
	route("GET /v1/customer-intelligence/prompts", PermissionPromptsView, promptsGet(service))
	route("GET /v1/customer-intelligence/prompt-bindings", PermissionPromptsView, promptBindingsGet(service))
	route("POST /v1/customer-intelligence/prompts/{processKey}/drafts", PermissionPromptsManage, promptDraftPost(service))
	route("PATCH /v1/customer-intelligence/prompt-versions/{id}", PermissionPromptsManage, promptPatch(service))
	route("POST /v1/customer-intelligence/prompt-versions/{id}/validate", PermissionPromptsManage, promptValidate(service))
	route("POST /v1/customer-intelligence/prompt-versions/{id}/test", PermissionPromptsManage, promptTest(service))
	route("GET /v1/customer-intelligence/prompt-versions/{id}/evaluations", PermissionPromptsView, promptEvaluationsGet(service))
	route("POST /v1/customer-intelligence/prompt-versions/{id}/publish", PermissionPromptsPublish, promptPublish(service))
	route("POST /v1/customer-intelligence/prompt-bindings/{id}/rollback", PermissionPromptsPublish, promptRollback(service))

	route("GET /v1/customer-intelligence/models", PermissionAgentsManage, modelsGet(service))
	route("PUT /v1/customer-intelligence/models", PermissionAgentsManage, modelPut(service))
	route("GET /v1/customer-intelligence/credentials", PermissionAgentsManage, credentialsGet(service))
	route("PUT /v1/customer-intelligence/credentials", PermissionAgentsManage, credentialPut(service))
	route("POST /v1/customer-intelligence/credentials", PermissionAgentsManage, credentialPut(service))
	route("DELETE /v1/customer-intelligence/credentials/{id}", PermissionAgentsManage, credentialDelete(service))
	route("GET /v1/customer-intelligence/agents", PermissionAgentsManage, agentsGet(service))
	route("POST /v1/customer-intelligence/agents", PermissionAgentsManage, agentPost(service))
	route("PATCH /v1/customer-intelligence/agents/{id}", PermissionAgentsManage, agentPatch(service))
	route("POST /v1/customer-intelligence/agents/{id}/versions", PermissionAgentsManage, agentVersionPost(service))
	route("POST /v1/customer-intelligence/agent-versions/{id}/publish", PermissionAgentsManage, agentVersionPublish(service))

	route("POST /v1/customer-intelligence/runtime/execute", PermissionProfileManage, runtimeExecute(service))
	route("GET /v1/customer-intelligence/runtime/runs", PermissionRunsView, runtimeRunsGet(service))
	route("GET /v1/customer-intelligence/runs", PermissionRunsView, runtimeRunsGet(service))
	route("POST /v1/customer-intelligence/outcomes", PermissionProfileManage, outcomePost(service))
	route("GET /v1/customer-intelligence/audit-events", PermissionAuditView, auditEventsGet(service))
	route("GET /v1/customer-intelligence/portfolio/opportunities", PermissionPortfolioView, portfolioGet(service))
}

func capabilityGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		item, err := service.Capability(
			r.Context(), requestScope(r, principal), r.PathValue("key"),
			r.URL.Query().Get("scopeKey"),
		)
		writeResult(w, r, http.StatusOK, item, err)
	}
}

func capabilitiesGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		items, err := service.Capabilities(r.Context(), requestScope(r, principal))
		writeResult(w, r, http.StatusOK, items, err)
	}
}

func capabilityPut(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input CapabilityInput
		if !decodeJSON(w, r, &input) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		input.ClientAccountID = defaultClient(principal.AccountID, input.ClientAccountID)
		input.Key = r.PathValue("key")
		item, err := service.SetCapability(r.Context(), principal.AccountID, principal.UserID, input)
		writeResult(w, r, http.StatusOK, item, err)
	}
}

func sourceCatalogGet() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, SourceCatalog())
	}
}

func sourcesGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		items, err := service.Sources(r.Context(), requestScope(r, principal))
		writeResult(w, r, http.StatusOK, items, err)
	}
}

func sourcePut(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input SourceConfigInput
		if !decodeJSON(w, r, &input) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		input.ClientAccountID = defaultClient(principal.AccountID, input.ClientAccountID)
		item, err := service.ConfigureSource(r.Context(), principal.AccountID, principal.UserID, input)
		writeResult(w, r, http.StatusOK, item, err)
	}
}

func sourceSync(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input SourceSyncRequest
		if !decodeJSON(w, r, &input) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		input.AccountID = principal.AccountID
		input.ClientAccountID = defaultClient(principal.AccountID, input.ClientAccountID)
		input.SourceConfigID = r.PathValue("id")
		run, created, err := service.TriggerSourceSync(r.Context(), input)
		status := http.StatusAccepted
		if !created {
			status = http.StatusOK
		}
		writeResult(w, r, status, run, err)
	}
}

func sourceRunsGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		items, err := service.SourceRuns(
			r.Context(), requestScope(r, principal), r.PathValue("id"), requestLimit(r),
		)
		writeResult(w, r, http.StatusOK, items, err)
	}
}

func manualFactPost(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input ManualFactInput
		if !decodeJSON(w, r, &input) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		input.ClientAccountID = defaultClient(principal.AccountID, input.ClientAccountID)
		input.RelationshipID = r.PathValue("relationshipId")
		item, err := service.CreateManualFact(r.Context(), principal.AccountID, principal.UserID, input)
		writeResult(w, r, http.StatusCreated, item, err)
	}
}

func factsGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		items, err := service.Facts(
			r.Context(), requestScope(r, principal),
			r.PathValue("relationshipId"), requestLimit(r),
		)
		writeResult(w, r, http.StatusOK, items, err)
	}
}

func observationsGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		items, err := service.Observations(
			r.Context(),
			requestScope(r, principal),
			r.PathValue("relationshipId"),
			requestSourceKeys(r),
			requestLimit(r),
		)
		writeResult(w, r, http.StatusOK, items, err)
	}
}

func observationGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		item, err := service.Observation(
			r.Context(),
			requestScope(r, principal),
			r.PathValue("id"),
			ObservationAccessInput{
				ActorUserID: principal.UserID,
				ReasonCode:  "audit_snapshot_view",
			},
		)
		writeResult(w, r, http.StatusOK, item, err)
	}
}

func observationReveal(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input ObservationRevealInput
		if !decodeJSON(w, r, &input) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		item, err := service.RevealObservation(
			r.Context(),
			requestScope(r, principal),
			principal.UserID,
			r.PathValue("id"),
			input,
		)
		writeResult(w, r, http.StatusOK, item, err)
	}
}

func profileGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		item, err := service.Profile(
			r.Context(), requestScope(r, principal), r.PathValue("relationshipId"),
		)
		writeResult(w, r, http.StatusOK, item, err)
	}
}

func relationshipRefreshPost(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input RelationshipRefreshInput
		if !decodeJSON(w, r, &input) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		input.ClientAccountID = defaultClient(principal.AccountID, input.ClientAccountID)
		input.RelationshipID = r.PathValue("relationshipId")
		item, err := service.EnqueueRelationshipRefresh(
			r.Context(),
			principal.AccountID,
			principal.UserID,
			input,
		)
		writeResult(w, r, http.StatusAccepted, item, err)
	}
}

func recommendationsGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		items, err := service.Recommendations(
			r.Context(), requestScope(r, principal), r.PathValue("relationshipId"),
			requestLimit(r),
		)
		writeResult(w, r, http.StatusOK, items, err)
	}
}

func recommendationReview(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input RecommendationFeedback
		if !decodeJSON(w, r, &input) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		item, err := service.ReviewRecommendation(
			r.Context(), requestScope(r, principal), principal.UserID,
			r.PathValue("id"), input,
		)
		writeResult(w, r, http.StatusOK, item, err)
	}
}

func sourceSuggestionsGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		items, err := service.SourceSuggestions(
			r.Context(),
			requestScope(r, principal),
			r.PathValue("relationshipId"),
			requestLimit(r),
		)
		writeResult(w, r, http.StatusOK, items, err)
	}
}

func sourceSuggestionReview(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input SourceSuggestionFeedback
		if !decodeJSON(w, r, &input) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		item, err := service.ReviewSourceSuggestion(
			r.Context(),
			requestScope(r, principal),
			principal.UserID,
			r.PathValue("id"),
			input,
		)
		writeResult(w, r, http.StatusOK, item, err)
	}
}

func processesGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := service.Processes(r.Context())
		writeResult(w, r, http.StatusOK, items, err)
	}
}

func promptsGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		items, err := service.PromptVersions(
			r.Context(), requestScope(r, principal), r.URL.Query().Get("processKey"),
		)
		writeResult(w, r, http.StatusOK, items, err)
	}
}

func promptBindingsGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		items, err := service.PromptBindings(
			r.Context(), requestScope(r, principal), r.URL.Query().Get("processKey"),
		)
		writeResult(w, r, http.StatusOK, items, err)
	}
}

func promptDraftPost(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input PromptDraftInput
		if !decodeJSON(w, r, &input) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		input.ProcessKey = r.PathValue("processKey")
		item, err := service.CreatePromptDraft(
			r.Context(), principal.AccountID, principal.UserID, input,
		)
		writeResult(w, r, http.StatusCreated, item, err)
	}
}

func promptValidate(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		item, validation, err := service.ValidatePromptVersion(
			r.Context(), principal.AccountID, principal.UserID, r.PathValue("id"),
		)
		writeResult(w, r, http.StatusOK, map[string]any{
			"promptVersion": item, "validation": validation,
		}, err)
	}
}

func promptPatch(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input PromptPatchInput
		if !decodeJSON(w, r, &input) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		item, err := service.UpdatePromptVersion(
			r.Context(), principal.AccountID, principal.UserID,
			r.PathValue("id"), input,
		)
		writeResult(w, r, http.StatusOK, item, err)
	}
}

func promptTest(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Fixture json.RawMessage `json:"fixture"`
		}
		if !decodeJSON(w, r, &input) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		item, err := service.TestPromptVersion(
			r.Context(), principal.AccountID, principal.UserID,
			r.PathValue("id"), input.Fixture,
		)
		writeResult(w, r, http.StatusCreated, item, err)
	}
}

func promptEvaluationsGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		items, err := service.PromptEvaluations(
			r.Context(), principal.AccountID, r.PathValue("id"),
		)
		writeResult(w, r, http.StatusOK, items, err)
	}
}

func promptPublish(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input PublishPromptInput
		if !decodeJSON(w, r, &input) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		item, err := service.PublishPromptVersion(
			r.Context(), principal.AccountID, principal.UserID,
			r.PathValue("id"), input,
		)
		writeResult(w, r, http.StatusCreated, item, err)
	}
}

func promptRollback(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input RollbackPromptInput
		if !decodeJSON(w, r, &input) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		item, err := service.RollbackPromptBinding(
			r.Context(), principal.AccountID, principal.UserID,
			r.PathValue("id"), input,
		)
		writeResult(w, r, http.StatusCreated, item, err)
	}
}

func modelsGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		items, err := service.Models(r.Context(), principal.AccountID)
		writeResult(w, r, http.StatusOK, items, err)
	}
}

func modelPut(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input AIModel
		if !decodeJSON(w, r, &input) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		item, err := service.ConfigureModel(
			r.Context(), principal.AccountID, principal.UserID, input,
		)
		writeResult(w, r, http.StatusOK, item, err)
	}
}

func credentialsGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		items, err := service.Credentials(r.Context(), principal.AccountID)
		writeResult(w, r, http.StatusOK, items, err)
	}
}

func credentialPut(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input CredentialInput
		if !decodeJSON(w, r, &input) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		item, err := service.SetCredential(
			r.Context(), principal.AccountID, principal.UserID, input,
		)
		writeResult(w, r, http.StatusOK, item, err)
	}
}

func credentialDelete(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		err := service.RevokeCredential(
			r.Context(), principal.AccountID, principal.UserID, r.PathValue("id"),
		)
		if err != nil {
			writeError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func agentsGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		items, err := service.Agents(r.Context(), requestScope(r, principal))
		writeResult(w, r, http.StatusOK, items, err)
	}
}

func agentPost(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			ClientAccountID string `json:"clientAccountId"`
			Slug            string `json:"slug"`
			Name            string `json:"name"`
		}
		if !decodeJSON(w, r, &input) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		item, err := service.CreateAgent(
			r.Context(), principal.AccountID, principal.UserID,
			input.ClientAccountID, input.Slug, input.Name,
		)
		writeResult(w, r, http.StatusCreated, item, err)
	}
}

func agentVersionPost(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input AIAgentVersionInput
		if !decodeJSON(w, r, &input) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		item, err := service.CreateAgentVersion(
			r.Context(), principal.AccountID, principal.UserID,
			r.PathValue("id"), input,
		)
		writeResult(w, r, http.StatusCreated, item, err)
	}
}

func agentPatch(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input AgentPatchInput
		if !decodeJSON(w, r, &input) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		item, err := service.UpdateAgent(
			r.Context(), principal.AccountID, principal.UserID,
			r.PathValue("id"), input,
		)
		writeResult(w, r, http.StatusOK, item, err)
	}
}

func agentVersionPublish(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		item, err := service.PublishAgentVersion(
			r.Context(), principal.AccountID, principal.UserID, r.PathValue("id"),
		)
		writeResult(w, r, http.StatusOK, item, err)
	}
}

func runtimeExecute(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input InteractionRequest
		if !decodeJSON(w, r, &input) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		input.AccountID = principal.AccountID
		input.ClientAccountID = defaultClient(principal.AccountID, input.ClientAccountID)
		item, err := service.ExecuteInteraction(r.Context(), input)
		writeResult(w, r, http.StatusOK, item, err)
	}
}

func runtimeRunsGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		items, err := service.RuntimeRuns(
			r.Context(), requestScope(r, principal), requestLimit(r),
		)
		writeResult(w, r, http.StatusOK, items, err)
	}
}

func outcomePost(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input AcceptedOutcome
		if !decodeJSON(w, r, &input) {
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		input.AccountID = principal.AccountID
		input.ClientAccountID = defaultClient(principal.AccountID, input.ClientAccountID)
		created, err := service.RecordOutcome(r.Context(), input)
		status := http.StatusCreated
		if !created {
			status = http.StatusOK
		}
		writeResult(w, r, status, map[string]bool{"created": created}, err)
	}
}

func auditEventsGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query, err := auditEventQueryFromRequest(r)
		if err != nil {
			writeResult(w, r, http.StatusOK, AuditEventPage{}, err)
			return
		}
		principal, _ := auth.PrincipalFromContext(r.Context())
		page, err := service.AuditEventPage(
			r.Context(), requestScope(r, principal), query,
		)
		writeResult(w, r, http.StatusOK, page, err)
	}
}

func portfolioGet(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := auth.PrincipalFromContext(r.Context())
		target := defaultClient(principal.AccountID, r.URL.Query().Get("targetClientAccountId"))
		items, err := service.PortfolioOpportunities(
			r.Context(), principal.AccountID, target, requestLimit(r),
		)
		writeResult(w, r, http.StatusOK, items, err)
	}
}

func requestScope(r *http.Request, principal auth.Principal) Scope {
	return Scope{
		AccountID:       principal.AccountID,
		ClientAccountID: defaultClient(principal.AccountID, r.URL.Query().Get("clientAccountId")),
	}
}

func defaultClient(accountID, clientAccountID string) string {
	clientAccountID = strings.TrimSpace(clientAccountID)
	if clientAccountID == "" {
		return accountID
	}
	return clientAccountID
}

func requestLimit(r *http.Request) int {
	value, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	return value
}

func auditEventQueryFromRequest(r *http.Request) (AuditEventQuery, error) {
	values := r.URL.Query()
	for _, key := range []string{
		"action",
		"entityType",
		"occurredFrom",
		"occurredTo",
		"cursor",
		"limit",
	} {
		if len(values[key]) > 1 {
			return AuditEventQuery{}, ErrInvalidInput
		}
	}
	limit := defaultAuditEventPageLimit
	if values.Has("limit") {
		raw := values.Get("limit")
		parsed, err := strconv.Atoi(raw)
		if err != nil ||
			raw != strconv.Itoa(parsed) ||
			parsed < 1 ||
			parsed > maxAuditEventPageLimit {
			return AuditEventQuery{}, ErrInvalidInput
		}
		limit = parsed
	}
	return AuditEventQuery{
		Action:       values.Get("action"),
		EntityType:   values.Get("entityType"),
		OccurredFrom: values.Get("occurredFrom"),
		OccurredTo:   values.Get("occurredTo"),
		Cursor:       values.Get("cursor"),
		Limit:        limit,
	}, nil
}

func requestSourceKeys(r *http.Request) []string {
	values := append([]string(nil), r.URL.Query()["sourceKey"]...)
	for _, group := range r.URL.Query()["sourceKeys"] {
		values = append(values, strings.Split(group, ",")...)
	}
	return values
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body JSON invalido.")
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body JSON invalido.")
		return false
	}
	return true
}

func writeResult(w http.ResponseWriter, r *http.Request, status int, value any, err error) {
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, status, value)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	if kind, code, retryable, ok := RuntimeFailureDetails(err); ok {
		if retryable {
			w.Header().Set("Retry-After", "1")
		}
		switch kind {
		case RuntimeFailureNotAuthorized:
			httpapi.WriteError(w, r, http.StatusForbidden, code, "Sem permissao para esta execucao.")
		case RuntimeFailureInvalidInput:
			httpapi.WriteError(w, r, http.StatusUnprocessableEntity, code, "Revise os dados da execucao.")
		case RuntimeFailureTimeout, RuntimeFailureTemporarilyUnavailable:
			httpapi.WriteError(w, r, http.StatusServiceUnavailable, code, "Runtime temporariamente indisponivel.")
		case RuntimeFailureInvalidResult:
			httpapi.WriteError(w, r, http.StatusBadGateway, code, "O provider devolveu um resultado invalido.")
		case RuntimeFailureBudgetExceeded:
			httpapi.WriteError(w, r, http.StatusTooManyRequests, code, "Orcamento da execucao excedido.")
		case RuntimeFailureDisabled, RuntimeFailurePermanent, RuntimeFailureShadowNoEffect:
			httpapi.WriteError(w, r, http.StatusConflict, code, "Runtime sem efeito operacional neste estado.")
		default:
			httpapi.WriteError(w, r, http.StatusInternalServerError, "runtime_failed", "Falha no runtime.")
		}
		return
	}
	switch {
	case errors.Is(err, ErrInvalidInput):
		httpapi.WriteError(w, r, http.StatusUnprocessableEntity, "validation_error", "Revise os dados informados.")
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para esta acao.")
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Recurso nao encontrado.")
	case errors.Is(err, ErrConflict), errors.Is(err, ErrPromptNotValidated),
		errors.Is(err, ErrAgentNotPublished):
		httpapi.WriteError(w, r, http.StatusConflict, "state_conflict", "Estado ou revisao conflitante.")
	case errors.Is(err, ErrPromptEvaluationRequired):
		httpapi.WriteError(
			w,
			r,
			http.StatusConflict,
			"prompt_evaluation_required",
			"Execute e aprove a avaliacao da versao atual antes de publicar.",
		)
	case errors.Is(err, ErrRetentionPolicyApprovalRequired):
		httpapi.WriteError(
			w,
			r,
			http.StatusConflict,
			"retention_policy_approval_required",
			"Crie e publique uma versao aprovada da politica de retencao antes de vincular a fonte.",
		)
	case errors.Is(err, ErrCapabilityDisabled):
		httpapi.WriteError(w, r, http.StatusConflict, "capability_disabled", "Capability desabilitada.")
	case errors.Is(err, ErrSecretsUnavailable):
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "secrets_unavailable", "Cofre de segredos indisponivel.")
	case errors.Is(err, ErrPromptNotPublished), errors.Is(err, ErrProviderNotConfigured):
		httpapi.WriteError(w, r, http.StatusConflict, "runtime_not_configured", "Runtime ainda nao esta publicado e configurado.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao processar a solicitacao.")
	}
}
