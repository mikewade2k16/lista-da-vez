package metaads

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestGraphActionExecutorRequiresExplicitMutationSuccess(t *testing.T) {
	t.Parallel()
	tokens := &fakeActionTokenProvider{token: "secret-token"}
	client := newFakeActionGraphClient()
	client.mutation = graphActionMutation{Success: false}
	executor := &graphActionExecutor{tokens: tokens, client: client}

	_, err := executor.Execute(context.Background(), executableActionProposal(ActionPauseCampaign))
	var executorErr *ActionExecutorError
	if !errors.As(err, &executorErr) || !executorErr.Ambiguous || executorErr.Code != "execution_outcome_unknown" {
		t.Fatalf("Execute() error = %#v", err)
	}
	if client.getCalls != 1 || client.updateCalls != 1 {
		t.Fatalf("Graph calls = GET %d, POST %d", client.getCalls, client.updateCalls)
	}
}

func TestGraphActionExecutorResumesOnlyKnownCBOBudgetWithinLiveCap(t *testing.T) {
	t.Parallel()
	tokens := &fakeActionTokenProvider{token: "secret-token"}
	client := newFakeActionGraphClient()
	client.current.ConfiguredStatus = "PAUSED"
	executor := &graphActionExecutor{tokens: tokens, client: client}
	proposal := executableActionProposal(ActionResumeCampaign)
	proposal.CampaignStatusSnapshot = "PAUSED"
	capValue := 12.0
	proposal.PolicyMaxDailySnapshot = &capValue

	if !executor.Supports(ActionResumeCampaign) {
		t.Fatal("resume should be available for live-verifiable CBO campaigns")
	}
	outcome, err := executor.Execute(context.Background(), proposal)
	if err != nil || outcome.Status != ActionSucceeded {
		t.Fatalf("Execute(resume) = %#v, err %#v", outcome, err)
	}
	if tokens.calls != 1 || client.getCalls != 1 || client.updateCalls != 1 || client.values.Get("status") != "ACTIVE" {
		t.Fatalf("resume calls = token %d, GET %d, POST %d, values %v", tokens.calls, client.getCalls, client.updateCalls, client.values)
	}
}

func TestGraphActionExecutorBlocksResumeWhenLiveBudgetCannotBeProved(t *testing.T) {
	t.Parallel()
	tokens := &fakeActionTokenProvider{token: "secret-token"}
	client := newFakeActionGraphClient()
	client.current.ConfiguredStatus = "PAUSED"
	client.current.DailyBudget = ""
	executor := &graphActionExecutor{tokens: tokens, client: client}
	proposal := executableActionProposal(ActionResumeCampaign)
	proposal.CampaignStatusSnapshot = "PAUSED"
	proposal.CampaignDailySnapshot = nil

	_, err := executor.Execute(context.Background(), proposal)
	var executorErr *ActionExecutorError
	if !errors.As(err, &executorErr) || executorErr.Code != "live_budget_unavailable" || executorErr.Ambiguous {
		t.Fatalf("Execute(resume) error = %#v", err)
	}
	if client.getCalls != 1 || client.updateCalls != 0 {
		t.Fatalf("Graph calls = GET %d, POST %d", client.getCalls, client.updateCalls)
	}
}

func TestGraphActionExecutorCreatesCampaignPaused(t *testing.T) {
	t.Parallel()
	tokens := &fakeActionTokenProvider{token: "secret-token"}
	client := newFakeActionGraphClient()
	client.created = graphActionCreate{ID: "987654321"}
	executor := &graphActionExecutor{tokens: tokens, client: client}
	proposal := executableActionProposal(ActionCreateCampaign)
	proposal.MetaAdAccountID = "123456789"
	proposal.TargetMetaCampaignID = ""
	proposal.Payload = []byte(`{"name":"Campanha segura","objective":"OUTCOME_TRAFFIC","specialAdCategories":[],"budget":{"type":"daily","amount":10},"status":"PAUSED"}`)
	capValue := 20.0
	proposal.PolicyMaxDailySnapshot = &capValue

	outcome, err := executor.Execute(context.Background(), proposal)
	if err != nil || outcome.Status != ActionSucceeded || outcome.ExternalEntityID != "987654321" {
		t.Fatalf("Execute(create) = %#v, err %#v", outcome, err)
	}
	if client.createCalls != 1 || client.getCalls != 0 || client.updateCalls != 0 {
		t.Fatalf("Graph calls = create %d, GET %d, update %d", client.createCalls, client.getCalls, client.updateCalls)
	}
	if client.values.Get("status") != "PAUSED" || client.values.Get("daily_budget") != "1000" ||
		client.values.Get("special_ad_categories") != "[]" {
		t.Fatalf("create values = %v", client.values)
	}
}

