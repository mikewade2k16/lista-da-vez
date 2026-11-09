# CT-12 — ingestão recuperável e decisão de busca

Data: 2026-11-05. Estado técnico: implementação local e avaliação entregues;
aceite manual/configuração do piloto pendentes. Não habilita emissão, cálculo
tributário, publicação automática nem comprova vigência de uma norma.

## Ingestão entregue

O owner neutro Knowledge reutiliza messaging.knowledge_bases/documents/chunks e
Storage. Subscrições e checkpoints novos em0315_knowledge_ingestion.sql; outbox e
worker Jobs existentes do host Accounting, handler knowledge.ingest.v1. Nenhum
framework/fila de execução paralelo, agente fictício ou documento fiscal novo.

Catálogo de segurança de origens imutável no servidor; o cliente seleciona sourceId,
nunca URL. A primeira seleção é o
[manual API de contribuintes NFS-e](https://www.gov.br/nfse/pt-br/biblioteca/documentacao-tecnica/documentacao-atual/manual-contribuintes-emissor-publico-api-sistema-nacional-nfs-e-v1-2-out2025.pdf),
identificado na [documentação oficial](https://www.gov.br/nfse/pt-br/biblioteca/documentacao-tecnica/documentacao-atual).
A fonte tem6páginas, observadas na pesquisa pública de 2026-11-05. Isso não comprova
atualidade de toda regra fiscal: o documento coletado nasce informativo, com
jurisdição/regime/vigência desconhecidos e sem publicação. Nenhuma subscrição foi
criada ou habilitada na conta do usuário nesta tarefa.

- Subscrição por conta/emitente/base/fonte, com ator autorizado, CAS, idempotência,
  habilitação explícita e intervalo6–168horas; máximo20por emitente. A base precisa
  estar ativa para habilitar. Catálogo é política pública de acesso, não fonte de
  cadastro/tributação do emitente; seleção/configuração efetiva vêm do banco.
- Ciclo do host a cada30s, lote20, executa agendamento e recuperação; não é watcher
  externo nem scheduler concorrente. Inserção run+job e avanço da próxima coleta
  são transacionais. Runs ativos têm identidade única por subscrição.
- Escopo/ator persistidos no banco, não no prompt nem no job arbitrário. Autorizar
  novamente ao agendar/processar/antes de importar. Alteração/pausa da subscrição
  cancela runs antigos e incrementa fence; importação automática valida guarda
  transacional da run/subscrição/base. Sem promoção de regra fiscal.
- GET público sem cookies, tokens ou dados privados da conta, HTTPS443, sem proxy,
  redirects, querystring ou credenciais; DNS validado e IP público fixado ao conectar,
  sem nova resolução para o dial. IP privado/local/link-local/reservado/metadata e
  transições mapeadas relevantes são bloqueados. TLS>=1.2 com verificação padrão.
- Até2MiB, timeoutHTTP20s/etapa30s, cabeçalho10s, formatos text/plain ou PDF.
  HTML,206, codificação comprimida inesperada, parcial/truncado e excesso são
  recusados. PDF reutiliza extrator CT-06: até10páginas/32kB, sem OCR/LLM. Não seguir
  links do documento, scripts ou instruções recuperadas. Não contornar bloqueio403.
- Hash/dedupe e storage/chunks do Import canônico: mesmos bytes não criam versão
  nem alteram metadados anteriores; fonte alterada prepara nova versão. Continuidade
  de fonte identificada por URL original; versões publicadas antigas são preservadas.
- Checkpoints queued/fetching/retry_wait/review_pending/unchanged/failed/cancelled;
  lease60s, fence,4tentativas de coleta. Jobs aplica backoff. Falha terminal fica
  persistida para revisão interna; nenhuma mensagem externa é enviada.
- Crash após importar retoma por hash, sem duplicar documento. Worker antigo não
  sobrescreve checkpoint. Fetching expirada e queued/retry_wait sem avanço30min
  recebem recuperação limitada;4recuperações no máximo. Isso também cobre job que
  falhou antes do claim/checkpoint. Não se promete exatamente uma requisição GET.
- Tela no bloco Fontes: escolher base, consultar catálogo/coletas, selecionar fonte,
  confirmar configuração periódica, consultar histórico e revisar documentos pela
  API/UI canônicas. Revisar e publicar permanecem ações humanas distintas.

Estados de run são histórico da coleta: review_pending significa que foi preparada
uma versão para revisão naquele momento, não prova que sua revisão continua aberta
após publicação manual posterior. Status autoritativo do documento vem do owner.
Alertas são registros internos/runs; não há envio de e-mail/chat para terceiros.

## Busca: resultado medido e decisão

Dataset sintético versionado em evaluations/CT-12-dataset.json:7documentos,
4perguntas de desenvolvimento separadas e16holdout, topK5. Dos16casos,13pedem
referência positiva e3são negativos/filtros. Sem legislação inventada como fato.
Nenhum ajuste de dicionário, embedding ou expansão usou as perguntas holdout.

Mesmos dados, joins de autorização, regime/jurisdição/competência e ordenação para
os dois candidatos. Português foi avaliado somente em SQL offline, sem alterar
índices, configuração de busca de conta ou o fluxo Omnichannel. Resultados brutos,
hash do dataset e tempos em evaluations/CT-12-retrieval.json.

| Método | Recall@5 | p50 | p95 |
|---|---|---|---|
| FTS simple atual | 6/13 =46,15% | 0,209ms | 1,631ms |
| Dicionário português, candidato offline | 8/13 =61,54% | 0,270ms | 1,672ms |

O candidato melhora algumas flexões, mas perde a sigla DAS por stopword e continua
falhando em sinônimos. **Decisão: manter FTS simple no runtime**. Planejar combinação
lexical que preserve siglas, dataset maior e medição em escala antes de rollout.
Não trocar dicionário global nem prometer recall universal por esta amostra.

É uma única passagem de16consultas por método num corpus pequeno, com filtros
negativos sem vazamento observado. Os tempos são medições locais do SQL, incluem
efeito de aquecimento e não são SLA do chat/produção. Não há benchmark de qualidade
com modelo real. Chamadas de embedding/modelo=0; custo de embeddings não medido
(null), sem fatura nem estimativa comercial inventada.

pgvector não foi adotado: nenhuma prova de benefício vetorial/custo/escala, extensão
não disponível na imagem PostgreSQL16.15 descartável testada. Nenhuma imagem,
volume, dimensão/modelo/índice vetorial ou banco paralelo foi alterado. Como o
runtime permanece FTS, não depende de provedor nem precisa de fallback vetorial
para continuar funcionando. Configuração/cache/reindexação de embeddings não
implementadas nem anunciadas disponíveis.

## Backup, retenção e limites

pg_dump -Fc + pg_restore --exit-on-error --no-owner passaram sobre o corpus
sintético e schema final em novo banco:34documentos/16runs restaurados. Não é
backup de produção nem afirmação sobre extensões/configuração da conta real.
Antes de qualquer pgvector futuro, repetir inventário e restauração no ambiente
alvo autorizado, com plano de índices filtrados/limites/rollback e custos.

Não houve exclusão de originais, chunks, versões citadas ou cache de autorização.
Não foi criado cache de embeddings. Retenção/hard delete de novos metadados e
limpeza de eventual original privado órfão após falha/revogação devem ser definidos
com o Storage/Knowledge canônicos; não existe promessa de expurgo automático.
Desativar uma fonte/base impede coleta/uso conforme política, não apaga evidência
histórica silenciosamente. Expansão da coleta e retenção estão no CT-15.

O [guia web oficial](https://www.gov.br/nfse/pt-br/biblioteca/documentacao-tecnica/documentacao-atual/guia-emissorpubliconacionalweb_snnfse-ern-v1-3.pdf)
possui130páginas na pesquisa e está fora da coleta inicial: não aumentar limites ou
importar só parte fingindo integralidade. Ampliar extrator por partes verificáveis
exige projeto/aceite próprios. Revalidar URLs/fontes ao expandir o catálogo; bloqueio
ou mudança incompatível gera falha visível/importação manual permitida, sem bypass.

## Roteiro de teste e limites do aceite

- Fixture PostgreSQL exige KNOWLEDGE_TEST_DATABASE_URL em knowledge_test*.
  Testes CT-12 em knowledge/ingestion*_test.go e retrieval_evaluation_test.go.
- Servidor fake local: conteúdo normal/alterado/repetido, redirect interno, tamanho,
  parcial, MIME, timeout/503, retry, pausa em voo, rollback e crash/worker antigo.
- Suítes CT-06/07 preservadas; repetir HTTP autenticado Accounting, Finance e Plus
  com seus bancos próprios. Sem as variáveis, testes DB pulam e não são aprovados.
- Avaliação: `go test -race -count=1 ./internal/modules/knowledge -run
  TestPortugueseRetrievalHeldOutEvaluation`, com KNOWLEDGE_EVAL_OUTPUT apontando ao
  JSON temporário. Mesmos dataset/hash/topK para comparar; não afinar no holdout.
- UI: Finance → Empresarial → emitente → Fontes → base → Atualização de fontes
  permitidas. Habilitar só se desejar coleta dessa origem; manter manual review.
  Conferir pausa, troca de conta/base, falha acionável e revisão/publicação separadas.

Pipeline de coleta viva não foi exercitado em conta real; pesquisa pública de
URLs e testes fake não são smoke de Storage/extração no runtime da conta. Browser,
layout e aceite do usuário não realizados. M1 continua sem aprovação do usuário;
emissão/cálculo real permanecem indisponíveis por seus gates anteriores.
