package calendar

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

const (
	metaActionAccountOne = "11111111-1111-4111-8111-111111111111"
	metaActionAccountTwo = "22222222-2222-4222-8222-222222222222"
	metaActionCampaign   = "33333333-3333-4333-8333-333333333333"
	metaActionProposal   = "44444444-4444-4444-8444-444444444444"
	metaActionClient     = "55555555-5555-4555-8555-555555555555"
)

func TestPrepareMetaActionProposalsDerivesInstagramSourceFromAuthoritativeContext(t *testing.T) {
	t.Parallel()
	proposal := sanitizeProposal(&ChatProposal{
		Kind: "metaAction", Fields: ChatProposalFields{MetaAction: &ChatProposalMetaAction{
			Action: "promote_instagram_post", AdAccountID: metaActionAccountOne,
			Name: "Post patrocinado", InstagramPostID: "77889900",
			Budget: &ChatProposalMetaBudget{Type: "daily", Amount: 25},
		}},
	})
	if proposal == nil {
		t.Fatal("valid Instagram promotion was dropped")
	}
	var captured MetaAssistantActionRequest
	result, dropped, err := prepareMetaActionProposals(
		context.Background(), "tenant-1", auth.Principal{UserID: "user-1"},
		"conversation-1", "message-1", []ChatProposal{*proposal},
		MetaAssistantContextResult{
			ActionAdAccounts: []MetaAssistantActionAdAccount{{
				ID: metaActionAccountOne, Name: "Conta", Currency: "BRL", ClientAccountID: metaActionClient,
			}},
			ActionInstagramPosts: []MetaAssistantActionInstagramPost{{
				ID: "77889900", Title: "Post real", IGUserID: "66778899", PageID: "55667788",
				ClientAccountID: metaActionClient,
			}},
		},
		func(_ context.Context, req MetaAssistantActionRequest) (MetaAssistantActionResult, error) {
			captured = req
			return MetaAssistantActionResult{
				ID: metaActionProposal, Status: "pending", ExecutionAvailable: true,
				CanConfirm: true, RequiresSpendAcknowledgement: true,
			}, nil
		},
	)
	if err != nil || dropped != 0 || len(result) != 1 {
		t.Fatalf("result=%#v dropped=%d err=%v", result, dropped, err)
	}
	var payload metaPromoteInstagramPostPayload
	if err := json.Unmarshal(captured.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.InstagramPostID != "77889900" || payload.IGUserID != "66778899" ||
		payload.PageID != "55667788" || payload.ClientAccountID != metaActionClient ||
		payload.Status != "PAUSED" || payload.AdSetName != "Post patrocinado - conjunto" ||
		payload.AdName != "Post patrocinado - anuncio" ||
		!reflect.DeepEqual(payload.Countries, []string{"BR"}) || payload.AgeMin != 18 || payload.AgeMax != 65 {
		t.Fatalf("authoritative payload = %#v", payload)
	}
	card := result[0].Fields.MetaAction
	if card == nil || card.IGUserID != "66778899" || card.PageID != "55667788" ||
		card.InstagramPostTitle != "Post real" || !card.CanConfirm {
		t.Fatalf("card = %#v", card)
	}
}

func TestPrepareMetaActionProposalsDropsUnknownInstagramPost(t *testing.T) {
	t.Parallel()
	proposal := sanitizeProposal(&ChatProposal{
		Kind: "metaAction", Fields: ChatProposalFields{MetaAction: &ChatProposalMetaAction{
			Action: "promote_instagram_post", AdAccountID: metaActionAccountOne,
			Name: "Post forjado", InstagramPostID: "77889900",
			Budget: &ChatProposalMetaBudget{Type: "daily", Amount: 25},
		}},
	})
	called := false
	result, dropped, err := prepareMetaActionProposals(
		context.Background(), "tenant-1", auth.Principal{UserID: "user-1"},
		"conversation-1", "message-1", []ChatProposal{*proposal},
		MetaAssistantContextResult{ActionAdAccounts: []MetaAssistantActionAdAccount{{
			ID: metaActionAccountOne, Name: "Conta", Currency: "BRL",
		}}},
		func(context.Context, MetaAssistantActionRequest) (MetaAssistantActionResult, error) {
			called = true
			return MetaAssistantActionResult{}, nil
		},
	)
	if err != nil || dropped != 1 || len(result) != 0 || called {
		t.Fatalf("result=%#v dropped=%d called=%v err=%v", result, dropped, called, err)
	}
}

func TestPrepareMetaActionProposalsDerivesAccountFromAuthoritativeCampaign(t *testing.T) {
	t.Parallel()
	intent := &ChatProposalMetaAction{
		Action: "update_campaign", AdAccountID: metaActionAccountTwo,
		CampaignID: metaActionCampaign, Name: "  Campanha   segura  ",
		Budget: &ChatProposalMetaBudget{Type: "daily", Amount: 125.50},
	}
	proposal := sanitizeProposal(&ChatProposal{
		Kind: "metaAction", Fields: ChatProposalFields{MetaAction: intent},
	})
	if proposal == nil {
		t.Fatal("a intencao valida foi descartada")
	}

	var captured MetaAssistantActionRequest
	provider := func(_ context.Context, req MetaAssistantActionRequest) (MetaAssistantActionResult, error) {
		captured = req
		return MetaAssistantActionResult{
			ID: metaActionProposal, Status: "pending", Summary: "Atualizar campanha.",
			ExecutionAvailable: true, CanConfirm: true, RequiresSpendAcknowledgement: true,
		}, nil
	}
	result, dropped, err := prepareMetaActionProposals(
		context.Background(), "tenant-1",
		auth.Principal{UserID: "user-1"}, "conversation-1", "message-1",
		[]ChatProposal{*proposal}, MetaAssistantContextResult{
			ActionAdAccounts: []MetaAssistantActionAdAccount{
				{ID: metaActionAccountOne, Name: "Conta correta", Currency: "brl"},
				{ID: metaActionAccountTwo, Name: "Outra conta", Currency: "usd"},
			},
			ActionCampaigns: []MetaAssistantActionCampaign{
				{ID: metaActionCampaign, AdAccountID: metaActionAccountOne, Name: "Campanha atual"},
			},
		}, provider,
	)
	if err != nil || dropped != 0 || len(result) != 1 {
		t.Fatalf("result=%#v dropped=%d err=%v", result, dropped, err)
	}
	if captured.AccountID != "tenant-1" || captured.ActorUserID != "user-1" ||
		captured.ConversationID != "conversation-1" || captured.MessageID != "message-1" {
		t.Fatalf("identidade/scope incorretos: %#v", captured)
	}
	if captured.AdAccountID != metaActionAccountOne {
		t.Fatalf("ad account do modelo venceu o contexto: %q", captured.AdAccountID)
	}
	if !reflect.DeepEqual(captured.AllowedAdAccountIDs, []string{metaActionAccountOne, metaActionAccountTwo}) {
		t.Fatalf("allowlist inesperada: %#v", captured.AllowedAdAccountIDs)
	}
	var payload map[string]any
	if err := json.Unmarshal(captured.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload) != 3 || payload["campaignId"] != metaActionCampaign || payload["name"] != "Campanha segura" {
		t.Fatalf("payload nao canonico: %#v", payload)
	}
	card := result[0].Fields.MetaAction
	if card == nil || card.AdAccountID != metaActionAccountOne || card.AdAccountName != "Conta correta" ||
		card.CampaignName != "Campanha atual" || !card.CanConfirm || !card.RequiresSpendAcknowledgement {
		t.Fatalf("card inesperado: %#v", card)
	}
}

func TestPrepareMetaActionProposalsKeepsExpectedUnavailableCard(t *testing.T) {
	t.Parallel()
	proposal := sanitizeProposal(&ChatProposal{
		Kind: "metaAction",
		Fields: ChatProposalFields{MetaAction: &ChatProposalMetaAction{
			Action: "create_campaign", AdAccountID: metaActionAccountOne,
			Name: "Nova campanha", Objective: "OUTCOME_TRAFFIC",
			SpecialAdCategories: []string{"NONE"},
		}},
	})
	if proposal == nil {
		t.Fatal("a intencao valida foi descartada")
	}
	provider := func(context.Context, MetaAssistantActionRequest) (MetaAssistantActionResult, error) {
		return MetaAssistantActionResult{
			Action: "create_campaign", AdAccountID: metaActionAccountOne, Status: "pending",
			ErrorCode:    "action_policy_required",
			ErrorMessage: "Configure os limites financeiros antes de confirmar.",
		}, nil
	}
	result, dropped, err := prepareMetaActionProposals(
		context.Background(), "tenant-1", auth.Principal{UserID: "user-1"},
		"conversation-1", "message-1", []ChatProposal{*proposal},
		MetaAssistantContextResult{ActionAdAccounts: []MetaAssistantActionAdAccount{
			{ID: metaActionAccountOne, Name: "Conta", Currency: "BRL"},
		}}, provider,
	)
	if err != nil || dropped != 0 || len(result) != 1 {
		t.Fatalf("result=%#v dropped=%d err=%v", result, dropped, err)
	}
	card := result[0].Fields.MetaAction
	if card == nil || card.ActionProposalID != "" || card.CanConfirm || card.ExecutionAvailable ||
		card.ErrorCode != "action_policy_required" {
		t.Fatalf("card bloqueado inesperado: %#v", card)
	}
}

func TestPrepareMetaActionProposalsDropsCampaignOutsideContext(t *testing.T) {
	t.Parallel()
	proposal := sanitizeProposal(&ChatProposal{
		Kind: "metaAction", Fields: ChatProposalFields{MetaAction: &ChatProposalMetaAction{
			Action: "pause_campaign", CampaignID: metaActionCampaign,
		}},
	})
	called := false
	provider := func(context.Context, MetaAssistantActionRequest) (MetaAssistantActionResult, error) {
		called = true
		return MetaAssistantActionResult{}, nil
	}
	result, dropped, err := prepareMetaActionProposals(
		context.Background(), "tenant-1", auth.Principal{UserID: "user-1"},
		"conversation-1", "message-1", []ChatProposal{*proposal},
		MetaAssistantContextResult{ActionAdAccounts: []MetaAssistantActionAdAccount{
			{ID: metaActionAccountOne, Name: "Conta", Currency: "BRL"},
		}}, provider,
	)
	if err != nil || dropped != 1 || len(result) != 0 || called {
		t.Fatalf("result=%#v dropped=%d called=%v err=%v", result, dropped, called, err)
	}
}

func TestPrepareMetaActionProposalsDeduplicatesCanonicalIntentWithinMessage(t *testing.T) {
	t.Parallel()
	newProposal := func() ChatProposal {
		proposal := sanitizeProposal(&ChatProposal{
			Kind: "metaAction", Fields: ChatProposalFields{MetaAction: &ChatProposalMetaAction{
				Action: "update_campaign", CampaignID: metaActionCampaign,
				Name: "  Nome   normalizado  ",
			}},
		})
		if proposal == nil {
			t.Fatal("valid proposal was dropped")
		}
		return *proposal
	}
	providerCalls := 0
	result, dropped, err := prepareMetaActionProposals(
		context.Background(), "tenant-1", auth.Principal{UserID: "user-1"},
		"conversation-1", "message-1", []ChatProposal{newProposal(), newProposal()},
		MetaAssistantContextResult{
			ActionAdAccounts: []MetaAssistantActionAdAccount{{ID: metaActionAccountOne, Name: "Conta", Currency: "BRL"}},
			ActionCampaigns:  []MetaAssistantActionCampaign{{ID: metaActionCampaign, AdAccountID: metaActionAccountOne, Name: "Campanha"}},
		},
		func(_ context.Context, req MetaAssistantActionRequest) (MetaAssistantActionResult, error) {
			providerCalls++
			if req.ProposalIndex != 0 {
				t.Fatalf("proposal index = %d", req.ProposalIndex)
			}
			return MetaAssistantActionResult{
				ID: metaActionProposal, Status: "pending", ExecutionAvailable: true, CanConfirm: true,
			}, nil
		},
	)
	if err != nil || providerCalls != 1 || len(result) != 1 || dropped != 1 {
		t.Fatalf("result=%#v dropped=%d providerCalls=%d err=%v", result, dropped, providerCalls, err)
	}
}

func TestBindThenHydrateMetaActionUsesDurableAuthoritativeState(t *testing.T) {
	t.Parallel()
	proposals := []StoredProposal{{
		ID: "proposal-1", Kind: "metaAction", Status: "pending",
		Fields: ChatProposalFields{MetaAction: &ChatProposalMetaAction{
			ActionProposalID: metaActionProposal,
			Action:           "pause_campaign",
			CanConfirm:       false,
		}},
	}}
	bound, err := bindPersistedMetaActionProposals(
		context.Background(), "tenant-1", auth.Principal{UserID: "user-1"},
		"conversation-1", "message-1", proposals,
		func(_ context.Context, req MetaAssistantActionLifecycleRequest) (MetaAssistantActionResult, error) {
			if req.ActionProposalID != metaActionProposal || req.ConversationID != "conversation-1" || req.MessageID != "message-1" {
				t.Fatalf("bind request = %#v", req)
			}
			return MetaAssistantActionResult{
				ID: metaActionProposal, Status: "pending", ExecutionAvailable: true, CanConfirm: true,
				ExpiresAt: "2026-08-18T12:30:00Z",
			}, nil
		}, nil,
	)
	if err != nil || !bound[0].Fields.MetaAction.CanConfirm {
		t.Fatalf("bound=%#v err=%v", bound, err)
	}

	// The JSON message was persisted before the bind, so a reload must repair it
	// from the durable action proposal instead of trusting the stale card snapshot.
	messages := []ChatMessage{{ID: "message-1", Proposals: proposals}}
	hydrated, err := hydrateMetaActionProposals(
		context.Background(), "tenant-1", messages,
		func(_ context.Context, accountID, actionProposalID string) (MetaAssistantActionResult, error) {
			if accountID != "tenant-1" || actionProposalID != metaActionProposal {
				t.Fatalf("status request account=%q proposal=%q", accountID, actionProposalID)
			}
			return MetaAssistantActionResult{
				ID: metaActionProposal, Status: "pending", ExecutionAvailable: true, CanConfirm: true,
				ExpiresAt: "2026-08-18T12:30:00Z",
			}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	card := hydrated[0].Proposals[0].Fields.MetaAction
	if !card.CanConfirm || !card.ExecutionAvailable || card.ExpiresAt != "2026-08-18T12:30:00Z" {
		t.Fatalf("hydrated card = %#v", card)
	}
}

func TestBindFailureCleanupUsesIndependentPersistenceContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cancelCalled := false
	_, err := bindPersistedMetaActionProposals(
		ctx, "tenant-1", auth.Principal{UserID: "user-1"}, "conversation-1", "message-1",
		[]StoredProposal{{
			ID: "proposal-1", Kind: "metaAction", Status: "pending",
			Fields: ChatProposalFields{MetaAction: &ChatProposalMetaAction{ActionProposalID: metaActionProposal}},
		}},
		func(context.Context, MetaAssistantActionLifecycleRequest) (MetaAssistantActionResult, error) {
			return MetaAssistantActionResult{}, errors.New("bind unavailable")
		},
		func(cleanupCtx context.Context, req MetaAssistantActionLifecycleRequest) (MetaAssistantActionResult, error) {
			cancelCalled = true
			if cleanupCtx.Err() != nil {
				t.Fatalf("cleanup inherited cancelled request context: %v", cleanupCtx.Err())
			}
			if req.IdempotencyKey != "assistant-bind-failure:"+metaActionProposal {
				t.Fatalf("cleanup key = %q", req.IdempotencyKey)
			}
			return MetaAssistantActionResult{Status: "cancelled"}, nil
		},
	)
	if err == nil || !cancelCalled {
		t.Fatalf("err=%v cancelCalled=%v", err, cancelCalled)
	}
}

func TestHydrateMetaActionFailsClosedWithoutHidingConversation(t *testing.T) {
	t.Parallel()
	messages := []ChatMessage{{
		ID: "message-1",
		Proposals: []StoredProposal{{
			ID: "proposal-1", Kind: "metaAction", Status: "pending",
			Fields: ChatProposalFields{MetaAction: &ChatProposalMetaAction{
				ActionProposalID: metaActionProposal, CanConfirm: true, ExecutionAvailable: true,
			}},
		}},
	}}
	hydrated, err := hydrateMetaActionProposals(
		context.Background(), "tenant-1", messages,
		func(context.Context, string, string) (MetaAssistantActionResult, error) {
			return MetaAssistantActionResult{}, errors.New("database unavailable")
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	card := hydrated[0].Proposals[0].Fields.MetaAction
	if card.CanConfirm || card.ExecutionAvailable || card.ErrorCode != "action_status_unavailable" {
		t.Fatalf("card did not fail closed: %#v", card)
	}
}
