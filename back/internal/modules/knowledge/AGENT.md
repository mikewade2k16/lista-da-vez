# Conhecimento neutro — CT-06

Owner único do repositório `messaging.knowledge_bases/documents/chunks`, IDs e FTS
simple existentes preservados. Não é módulo comercial obrigatório. Omnichannel
usa bridge e mantém políticas/bindings de agente; Accounting usa política própria
e bindings por emitente. Nenhum agentId fictício. Bases fiscais são criadas com
search_config.consumer=accounting e ficam fora das APIs/busca legadas.

Service → Store. Escopo interno contém conta/ator/emitente; Authorize é obrigatório
e revalida módulo, RBAC e grant via owner. SQL repete conta/binding/base/publicação
e filtros de competência/jurisdição/regime antes do ranking. Sem cache de acesso.
Campos de contexto confirmado não são aceitos no JSON. Runtime M1 apresenta
material informativo enquanto o contexto fiscal não tiver verificação própria.

Importação texto/PDF até2MiB, texto extraído até256kB, chunks1600codepoints com
offsets, hash SHA256 do original. PDF reutiliza extrator limitado Finance via porta
(10páginas/32kB de texto); sem OCR/LLM, parcial/digitalizado falha explicitamente.
Original byte a byte pelo Storage privado; sem storage não cria documento. Não
baixar URLs/seguir scripts. HTML é recusado. Import idêntico não altera metadados.

Revisão CAS separada de publicação. Publicação transacional exige revisão, original
e chunks completos; falha preserva versão ativa. Versões antigas mantidas por
competência. Documentos/chunks publicados imutáveis por service e triggers0312;
arquivamento preserva conteúdo. Não há cálculo nem promoção automática de
respostas a confirmed. EvidenceVerifier do diagnóstico permanece sem implementação
de verificação factual: referência recuperada sozinha não comprova uma resposta.

Busca topK1–20/page até10000, trechos por1600caracteres e saída até24KiB com
truncamento explícito. Score ordena texto, não confiança jurídica; múltiplas fontes
geram sinal de potencial conflito. Resolve expõe apenas trecho autorizado por ID,
nunca original inteiro. Listas de documentos paginadas, bases limitadas100.

`KNOWLEDGE_TEST_DATABASE_URL` somente banco descartável `knowledge_test*`; fixtures
sintéticas. Handoff: docs/accounting/handoffs/CT-06.md. Corpus selecionado/manifesto
em docs/accounting/corpus, não carregado em conta real.

## Referências em lote — CT-07

ValidateReferences valida até500IDs internos de uma página de histórico em uma
query account/issuer/binding/base/publicação e revalida política antes/depois.
ID inválido404; referência revogada/inativa409. Não retorna corpo nem amplia acesso.
Resolve e Search individuais continuam com contratos anteriores.

## Ingestão e avaliação CT-12 — 2026-11-05

0315 adiciona messaging.knowledge_source_subscriptions/ingestion_runs/ingestion_receipts.
Catálogo de segurança fechado no servidor (IngestionSources); nenhum URL livre,
subscrição ou publicação seedada. Configuração efetiva por conta/emitente/base e
ator no banco, CAS/receipts/limite20; opt-in6–168h. Não confundir catálogo público
de origens com cadastro/regra tributária do emitente. Metadata automática desconhecida.

Host Accounting reutiliza outbox/Worker Jobs e ciclo30s/lote20. knowledge.ingest.v1
usa checkpoint/lease60s/fence,4tentativas e recuperação limitada de fetching expirada
ou queued/retry_wait abandonada30min. Source pausa/versão alterada cancela runs;
guarda interna não serializável de Import checa run/subscrição/base na transação.
Revogação cancela; falha temporária de autorização posterga, não desativa configuração.
Novos documentos sempre exigem revisão/publicação canônicas; hash repetido não copia
metadados nem nova versão. Não publicar automaticamente nem seguir instruções do PDF.

Fetcher HTTPS443: URL exata do catálogo, sem proxy/redirect/credencial/query, DNS
público validado e dial fixado ao IP; limites2MiB/timeout/Content-Type/parcial e PDF
CT-06 preservados. Originais via mesma porta Storage, sem novo blob/cache/índice.
Falha/log só códigos seguros; runs são alertas internos históricos, não notificações.

