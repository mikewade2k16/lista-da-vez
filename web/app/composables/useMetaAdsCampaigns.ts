import { storeToRefs } from 'pinia'

import { useMetaAdsStore } from '~/stores/meta-ads'

// Composable fino de campanhas e contas de anuncio Meta Ads. Embrulha o
// useMetaAdsStore expondo o que AccountPicker e CampaignTable precisam (lista de
// contas, conta selecionada e lista de campanhas do cache). Estado e logica
// vivem no store; aqui so projetamos refs reativas.
export function useMetaAdsCampaigns() {
  const store = useMetaAdsStore()
  const { adAccounts, selectedAdAccountId, selectedAdAccount, campaigns, pending, error } =
    storeToRefs(store)

  return {
    adAccounts,
    selectedAdAccountId,
    selectedAdAccount,
    campaigns,
    pending,
    error,
    loadAdAccounts: store.loadAdAccounts,
    selectAdAccount: store.selectAdAccount,
    loadCampaigns: store.loadCampaigns,
  }
}
