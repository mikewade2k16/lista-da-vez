import { defineAsyncComponent, type AsyncComponentLoader, type Component } from 'vue'
import CoreAsyncError from '../../layers/core/components/CoreAsyncError.vue'

export interface LazyComponentOptions {
  // Componente exibido se o carregamento falhar de vez (default: CoreAsyncError).
  errorComponent?: Component
  // Componente exibido enquanto carrega (default: nenhum — nao pisca loader em load rapido).
  loadingComponent?: Component
  // Quantas vezes re-tentar o import em falha transitoria antes de desistir (default: 1).
  retries?: number
  // Timeout do carregamento antes de cair no errorComponent, em ms (default: 20000).
  timeout?: number
}

// Wrapper canonico para componentes carregados sob demanda (chunks lazy).
//
// Um `defineAsyncComponent(() => import(...))` "pelado" que falhe o import — chunk 404
// pos-deploy, bare specifier deixado no bundle, queda de rede — renderiza NADA: o
// componente (ex.: um modal) simplesmente nao aparece, sem erro visivel. Foi a causa
// raiz do "modal de tasks em branco em producao" (ver references/registro-de-falhas.md
// #10). Este helper:
//   - re-tenta o import automaticamente em falha transitoria (default: 1 retry);
//   - se ainda falhar, mostra um errorComponent acionavel ("Recarregar") no lugar do
//     componente, em vez de sumir;
//   - aplica timeout pra nao ficar em loading infinito.
//
// Use SEMPRE no lugar de defineAsyncComponent para componentes lazy reais.
export function defineLazyComponent(
  loader: AsyncComponentLoader,
  options: LazyComponentOptions = {},
) {
  const maxRetries = options.retries ?? 1
  return defineAsyncComponent({
    loader,
    errorComponent: options.errorComponent ?? CoreAsyncError,
    ...(options.loadingComponent ? { loadingComponent: options.loadingComponent } : {}),
    timeout: options.timeout ?? 20000,
    onError(_error, retry, fail, attempts) {
      // attempts e 1-based: 1 = primeira tentativa. Re-tenta enquanto <= maxRetries,
      // senao desiste e deixa o errorComponent renderizar.
      if (attempts <= maxRetries) {
        retry()
        return
      }
      fail()
    },
  })
}
