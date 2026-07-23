# Social Publishing frontend

Escopo isolado do módulo `social_publishing`. Herda as regras de
`web/AGENT.md` e `web/app/components/AGENT.md`.

## Contratos

- O tenant vem exclusivamente de `X-Account-Id` por `createApiRequest`.
- Permissões: `social_publishing.view`, `.manage`, `.connect` e `.analytics`;
  `platform_admin` tem bypass visual, mas o backend continua autoritativo.
- Base HTTP: `/v1/social-publishing`.
- A conexão técnica Beta recebe `{ accessToken }`, mas o token nunca entra em
  store, log ou estado persistente. A UI só exibe `secret.set` e `secret.last4`.
- O MVP publica somente uma imagem por URL HTTPS. Reels e carrossel são apenas
  formatos futuros informativos, sem ações falsas.
- Analytics Instagram v23: `views`, `reach`, `totalInteractions`, `likes`,
  `comments`, `saved` e `shares`.
- Não integrar com Calendário nesta fase. O vínculo futuro deve consumir a API
  pública do módulo, sem duplicar agendamento no frontend.

## Componentização

- `SocialPublishingWorkspace.vue` coordena permissões, abas e store.
- O editor usa obrigatoriamente `OmniEntityDrawer`.
- Estados de carregamento, erro e vazio são reais; dados de demonstração são
  proibidos.
- Use somente tokens semânticos existentes e preserve navegação por teclado,
  foco visível e layout móvel.