func TestGraphActionExecutorDeepCopiesCampaignPaused(t *testing.T) {
	t.Parallel()
	tokens := &fakeActionTokenProvider{token: "secret-token"}
	client := newFakeActionGraphClient()
	client.copied = graphActionCopy{CopiedCampaignID: "987654321"}
	executor := &graphActionExecutor{tokens: tokens, client: client}
	proposal := executableActionProposal(ActionDuplicateCampaign)
	proposal.Payload = []byte(`{"campaignId":"` + actionTestCampaign + `","name":"Copia controlada","status":"PAUSED"}`)

	outcome, err := executor.Execute(context.Background(), proposal)
	if err != nil || outcome.Status != ActionSucceeded || outcome.ExternalEntityID != "987654321" {
		t.Fatalf("Execute(copy) = %#v, err %#v", outcome, err)
	}
	if client.getCalls != 1 || client.copyCalls != 1 || client.updateCalls != 0 {
		t.Fatalf("Graph calls = GET %d, copy %d, update %d", client.getCalls, client.copyCalls, client.updateCalls)
	}
	if client.values.Get("deep_copy") != "true" || client.values.Get("status_option") != "PAUSED" ||
		!strings.Contains(client.values.Get("parameter_overrides"), "Copia controlada") {
		t.Fatalf("copy values = %v", client.values)
	}
}

func TestGraphActionExecutorSendsCanonicalMinorUnitsAfterLivePreflight(t *testing.T) {
	t.Parallel()
	tokens := &fakeActionTokenProvider{token: "secret-token"}
	client := newFakeActionGraphClient()
	client.current.Name = "Campanha antiga"
	client.current.DailyBudget = "500"
	executor := &graphActionExecutor{tokens: tokens, client: client}
	proposal := executableActionProposal(ActionUpdateCampaign)
	proposal.CampaignNameSnapshot = "Campanha antiga"
	oldBudget := 5.0
	proposal.CampaignDailySnapshot = &oldBudget
	proposal.Payload = []byte(`{"campaignId":"` + actionTestCampaign + `","name":"Campanha nova","budget":{"type":"daily","amount":10.25}}`)

	outcome, err := executor.Execute(context.Background(), proposal)
	if err != nil || outcome.Status != ActionSucceeded {
		t.Fatalf("Execute() = %#v, err %v", outcome, err)
	}
	if client.values.Get("name") != "Campanha nova" || client.values.Get("daily_budget") != "1025" {
		t.Fatalf("Graph values = %v", client.values)
	}
	if tokens.connectionID != actionTestConnection || tokens.revision != actionTestRevision {
		t.Fatalf("lease = connection %q revision %q", tokens.connectionID, tokens.revision)
	}
}

func TestGraphActionExecutorUsesDesiredStateWithoutDuplicatePost(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		proposal ActionProposal
		current  graphActionCampaign
	}{
		{
			name:     "pause already applied",
			proposal: executableActionProposal(ActionPauseCampaign),
			current: graphActionCampaign{
				ID: actionTestMetaCampaign, Name: "Campanha", ConfiguredStatus: "PAUSED", DailyBudget: "1000",
			},
		},
		{
			name: "update already applied",
			proposal: func() ActionProposal {
				proposal := executableActionProposal(ActionUpdateCampaign)
				proposal.Payload = []byte(`{"campaignId":"` + actionTestCampaign + `","name":"Nome final","budget":{"type":"daily","amount":10}}`)
				return proposal
			}(),
			current: graphActionCampaign{
				ID: actionTestMetaCampaign, Name: "Nome final", ConfiguredStatus: "ACTIVE", DailyBudget: "1000",
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			tokens := &fakeActionTokenProvider{token: "secret-token"}
			client := newFakeActionGraphClient()
			client.current = test.current
			executor := &graphActionExecutor{tokens: tokens, client: client}

			outcome, err := executor.Execute(context.Background(), test.proposal)
			if err != nil || outcome.Status != ActionSucceeded {
				t.Fatalf("Execute() = %#v, err %v", outcome, err)
			}
			if client.getCalls != 1 || client.updateCalls != 0 {
				t.Fatalf("Graph calls = GET %d, POST %d", client.getCalls, client.updateCalls)
			}
			var result map[string]any
			if err := json.Unmarshal(outcome.Result, &result); err != nil || result["alreadyApplied"] != true {
				t.Fatalf("result = %s, err %v", outcome.Result, err)
			}
		})
	}
}

