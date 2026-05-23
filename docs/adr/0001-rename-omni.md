# ADR-0001: Renomear o produto para Omni

- **Status**: Aceito
- **Data**: 2026-05-16
- **Decisores**: Mike (product owner)

## Contexto

Convivem 4 nomes no projeto: `fila-atendimento` (pasta), `lista-da-vez` (compose/package), `Fila de Atendimento` (UI/README), `listaatendimento` (prod). Inconsistencia presente em 11 pontos do codigo + 1 banco prod.

O nome `omni` ja esta difuso pela base (design system `omni-tokens.css`, componentes `OmniEditor`, composable `useOmniTheme`, dominio tecnico `acesso.omni.local`, `docs/BACKLOG.md` chama produto de "Omni").

A Fase 4 do plano de refatoracao ja fixou os valores canonicos para a transicao: display `Omni`, slug `omni`, `APP_NAME=omni-api` e rename do banco de producao com janela curta de manutencao. Ao mesmo tempo, alguns identificadores externos devem permanecer intactos, como `omnichannel-mvp_default`, `omnichannel-mvp-caddy-1` e `/opt/omnichannel/Caddyfile`.

## Decisao

Padronizar para **Omni** (display) / **omni** (slug). Renomear em todos os 11 pontos + `ALTER DATABASE` em prod com janela.

## Alternativas consideradas

1. **Manter `lista-da-vez`** — coerente com slug historico, mas dissonante com UI/README e direcao do produto.
2. **Manter `Fila de Atendimento`** — bate com README mas ignora o que ja existe codificado como `omni`.
3. **Nome novo (outro)** — gera mais retrabalho e ignora afinidade existente.

Vencedor: opcao atual (Omni) por minimizar retrabalho — o nome ja estava sendo usado tacitamente.

## Consequencias

**Positivas:**
- 1 nome em todo o stack.
- Documentacao coerente.
- Branding alinhado entre UI, codigo e infra.

**Negativas / riscos:**
- `ALTER DATABASE` em prod exige janela curta de manutencao.
- Risco de quebrar integracoes externas que referenciam nome antigo (se houver — auditar).
- Bookmark/atalhos do usuario precisam ser atualizados.

## NAO confundir com

- `omnichannel` — modulo de chat (continua existindo)
- `omnichannel-mvp_default` — rede Docker de outro projeto na VPS

## Referencias

- [PLANO_REFATORACAO.md](../PLANO_REFATORACAO.md) Fase 4
- [PARALELIZACAO.md](../PARALELIZACAO.md) Onda 3
