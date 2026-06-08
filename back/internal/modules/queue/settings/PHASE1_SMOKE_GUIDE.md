# PHASE 1 SMOKE GUIDE

## Objetivo

Validar que falha em `GET /v1/settings` nao derruba mais:

- login
- bootstrap do painel autenticado
- troca de loja
- sessao ja valida

Esta fase valida apenas a blindagem do frontend. Ela nao testa ainda a
refatoracao estrutural de persistencia das configuracoes.

## Pre-condicoes

- ambiente local com `APP_ENV` diferente de `production`
- stack local subida
- frontend acessivel
- API acessivel
- usuario autenticado com acesso ao painel

Credencial local util para smoke:

- `terminal.jardins@acesso.omni.local`
- senha: `Terminal@2026!`

## Gatilho local de falha controlada

Somente em ambiente nao-produtivo, `GET /v1/settings` aceita um gatilho de falha local.

Formas de ativar:

- query string: `__debugSettingsFailure=500`
- cookie: `ldv_debug_settings_failure=500`

Modos suportados:

- `500`
  - responde `500 internal_error` imediatamente
- `slow-500`
  - espera `12s` e responde `500 internal_error`

Observacao importante:

- `slow-500` simula backend lento com erro tardio
- ele nao representa timeout HTTP real do cliente
- hoje `web/app/utils/api-client.ts` nao define timeout explicito no `$fetch`

## Como ativar e limpar no navegador

No console do navegador:

```js
document.cookie = "ldv_debug_settings_failure=500; path=/";
```

Para modo lento:

```js
document.cookie = "ldv_debug_settings_failure=slow-500; path=/";
```

Para limpar:

```js
document.cookie = "ldv_debug_settings_failure=; Max-Age=0; path=/";
```

## Checklist base

Antes de testar falha:

1. Entrar normalmente no painel
2. Confirmar que nao existe banner de modo degradado
3. Abrir `/configuracoes` e confirmar que a tela sobe sem erro
4. Confirmar no Network que `GET /v1/settings` responde `200`

## Cenario 1 - Erro 500 no bootstrap inicial

Passos:

1. Fazer login normal
2. Abrir o console do navegador
3. Definir o cookie `ldv_debug_settings_failure=500`
4. Dar refresh completo na rota autenticada atual

Esperado:

1. O usuario continua autenticado
2. O painel autenticado abre
3. `GET /v1/settings` responde `500`
4. `GET /v1/consultants` continua respondendo `200`
5. `GET /v1/operations/snapshot` continua respondendo `200`
6. O banner `Modo degradado de configuracoes` aparece no layout
7. A tela `/configuracoes` tambem exibe o aviso contextual
8. O console do navegador registra warning com prefixo `[runtime-settings]`

Nao esperado:

1. Voltar para login
2. Limpar sessao/cookie de auth
3. Tela branca
4. Toast generico de auth no lugar do banner degradado

## Cenario 2 - Troca de loja com settings quebrado

Passos:

1. Manter o cookie `ldv_debug_settings_failure=500`
2. Com o painel autenticado aberto, trocar a loja no header

Esperado:

1. A troca de loja continua funcionando
2. O painel continua aberto
3. O aviso de modo degradado permanece visivel
4. Nao ocorre logout

## Cenario 3 - Realtime/refresh de settings com falha

Passos sugeridos:

1. Manter o cookie `ldv_debug_settings_failure=500`
2. Abrir o painel autenticado
3. Abrir a tela de configuracoes em outra aba autenticada do mesmo tenant
4. Nessa segunda aba, limpar o cookie, fazer uma alteracao de config e salvar
5. Na primeira aba, reativar o cookie e repetir um refresh da tela

Esperado:

1. O frontend entra em degradacao sem perder a sessao
2. O aviso degradado e atualizado pela mesma store de auth
3. Ao limpar o cookie e recarregar com `GET /v1/settings = 200`, o aviso some

## Cenario 4 - Backend lento com erro tardio

Passos:

1. Definir o cookie `ldv_debug_settings_failure=slow-500`
2. Dar refresh na rota autenticada

Esperado:

1. O painel pode demorar mais para estabilizar
2. Ao final, entra em modo degradado
3. O usuario continua autenticado

Observacao:

- este cenario valida tolerancia a resposta lenta seguida de erro
- ele nao substitui teste de timeout de rede real

## Cenario 5 - Indisponibilidade de rede so em `/v1/settings`

Como o cliente nao tem timeout explicito hoje, o melhor smoke de rede e feito no
browser:

1. Abrir DevTools
2. Ir em Network request blocking
3. Bloquear apenas URLs contendo `/v1/settings`
4. Dar refresh na rota autenticada

Esperado:

1. O painel continua subindo em modo degradado
2. O usuario continua logado
3. `consultants` e `operationsSnapshot` continuam carregando

## Cenario 6 - Erro de schema/coluna faltando no backend

Do ponto de vista do frontend, este caso deve ser tratado como equivalente a um
`500` em `/v1/settings`.

Regra de seguranca para smoke:

- nao dropar coluna real local so para testar a UX
- usar o modo `500` como representante do comportamento esperado do cliente

## Encerramento do smoke

1. Limpar o cookie `ldv_debug_settings_failure`
2. Dar refresh
3. Confirmar que o banner some
4. Confirmar que `GET /v1/settings` volta a `200`

## Resultado esperado da Fase 1

Se todos os cenarios acima estiverem corretos, a conclusao da fase e:

- falha em settings nao derruba mais a sessao
- bootstrap do painel fica resiliente
- degradacao fica explicita para o usuario/admin
- o caminho esta pronto para a Fase 2 sem repetir o bug classico de login