func TestGraphActionExecutorFailsClosedBeforePostOnLiveDrift(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		proposal ActionProposal
		mutate   func(*graphActionCampaign)
	}{
		{
			name:     "pause status drift",
			proposal: executableActionProposal(ActionPauseCampaign),
			mutate: func(current *graphActionCampaign) {
				current.ConfiguredStatus = "ARCHIVED"
			},
		},
		{
			name: "update name drift",
			proposal: func() ActionProposal {
				proposal := executableActionProposal(ActionUpdateCampaign)
				proposal.Payload = []byte(`{"campaignId":"` + actionTestCampaign + `","name":"Nome final"}`)
				return proposal
			}(),
			mutate: func(current *graphActionCampaign) {
				current.Name = "Alterada fora do Omni"
			},
		},
		{
			name: "update budget drift",
			proposal: func() ActionProposal {
				proposal := executableActionProposal(ActionUpdateCampaign)
				proposal.Payload = []byte(`{"campaignId":"` + actionTestCampaign + `","budget":{"type":"daily","amount":20}}`)
				return proposal
			}(),
			mutate: func(current *graphActionCampaign) {
				current.DailyBudget = "1500"
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			tokens := &fakeActionTokenProvider{token: "secret-token"}
			client := newFakeActionGraphClient()
			test.mutate(&client.current)
			executor := &graphActionExecutor{tokens: tokens, client: client}

			_, err := executor.Execute(context.Background(), test.proposal)
			var executorErr *ActionExecutorError
			if !errors.As(err, &executorErr) || executorErr.Code != "proposal_stale_live_target" || executorErr.Ambiguous {
				t.Fatalf("Execute() error = %#v", err)
			}
			if client.getCalls != 1 || client.updateCalls != 0 {
				t.Fatalf("Graph calls = GET %d, POST %d", client.getCalls, client.updateCalls)
			}
		})
	}
}

func TestGraphActionExecutorRejectsStaleClaimAndRevisionWithoutGraph(t *testing.T) {
	t.Parallel()
	t.Run("claim mismatch", func(t *testing.T) {
		t.Parallel()
		tokens := &fakeActionTokenProvider{token: "secret-token"}
		client := newFakeActionGraphClient()
		executor := &graphActionExecutor{tokens: tokens, client: client}
		proposal := executableActionProposal(ActionPauseCampaign)
		otherRevision := "22222222-2222-4222-8222-222222222222"
		proposal.ClaimedConnectionRevision = &otherRevision

		_, err := executor.Execute(context.Background(), proposal)
		var executorErr *ActionExecutorError
		if !errors.As(err, &executorErr) || executorErr.Code != "proposal_claim_stale" {
			t.Fatalf("Execute() error = %#v", err)
		}
		if tokens.calls != 0 || client.getCalls != 0 || client.updateCalls != 0 {
			t.Fatalf("calls = token %d, GET %d, POST %d", tokens.calls, client.getCalls, client.updateCalls)
		}
	})

	t.Run("connection revision rotated", func(t *testing.T) {
		t.Parallel()
		tokens := &fakeActionTokenProvider{err: ErrConnectionChanged}
		client := newFakeActionGraphClient()
		executor := &graphActionExecutor{tokens: tokens, client: client}

		_, err := executor.Execute(context.Background(), executableActionProposal(ActionPauseCampaign))
		var executorErr *ActionExecutorError
		if !errors.As(err, &executorErr) || executorErr.Code != "connection_revision_stale" || executorErr.Ambiguous {
			t.Fatalf("Execute() error = %#v", err)
		}
		if client.getCalls != 0 || client.updateCalls != 0 {
			t.Fatalf("Graph calls = GET %d, POST %d", client.getCalls, client.updateCalls)
		}
	})
}

