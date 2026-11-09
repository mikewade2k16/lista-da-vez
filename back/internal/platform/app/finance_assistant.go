package app

import (
	"context"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/automation"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/calendar"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/finance"
)

func financeCredentialResolver(module *automation.Module) finance.AssistantCredentialResolver {
	return func(ctx context.Context, account string, cfg finance.AssistantConfig) (finance.AssistantRuntime, error) {
		if module.Service() == nil {
			return finance.AssistantRuntime{}, calendar.ErrAssistantRuntimeUnavailable
		}
		value, err := module.Service().ResolveAssistantSettings(ctx, account, cfg.CredentialID, cfg.Model, cfg.Temperature, cfg.HistoryWindow)
		return finance.AssistantRuntime{Provider: value.Provider, Model: value.Model, APIKey: value.APIKey, Temperature: value.Temperature, HistoryWindow: value.HistoryWindow}, err
	}
}
func financeRuntimeProvider(module *finance.Module) calendar.AssistantRuntimeProvider {
	return func(ctx context.Context, account string) (calendar.AssistantRuntime, error) {
		if module.Assistant() == nil {
			return calendar.AssistantRuntime{}, calendar.ErrAssistantRuntimeUnavailable
		}
		cfg, err := module.Assistant().Config(ctx, account)
		if err != nil {
			return calendar.AssistantRuntime{}, err
		}
		if !cfg.Enabled {
			return calendar.AssistantRuntime{}, calendar.ErrAssistantDisabled
		}
		value, err := module.Assistant().Runtime(ctx, account)
		if err != nil {
			return calendar.AssistantRuntime{}, calendar.ErrAssistantCredentialUnavailable
		}
		return calendar.AssistantRuntime{Enabled: true, Provider: value.Provider, Model: value.Model, APIKey: value.APIKey, SystemPrompt: value.Instructions, Temperature: value.Temperature, HistoryWindow: value.HistoryWindow, SurfaceModules: map[string]map[string]string{"finance": {"finance": "read"}}}, nil
	}
}