FTS simple permanece produção/Omnichannel. Candidato português avaliado offline no
mesmo SQL/filtros; perdeu sigla DAS. pgvector/embeddings não adotados. Dataset/metricas
em docs/accounting/evaluations/CT-12*.json; não confundir recall sintético com cobertura
fiscal/qualidade universal. Plano de teste/backup/limites em INGESTAO_E_BUSCA.md.
Retenção/hard delete e smoke vivo permanecem no CT-15; não apagar versões citadas.

## Busca pública no chat — pacotes 1/2, 2026-11-06

0317 cria `knowledge.public_sources`: acervo global exclusivamente público,
curado e informativo; não possui tenant porque não recebe uploads ou dados privados.
Não copia nem abre `messaging.knowledge_*`/bindings Accounting. Sem endpoint de
publicação pública, sem scraping no chat. Versões publicadas imutáveis (trigger),
retirada por status; nova busca seleciona versão publicada mais recente. Referências
históricas publicadas permanecem válidas; retiradas são ocultadas em lote no GET.
0318 adiciona cinco sínteses curtas com origem/revisão/limites em manifesto, sem
alterar configurações, contas, credenciais ou seeds anteriores. Vigência fiscal
não confirmada; são evidências informativas, nunca regra de cálculo do emitente.

SearchPublic → Store, FTS simple com normalização de acentos, até12termos e3trechos.
Aliases de vocabulário no roteador não contêm valores fiscais. Finance usa portas
PublicKnowledge/ValidatePublicEvidence, autorizadas pelo Finance canônico antes/
depois. Não exige Accounting contratado, emitente, modelo, cofre, Jeff ou API paga
para ler material público. Corpus privado empresarial conserva todos os gates.

finance.access_messages.public_evidence persiste snapshot estruturado separado do
texto; List revalida até180IDs e oculta fontes retiradas/indisponíveis. Conta/usuário
continuam filtrados no SQL. Perguntas fiscais explícitas no chat chamam busca antes
de qualquer modelo. Falta de fonte difere de falta de serviço. Nenhuma resposta é
marcada como confirmação jurídica ou execução de nota/declaração.

Frontend reutiliza AccountingEvidence e cartões/mensagens do Crow; mostra título,
trecho, fonte HTTPS e versão. Roteiro manual em docs/finance/ROTEIRO_TESTES.md.
Testes PostgreSQL sintéticos, corpus público inicial e regressões de conversa
passaram. Comparação visual, microfone real e aceite do usuário não executados.

## Curadoria do acervo público — CT-15 fase C, 2026-11-09

0323 cria knowledge.public_collections (versão por chave do catálogo, hash/tamanho/
páginas, status collected|published|superseded|discarded), public_collection_attempts
(toda tentativa, inclusive falha com código seguro) e public_reviews (append-only:
owner|ai|accountant, approved|changes_requested|rejected). public_sources ganha
collection_id/chunk_index; trechos coletados nascem draft com source_key
`<chave>:<nnnn>`. Catálogo fechado em PublicCatalog (código); cliente envia só a chave.

Coleta (public_catalog/public_html/public_chunk): dial CT-12 (HTTPS443, IP público
fixado, sem redirect/proxy/compressão), até16MiB, User-Agent honesto, 403/login é falha
visível. HTML → texto puro via stdlib (UTF-8 ou windows-1252), PDF por página via porta
ExtractPublicPDF (finance, pdftotext, até400 páginas, sem OCR; maioria vazia = recusa).
Ruído (sumário pontilhado, número de página, rodapé gov.br) descartado. Original binário
não é guardado (URL+SHA-256). Publicar exige última decisão do dono = approved, publica
drafts e retira a versão publicada anterior da mesma chave na mesma transação.

Busca (public_query): conceitos por termo — siglas/curtas exatas, longas por radical
conservador (`:*`), grupos de sinônimos só de vocabulário; ordem por conceitos cobertos
e ts_rank_cd; mínimo de 2 conceitos quando a pergunta tem 2+. Evidence.review mostra
dono/IA/contador da coleta. Rotas /v1/knowledge/public/* só platform_admin
(public_curation_http). Medição e limites em docs/accounting/BASE_PUBLICA.md.
Testes: public_curation_test (PUBLIC_KNOWLEDGE_TEST_DATABASE_URL, banco migrado
accounting_test_*); coleta real opt-in PUBLIC_COLLECT_LIVE=1 com poppler.