func TestGraphActionExecutorRejectsNonBRLBudgetBeforeTokenOrGraph(t *testing.T) {
	t.Parallel()
	tokens := &fakeActionTokenProvider{token: "secret-token"}
	client := newFakeActionGraphClient()
	executor := &graphActionExecutor{tokens: tokens, client: client}
	proposal := executableActionProposal(ActionUpdateCampaign)
	proposal.Currency = "JPY"
	proposal.PolicyCurrencySnapshot = "JPY"
	proposal.Payload = []byte(`{"campaignId":"` + actionTestCampaign + `","budget":{"type":"daily","amount":1000}}`)

	_, err := executor.Execute(context.Background(), proposal)
	var executorErr *ActionExecutorError
	if !errors.As(err, &executorErr) || executorErr.Code != "unsupported_budget_currency" || executorErr.Ambiguous {
		t.Fatalf("Execute() error = %#v", err)
	}
	if tokens.calls != 0 || client.getCalls != 0 || client.updateCalls != 0 {
		t.Fatalf("calls = token %d, GET %d, POST %d", tokens.calls, client.getCalls, client.updateCalls)
	}
}

func TestActionTokenLeaseSerializesRotationAndDeletionDuringGraph(t *testing.T) {
	for _, operation := range []string{"rotate", "delete"} {
		operation := operation
		t.Run(operation, func(t *testing.T) {
			provider := newSerialActionTokenProvider()
			client := newFakeActionGraphClient()
			client.updateEntered = make(chan struct{})
			client.releaseUpdate = make(chan struct{})
			executor := &graphActionExecutor{tokens: provider, client: client}
			executionDone := make(chan error, 1)
			go func() {
				_, err := executor.Execute(context.Background(), executableActionProposal(ActionPauseCampaign))
				executionDone <- err
			}()

			select {
			case <-client.updateEntered:
			case <-time.After(time.Second):
				t.Fatal("Graph mutation did not start")
			}
			mutationDone := make(chan struct{})
			go func() {
				provider.mutateConnection(operation)
				close(mutationDone)
			}()
			select {
			case <-mutationDone:
				t.Fatalf("%s bypassed active token lease", operation)
			case <-time.After(50 * time.Millisecond):
			}

			close(client.releaseUpdate)
			select {
			case err := <-executionDone:
				if err != nil {
					t.Fatalf("Execute() error = %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("execution did not finish")
			}
			select {
			case <-mutationDone:
			case <-time.After(time.Second):
				t.Fatalf("%s did not resume after lease release", operation)
			}
		})
	}
}

func TestGraphActionErrorsAfterRequestAreClassifiedConservatively(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		status    int
		ambiguous bool
		code      string
	}{
		{name: "network", status: 0, ambiguous: true, code: "execution_outcome_unknown"},
		{name: "invalid 2xx body", status: http.StatusOK, ambiguous: true, code: "execution_outcome_unknown"},
		{name: "server error", status: http.StatusBadGateway, ambiguous: true, code: "execution_outcome_unknown"},
		{name: "request timeout", status: http.StatusRequestTimeout, ambiguous: true, code: "execution_outcome_unknown"},
		{name: "throttled", status: http.StatusTooManyRequests, ambiguous: true, code: "execution_outcome_unknown"},
		{name: "definitive rejection", status: http.StatusBadRequest, ambiguous: false, code: "meta_rejected"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			classified := classifyGraphActionError(&graphActionRequestError{
				status: test.status, cause: errors.New("graph request failed"),
			}, false)
			if classified.Ambiguous != test.ambiguous || classified.Code != test.code {
				t.Fatalf("classified = %#v", classified)
			}
		})
	}
}

func TestMetaClientInvalidSuccessBodyIsAmbiguous(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer secret-token" {
			t.Errorf("Authorization = %q", request.Header.Get("Authorization"))
		}
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte(`{"success":`))
	}))
	t.Cleanup(server.Close)
	client := NewMetaClient(server.URL)

	_, err := client.UpdateCampaignAction(
		context.Background(), "secret-token", "123456789", url.Values{"status": {"PAUSED"}},
	)
	classified := classifyGraphActionError(err, false)
	if classified == nil || !classified.Ambiguous || classified.Code != "execution_outcome_unknown" {
		t.Fatalf("classified = %#v, raw err %v", classified, err)
	}
}

