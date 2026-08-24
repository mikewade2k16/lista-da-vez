package calendar

import (
	"context"
	"errors"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func TestNormalizeAssistantSurface(t *testing.T) {
	t.Parallel()
	for _, surface := range []string{AssistantSurfaceCalendar, AssistantSurfaceMetaAds, AssistantSurfaceGlobal} {
		got, err := normalizeAssistantSurface("  " + surface + "  ")
		if err != nil || got != surface {
			t.Fatalf("normalizeAssistantSurface(%q) = %q, %v", surface, got, err)
		}
	}
	if _, err := normalizeAssistantSurface("unknown"); !errors.Is(err, ErrInvalidAssistantSurface) {
		t.Fatalf("invalid surface error = %v", err)
	}
}

func TestConversationSurfaceIsImmutable(t *testing.T) {
	t.Parallel()
	got, err := immutableAssistantConversationSurface(AssistantSurfaceCalendar, "")
	if err != nil || got != AssistantSurfaceCalendar {
		t.Fatalf("stored surface = %q, %v", got, err)
	}
	got, err = immutableAssistantConversationSurface(AssistantSurfaceCalendar, AssistantSurfaceCalendar)
	if err != nil || got != AssistantSurfaceCalendar {
		t.Fatalf("same requested surface = %q, %v", got, err)
	}
	if _, err = immutableAssistantConversationSurface(AssistantSurfaceCalendar, AssistantSurfaceMetaAds); !errors.Is(err, ErrAssistantSurfaceMismatch) {
		t.Fatalf("mismatch error = %v", err)
	}
}

func TestResolveAssistantCapabilitiesIntersectsModuleRBACAndReadOnly(t *testing.T) {
	t.Parallel()
	enabled := map[string]bool{"calendar": true, "tasks": true, "meta_ads": true, "core": true}
	service := &Service{assistantModuleAvailability: func(_ context.Context, _, moduleID string) (bool, error) {
		return enabled[moduleID], nil
	}}
	principal := auth.Principal{
		Role: auth.RoleMarketing, PermissionsResolved: true,
		Permissions: []string{"calendar.view", "tasks.tasks.view", "tasks.tasks.create", "meta_ads.view"},
	}
	matrix := map[string]map[string]string{
		AssistantSurfaceMetaAds: {
			"calendar": "write", "tasks": "write", "meta_ads": "write", "users": "read",
		},
	}

	capabilities, err := service.resolveAssistantCapabilities(context.Background(), "account", AssistantSurfaceMetaAds, principal, matrix)
	if err != nil {
		t.Fatal(err)
	}
	assertCapability(t, capabilities, "calendar", assistantModeRead, true, "write_permission_denied")
	assertCapability(t, capabilities, "tasks", assistantModeWrite, true, "")
	assertCapability(t, capabilities, "meta_ads", assistantModeRead, true, "write_permission_denied")
	assertCapability(t, capabilities, "users", assistantModeOff, false, "permission_denied")
}

func TestResolveAssistantCapabilitiesOwnerStillHonorsModuleState(t *testing.T) {
	t.Parallel()
	service := &Service{assistantModuleAvailability: func(_ context.Context, _, moduleID string) (bool, error) {
		return moduleID != "calendar", nil
	}}
	capabilities, err := service.resolveAssistantCapabilities(context.Background(), "account", AssistantSurfaceCalendar,
		auth.Principal{Role: auth.RoleOwner}, legacyCalendarSurfaceModules())
	if err != nil {
		t.Fatal(err)
	}
	assertCapability(t, capabilities, "calendar", assistantModeOff, false, "module_disabled")
	assertCapability(t, capabilities, "tasks", assistantModeWrite, true, "")
}

func TestResolveAssistantCapabilitiesAllowsMetaWriteOnlyWithManage(t *testing.T) {
	t.Parallel()
	service := &Service{assistantModuleAvailability: func(context.Context, string, string) (bool, error) {
		return true, nil
	}}
	capabilities, err := service.resolveAssistantCapabilities(
		context.Background(), "account", AssistantSurfaceMetaAds,
		auth.Principal{
			Role: auth.RoleMarketing, PermissionsResolved: true,
			Permissions: []string{"meta_ads.view", "meta_ads.manage"},
		},
		map[string]map[string]string{AssistantSurfaceMetaAds: {"meta_ads": assistantModeWrite}},
	)
	if err != nil {
		t.Fatal(err)
	}
	assertCapability(t, capabilities, "meta_ads", assistantModeWrite, true, "")
}

func TestResolveAssistantCapabilitiesFailsClosedWithoutResolvedPermissions(t *testing.T) {
	t.Parallel()
	service := &Service{assistantModuleAvailability: func(context.Context, string, string) (bool, error) { return true, nil }}
	capabilities, err := service.resolveAssistantCapabilities(context.Background(), "account", AssistantSurfaceGlobal,
		auth.Principal{Role: auth.RoleMarketing, Permissions: []string{"calendar.manage"}}, map[string]map[string]string{
			AssistantSurfaceGlobal: {"calendar": "write"},
		})
	if err != nil {
		t.Fatal(err)
	}
	assertCapability(t, capabilities, "calendar", assistantModeOff, false, "permission_denied")
}

func TestResolveAssistantCapabilitiesAllowsCoreUsersWithoutAccountModuleRow(t *testing.T) {
	t.Parallel()
	service := &Service{assistantModuleAvailability: func(_ context.Context, _, moduleID string) (bool, error) {
		return moduleID == "core", nil
	}}
	capabilities, err := service.resolveAssistantCapabilities(context.Background(), "account", AssistantSurfaceGlobal,
		auth.Principal{Role: auth.RoleMarketing, PermissionsResolved: true, Permissions: []string{"core.users.view"}},
		map[string]map[string]string{AssistantSurfaceGlobal: {"users": "read"}})
	if err != nil {
		t.Fatal(err)
	}
	assertCapability(t, capabilities, "users", assistantModeRead, true, "")
}

func TestFilterAssistantProposalsUsesEffectiveModeAndActionPermission(t *testing.T) {
	t.Parallel()
	capabilities := []AssistantCapability{{Module: "tasks", EffectiveMode: assistantModeWrite, Available: true}}
	principal := auth.Principal{PermissionsResolved: true, Permissions: []string{"tasks.tasks.create"}}
	proposals := []ChatProposal{
		{Kind: "task", Action: "create"},
		{Kind: "task", Action: "update"},
		{Kind: "event", Action: "create"},
	}
	kept, dropped := filterAssistantProposals(proposals, capabilities, principal)
	if len(kept) != 1 || kept[0].Kind != "task" || kept[0].Action != "create" || dropped != 2 {
		t.Fatalf("kept=%#v dropped=%d", kept, dropped)
	}
}

func TestFilterAssistantProposalsFailsClosedForUnresolvedMetaManage(t *testing.T) {
	t.Parallel()
	capabilities := []AssistantCapability{{
		Module: "meta_ads", EffectiveMode: assistantModeWrite, Available: true,
	}}
	proposals := []ChatProposal{{
		Kind: "metaAction", Fields: ChatProposalFields{MetaAction: &ChatProposalMetaAction{
			Action: "pause_campaign", CampaignID: metaActionCampaign,
		}},
	}}
	kept, dropped := filterAssistantProposals(proposals, capabilities, auth.Principal{
		PermissionsResolved: false, Permissions: []string{"meta_ads.manage"},
	})
	if len(kept) != 0 || dropped != 1 {
		t.Fatalf("permissao nao resolvida aceitou Meta: kept=%#v dropped=%d", kept, dropped)
	}
}

func TestCalendarLegacyFallbackIsNarrow(t *testing.T) {
	t.Parallel()
	if !assistantCalendarFallbackAllowed(ErrAssistantCredentialUnavailable, AssistantSurfaceCalendar) {
		t.Fatal("calendar credential gap should use legacy fallback")
	}
	if assistantCalendarFallbackAllowed(ErrAssistantDisabled, AssistantSurfaceCalendar) {
		t.Fatal("explicitly disabled canonical runtime must not bypass kill switch")
	}
	if assistantCalendarFallbackAllowed(ErrAssistantCredentialUnavailable, AssistantSurfaceMetaAds) {
		t.Fatal("meta_ads must never inherit the Calendar credential")
	}
}

func TestResolveAssistantTranscriptionAllowsMetaOnlyWithSharedCredential(t *testing.T) {
	t.Parallel()
	service := &Service{
		assistantRuntimeProvider: func(context.Context, string) (AssistantRuntime, error) {
			return AssistantRuntime{
				Enabled: true, Provider: "openai", Model: "gpt-5-mini", APIKey: "shared-secret",
				SurfaceModules: map[string]map[string]string{
					AssistantSurfaceMetaAds: {"meta_ads": assistantModeRead},
				},
			}, nil
		},
		assistantModuleAvailability: func(context.Context, string, string) (bool, error) { return true, nil },
	}
	principal := auth.Principal{
		Role: auth.RoleMarketing, PermissionsResolved: true, Permissions: []string{"meta_ads.view"},
	}
	provider, apiKey, model, err := service.resolveAssistantTranscription(
		context.Background(), "account", AssistantSurfaceMetaAds, principal,
	)
	if err != nil {
		t.Fatal(err)
	}
	if provider != "openai" || apiKey != "shared-secret" || model != "whisper-1" {
		t.Fatalf("dispatch compartilhado inesperado: provider=%q key=%q model=%q", provider, apiKey, model)
	}
}

func TestResolveAssistantTranscriptionDeniesRevokedPrimarySurface(t *testing.T) {
	t.Parallel()
	service := &Service{
		assistantRuntimeProvider: func(context.Context, string) (AssistantRuntime, error) {
			return AssistantRuntime{
				Enabled: true, Provider: "gemini", Model: "gemini-2.5-flash", APIKey: "shared-secret",
				SurfaceModules: map[string]map[string]string{
					AssistantSurfaceMetaAds: {"calendar": assistantModeRead, "meta_ads": assistantModeOff},
				},
			}, nil
		},
		assistantModuleAvailability: func(context.Context, string, string) (bool, error) { return true, nil },
	}
	principal := auth.Principal{
		Role: auth.RoleMarketing, PermissionsResolved: true,
		Permissions: []string{"calendar.view", "meta_ads.view"},
	}
	if _, _, _, err := service.resolveAssistantTranscription(
		context.Background(), "account", AssistantSurfaceMetaAds, principal,
	); !errors.Is(err, ErrAssistantNoCapability) {
		t.Fatalf("surface Meta revogada deveria negar voz, veio %v", err)
	}
}

func TestResolveTranscribeSurfaceKeepsLegacyAliasCalendarOnly(t *testing.T) {
	t.Parallel()
	if surface, err := resolveTranscribeSurface("", false, ""); err != nil || surface != AssistantSurfaceCalendar {
		t.Fatalf("alias legado vazio = %q, %v", surface, err)
	}
	if _, err := resolveTranscribeSurface("", false, AssistantSurfaceMetaAds); !errors.Is(err, ErrAssistantSurfaceMismatch) {
		t.Fatalf("alias legado aceitou Meta: %v", err)
	}
	if surface, err := resolveTranscribeSurface(AssistantSurfaceGlobal, true, AssistantSurfaceMetaAds); err != nil || surface != AssistantSurfaceMetaAds {
		t.Fatalf("rota canonica Meta = %q, %v", surface, err)
	}
}

func assertCapability(t *testing.T, capabilities []AssistantCapability, module, mode string, available bool, reason string) {
	t.Helper()
	for _, capability := range capabilities {
		if capability.Module != module {
			continue
		}
		if capability.EffectiveMode != mode || capability.Available != available || capability.Reason != reason {
			t.Fatalf("capability %s = %#v", module, capability)
		}
		return
	}
	t.Fatalf("capability %s not found", module)
}
