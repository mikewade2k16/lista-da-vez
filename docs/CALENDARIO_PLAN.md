# Calendário — Plano (front preview → back)

> Doc canônico do módulo Calendário (`/calendario`). Fonte de verdade da visão e do
> escopo do back. Espelhar mudanças aqui + em `web/app/components/AGENT.md` (seção
> `calendar`) + em `roadmap-data.ts` (regra dos 3 docs).

## 1. Visão / objetivo

Agenda de **conteúdo por cliente** da agência. Cada dia carrega o que vai ter naquele
dia: qual **cliente**, quais **postagens** (post/story/reels/reunião/gravação/evento),
com **status / prioridade / responsável** e anotações. Objetivos:

- **Cliente pode ver** o que está agendado (o que será entregue e até quando).
- **Equipe se programa** — sabe o que precisa entregar até qual data.
- Anotações por dia/mês sobre o que vai rolar.

## 2. Estado atual

- Página `/calendario`: visões **Mês** e **Semana**, scroll + **CSS Scroll Snap**
  (foco centralizado com peek), rail lateral `M / S1..Sn`, notas por mês, drawer do dia.
- **✅ Fase 1 (CRUD) LIGADA**: EVENTOS e NOTAS vêm do **back real** (`/v1/calendar/*`,
  módulo Go `back/internal/modules/calendar`, migration 0181). Front: `stores/calendar.ts`
  busca eventos por janela (`from`/`to`), notas por mês (save debounced), e faz CRUD via
  `CalendarEventForm` + Editar/Excluir no drawer. **CLIENTES = reais** (`useTenantsStore`).
- **✅ Fase 2 LIGADA (dados 100% reais, zero mock)**:
  - **Responsáveis = usuários reais** da conta (`GET /v1/calendar/responsibles`, subconjunto
    configurável). `store.people` = responsáveis; `useCalendarData` (mock) **removido**.
  - **Feriados/datas comemorativas** computados no back (`GET /v1/calendar/holidays?from=&to=`,
    conjuntos BR nacional / Sergipe / Aracaju / luxo internacional, móveis via Páscoa) e
    renderizados em `DayCell`/`WeekView`.
  - **Modal de configuração** na página (engrenagem): escolhe quais usuários aparecem como
    responsáveis + liga/desliga cada conjunto de feriados. Persistido por conta em
    `calendar.config` (jsonb) via `GET/PUT /v1/calendar/config` (migration 0182).
- Gate de API `/v1/calendar` existe (platform_admin bypassa); gate de rota/nav no front ainda
  é preview.

## 3. Back — escopo (várias fases)

### 3.1 Dados (troca o mock)
- `calendar_events`: id, **account_id** (tenant, do Principal — nunca do body), date,
  time, client_id, type, title, status, priority, responsible_id, involved[], media[],
  description, timestamps.
- `calendar_notes`: account_id, month_key (`YYYY-MM`), content, updated_by, updated_at.
- Endpoints: `GET/POST/PATCH/DELETE /v1/calendar/events`, `GET/PUT /v1/calendar/notes/{month}`.
- Isolamento multi-tenant e projeção lean (só a janela pedida).

### 3.2 Datas comemorativas & feriados — ✅ FEITO
Marcados no dia (faixa/badge em `--accent-warning`), cada conjunto **ativável/desativável**:
- **Nacionais (BR)** — fixos + móveis calculados (Carnaval/Sexta-feira Santa/Corpus Christi via
  Páscoa/Meeus; Dia das Mães/Pais por n-ésimo domingo).
- **Estaduais (Sergipe)** e **municipais (Aracaju)**.
- **Internacionais p/ marcas de luxo** — Valentine's, Dia da Mulher, Halloween, Natal, Réveillon,
  Black Friday + Cyber Monday (móveis).
- Implementação: **sem tabela de seed** — `back/.../calendar/holidays.go` gera as datas do range
  sob demanda (`HolidaysInRange`) e filtra pelos conjuntos ligados na config da conta.

### 3.3 Configuração / White-label (modal na página)
Modal de config (`CalendarConfigModal.vue`, engrenagem nos controles), persistido por conta em
`calendar.config` (jsonb, migration 0182):
- ✅ **Responsáveis**: quais usuários reais da conta aparecem na lista (vazio = todos).
- ✅ **Feriados**: liga/desliga cada conjunto (nacional, SE, Aracaju, luxo internacional).
- ⏳ **Cores**: por cliente e/ou por tipo de evento; opção **"sem cor"**.
- ⏳ Início da semana (domingo/segunda).
- ⏳ **White-label**: aparência do painel/visão exposta ao cliente (logo/cores) — configurável
  direto no painel, não no código.

### 3.4 Visão compartilhável (cliente vê)
- Visão read-only do calendário do cliente (o que será entregue até quando), respeitando
  o white-label.

### 3.5 Perfil do cliente (insumo p/ a IA de conteúdo)

Para o botão **"Gerar sugestões"** nas notas do mês (IA sugere pauta/conteúdo do mês por
cliente), cada cliente tem um **perfil** (jsonb no client/account ou tabela `client_profile`).
Campos que alimentam a IA:

- **Identidade**: segmento/nicho, descrição do negócio, posicionamento (luxo/popular/…), região.
- **Público-alvo**: persona (idade/gênero/interesses/dor-desejo/classe).
- **Voz da marca**: tom de voz, do's & don'ts, palavras/hashtags fixas, emojis sim/não.
- **Oferta**: produtos/serviços, carro-chefe, lançamentos, faixa de preço, promoções recorrentes.
- **Estratégia**: objetivos do mês, pilares de conteúdo (+ proporção), cadência, canais ativos,
  datas fixas do cliente (aniversário da loja, coleções).