func TestMetaClientReadFailureAfterSuccessStatusIsAmbiguous(t *testing.T) {
	t.Parallel()
	client := &MetaClient{
		base: "https://graph.invalid",
		http: &http.Client{Transport: roundTripActionFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       &failingActionBody{err: errors.New("body read failed")},
				Header:     make(http.Header),
			}, nil
		})},
	}

	_, err := client.UpdateCampaignAction(
		context.Background(), "secret-token", "123456789", url.Values{"status": {"PAUSED"}},
	)
	classified := classifyGraphActionError(err, false)
	if classified == nil || !classified.Ambiguous || classified.Code != "execution_outcome_unknown" {
		t.Fatalf("classified = %#v, raw err %v", classified, err)
	}
}

func TestGraphActionExecutorPromotesLiveInstagramPostAsFullyPausedTree(t *testing.T) {
	t.Parallel()

	tokens := &fakeActionTokenProvider{token: "secret-token"}
	client := newFakeActionGraphClient()
	client.created = graphActionCreate{ID: "11223344"}
	client.pages = []GraphInstagramPage{{PageID: "55667788", IGUserID: "66778899"}}
	client.media = []GraphInstagramMedia{{ID: "77889900"}}
	steps := newFakeActionStepRepository()
	scope := &fakeInstagramActionScope{}
	executor := &graphActionExecutor{
		tokens: tokens, client: client, steps: steps, instagramScopes: scope,
	}
	proposal := executableActionProposal(ActionPromoteInstagramPost)
	proposal.ID = actionTestProposal
	proposal.AccountID = actionTestAccount
	proposal.AdAccountID = actionTestAdAccount
	proposal.MetaAdAccountID = "987654321"
	proposal.Payload = json.RawMessage(`{
		"name":"Campanha post","instagramPostId":"77889900","igUserId":"66778899",
		"pageId":"55667788","adSetName":"Conjunto post","adName":"Anuncio post",
		"budget":{"type":"daily","amount":25},"countries":["BR"],
		"ageMin":18,"ageMax":65,"status":"PAUSED"
	}`)

	outcome, err := executor.Execute(context.Background(), proposal)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if outcome.Status != ActionSucceeded || outcome.ExternalEntityID != "44556677" {
		t.Fatalf("outcome = %#v", outcome)
	}
	if scope.calls != 1 || client.createCalls != 1 || client.adSetCalls != 1 ||
		client.creativeCalls != 1 || client.adCalls != 1 || len(steps.rows) != 4 {
		t.Fatalf("scope=%d calls create/adset/creative/ad=%d/%d/%d/%d steps=%d",
			scope.calls, client.createCalls, client.adSetCalls, client.creativeCalls,
			client.adCalls, len(steps.rows))
	}
	for label, values := range map[string]url.Values{
		"campaign": client.campaignValues,
		"adset":    client.adSetValues,
		"ad":       client.adValues,
	} {
		if values.Get("status") != "PAUSED" {
			t.Errorf("%s status = %q", label, values.Get("status"))
		}
	}
	if client.adSetValues.Get("daily_budget") != "2500" ||
		!strings.Contains(client.adSetValues.Get("targeting"), `"countries":["BR"]`) ||
		client.creativeValues.Get("source_instagram_media_id") != "77889900" {
		t.Fatalf("adset=%#v creative=%#v", client.adSetValues, client.creativeValues)
	}
}

func TestGraphActionExecutorDoesNotCreateWhenInstagramSourceDrifted(t *testing.T) {
	t.Parallel()

	client := newFakeActionGraphClient()
	steps := newFakeActionStepRepository()
	executor := &graphActionExecutor{
		tokens: &fakeActionTokenProvider{token: "secret-token"}, client: client,
		steps: steps, instagramScopes: &fakeInstagramActionScope{err: pgx.ErrNoRows},
	}
	proposal := executableActionProposal(ActionPromoteInstagramPost)
	proposal.ID = actionTestProposal
	proposal.AccountID = actionTestAccount
	proposal.AdAccountID = actionTestAdAccount
	proposal.MetaAdAccountID = "987654321"
	proposal.Payload = json.RawMessage(`{
		"name":"Campanha post","instagramPostId":"77889900","igUserId":"66778899",
		"pageId":"55667788","adSetName":"Conjunto post","adName":"Anuncio post",
		"budget":{"type":"daily","amount":25},"countries":["BR"],
		"ageMin":18,"ageMax":65,"status":"PAUSED"
	}`)

	_, err := executor.Execute(context.Background(), proposal)
	var executorErr *ActionExecutorError
	if !errors.As(err, &executorErr) || executorErr.Code != "proposal_stale_live_target" {
		t.Fatalf("error = %#v", err)
	}
	if client.createCalls != 0 || len(steps.rows) != 0 {
		t.Fatalf("createCalls=%d steps=%d", client.createCalls, len(steps.rows))
	}
}

