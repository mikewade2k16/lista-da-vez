import { normalizeCampaign } from "~/domain/utils/campaigns";
import { DEFAULT_REPORT_FILTERS, normalizeReportFilters } from "~/domain/utils/reports";
import { appendUniqueOption, createOptionId } from "~/stores/dashboard/runtime/shared";
import { applyOperationTemplateToState } from "~/stores/dashboard/runtime/state";

export function createSettingsActions({ getState, updateState }) {
  return {
    updateReportFilter(filterId, value) {
      const state = getState();

      if (!(filterId in state.reportFilters)) {
        return;
      }

      const normalizedValue = Array.isArray(value)
        ? [...new Set(value.map((item) => String(item || "").trim()).filter(Boolean))]
        : ["minSaleAmount", "maxSaleAmount"].includes(filterId) && value !== ""
          ? String(Math.max(0, Number(value) || 0))
          : String(value ?? "");

      updateState({
        ...state,
        reportFilters: {
          ...state.reportFilters,
          [filterId]: normalizedValue
        }
      });
    },

    resetReportFilters() {
      const state = getState();

      updateState({
        ...state,
        reportFilters: normalizeReportFilters(DEFAULT_REPORT_FILTERS)
      });
    },

    createCampaign(campaignInput) {
      const state = getState();
      const name = String(campaignInput?.name || "").trim();

      if (!name) {
        return { ok: false, message: "Nome da campanha e obrigatorio." };
      }

      const campaignId = createOptionId("campanha", name, state.campaigns);
      const campaign = normalizeCampaign({
        ...campaignInput,
        id: campaignId,
        name
      });

      updateState({
        ...state,
        campaigns: [...state.campaigns, campaign]
      });

      return { ok: true };
    },

    updateCampaign(campaignId, patch) {
      const state = getState();
      const existing = state.campaigns.find((campaign) => campaign.id === campaignId);

      if (!existing) {
        return { ok: false, message: "Campanha nao encontrada." };
      }

      const nextCampaign = normalizeCampaign({
        ...existing,
        ...patch,
        id: campaignId
      });

      if (!nextCampaign.name) {
        return { ok: false, message: "Nome da campanha e obrigatorio." };
      }

      updateState({
        ...state,
        campaigns: state.campaigns.map((campaign) => (campaign.id === campaignId ? nextCampaign : campaign))
      });

      return { ok: true };
    },

    removeCampaign(campaignId) {
      const state = getState();

      updateState({
        ...state,
        campaigns: state.campaigns.filter((campaign) => campaign.id !== campaignId)
      });
    },

    updateSetting(settingId, value) {
      const state = getState();

      if (!(settingId in state.settings)) {
        return;
      }

      updateState({
        ...state,
        settings: {
          ...state.settings,
          [settingId]: value
        }
      });
    },

    updateModalConfig(configKey, value) {
      const state = getState();

      if (!(configKey in state.modalConfig)) {
        return;
      }

      updateState({
        ...state,
        modalConfig: {
          ...state.modalConfig,
          [configKey]: value
        }
      });
    },

    applyOperationTemplate(templateId) {
      const state = getState();

      updateState(applyOperationTemplateToState(state, templateId));
    },

    addVisitReasonOption(label) {
      const state = getState();
      const normalized = String(label || "").trim();

      if (!normalized) {
        return;
      }

      updateState({
        ...state,
        visitReasonOptions: [
          ...state.visitReasonOptions,
          {
            id: createOptionId("motivo", normalized, state.visitReasonOptions),
            label: normalized
          }
        ]
      });
    },

    updateVisitReasonOption(optionId, label) {
      const state = getState();
      const normalized = String(label || "").trim();

      if (!normalized) {
        return;
      }

      updateState({
        ...state,
        visitReasonOptions: state.visitReasonOptions.map((item) =>
          item.id === optionId ? { ...item, label: normalized } : item
        )
      });
    },

    removeVisitReasonOption(optionId) {
      const state = getState();

      updateState({
        ...state,
        visitReasonOptions: state.visitReasonOptions.filter((item) => item.id !== optionId)
      });
    },

    addCustomerSourceOption(label) {
      const state = getState();
      const normalized = String(label || "").trim();

      if (!normalized) {
        return;
      }

      updateState({
        ...state,
        customerSourceOptions: [
          ...state.customerSourceOptions,
          {
            id: createOptionId("origem", normalized, state.customerSourceOptions),
            label: normalized
          }
        ]
      });
    },

    updateCustomerSourceOption(optionId, label) {
      const state = getState();
      const normalized = String(label || "").trim();

      if (!normalized) {
        return;
      }

      updateState({
        ...state,
        customerSourceOptions: state.customerSourceOptions.map((item) =>
          item.id === optionId ? { ...item, label: normalized } : item
        )
      });
    },

    removeCustomerSourceOption(optionId) {
      const state = getState();

      updateState({
        ...state,
        customerSourceOptions: state.customerSourceOptions.filter((item) => item.id !== optionId)
      });
    },

    addQueueJumpReasonOption(label) {
      const state = getState();
      const { item, items } = appendUniqueOption(state.queueJumpReasonOptions, "motivo-fora-da-vez", label);

      if (!item || items === state.queueJumpReasonOptions) {
        return;
      }

      updateState({
        ...state,
        queueJumpReasonOptions: items
      });
    },

    updateQueueJumpReasonOption(optionId, label) {
      const state = getState();
      const normalized = String(label || "").trim();

      if (!normalized) {
        return;
      }

      const duplicate = state.queueJumpReasonOptions.find(
        (item) => item.id !== optionId && String(item.label || "").trim().toLowerCase() === normalized.toLowerCase()
      );

      if (duplicate) {
        return;
      }

      updateState({
        ...state,
        queueJumpReasonOptions: state.queueJumpReasonOptions.map((item) =>
          item.id === optionId ? { ...item, label: normalized } : item
        )
      });
    },

    removeQueueJumpReasonOption(optionId) {
      const state = getState();

      updateState({
        ...state,
        queueJumpReasonOptions: state.queueJumpReasonOptions.filter((item) => item.id !== optionId)
      });
    },

    addLossReasonOption(label) {
      const state = getState();
      const { item, items } = appendUniqueOption(state.lossReasonOptions, "motivo-perda", label);

      if (!item || items === state.lossReasonOptions) {
        return;
      }

      updateState({
        ...state,
        lossReasonOptions: items
      });
    },

    updateLossReasonOption(optionId, label) {
      const state = getState();
      const normalized = String(label || "").trim();

      if (!normalized) {
        return;
      }

      const duplicate = state.lossReasonOptions.find(
        (item) => item.id !== optionId && String(item.label || "").trim().toLowerCase() === normalized.toLowerCase()
      );

      if (duplicate) {
        return;
      }

      updateState({
        ...state,
        lossReasonOptions: state.lossReasonOptions.map((item) =>
          item.id === optionId ? { ...item, label: normalized } : item
        )
      });
    },

    removeLossReasonOption(optionId) {
      const state = getState();

      updateState({
        ...state,
        lossReasonOptions: state.lossReasonOptions.filter((item) => item.id !== optionId)
      });
    },

    addProfessionOption(label) {
      const state = getState();
      const { item, items } = appendUniqueOption(state.professionOptions, "profissao", label);

      if (!item || items === state.professionOptions) {
        return;
      }

      updateState({
        ...state,
        professionOptions: items
      });
    },

    updateProfessionOption(optionId, label) {
      const state = getState();
      const normalized = String(label || "").trim();

      if (!normalized) {
        return;
      }

      const duplicate = state.professionOptions.find(
        (item) => item.id !== optionId && String(item.label || "").trim().toLowerCase() === normalized.toLowerCase()
      );

      if (duplicate) {
        return;
      }

      updateState({
        ...state,
        professionOptions: state.professionOptions.map((item) =>
          item.id === optionId ? { ...item, label: normalized } : item
        )
      });
    },

    removeProfessionOption(optionId) {
      const state = getState();

      updateState({
        ...state,
        professionOptions: state.professionOptions.filter((item) => item.id !== optionId)
      });
    },

    addCatalogProduct(name, category, basePrice, code = "") {
      const state = getState();
      const normalizedName = String(name || "").trim();
      const normalizedCategory = String(category || "").trim();
      const normalizedCode = String(code || "").trim().toUpperCase();
      const price = Math.max(0, Number(basePrice) || 0);

      if (!normalizedName) {
        return;
      }

      const id = createOptionId("produto", normalizedName, state.productCatalog);

      updateState({
        ...state,
        productCatalog: [
          ...state.productCatalog,
          {
            id,
            name: normalizedName,
            code: normalizedCode,
            category: normalizedCategory || "Sem categoria",
            basePrice: price
          }
        ]
      });
    },

    updateCatalogProduct(productId, patch) {
      const state = getState();

      updateState({
        ...state,
        productCatalog: state.productCatalog.map((product) =>
          product.id === productId
            ? {
                ...product,
                ...patch,
                name: String((patch.name ?? product.name) || "").trim() || product.name,
                code: String((patch.code ?? product.code) || "").trim().toUpperCase(),
                category: String((patch.category ?? product.category) || "").trim() || "Sem categoria",
                basePrice: Math.max(0, Number(patch.basePrice ?? product.basePrice) || 0)
              }
            : product
        )
      });
    },

    removeCatalogProduct(productId) {
      const state = getState();

      updateState({
        ...state,
        productCatalog: state.productCatalog.filter((product) => product.id !== productId)
      });
    }
  };
}
