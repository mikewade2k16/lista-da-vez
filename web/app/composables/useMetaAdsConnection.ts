import { storeToRefs } from 'pinia'

import { useMetaAdsStore } from '~/stores/meta-ads'

// Composable fino de conexao Meta Ads. Embrulha o useMetaAdsStore expondo apenas
// o que o ConnectionCard precisa (status + colar token + conectar/desconectar).
// Toda a logica e estado vivem no store; aqui so projetamos refs reativas.
export function useMetaAdsConnection() {
  const store = useMetaAdsStore()
  const { connection, connected, connecting, error } = storeToRefs(store)

  return {
    connection,
    connected,
    connecting,
    error,
    init: store.init,
    saveConnection: store.saveConnection,
    deleteConnection: store.deleteConnection,
  }
}