const (
	actionTestConnection   = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	actionTestRevision     = "11111111-1111-4111-8111-111111111111"
	actionTestMetaCampaign = "123456789"
)

func executableActionProposal(action ActionKind) ActionProposal {
	connectionID := actionTestConnection
	revision := actionTestRevision
	daily := 10.0
	return ActionProposal{
		Action: action, ResourceAccountID: actionTestResource,
		TargetMetaCampaignID: actionTestMetaCampaign,
		Currency:             "BRL", PolicyCurrencySnapshot: "BRL",
		GuardSnapshotVersion:       actionGuardSnapshotVersion,
		ConnectionIDSnapshot:       &connectionID,
		ConnectionRevisionSnapshot: &revision,
		ClaimedConnectionID:        &connectionID,
		ClaimedConnectionRevision:  &revision,
		CampaignNameSnapshot:       "Campanha",
		CampaignStatusSnapshot:     "ACTIVE",
		CampaignDailySnapshot:      &daily,
	}
}

type fakeActionTokenProvider struct {
	token        string
	err          error
	calls        int
	accountID    string
	connectionID string
	revision     string
}

func (p *fakeActionTokenProvider) WithDecryptedTokenAtRevision(
	_ context.Context,
	accountID, connectionID, revision string,
	use func(string) error,
) error {
	p.calls++
	p.accountID = accountID
	p.connectionID = connectionID
	p.revision = revision
	if p.err != nil {
		return p.err
	}
	return use(p.token)
}

type serialActionTokenProvider struct {
	mu       sync.Mutex
	token    string
	revision string
	deleted  bool
}

func newSerialActionTokenProvider() *serialActionTokenProvider {
	return &serialActionTokenProvider{token: "secret-token", revision: actionTestRevision}
}

func (p *serialActionTokenProvider) WithDecryptedTokenAtRevision(
	_ context.Context,
	_, _ string,
	revision string,
	use func(string) error,
) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.deleted || revision != p.revision {
		return ErrConnectionChanged
	}
	return use(p.token)
}

func (p *serialActionTokenProvider) mutateConnection(operation string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if operation == "delete" {
		p.deleted = true
		return
	}
	p.revision = "33333333-3333-4333-8333-333333333333"
}

type fakeActionGraphClient struct {
	mutation       graphActionMutation
	created        graphActionCreate
	copied         graphActionCopy
	err            error
	createErr      error
	copyErr        error
	current        graphActionCampaign
	currentErr     error
	values         url.Values
	campaignValues url.Values
	adSetValues    url.Values
	creativeValues url.Values
	adValues       url.Values
	getCalls       int
	updateCalls    int
	createCalls    int
	copyCalls      int
	adSetCalls     int
	creativeCalls  int
	adCalls        int
	pages          []GraphInstagramPage
	media          []GraphInstagramMedia
	updateEntered  chan struct{}
	releaseUpdate  chan struct{}
}

func newFakeActionGraphClient() *fakeActionGraphClient {
	return &fakeActionGraphClient{
		mutation: graphActionMutation{Success: true},
		current: graphActionCampaign{
			ID: actionTestMetaCampaign, Name: "Campanha",
			ConfiguredStatus: "ACTIVE", DailyBudget: "1000",
		},
	}
}

func (c *fakeActionGraphClient) UpdateCampaignAction(
	_ context.Context,
	_, _ string,
	values url.Values,
) (graphActionMutation, error) {
	c.updateCalls++
	c.values = values
	if c.updateEntered != nil {
		close(c.updateEntered)
	}
	if c.releaseUpdate != nil {
		<-c.releaseUpdate
	}
	return c.mutation, c.err
}

func (c *fakeActionGraphClient) CreateCampaignAction(
	_ context.Context,
	_, _ string,
	values url.Values,
) (graphActionCreate, error) {
	c.createCalls++
	c.values = values
	c.campaignValues = values
	return c.created, c.createErr
}

