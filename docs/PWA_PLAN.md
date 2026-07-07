# PWA do Omni

## Objetivo

Tornar o painel Omni instalavel e resiliente a falhas de navegacao, mantendo os dados
autenticados isolados por conta. A fundacao usa `@vite-pwa/nuxt` 1.1.1.

> Status: estacionado. O PWA fica desativado por padrao e so e habilitado com
> `NUXT_PWA_ENABLED=true`.

## Decisoes da fundacao

- Atualizacoes usam `registerType: 'prompt'`: o painel nunca recarrega sozinho durante
  atendimento, chat ou preenchimento de formularios.
- O precache contem somente o app shell e assets estaticos.
- Chunks hasheados em `/_nuxt/` usam `CacheFirst`.
- Navegacoes usam `NetworkOnly` e, sem rede, caem em `/offline/index.html`.
- O service worker fica desligado em desenvolvimento por padrao e pode ser habilitado
  com `NUXT_PWA_DEV=true`.

## Regra de seguranca multi-tenant

O service worker nunca deve cachear a origem autenticada da API nem rotas `/v1/*`.
Essas respostas variam por `X-Account-Id`, mas esse header nao faz parte da chave padrao
do Cache Storage. Cachea-las poderia expor dados de uma conta em outra sessao/conta.

Os snapshots offline de dominio previstos nas specs PWA-03 a PWA-05 devem usar cache
explicitamente particionado por conta, fora do runtime cache generico do Workbox.

## Notas de deploy

- Nova env opcional: `NUXT_PWA_DEV`, usada somente para validar o service worker em dev.
- `NUXT_PWA_ENABLED` controla toda a feature e tem default `false`.
- Producao nao exige env nova: manifest e service worker sao gerados no build.
- A validacao do build/imagem de producao continua sujeita a aprovacao explicita do dono.
