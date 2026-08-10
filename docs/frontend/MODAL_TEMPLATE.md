# Template-core de modal — `OmniEntityDrawer`

Todo modal/drawer da aplicação Omni usa **`web/app/components/ui/OmniEntityDrawer.vue`** como
casca. É o "core" do modal: o comportamento (header, modos, resize, fullscreen, Esc, overlay)
fica num lugar só — ajustes futuros são feitos aqui e valem para todos os modais. **Só o
conteúdo muda por tela** (vai nos slots).

Extraído e unificado a partir do modal mais elaborado do painel (o de Tasks). Hoje consomem o
template: o **modal de Tasks** (`web/layers/tasks/components/TasksTaskModal.vue`) e o **modal de
edição de usuário** (`web/app/components/admin/AdminUserEditDrawer.vue`). Demais modais migram
incrementalmente conforme forem tocados.

## O que o core já entrega

- **Header canônico** (esquerda): fechar (`chevrons-right`), expandir/encolher (toggle de tela
  cheia — entra E sai pelo mesmo botão) e popover de modo (lado / centro / tela cheia).
- **Modos** (`v-model:mode`): `side` (drawer à direita, não-bloqueante, **redimensionável**) ·
  `center` (modal central com overlay) · `fullscreen` (tela cheia). Em telas estreitas, `side` e
  `center` viram tela cheia.
- **Resize** no modo `side`: handle na borda esquerda do painel (`v-model:width` opcional, em px;
  mín 560, máx `min(innerWidth-80, 1120)`). A largura é publicada na custom property
  `--omni-drawer-side-width` (o CSS do painel e a página hospedeira podem ler para se ajustar).
- **Esc** fecha em qualquer modo; clique-fora fecha só nos modos com overlay (center/fullscreen) —
  `side` não fecha no clique-fora (features coexistem: quem precisa de clique-fora no `side` trata
  no próprio consumidor, como o Tasks faz).
- **Corpo rolável** centralizado (máx 860px) e **rodapé** opcional.

## Slots

| Slot | Uso |
| --- | --- |
| default | Conteúdo do modal (corpo). |
| `#header-extra` | Ações específicas à direita do header (ex.: presença/Compartilhar/link/estrela do Tasks). Vazio por padrão — o modal de usuário não usa (sem Compartilhar/link/estrela). |
| `#footer` | Rodapé (ex.: Excluir + autosave + Fechar do Tasks). Só renderiza se fornecido. |

## Props / eventos

- `modelValue` (v-model) — aberto/fechado.
- `title?`, `subtitle?` — cabeçalho opcional (o Tasks não usa: põe o título no corpo).
- `mode` (`v-model:mode`) — `side | center | fullscreen`.
- `width` (`v-model:width`) — largura do modo `side` em px. Sem binding, usa estado interno.
- `preferenceKey?` — quando informado, persiste no navegador o último modo e a largura lateral daquele drawer.

## Exemplo (mínimo)

```vue
<OmniEntityDrawer
  :model-value="open"
  v-model:mode="mode"
  title="Editar usuario"
  :subtitle="email"
  @update:model-value="emit('update:open', $event)"
>
  <!-- conteúdo -->
  <template #footer> ... </template>
</OmniEntityDrawer>
```

## Regras

- **Modal novo = `OmniEntityDrawer`.** Não criar `USlideover`/`UModal` à mão; se faltar algo no
  core, adicionar no core (não fork). Assim o ajuste vale para todos.
- Conteúdo específico vive nos slots; nada de lógica de posicionamento/resize/modo no consumidor.
- Ao mudar o core, validar os dois consumidores atuais (Tasks e edição de usuário).
- **CSS de posição/tamanho do painel (`--side`/`--center`/`--fullscreen`) é GLOBAL, não
  `scoped`.** O `USlideover` teleporta o painel de conteúdo para o `body` (Portal do Reka Dialog);
  um seletor `scoped`/`:deep` vira `[data-v-hash] .x` e exige um ancestral com o `data-v` no
  destino do teleport — que não existe. Resultado de errar isso: trocar de modo "não faz nada" e o
  resize "só move o fundo" (a var de largura é setada mas a regra que a consome nunca casa). Por
  isso essas regras ficam num bloco `<style>` sem `scoped` no core (prefixadas com
  `.omni-entity-drawer` para não colidir). Estilo de elemento que o NOSSO template renderiza
  (header, menu, corpo, handle, rodapé) pode seguir `scoped`, pois carrega o `data-v` no próprio
  elemento.

## Modais ainda a migrar (incremental)

`AccountCreateModal`, `AccountDetailModal`, `BioCreateModal`, `ConsultantDetailsDrawer`,
`RankingDetailsDrawer`, `WebhookSourcesDrawer`, `SiteProductCreateDialog`, `RoadmapModuleTasksModal`,
`OperationFinishModal`, entre outros (hoje usam `USlideover`/`UModal`/markup próprio). Migrar quando
forem tocados.