func (c *fakeActionGraphClient) CopyCampaignAction(
	_ context.Context,
	_, _ string,
	values url.Values,
) (graphActionCopy, error) {
	c.copyCalls++
	c.values = values
	return c.copied, c.copyErr
}

func (c *fakeActionGraphClient) CreateAdSetAction(
	_ context.Context, _, _ string, values url.Values,
) (graphActionCreate, error) {
	c.adSetCalls++
	c.adSetValues = values
	return graphActionCreate{ID: "22334455"}, nil
}

func (c *fakeActionGraphClient) CreateAdCreativeAction(
	_ context.Context, _, _ string, values url.Values,
) (graphActionCreate, error) {
	c.creativeCalls++
	c.creativeValues = values
	return graphActionCreate{ID: "33445566"}, nil
}

func (c *fakeActionGraphClient) CreateAdAction(
	_ context.Context, _, _ string, values url.Values,
) (graphActionCreate, error) {
	c.adCalls++
	c.adValues = values
	return graphActionCreate{ID: "44556677"}, nil
}

func (c *fakeActionGraphClient) ListPagesWithInstagram(
	context.Context, string,
) ([]GraphInstagramPage, error) {
	return c.pages, nil
}

func (c *fakeActionGraphClient) ListInstagramMedia(
	context.Context, string, string, int,
) ([]GraphInstagramMedia, error) {
	return c.media, nil
}

func (c *fakeActionGraphClient) GetCampaignAction(
	context.Context,
	string,
	string,
) (graphActionCampaign, error) {
	c.getCalls++
	return c.current, c.currentErr
}

type roundTripActionFunc func(*http.Request) (*http.Response, error)

func (fn roundTripActionFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type failingActionBody struct {
	err error
}

func (b *failingActionBody) Read([]byte) (int, error) { return 0, b.err }
func (b *failingActionBody) Close() error             { return nil }

var _ io.ReadCloser = (*failingActionBody)(nil)

type fakeInstagramActionScope struct {
	calls int
	err   error
}

func (s *fakeInstagramActionScope) ValidateActionInstagramIdentityScope(
	context.Context, string, string, string, string, string, string,
) error {
	s.calls++
	return s.err
}

type fakeActionStepRepository struct {
	rows map[actionStepName]actionStep
}

func newFakeActionStepRepository() *fakeActionStepRepository {
	return &fakeActionStepRepository{rows: make(map[actionStepName]actionStep)}
}

func (r *fakeActionStepRepository) BeginActionStep(
	_ context.Context,
	accountID, proposalID string,
	step actionStepName,
	requestHash string,
) (actionStep, bool, error) {
	if existing, ok := r.rows[step]; ok {
		if existing.RequestHash != requestHash {
			return actionStep{}, false, ErrActionIdempotencyConflict
		}
		if existing.Status != ActionSucceeded {
			return existing, false, ErrActionStepUncertain
		}
		return existing, false, nil
	}
	row := actionStep{
		ID: string(step), AccountID: accountID, ProposalID: proposalID,
		Step: step, RequestHash: requestHash, Status: ActionExecuting,
	}
	r.rows[step] = row
	return row, true, nil
}

func (r *fakeActionStepRepository) CompleteActionStep(
	_ context.Context,
	_, _ string,
	step actionStepName,
	requestHash string,
	outcome ActionExecutionOutcome,
) (actionStep, error) {
	row, ok := r.rows[step]
	if !ok || row.RequestHash != requestHash || row.Status != ActionExecuting {
		return actionStep{}, pgx.ErrNoRows
	}
	row.Status = outcome.Status
	row.ExternalEntityID = outcome.ExternalEntityID
	row.Result = outcome.Result
	row.ErrorCode = outcome.ErrorCode
	row.ErrorMessage = outcome.ErrorMessage
	r.rows[step] = row
	return row, nil
}

func (r *fakeActionStepRepository) ListActionSteps(
	context.Context, string, string,
) ([]actionStep, error) {
	out := make([]actionStep, 0, len(r.rows))
	for _, name := range []actionStepName{actionStepCampaign, actionStepAdSet, actionStepCreative, actionStepAd} {
		if row, ok := r.rows[name]; ok {
			out = append(out, row)
		}
	}
	return out, nil
}