- **Restrições**: temas proibidos, compliance (farmácia/bebida/saúde etc.).
- **Histórico**: posts que performaram (métricas) + feedbacks — a IA aprende com o tempo.
- **Assets**: logo, paleta, moodboard.

Mínimo para começar a gerar algo bom: **segmento, descrição, tom de voz, público, produtos,
objetivos, pilares, cadência**. A IA (Claude) monta o prompt com **perfil + mês + datas
comemorativas + histórico** → devolve **pilares + ideias por dia + rascunho de copy** direto nas
notas do mês. Também dá pra gerar por cliente OU pra o mês inteiro de vários clientes.

### 3.6 Anexos / mídia (imagens e vídeos) — Fase 3 (em andamento)

Anexos em **dois lugares** (decisão do dono):
- **No evento/post** (`calendar.events.media` — jsonb já existe): a criação daquele post.
- **No dia** (avulso): referências/moodboard do dia, sem vínculo com um evento.

**MediaItem** (item de mídia, jsonb):
`{ id, url, name, type: "image"|"video", contentType, sizeBytes }`. `url` sempre aponta para
`/uploads/calendar/{accountId}/{arquivo}` (nunca URL externa).

**Fluxo de upload (stateless, espelha cardápio/tasks):**
1. `POST /v1/calendar/media` (multipart, campo `file`) — valida mime (imagem: jpg/png/webp/gif;
   vídeo: mp4/webm/mov) + tamanho, grava em `uploads/calendar/{accountId}/` e devolve o MediaItem.
2. O front adiciona o item ao array de mídia do evento (ou do dia) e **salva** (full replace, padrão
   do módulo). Assim o `PUT` do evento/dia não precisa saber de upload.

**Limite de tamanho — definido na plataforma (global, editável por platform_admin):**
- Guardado em `core.platform_settings`, chave `media_limits`:
  `{ imageMaxBytes (default 10MB), videoMaxBytes (default 300MB) }`.
- `GET /v1/calendar/media-limits` (qualquer autenticado — o front mostra o limite e valida no
  cliente antes de subir) · `PUT /v1/calendar/media-limits` (só platform_admin).
- O handler de upload lê o limite no ato; vídeo acima do teto → 400 `invalid_media`.

**Anexos do dia:** tabela `calendar.day_media (account_id, event_date, media jsonb)` —
`GET /v1/calendar/day-media?from=&to=` (janela, igual eventos) + `PUT /v1/calendar/day-media/{date}`
(full replace da lista daquele dia).

**Orfãos:** remover um item do array NÃO apaga o arquivo do disco (v1 tolera; GC/limpeza fica p/
depois). **Multi-tenant:** account do Principal (nunca do body); arquivos por `uploads/calendar/{account}/`.

## 4. Fases sugeridas
1. **✅ Back CRUD** de eventos + notas (feito: migration 0181, módulo Go, front integrado).
   Falta o gate de rota/nav próprio no front (hoje preview).
2. **✅ Feriados/datas comemorativas** (cálculo no back + toggles na config) + render no calendário
   + **responsáveis reais** + **modal de config** (migration 0182). Mock 100% removido.
3. **Config avançada + white-label** (cores por cliente/tipo, início da semana, aparência do
   cliente) + **perfil do cliente** (§3.5) + **IA de sugestão** de conteúdo nas notas +
   **anexos/mídia** (§3.6: imagem/vídeo no evento e no dia, limite de vídeo 300MB configurável).
4. **Aprovação via WhatsApp** (n8n/WAHA) + **visão compartilhável** read-only para o cliente.

## 5. Notas de deploy
- Migrations aplicadas: **0181** (`calendar.events` + `calendar.notes`), **0182** (`calendar.config`
  jsonb por conta). Ordem: 0181 → 0182.
- Fase 3 (anexos): **0183** (`calendar.day_media`). Limite de vídeo mora em `core.platform_settings`
  (chave `media_limits`, sem migration — linha criada no 1º PUT; default 300MB no código).
- Feriados **não** têm tabela/seed (computados em `holidays.go`); nada a migrar por ano.
- **Upload de 300MB**: o proxy da frente (Caddy/nginx) precisa aceitar body grande
  (`client_max_body_size`/limite de request) e o volume de `UPLOADS_DIR` (`data/uploads`) precisa de
  espaço. Em dev (direto na :9091) o Go aceita. Arquivos servidos em `/uploads/...` (rota não gateada).
- Rebuild da api ao mexer no back: `docker compose up -d --build api` (feito a cada fase de back).

## 6. Legado/mock — ✅ removido
- `useCalendarData.ts` (`CALENDAR_DATA_IS_MOCK`) **deletado**; responsáveis, feriados, config e
  membros vêm do back real. Notas persistem via API (`/v1/calendar/notes/{month}`).

## 7. Organização do código (skill de engenharia, < 450 linhas/arquivo)
- CSS: `assets/styles/calendar.css` virou **barrel** que importa
  `calendar/{shell,grid,notes-drawer,week-form}.css` (cada um < 450).
- Store: `stores/calendar.ts` (407) + `composables/useCalendarViewport.ts` (foco/scroll/rail/nav)
  + `domain/calendar/calendar-api.ts` (I/O). Back: módulo em arquivos < 450.
