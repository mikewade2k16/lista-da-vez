# Segurança multi-tenant e otimização

Leia ao criar/alterar qualquer endpoint, fetch no boot, ou checagem de permissão.

> Regra de ouro: **um usuário NUNCA vê dado de outra account/loja**, e nenhum payload/bypass/parâmetro forjado contorna isso. Fundamentado em práticas de SaaS em escala (ex.: OWASP Multi-Tenant Cheat Sheet).

## Camadas de defesa em profundidade (inventário)
Cada uma é uma rede; nenhuma é única:

1. **Auth por token assinado.** Papel, tenant e claims vêm do token assinado; o cliente não forja `role`/`tenantId`.
2. **Revogação de sessão.** Sessão com `revoked_at` checado no mesmo lookup do Principal. Logout/kill invalida na hora.
3. **Membership obrigatória por account.** O middleware valida o header de account contra a tabela de membership ANTES de injetar o escopo no Principal. Header forjado → erro de membership.
4. **Gating de módulo por rota.** Rota de um módulo só abre se a account contratou o módulo. Admin de plataforma isento.
5. **Escopo de tenant validado no service.** Não-admin pedindo tenant diferente do seu → erro. O id vindo do query é filtro DENTRO do permitido, nunca bypass.
6. **Escopo de loja validado** da mesma forma.
7. **Defesa em profundidade na query.** Mesmo se o escopo passasse, o repo filtra por tenant sempre. Recurso de outro tenant → query retorna vazio.
8. **RBAC por permissão.** Checagem em tabela de permissões. 403 de papel sem permissão é correto.
9. **SQL 100% parametrizado.** Zero concatenação.
10. **Rate limit** e **CORS allowlist** (sem `*` em produção).

## Erro uniforme
Recurso fora do escopo retorna `404 not_found`, não `403`. Retornar 403 vs 404 diferentes vaza a existência de um recurso de outro tenant (enumeration).

## Gaps conhecidos a considerar
- **Row-Level Security (RLS) no banco** como rede final: se um handler novo esquecer a validação de escopo, sem RLS vaza. RLS com `USING (tenant_id = current_setting(...))` fecha por baixo.
- **Security headers** (HSTS, X-Content-Type-Options, X-Frame-Options, CSP) no chain de middleware.
- **Rate limit por tenant**, não só por usuário (evita "noisy neighbor").
- **Escopo em cache keys e paths de upload**: toda chave de cache e todo path de arquivo inclui o tenant (evita cross-tenant em cache/arquivos).

## Pedir só o necessário (otimização + UX de resposta imediata)
Carregar primeiro o essencial da tela, lazy-load do resto; payload mínimo; nunca buscar o que não se usa.

- **Não pedir o que a role não vê.** Gatear o fetch por permissão ANTES de disparar (espelhando o back). Evita 403 de ruído no bootstrap e round-trip desperdiçado.
- **Sem N+1.** Agregar por `WHERE id = ANY($1)`, nunca 1 query por item em loop.
- **Paginação** em lista grande; cursor (`?after=`) para listas que crescem (evita `OFFSET` caro e `COUNT` por página).
- **Projeção lean / field selection.** Retornar só os campos que a tela usa, não o objeto inteiro.
- **Prefetch só do above-the-fold.** Detalhe (memberships, stores) só ao abrir o modal, não na listagem.
- **Compressão** (gzip/brotli) para respostas JSON grandes.
- UI: skeleton < 100ms no clique, `AbortController` ao trocar de rota, otimismo no toggle.
